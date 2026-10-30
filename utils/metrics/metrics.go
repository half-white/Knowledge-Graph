// Package metrics 提供知识图谱构建全流程的轻量级耗时统计（REQ-003）。
//
// 设计（变更 #3 收缩后）：唯一数据出口为 Finish() 输出的一条"流程耗时汇总"
// 结构化 slog 日志（控制台 + logs/sse-*.jsonl 落盘），由 Promtail → Loki 采集，
// 在 Grafana 看板（deploy/observability）可视化与告警。
//
// 日志字段契约（Grafana LogQL 依赖）：
//	trace_id, source, total_ms, {stage}_ms（10 个阶段）, {stage}_errs（仅失败时）,
//	chunk_count, triplet_count, llm_wall_ms, filename 等 meta。
//	含点号的 stage key 由 Loki json 解析规范化为下划线（graph.store_ms → graph_store_ms）。
//
// 旁路安全：nil 接收者全部为 no-op（调用方无需判空），Observe/SetMeta/Finish
// 内部 recover 兜底，任何监控异常不影响主流程。
package metrics

import (
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// 阶段 key 常量（全项目统一口径；OCR 仅 PDF 流程存在）
const (
	StageOCR        = "ocr"           // OCR识别
	StageSplit      = "split"         // 文本切片
	StageLLM        = "llm"           // LLM三元组抽取（单次调用粒度，并发执行）
	StageTriplet    = "triplet"       // 三元组提取
	StageRAGEmbed   = "rag.embedding" // RAG子环节：embedding生成
	StageRAGSearch  = "rag.search"    // RAG子环节：向量相似度检索
	StageRAGJudge   = "rag.judge"     // RAG子环节：LLM高价值判定
	StageRAGSave    = "rag.save"      // RAG子环节：向量写入
	StageGraphStore = "graph.store"   // Neo4j三元组写入
	StageGraphQuery = "graph.query"   // 图谱查询与JSON组装
)

// stageAccum 单次请求内某阶段的累计值（耗时之和 / 调用次数 / 失败次数）
type stageAccum struct {
	sumMS    float64
	count    int64
	errCount int64
}

// Trace 单次请求的阶段耗时追踪器（并发安全）
type Trace struct {
	mu      sync.Mutex
	id      string
	source  string
	startAt time.Time

	accums map[string]*stageAccum
	meta   map[string]any

	llmFirstStart time.Time // LLM 并发墙钟区间：最早一次开始
	llmLastEnd    time.Time // LLM 并发墙钟区间：最晚一次结束
	finished      bool      // Finish 幂等标记
}

// StartTrace 创建请求级追踪器（UseModel/GetPdf 入口各调一次）
func StartTrace(source string) *Trace {
	return &Trace{
		id:      fmt.Sprintf("%d-%s", time.Now().UnixMilli(), uuid.New().String()[:8]),
		source:  source,
		startAt: time.Now(),
		accums:  make(map[string]*stageAccum),
		meta:    make(map[string]any),
	}
}

// Observe 记录一次阶段耗时。nil 接收者为 no-op（调用方无需判空），
// 内部 recover 兜底，任何监控异常都不外泄到主流程。
func (t *Trace) Observe(stage string, d time.Duration, failed bool) {
	if t == nil {
		return
	}
	defer func() { _ = recover() }()

	ms := float64(d.Microseconds()) / 1000.0
	now := time.Now()

	t.mu.Lock()
	defer t.mu.Unlock()

	a, ok := t.accums[stage]
	if !ok {
		a = &stageAccum{}
		t.accums[stage] = a
	}
	a.sumMS += ms
	a.count++
	if failed {
		a.errCount++
	}

	// LLM 并发墙钟区间：所有 llm 样本的最早开始 ~ 最晚结束
	if stage == StageLLM {
		if start := now.Add(-d); t.llmFirstStart.IsZero() || start.Before(t.llmFirstStart) {
			t.llmFirstStart = start
		}
		if now.After(t.llmLastEnd) {
			t.llmLastEnd = now
		}
	}
}

// SetMeta 记录请求级元信息（chunk_count、triplet_count、filename 等）。nil 接收者安全。
func (t *Trace) SetMeta(key string, value any) {
	if t == nil {
		return
	}
	defer func() { _ = recover() }()

	t.mu.Lock()
	defer t.mu.Unlock()
	t.meta[key] = value
}

// Finish 结束追踪：输出一条"流程耗时汇总"结构化日志（监控的唯一出口）。
// 幂等，重复调用只生效一次。nil 接收者安全。
func (t *Trace) Finish() {
	if t == nil {
		return
	}
	defer func() { _ = recover() }()

	t.mu.Lock()
	if t.finished {
		t.mu.Unlock()
		return
	}
	t.finished = true
	totalMS := float64(time.Since(t.startAt).Microseconds()) / 1000.0

	// 阶段字段：{stage}_ms 累计耗时；{stage}_errs 失败次数（仅 >0 时输出，供 Grafana 查询/告警）
	stages := make([]string, 0, len(t.accums))
	for stage := range t.accums {
		stages = append(stages, stage)
	}
	sort.Strings(stages)

	logAttrs := make([]any, 0, 6+2*len(stages)+2*len(t.meta))
	logAttrs = append(logAttrs,
		"trace_id", t.id,
		"source", t.source,
		"total_ms", round2(totalMS),
	)
	for _, stage := range stages {
		a := t.accums[stage]
		logAttrs = append(logAttrs, stage+"_ms", round2(a.sumMS))
		if a.errCount > 0 {
			logAttrs = append(logAttrs, stage+"_errs", a.errCount)
		}
	}
	// LLM 并发墙钟时间（区别于各单次调用耗时之和，体现并发收益）
	if !t.llmFirstStart.IsZero() && t.llmLastEnd.After(t.llmFirstStart) {
		logAttrs = append(logAttrs, "llm_wall_ms",
			round2(float64(t.llmLastEnd.Sub(t.llmFirstStart).Microseconds())/1000.0))
	}
	for k, v := range t.meta {
		logAttrs = append(logAttrs, k, v)
	}
	t.mu.Unlock()

	slog.Info("流程耗时汇总", logAttrs...)
}

// round2 保留两位小数，仅用于日志展示
func round2(v float64) float64 {
	return float64(int64(v*100+0.5)) / 100
}
