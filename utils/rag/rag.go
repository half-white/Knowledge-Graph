package rag

import (
	"SSE/utils/embedding"
	"SSE/utils/vector"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// 默认配置
const (
	// 默认相似度阈值：高于该值视为相似关系（可通过环境变量RAG_THRESHOLD覆盖）
	DefaultThreshold = 0.75
	// 默认检索返回的最大相似关系数量（可通过环境变量RAG_TOP_K覆盖）
	DefaultTopK = 5
	// 高价值关系判定使用的LLM模型
	judgeModel = "glm-4-flash"
	// 高价值关系判定接口地址
	judgeURL = "https://open.bigmodel.cn/api/paas/v4/chat/completions"
)

// judgeAPIKey 读取GLM API密钥（环境变量GLM_API_KEY，与use_model.go共用）
func judgeAPIKey() string {
	return os.Getenv("GLM_API_KEY")
}

// threshold 读取可配置的相似度阈值（环境变量RAG_THRESHOLD覆盖，默认DefaultThreshold）
func threshold() float64 {
	if v := os.Getenv("RAG_THRESHOLD"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 0 && f <= 1 {
			return f
		}
	}
	return DefaultThreshold
}

// topK 读取可配置的检索数量（环境变量RAG_TOP_K覆盖，默认DefaultTopK）
func topK() int {
	if v := os.Getenv("RAG_TOP_K"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return DefaultTopK
}

// ensureOnce 保证向量索引只创建一次
var (
	indexEnsureMu sync.Mutex
	indexEnsured  bool
)

// ensureIndex 懒初始化向量索引（幂等，失败可重试，不影响主流程）
func ensureIndex(driver neo4j.Driver) {
	indexEnsureMu.Lock()
	defer indexEnsureMu.Unlock()
	// 已创建成功则直接返回
	if indexEnsured {
		return
	}
	// 创建失败不置位，下次调用可重试
	if err := vector.EnsureVectorIndex(driver); err != nil {
		log.Printf("创建向量索引失败: %v", err)
		return
	}
	indexEnsured = true
}

// BuildRAGContext 生成RAG参考上下文，失败返回空串（降级为普通抽取）
func BuildRAGContext(driver neo4j.Driver, content string) string {
	ensureIndex(driver)

	// 生成文本向量
	vec, err := embedding.Embed(content)
	if err != nil {
		log.Printf("生成embedding失败，降级为普通抽取: %v", err)
		return ""
	}

	// 向量相似度检索相似高价值关系（阈值与数量可配置）
	similar, err := vector.SearchSimilar(driver, vec, topK(), threshold())
	if err != nil {
		log.Printf("向量检索失败，降级为普通抽取: %v", err)
		return ""
	}
	if len(similar) == 0 {
		return ""
	}

	// 拼装参考上下文，注入到抽取prompt
	context := "以下是知识图谱中与当前文本最相似的高价值关系，可作为抽取参考：\n"
	for i, text := range similar {
		context += fmt.Sprintf("%d. %s\n", i+1, text)
	}
	return context
}

// SaveIfHighValue 判断并保存高价值关系向量（embedding→相似度去重→LLM判定→写入），embedding不可用时降级跳过
func SaveIfHighValue(driver neo4j.Driver, head, rel, tail, uuid string) {
	ensureIndex(driver)

	// 组合三元组文本
	text := fmt.Sprintf("%s-%s-%s", head, rel, tail)

	// 生成三元组向量
	vec, err := embedding.Embed(text)
	if err != nil {
		log.Printf("生成embedding失败，跳过向量写入: %v", err)
		return
	}

	// 检查库中是否已有相似度较高的关系（阈值可配置），有则不重复写入
	similar, err := vector.SearchSimilar(driver, vec, 1, threshold())
	if err != nil {
		log.Printf("向量检索失败，跳过向量写入: %v", err)
		return
	}
	if len(similar) > 0 {
		return
	}

	// 由LLM判定当前关系是否为高价值关系，是才写入向量
	if !isHighValueRelation(head, rel, tail) {
		return
	}

	// 写入高价值三元组及向量
	if err := vector.SaveTriplet(driver, text, uuid, vec); err != nil {
		log.Printf("保存三元组向量失败 (%s,%s,%s): %v", head, rel, tail, err)
	} else {
		fmt.Printf("已保存高价值关系向量 (%s,%s,%s) \n", head, rel, tail)
	}
}

// isHighValueRelation 调用LLM轻量判定关系是否为高价值关系
func isHighValueRelation(head, rel, tail string) bool {
	// 构造判定提示词
	prompt := fmt.Sprintf("判断以下三元组关系是否是知识图谱中值得保留的高价值关系，只回答'是'或'否'：%s-%s-%s", head, rel, tail)

	// 构造请求数据
	payload := map[string]interface{}{
		"model": judgeModel,
		"messages": []map[string]interface{}{
			{
				"role":    "user",
				"content": prompt,
			},
		},
	}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		log.Printf("构造判定请求失败: %v", err)
		return false
	}

	// 创建HTTP请求
	req, err := http.NewRequest("POST", judgeURL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		log.Printf("创建判定请求失败: %v", err)
		return false
	}
	req.Header.Set("Authorization", "Bearer "+judgeAPIKey())
	req.Header.Set("Content-Type", "application/json")

	// 发送请求并获取响应（设置超时避免LLM挂起阻塞主流程，与embedding调用一致）
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("调用LLM判定失败: %v", err)
		return false
	}
	defer resp.Body.Close()

	// 读取响应体
	responseBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("读取判定响应失败: %v", err)
		return false
	}

	// 解析响应
	var responseData struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(responseBody, &responseData); err != nil {
		log.Printf("解析判定响应失败: %v", err)
		return false
	}
	if len(responseData.Choices) == 0 {
		return false
	}

	// 判断LLM回答是否为"是"
	answer := strings.TrimSpace(responseData.Choices[0].Message.Content)
	return strings.HasPrefix(answer, "是") || strings.HasPrefix(answer, "true") || strings.HasPrefix(answer, "YES")
}
