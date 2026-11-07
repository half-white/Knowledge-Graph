package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"SSE/utils/metrics"
	"SSE/utils/triplet"
)

// mockLLMByPrompt 启动模拟 GLM 接口：按 prompt 内容路由（抽取/验证 prompt 文案不同），
// 并统计各端点调用次数
func mockLLMByPrompt(t *testing.T, extractFn, verifyFn func() string) (*httptest.Server, *int64, *int64) {
	t.Helper()
	var extractCalls, verifyCalls int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		prompt := ""
		if len(body.Messages) > 0 {
			prompt = body.Messages[0].Content
		}

		w.Header().Set("Content-Type", "application/json")
		var content string
		if strings.Contains(prompt, "知识图谱构建专家") { // 抽取 prompt 标识
			atomic.AddInt64(&extractCalls, 1)
			content = extractFn()
		} else if strings.Contains(prompt, "知识图谱质量审核员") { // 验证 prompt 标识
			atomic.AddInt64(&verifyCalls, 1)
			content = verifyFn()
		} else {
			content = "未知调用"
		}

		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": content}},
			},
		}
		b, _ := json.Marshal(resp)
		w.Write(b)
	}))
	t.Cleanup(server.Close)
	return server, &extractCalls, &verifyCalls
}

// swapLLMTarget 将抽取与验证接口都指向 mock 并在测试结束后恢复
func swapLLMTarget(t *testing.T, url string) {
	t.Helper()
	oldLLM := llmURL
	oldVerifyURL, oldVerifyTimeout := triplet.VerifyURL, triplet.VerifyTimeout
	llmURL = url
	triplet.VerifyURL = url
	triplet.VerifyTimeout = 2 * time.Second
	t.Cleanup(func() {
		llmURL = oldLLM
		triplet.VerifyURL, triplet.VerifyTimeout = oldVerifyURL, oldVerifyTimeout
	})
}

// TestParseTriplets JSON 优先、围栏/杂文字容忍、正则降级、完全失败返回空
func TestParseTriplets(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  []triplet.Triplet
	}{
		{
			name:  "标准 JSON 数组",
			input: `[{"head":"乔布斯","relation":"创立","tail":"苹果公司"}]`,
			want:  []triplet.Triplet{{Head: "乔布斯", Relation: "创立", Tail: "苹果公司"}},
		},
		{
			name:  "markdown 围栏包裹",
			input: "```json\n[{\"head\":\"A\",\"relation\":\"关系\",\"tail\":\"B\"}]\n```",
			want:  []triplet.Triplet{{Head: "A", Relation: "关系", Tail: "B"}},
		},
		{
			name:  "前后杂文字包围",
			input: `抽取结果如下：[{"head":"A","relation":"关系","tail":"B"}] 以上。`,
			want:  []triplet.Triplet{{Head: "A", Relation: "关系", Tail: "B"}},
		},
		{
			name:  "正则降级：括号自由文本",
			input: "（乔布斯，创立，苹果公司）\n（苹果公司，发布，iPhone）",
			want: []triplet.Triplet{
				{Head: "乔布斯", Relation: "创立", Tail: "苹果公司"},
				{Head: "苹果公司", Relation: "发布", Tail: "iPhone"},
			},
		},
		{
			name:  "完全无法解析返回空",
			input: "抱歉，我无法处理这段文字。",
			want:  nil,
		},
		{
			name:  "空输入返回空",
			input: "",
			want:  nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseTriplets(tc.input)
			if len(got) != len(tc.want) {
				t.Fatalf("解析数量 = %d, want %d: %+v", len(got), len(tc.want), got)
			}
			for i, w := range tc.want {
				if got[i] != w {
					t.Errorf("第 %d 条 = %+v, want %+v", i, got[i], w)
				}
			}
		})
	}
}

// TestRenderTriplets 渲染为（头，关系，尾）每行一条
func TestRenderTriplets(t *testing.T) {
	if got := renderTriplets([]triplet.Triplet{
		{Head: "乔布斯", Relation: "创立", Tail: "苹果公司"},
		{Head: "苹果公司", Relation: "发布", Tail: "iPhone"},
	}); got != "（乔布斯，创立，苹果公司）\n（苹果公司，发布，iPhone）" {
		t.Errorf("渲染结果不符: %q", got)
	}
	if got := renderTriplets(nil); got != "" {
		t.Errorf("空集合应渲染为空串: %q", got)
	}
}

// TestRunExtractionPipeline_Success 全链路：抽取→解析→硬校验→验证归一→去重→渲染
func TestRunExtractionPipeline_Success(t *testing.T) {
	const chunk = "乔布斯于1976年创立了苹果公司，该公司后来发布了iPhone。库克任职于苹果公司。"
	server, _, verifyCalls := mockLLMByPrompt(t,
		func() string {
			// 抽取：4 个候选（第 3 条为评论类，预期被验证器剔除）
			return `[{"head":"乔布斯","relation":"创立","tail":"苹果公司"},
			         {"head":"苹果","relation":"发布","tail":"iPhone"},
			         {"head":"某网友","relation":"认为","tail":"很好"},
			         {"head":"库克","relation":"任职","tail":"苹果公司"}]`
		},
		func() string {
			return `[{"index":0,"keep":true,"head":"苹果公司","relation":"创立","tail":"乔布斯"},
			         {"index":1,"keep":true,"head":"苹果公司","relation":"发布","tail":"iPhone"},
			         {"index":2,"keep":false}]`
		})
	swapLLMTarget(t, server.URL)

	trace := metrics.StartTrace("test")
	triplets, text := runExtractionPipeline([]string{chunk}, trace)

	// 第二层：结果验证（keep=false 剔除；规范化生效；缺失候选保留原名）
	want := []triplet.Triplet{
		{Head: "苹果公司", Relation: "创立", Tail: "乔布斯"},
		{Head: "苹果公司", Relation: "发布", Tail: "iPhone"},
		{Head: "库克", Relation: "任职", Tail: "苹果公司"}, // index3 缺失 → 保留原名
	}
	if len(triplets) != len(want) {
		t.Fatalf("期望 %d 条，实际 %d 条: %+v", len(want), len(triplets), triplets)
	}
	for i, w := range want {
		if triplets[i] != w {
			t.Errorf("第 %d 条 = %+v, want %+v", i, triplets[i], w)
		}
	}

	// 前端展示文本：验证后集合渲染
	wantText := "（苹果公司，创立，乔布斯）\n（苹果公司，发布，iPhone）\n（库克，任职，苹果公司）"
	if text != wantText {
		t.Errorf("渲染文本不符:\n got %q\nwant %q", text, wantText)
	}

	// 第三层：过程验证（每块抽取 1 次 + 验证 1 次）
	if atomic.LoadInt64(verifyCalls) != 1 {
		t.Errorf("验证应调用 1 次，实际 %d 次", atomic.LoadInt64(verifyCalls))
	}
}

// TestRunExtractionPipeline_Dedup 跨块精确去重：两块产出归一后相同的三元组只保留一条
func TestRunExtractionPipeline_Dedup(t *testing.T) {
	server, _, _ := mockLLMByPrompt(t,
		func() string {
			return `[{"head":"甲实体","relation":"创立","tail":"乙实体"}]`
		},
		func() string {
			return `[{"index":0,"keep":true,"head":"甲实体","relation":"创立","tail":"乙实体"}]`
		})
	swapLLMTarget(t, server.URL)

	trace := metrics.StartTrace("test")
	triplets, _ := runExtractionPipeline([]string{"第一块内容", "第二块内容"}, trace)

	if len(triplets) != 1 {
		t.Fatalf("跨块去重后应剩 1 条，实际 %d 条: %+v", len(triplets), triplets)
	}
	if triplets[0].Head != "甲实体" {
		t.Errorf("去重后内容不符: %+v", triplets[0])
	}
}

// TestRunExtractionPipeline_Degradation 验证持续失败：降级为候选全部保留，主流程不阻断
func TestRunExtractionPipeline_Degradation(t *testing.T) {
	server, _, verifyCalls := mockLLMByPrompt(t,
		func() string {
			return `[{"head":"甲实体","relation":"创立","tail":"乙实体"},{"head":"丙实体","relation":"位于","tail":"丁实体"}]`
		},
		func() string {
			return "始终不是 JSON"
		})
	swapLLMTarget(t, server.URL)

	trace := metrics.StartTrace("test")
	triplets, text := runExtractionPipeline([]string{"测试内容"}, trace)

	// 降级：两个通过硬校验的候选全部保留
	if len(triplets) != 2 {
		t.Fatalf("降级应保留全部候选，实际 %d 条: %+v", len(triplets), triplets)
	}
	if !strings.Contains(text, "（甲实体，创立，乙实体）") {
		t.Errorf("降级路径渲染文本不符: %q", text)
	}
	// 重试 1 次共 2 次调用
	if atomic.LoadInt64(verifyCalls) != 2 {
		t.Errorf("验证应调用 2 次（含重试），实际 %d 次", atomic.LoadInt64(verifyCalls))
	}
}

// TestRunExtractionPipeline_EmptyExtract 空抽取结果：跳过验证调用，返回空集合
func TestRunExtractionPipeline_EmptyExtract(t *testing.T) {
	server, extractCalls, verifyCalls := mockLLMByPrompt(t,
		func() string { return `[]` },
		func() string { return `[]` })
	swapLLMTarget(t, server.URL)

	trace := metrics.StartTrace("test")
	triplets, text := runExtractionPipeline([]string{"没有知识的文本"}, trace)

	if len(triplets) != 0 || text != "" {
		t.Errorf("空抽取应返回空集合与空文本: %+v %q", triplets, text)
	}
	if atomic.LoadInt64(extractCalls) != 1 || atomic.LoadInt64(verifyCalls) != 0 {
		t.Errorf("空候选应跳过验证调用: extract=%d verify=%d",
			atomic.LoadInt64(extractCalls), atomic.LoadInt64(verifyCalls))
	}
}

// TestRunExtractionPipeline_HardValidation 硬校验在验证前生效：黑名单/格式垃圾不出现在结果中
func TestRunExtractionPipeline_HardValidation(t *testing.T) {
	server, _, _ := mockLLMByPrompt(t,
		func() string {
			return `[{"head":"甲实体","relation":"相关","tail":"乙实体"},
			         {"head":"丙实体","relation":"创立 创办","tail":"丁实体"},
			         {"head":"戊实体","relation":"创立","tail":"己实体"}]`
		},
		func() string {
			// 验证器只对唯一通过硬校验的候选（index0 = 戊实体条目）回填
			return `[{"index":0,"keep":true,"head":"戊实体","relation":"创立","tail":"己实体"}]`
		})
	swapLLMTarget(t, server.URL)

	trace := metrics.StartTrace("test")
	triplets, _ := runExtractionPipeline([]string{"测试内容"}, trace)

	if len(triplets) != 1 || triplets[0].Head != "戊实体" {
		t.Fatalf("黑名单/格式垃圾应被硬校验拦截，实际: %+v", triplets)
	}
}
