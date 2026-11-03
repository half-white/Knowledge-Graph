package triplet

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// glmContent 生成 GLM 格式响应体（choices[0].message.content = content）
func glmContent(content string) string {
	resp := map[string]interface{}{
		"choices": []map[string]interface{}{
			{"message": map[string]string{"content": content}},
		},
	}
	b, _ := json.Marshal(resp)
	return string(b)
}

// startGLMMock 启动模拟 GLM 接口：每次请求回调 promptHandler 决定返回内容，并统计调用次数
func startGLMMock(t *testing.T, promptHandler func(prompt string) string) (*httptest.Server, *int64) {
	t.Helper()
	var calls int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&calls, 1)
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
		fmt.Fprint(w, glmContent(promptHandler(prompt)))
	}))
	t.Cleanup(server.Close)
	return server, &calls
}

// swapVerifyTarget 将验证目标指向 mock 并在测试结束后恢复
func swapVerifyTarget(t *testing.T, url string, timeout time.Duration) {
	t.Helper()
	oldURL, oldTimeout := VerifyURL, VerifyTimeout
	VerifyURL, VerifyTimeout = url, timeout
	t.Cleanup(func() { VerifyURL, VerifyTimeout = oldURL, oldTimeout })
}

// TestVerifyTriplets_Success 正常路径：围栏包裹的 JSON、keep 过滤、实体规范化、空规范名保留原名
func TestVerifyTriplets_Success(t *testing.T) {
	chunk := "乔布斯于1976年创立了苹果公司，该公司后来发布了iPhone。蒂姆·库克任职于苹果公司。"
	var lastPrompt string
	server, calls := startGLMMock(t, func(prompt string) string {
		lastPrompt = prompt
		// markdown 围栏包裹，验证解析器容忍性
		return "```json\n" + `[
			{"index":0,"keep":true,"head":"苹果公司","relation":"创立","tail":"乔布斯"},
			{"index":1,"keep":true,"head":"苹果公司","relation":"发布","tail":"iPhone"},
			{"index":2,"keep":false},
			{"index":3,"keep":true,"head":"","relation":"","tail":""}
		]` + "\n```"
	})
	swapVerifyTarget(t, server.URL, time.Second)

	candidates := []Triplet{
		{Head: "乔布斯", Relation: "创立", Tail: "苹果公司"},
		{Head: "苹果", Relation: "发布", Tail: "iPhone"},
		{Head: "某网友", Relation: "认为", Tail: "很好"},
		{Head: "库克", Relation: "任职", Tail: "苹果公司"},
	}

	verified, err := VerifyTriplets(chunk, candidates)

	// 第一层：功能验证
	if err != nil {
		t.Fatalf("调用失败: %v", err)
	}

	// 第二层：结果验证（keep=false 剔除；规范化生效；空规范名保留原名）
	if len(verified) != 3 {
		t.Fatalf("期望保留 3 条，实际 %d 条: %+v", len(verified), verified)
	}
	want := []Triplet{
		{Head: "苹果公司", Relation: "创立", Tail: "乔布斯"},
		{Head: "苹果公司", Relation: "发布", Tail: "iPhone"}, // 规范名：苹果 → 苹果公司
		{Head: "库克", Relation: "任职", Tail: "苹果公司"},   // 空规范名回退原名
	}
	for i, w := range want {
		if verified[i] != w {
			t.Errorf("第 %d 条 = %+v, want %+v", i, verified[i], w)
		}
	}

	// 第三层：过程验证（调用 1 次；prompt 包含原文与编号候选）
	if atomic.LoadInt64(calls) != 1 {
		t.Errorf("期望调用 1 次，实际 %d 次", atomic.LoadInt64(calls))
	}
	if !strings.Contains(lastPrompt, "知识图谱质量审核员") ||
		!strings.Contains(lastPrompt, chunk) ||
		!strings.Contains(lastPrompt, "2. (某网友, 认为, 很好)") {
		t.Errorf("验证 prompt 缺少原文/候选列表: %s", lastPrompt)
	}
}

// TestVerifyTriplets_MissingIndex 候选缺失条目：缺失的候选保留原名入库（防误杀）
func TestVerifyTriplets_MissingIndex(t *testing.T) {
	server, _ := startGLMMock(t, func(prompt string) string {
		// 8 个候选只回 5 条（index 0-4）
		return `[{"index":0,"keep":true},{"index":1,"keep":true},{"index":2,"keep":true},{"index":3,"keep":true},{"index":4,"keep":true}]`
	})
	swapVerifyTarget(t, server.URL, time.Second)

	var candidates []Triplet
	for i := 0; i < 8; i++ {
		candidates = append(candidates, Triplet{
			Head:     fmt.Sprintf("实体%d", i),
			Relation: "关系",
			Tail:     "目标",
		})
	}

	verified, err := VerifyTriplets("测试原文", candidates)
	if err != nil {
		t.Fatalf("调用失败: %v", err)
	}
	if len(verified) != 8 {
		t.Fatalf("缺失候选应保留原名，期望 8 条，实际 %d 条", len(verified))
	}
	// 缺失的 index 5-7 保留原名
	if verified[7].Head != "实体7" {
		t.Errorf("缺失候选未保留原名: %+v", verified[7])
	}
}

// TestVerifyTriplets_IndexAlignment index 越界丢弃、重复取首个、keep=false 剔除
func TestVerifyTriplets_IndexAlignment(t *testing.T) {
	server, _ := startGLMMock(t, func(prompt string) string {
		return `[{"index":9,"keep":true,"head":"越界"},{"index":0,"keep":true,"head":"首个"},{"index":0,"keep":true,"head":"重复"},{"index":1,"keep":false}]`
	})
	swapVerifyTarget(t, server.URL, time.Second)

	candidates := []Triplet{
		{Head: "原头", Relation: "关系", Tail: "原尾"},
		{Head: "被删", Relation: "关系", Tail: "被删"},
	}

	verified, err := VerifyTriplets("测试原文", candidates)
	if err != nil {
		t.Fatalf("调用失败: %v", err)
	}
	if len(verified) != 1 {
		t.Fatalf("期望保留 1 条，实际 %d 条: %+v", len(verified), verified)
	}
	if verified[0].Head != "首个" {
		t.Errorf("重复 index 应取首个，实际: %+v", verified[0])
	}
}

// TestVerifyTriplets_RetryOnBadJSON 首次坏 JSON 重试后成功（共调用 2 次）
func TestVerifyTriplets_RetryOnBadJSON(t *testing.T) {
	var calls int64
	server, _ := startGLMMock(t, func(prompt string) string {
		if atomic.AddInt64(&calls, 1) == 1 {
			return "这不是 JSON 输出，抱歉"
		}
		return `[{"index":0,"keep":true,"head":"甲","relation":"关系","tail":"乙"}]`
	})
	swapVerifyTarget(t, server.URL, time.Second)

	verified, err := VerifyTriplets("原文", []Triplet{{Head: "甲", Relation: "关系", Tail: "乙"}})
	if err != nil {
		t.Fatalf("重试后应成功: %v", err)
	}
	if len(verified) != 1 || verified[0].Head != "甲" {
		t.Errorf("结果不符: %+v", verified)
	}
	if atomic.LoadInt64(&calls) != 2 {
		t.Errorf("期望调用 2 次（重试1次），实际 %d 次", atomic.LoadInt64(&calls))
	}
}

// TestVerifyTriplets_AlwaysBadJSON 持续坏格式：重试后仍失败返回 error（调用方降级）
func TestVerifyTriplets_AlwaysBadJSON(t *testing.T) {
	var calls int64
	server, _ := startGLMMock(t, func(prompt string) string {
		atomic.AddInt64(&calls, 1)
		return "始终不是 JSON"
	})
	swapVerifyTarget(t, server.URL, time.Second)

	_, err := VerifyTriplets("原文", []Triplet{{Head: "甲", Relation: "关系", Tail: "乙"}})
	if err == nil {
		t.Fatal("持续坏格式应返回 error")
	}
	if atomic.LoadInt64(&calls) != 2 {
		t.Errorf("期望调用 2 次（重试1次），实际 %d 次", atomic.LoadInt64(&calls))
	}
}

// TestVerifyTriplets_HTTPError 接口 500：解析失败重试后返回 error
func TestVerifyTriplets_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "server error", http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)
	swapVerifyTarget(t, server.URL, time.Second)

	_, err := VerifyTriplets("原文", []Triplet{{Head: "甲", Relation: "关系", Tail: "乙"}})
	if err == nil {
		t.Fatal("接口异常应返回 error")
	}
}

// TestVerifyTriplets_Timeout 超时：注入短超时触发失败重试后返回 error
func TestVerifyTriplets_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		fmt.Fprint(w, glmContent(`[{"index":0,"keep":true}]`))
	}))
	t.Cleanup(server.Close)
	swapVerifyTarget(t, server.URL, 50*time.Millisecond)

	_, err := VerifyTriplets("原文", []Triplet{{Head: "甲", Relation: "关系", Tail: "乙"}})
	if err == nil {
		t.Fatal("超时应返回 error")
	}
}

// TestVerifyTriplets_EmptyCandidates 空候选：直接跳过，不发起 LLM 调用
func TestVerifyTriplets_EmptyCandidates(t *testing.T) {
	server, calls := startGLMMock(t, func(prompt string) string {
		return `[]`
	})
	swapVerifyTarget(t, server.URL, time.Second)

	verified, err := VerifyTriplets("原文", nil)
	if err != nil {
		t.Fatalf("空候选不应报错: %v", err)
	}
	if verified != nil {
		t.Errorf("空候选应返回 nil，实际: %+v", verified)
	}
	if atomic.LoadInt64(calls) != 0 {
		t.Errorf("空候选不应发起调用，实际 %d 次", atomic.LoadInt64(calls))
	}
}

// TestVerifyTriplets_PostValidateRecheck 复检：验证器输出的超长/黑名单/非法格式同样被硬校验拦截
func TestVerifyTriplets_PostValidateRecheck(t *testing.T) {
	server, _ := startGLMMock(t, func(prompt string) string {
		return `[
			{"index":0,"keep":true,"head":"甲","relation":"相关","tail":"乙"},
			{"index":1,"keep":true,"head":"超长实体超长实体超长实体超长实体超长实体超长实体超长实体超长实体","relation":"关系","tail":"乙"},
			{"index":2,"keep":true,"head":"丙","relation":"关 系","tail":"乙"}
		]`
	})
	swapVerifyTarget(t, server.URL, time.Second)

	candidates := []Triplet{
		{Head: "甲", Relation: "关系", Tail: "乙"},
		{Head: "超长", Relation: "关系", Tail: "乙"},
		{Head: "丙", Relation: "关系", Tail: "乙"},
	}

	verified, err := VerifyTriplets("原文", candidates)
	if err != nil {
		t.Fatalf("调用失败: %v", err)
	}
	if len(verified) != 0 {
		t.Errorf("验证器输出的垃圾应被复检全部拦截，实际保留: %+v", verified)
	}
}
