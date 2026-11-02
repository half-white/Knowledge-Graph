package rag

import (
	"SSE/utils/embedding"
	"SSE/utils/metrics"
	"SSE/utils/vector"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// 默认配置
const (
	// 默认相似度阈值：高于该值视为相似原子项（可通过环境变量RAG_THRESHOLD覆盖）。
	// 注意：阈值为Neo4j向量索引的score空间（score=(1+数学余弦)/2，实际值域[0.5,1]），
	// 0.75约等于数学余弦0.5；设高阈值（如0.98≈余弦0.96）仅命中近乎相同的文本。
	DefaultThreshold = 0.75
	// 默认检索返回的最大相似原子项数量（可通过环境变量RAG_TOP_K覆盖）
	DefaultTopK = 5
)

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
		slog.Warn("创建向量索引失败", "error", err)
		return
	}
	indexEnsured = true
}

// BuildRAGContext 生成RAG参考上下文，失败返回空串（降级为普通抽取）
func BuildRAGContext(driver neo4j.Driver, content string, trace *metrics.Trace) string {
	ensureIndex(driver)

	// 生成文本向量（子环节计时：embedding）
	embedStart := time.Now()
	vec, err := embedding.Embed(content)
	trace.Observe(metrics.StageRAGEmbed, time.Since(embedStart), err != nil)
	if err != nil {
		slog.Warn("生成embedding失败，降级为普通抽取", "error", err)
		return ""
	}

	// 向量相似度检索相似高价值关系（阈值与数量可配置，子环节计时：检索）
	searchStart := time.Now()
	similar, err := vector.SearchSimilar(driver, vec, topK(), threshold())
	trace.Observe(metrics.StageRAGSearch, time.Since(searchStart), err != nil)
	if err != nil {
		slog.Warn("向量检索失败，降级为普通抽取", "error", err)
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

// SaveIfNew 对三元组的头实体、关系、尾实体逐一做原子粒度保留判定：
// 向量化 → 与现存原子向量相似匹配 → 未命中（新项）入库保留，命中（已存在）跳过。
// embedding/检索失败时降级跳过该原子项，不阻断主流程（图谱写入已在调用方先行完成）。
func SaveIfNew(driver neo4j.Driver, head, rel, tail, uuid string, trace *metrics.Trace) {
	ensureIndex(driver)

	// 头实体、关系、尾实体作为独立原子项逐一判定
	for _, item := range []string{head, rel, tail} {
		item = strings.TrimSpace(item)
		if item == "" {
			continue // 空原子项跳过
		}

		// 生成原子项向量（子环节计时：embedding）
		embedStart := time.Now()
		vec, err := embedding.Embed(item)
		trace.Observe(metrics.StageRAGEmbed, time.Since(embedStart), err != nil)
		if err != nil {
			slog.Warn("生成embedding失败，跳过该原子项", "item", item, "error", err)
			continue
		}

		// 与库中现存原子向量做相似匹配（阈值可配置），命中说明已存在，不重复入库（子环节计时：检索）
		searchStart := time.Now()
		similar, err := vector.SearchSimilar(driver, vec, 1, threshold())
		trace.Observe(metrics.StageRAGSearch, time.Since(searchStart), err != nil)
		if err != nil {
			slog.Warn("向量检索失败，跳过该原子项", "item", item, "error", err)
			continue
		}
		if len(similar) > 0 {
			continue
		}

		// 未命中：新原子项入库保留（子环节计时：写入）
		saveStart := time.Now()
		if err := vector.SaveAtom(driver, item, uuid, vec); err != nil {
			trace.Observe(metrics.StageRAGSave, time.Since(saveStart), true)
			slog.Error("保存原子项向量失败", "item", item, "uuid", uuid, "error", err)
		} else {
			trace.Observe(metrics.StageRAGSave, time.Since(saveStart), false)
			slog.Debug("已保存原子项向量", "item", item)
		}
	}
}
