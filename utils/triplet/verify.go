package triplet

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
)

// 默认配置
const (
	// 验证失败重试次数（总尝试 = 1 + verifyRetry）
	verifyRetry = 1
	// 验证接口默认地址
	DefaultVerifyURL = "https://open.bigmodel.cn/api/paas/v4/chat/completions"
	// 验证用模型缺省值（GLM_MODEL 未配置时，与抽取/判定共用免费模型）
	DefaultVerifyModel = "glm-4-flash"
)

// 包级变量（导出以便测试替换为 mock 地址）：验证接口地址、单次调用超时（与 rag 包判定超时一致）
var (
	VerifyURL     = DefaultVerifyURL
	VerifyTimeout = 15 * time.Second
)

// verifyModelName 读取验证用模型名（环境变量GLM_MODEL覆盖，与 use_model.go/rag 共用）
func verifyModelName() string {
	if v := os.Getenv("GLM_MODEL"); v != "" {
		return v
	}
	return DefaultVerifyModel
}

// verifyPromptTpl 验证归一 prompt：逐条判定忠实性 + 返回实体规范名
const verifyPromptTpl = `你是知识图谱质量审核员。下面是一段原文和从中抽取的候选三元组，请逐条判定：
1. keep：该三元组是否有原文明确支持、属于事实性知识（问候、评论、无实质内容 → keep=false）；
2. keep=true 时，给出规范化的 head/tail：使用原文中的完整规范名称，修正代词指代（如"该公司"→实际名称）、补全截断短语、简称统一为原文中的全称；
3. relation 保持简短规范，可微调为更准确的关系词。

输出格式（严格遵守，输出 JSON 数组，与候选一一对应，不要输出任何其他文字）：
[{"index":0,"keep":true,"head":"规范头实体","relation":"关系","tail":"规范尾实体"}]

原文：
%s

候选三元组：
%s`

// verifyItem 验证器单条输出（index 与候选编号对齐）
type verifyItem struct {
	Index    int    `json:"index"`
	Keep     bool   `json:"keep"`
	Head     string `json:"head"`
	Relation string `json:"relation"`
	Tail     string `json:"tail"`
}

// VerifyTriplets 调用 LLM 逐条判定候选三元组的忠实性并返回规范化三元组。
// 规则：keep=false 剔除；候选缺失条目保留原名；规范化后字段为空保留原名；
// 全部输出复过硬校验（Validate，内部含清洗）。超时/调用失败/输出坏格式重试 1 次，
// 仍失败返回 error，由调用方降级为候选全保留。
func VerifyTriplets(chunk string, candidates []Triplet) ([]Triplet, error) {
	if len(candidates) == 0 {
		return nil, nil
	}

	// 编号候选列表，供验证器按 index 回填
	var listed strings.Builder
	for i, t := range candidates {
		fmt.Fprintf(&listed, "%d. (%s, %s, %s)\n", i, t.Head, t.Relation, t.Tail)
	}
	prompt := fmt.Sprintf(verifyPromptTpl, chunk, listed.String())

	var verified []Triplet
	var lastErr error
	for attempt := 0; attempt <= verifyRetry; attempt++ {
		respText, err := callVerifyLLM(prompt)
		if err != nil {
			lastErr = err
			continue
		}
		verified, err = alignVerified(respText, candidates)
		if err == nil {
			return verified, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

// alignVerified 解析验证器输出并与候选按 index 对齐（越界丢弃、重复取首、缺失保留原名、
// keep=true 空字段回退原名），再复过硬校验
func alignVerified(respText string, candidates []Triplet) ([]Triplet, error) {
	items, err := parseVerifyResponse(respText)
	if err != nil {
		return nil, err
	}

	// 按 index 回填，缺失的候选默认保留原名
	final := make([]Triplet, len(candidates))
	copy(final, candidates)
	seen := make(map[int]bool)
	for _, item := range items {
		// index 越界丢弃；重复 index 取首个
		if item.Index < 0 || item.Index >= len(candidates) || seen[item.Index] {
			continue
		}
		seen[item.Index] = true
		if !item.Keep {
			final[item.Index] = Triplet{} // keep=false：剔除
			continue
		}
		// 规范化后字段为空则回退原名
		if item.Head == "" {
			item.Head = candidates[item.Index].Head
		}
		if item.Relation == "" {
			item.Relation = candidates[item.Index].Relation
		}
		if item.Tail == "" {
			item.Tail = candidates[item.Index].Tail
		}
		final[item.Index] = Triplet{Head: item.Head, Relation: item.Relation, Tail: item.Tail}
	}

	// 复检：验证器也是 LLM，输出同样可能产出垃圾，统一过硬校验后剔除空壳（keep=false 占位）
	kept, dropped := Validate(final)
	if n := len(final) - len(kept); n > 0 {
		slog.Debug("验证归一复检丢弃三元组", "count", n, "dropped", dropped)
	}
	return kept, nil
}

// parseVerifyResponse 解析验证器 JSON 输出（容忍 markdown 围栏与前后杂文字）
func parseVerifyResponse(text string) ([]verifyItem, error) {
	var items []verifyItem
	if err := json.Unmarshal([]byte(text), &items); err != nil {
		start := strings.Index(text, "[")
		end := strings.LastIndex(text, "]")
		if start < 0 || end <= start {
			return nil, fmt.Errorf("验证器输出中未找到 JSON 数组")
		}
		if err := json.Unmarshal([]byte(text[start:end+1]), &items); err != nil {
			return nil, fmt.Errorf("解析验证器输出失败: %w", err)
		}
	}
	return items, nil
}

// callVerifyLLM 调用 GLM 接口执行验证，返回模型文本输出
func callVerifyLLM(prompt string) (string, error) {
	payload := map[string]interface{}{
		"model": verifyModelName(),
		"messages": []map[string]interface{}{
			{"role": "user", "content": prompt},
		},
	}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("构造验证请求失败: %w", err)
	}

	req, err := http.NewRequest("POST", VerifyURL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return "", fmt.Errorf("创建验证请求失败: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+os.Getenv("GLM_API_KEY"))
	req.Header.Set("Content-Type", "application/json")

	// 发送请求并获取响应（超时可配置，避免 LLM 挂起阻塞抽取流程）
	client := &http.Client{Timeout: VerifyTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("调用验证接口失败: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取验证响应失败: %w", err)
	}

	var responseData struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(responseBody, &responseData); err != nil {
		return "", fmt.Errorf("解析验证响应失败: %w", err)
	}
	if len(responseData.Choices) == 0 {
		return "", fmt.Errorf("验证响应为空: %s", string(responseBody))
	}
	return responseData.Choices[0].Message.Content, nil
}
