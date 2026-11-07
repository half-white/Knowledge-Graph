// Package service 图谱业务层（REQ-008 分层重构）：
// 本文件为 LLM 抽取与解析管道，函数体自 api/model_api/use_model.go 逐行搬迁。
package service

import (
	"SSE/utils/llm"
	"SSE/utils/metrics"
	"SSE/utils/triplet"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"
)

// llmURL 三元组抽取接口地址（完整端点，包级变量，测试可替换为 mock 地址；
// 实际请求经 llm.ChatWithURL 派生 OpenAI 兼容 BaseURL 走 eino ChatModel）
var llmURL = "https://open.bigmodel.cn/api/paas/v4/chat/completions"

// TypeInModel 输入文本，调用 LLM 抽取知识三元组
func TypeInModel(content string) string {
	prompt := `你是知识图谱构建专家。请从下面的文本中抽取知识三元组。

抽取规则：
1. 只抽取事实性知识点，忽略问候、评论、过渡句等非事实内容；
2. 头实体和尾实体使用文本中的规范名称，遇到代词（如"他"、"该公司"）必须还原为所指的具体实体；
3. 同一实体在不同句子中出现时使用完全相同的名称，不要有时写全称有时写简称；
4. 关系用一个简短的词或短语概括（如"创始人"、"位于"、"提出"）；
5. 实体和关系的名称中不要包含逗号；
6. 最多输出8个最重要的三元组，不足8个则有多少输出多少；
7. 头实体和尾实体必须是文本中出现的完整规范名称，不能是代词、截断短语或你自己缩写的简称。

输出格式（严格遵守，输出 JSON 数组，不要有任何额外文字、序号或代码块标记）：
[{"head":"头实体","relation":"关系","tail":"尾实体"}]

示例：
输入：乔布斯于1976年创立了苹果公司，该公司后来发布了风靡全球的iPhone。
输出：
[{"head":"乔布斯","relation":"创立","tail":"苹果公司"},{"head":"苹果公司","relation":"发布","tail":"iPhone"}]

以下是需要处理的文本内容：
`

	// RAG向量检索已移除：长文本与精简三元组的向量相似度对比意义不大，
	// 改为通过提示词工程提升抽取质量（实体规范化、few-shot示例等）

	// REQ-006：经 eino ChatModel 调用（utils/llm 统一封装）；
	// llmURL 保持完整端点语义（测试可替换为 mock 地址，由 ChatWithURL 派生 BaseURL）；
	// 超时默认 60s（GLM_TIMEOUT 可配），失败/超时返回空串 → 该块跳过（降级①）
	respText, err := llm.ChatWithURL(context.Background(), llmURL, prompt+content, llm.GLMTimeout())
	if err != nil {
		slog.Error("调用LLM接口失败", "error", err)
		return ""
	}
	return respText

}

// parseTriplets 解析 LLM 抽取输出为候选三元组：JSON 数组优先（容忍 markdown 围栏与前后杂文字），
// 失败降级正则匹配（头，关系，尾）自由文本格式（历史兼容路径）
func parseTriplets(text string) []triplet.Triplet {
	trimmed := strings.TrimSpace(text)
	if trimmed != "" {
		if items, err := parseJSONArray(trimmed); err == nil {
			return items
		}
	}

	// 正则降级：匹配括号中的三元组（match[1..3] 分别是头实体、关系、尾实体）
	re := regexp.MustCompile(`[（(]([^，,]+)[，,]\s*([^，,]+)[，,]\s*([^)）]+)[）)]`)
	matches := re.FindAllStringSubmatch(text, -1)
	var triplets []triplet.Triplet
	for _, match := range matches {
		triplets = append(triplets, triplet.Triplet{
			Head:     strings.TrimSpace(match[1]),
			Relation: strings.TrimSpace(match[2]),
			Tail:     strings.TrimSpace(match[3]),
		})
	}
	return triplets
}

// parseJSONArray 解析 JSON 数组形式的三元组（直接解析失败时截取首 [ 至末 ] 再解析）
func parseJSONArray(text string) ([]triplet.Triplet, error) {
	var items []triplet.Triplet
	if err := json.Unmarshal([]byte(text), &items); err != nil {
		start := strings.Index(text, "[")
		end := strings.LastIndex(text, "]")
		if start < 0 || end <= start {
			return nil, fmt.Errorf("未找到 JSON 数组")
		}
		if err := json.Unmarshal([]byte(text[start:end+1]), &items); err != nil {
			return nil, err
		}
	}
	return items, nil
}

// renderTriplets 将三元组集合渲染为（头，关系，尾）文本格式（前端展示用，与历史展示格式一致）
func renderTriplets(triplets []triplet.Triplet) string {
	var lines []string
	for _, t := range triplets {
		lines = append(lines, fmt.Sprintf("（%s，%s，%s）", t.Head, t.Relation, t.Tail))
	}
	return strings.Join(lines, "\n")
}

// chunkResult 单块抽取结果：候选/校验计数（漏斗用）+ 通过验证的三元组
type chunkResult struct {
	candidates int
	validated  int
	verified   []triplet.Triplet
	dropped    map[string]int
}

// runExtractionPipeline 分块抽取公共管道（UseModel/GetPdf 共用）：
// 并发抽取→解析→硬校验→验证归一→汇总去重。
// 每块在现有并发 goroutine 内顺序执行各环节；验证失败降级为该块候选全部保留（可用性优先）。
// 返回最终入库的三元组集合（规范名、跨块已去重）与前端展示文本，漏斗计数写入 trace。
func runExtractionPipeline(parts []string, trace *metrics.Trace) ([]triplet.Triplet, string) {
	results := make([]chunkResult, len(parts))
	ch := make(chan struct{}, 5) // 生成一个线程池

	var wg sync.WaitGroup
	var mu sync.Mutex

	for i, part := range parts {
		wg.Add(1)
		ch <- struct{}{} // 阻塞直到有空闲 Goroutine
		go func(i int, part string) {
			defer wg.Done()
			defer func() {
				<-ch // 释放go线程
			}()

			// 抽取（子环节计时：LLM）
			llmStart := time.Now()
			raw := TypeInModel(part)
			trace.Observe(metrics.StageLLM, time.Since(llmStart), raw == "")

			// 解析候选三元组 + 硬校验（子环节计时：triplet）
			parseStart := time.Now()
			candidates := parseTriplets(raw)
			validated, dropped := triplet.Validate(candidates)
			trace.Observe(metrics.StageTriplet, time.Since(parseStart), false)

			// 验证归一（子环节计时：verify）；失败降级为全部保留，不阻断主流程
			verifyStart := time.Now()
			verified, err := triplet.VerifyTriplets(part, validated)
			trace.Observe(metrics.StageVerify, time.Since(verifyStart), err != nil)
			if err != nil {
				slog.Warn("三元组验证失败，降级为全部保留", "chunk_index", i, "count", len(validated), "error", err)
				verified = validated
			}

			// 使用 Mutex 保护对共享资源的操作
			mu.Lock()
			results[i] = chunkResult{
				candidates: len(candidates),
				validated:  len(validated),
				verified:   verified,
				dropped:    dropped,
			}
			mu.Unlock()

		}(i, part)
	}
	wg.Wait()

	// 聚合漏斗计数
	candidateTotal, validatedTotal, verifiedTotal := 0, 0, 0
	droppedTotal := map[string]int{}
	var merged []triplet.Triplet
	for _, r := range results {
		candidateTotal += r.candidates
		validatedTotal += r.validated
		verifiedTotal += len(r.verified)
		for k, v := range r.dropped {
			droppedTotal[k] += v
		}
		merged = append(merged, r.verified...)
	}

	// 跨块精确去重：归一后同 head+relation+tail 仅保留一条
	// （图谱层 MERGE 本身幂等不重复画边，去重收益为省冗余写入与原子 embedding 调用）
	seen := make(map[string]bool)
	var final []triplet.Triplet
	for _, t := range merged {
		key := t.Head + "\x00" + t.Relation + "\x00" + t.Tail
		if seen[key] {
			continue
		}
		seen[key] = true
		final = append(final, t)
	}

	// 漏斗四级计数与丢弃分类写入监控（triplet_count 键名保留，兼容既有看板）
	trace.SetMeta("candidate_count", candidateTotal)
	trace.SetMeta("validated_count", validatedTotal)
	trace.SetMeta("verified_count", verifiedTotal)
	trace.SetMeta("dropped_empty", droppedTotal[triplet.DroppedEmpty])
	trace.SetMeta("dropped_len", droppedTotal[triplet.DroppedLen])
	trace.SetMeta("dropped_blacklist", droppedTotal[triplet.DroppedBlacklist])
	trace.SetMeta("dropped_format", droppedTotal[triplet.DroppedFormat])
	trace.SetMeta("triplet_count", len(final))

	return final, renderTriplets(final)
}
