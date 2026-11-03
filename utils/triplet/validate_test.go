package triplet

import (
	"testing"

	"unicode/utf8"
)

// 生成指定 rune 长度的字符串
func repeatStr(s string, n int) string {
	out := make([]rune, 0, n*utf8.RuneCountInString(s))
	for i := 0; i < n; i++ {
		out = append(out, []rune(s)...)
	}
	return string(out)
}

// TestValidate 表驱动硬校验：覆盖非空/长度边界/黑名单精确匹配/清洗后命中/白名单
func TestValidate(t *testing.T) {
	cases := []struct {
		name      string
		input     Triplet
		wantKept  bool
		wantDrops map[string]int
	}{
		{
			name:     "合法中文三元组通过",
			input:    Triplet{Head: "乔布斯", Relation: "创立", Tail: "苹果公司"},
			wantKept: true,
		},
		{
			name:     "英文实体与关系通过",
			input:    Triplet{Head: "iPhone", Relation: "belongs_to", Tail: "Apple"},
			wantKept: true,
		},
		{
			name:      "头实体为空丢弃",
			input:     Triplet{Head: "", Relation: "创立", Tail: "苹果公司"},
			wantDrops: map[string]int{DroppedEmpty: 1},
		},
		{
			name:      "纯空白字段丢弃",
			input:     Triplet{Head: "  ", Relation: "创立", Tail: "苹果公司"},
			wantDrops: map[string]int{DroppedEmpty: 1},
		},
		{
			name:      "实体长度恰30字通过",
			input:     Triplet{Head: repeatStr("实", 30), Relation: "创立", Tail: "苹果公司"},
			wantKept:  true,
		},
		{
			name:      "实体长度31字丢弃",
			input:     Triplet{Head: repeatStr("实", 31), Relation: "创立", Tail: "苹果公司"},
			wantDrops: map[string]int{DroppedLen: 1},
		},
		{
			name:      "关系长度恰12字通过",
			input:     Triplet{Head: "乔布斯", Relation: repeatStr("创", 12), Tail: "苹果公司"},
			wantKept:  true,
		},
		{
			name:      "关系长度13字丢弃",
			input:     Triplet{Head: "乔布斯", Relation: repeatStr("创", 13), Tail: "苹果公司"},
			wantDrops: map[string]int{DroppedLen: 1},
		},
		{
			name:      "黑名单精确命中丢弃",
			input:     Triplet{Head: "苹果公司", Relation: "相关", Tail: "环保"},
			wantDrops: map[string]int{DroppedBlacklist: 1},
		},
		{
			name:     "黑名单非精确匹配放行（防误杀）",
			input:    Triplet{Head: "苹果公司", Relation: "相关性", Tail: "环保"},
			wantKept: true,
		},
		{
			name:      "清洗后露出黑名单词丢弃",
			input:     Triplet{Head: "苹果公司", Relation: "相关。", Tail: "环保"},
			wantDrops: map[string]int{DroppedBlacklist: 1},
		},
		{
			name:      "关系词含空格丢弃（白名单）",
			input:     Triplet{Head: "乔布斯", Relation: "创立 创办", Tail: "苹果公司"},
			wantDrops: map[string]int{DroppedFormat: 1},
		},
		{
			name:      "关系词含标点丢弃（白名单）",
			input:     Triplet{Head: "乔布斯", Relation: "创立（早期）", Tail: "苹果公司"},
			wantDrops: map[string]int{DroppedFormat: 1},
		},
		{
			name:      "关系词内嵌引号丢弃（白名单，首尾引号已被清洗）",
			input:     Triplet{Head: "乔布斯", Relation: "创\"立", Tail: "苹果公司"},
			wantDrops: map[string]int{DroppedFormat: 1},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			kept, dropped := Validate([]Triplet{tc.input})

			// 第一层：功能验证（调用不报错由运行时保证，检查返回结构）
			if tc.wantKept && len(kept) != 1 {
				t.Fatalf("期望保留 1 条，实际保留 %d 条", len(kept))
			}
			if !tc.wantKept && len(kept) != 0 {
				t.Fatalf("期望丢弃，实际保留了: %+v", kept)
			}

			// 第二层：结果验证（保留内容 = 清洗后输入）
			if tc.wantKept && kept[0] != Clean(tc.input) {
				t.Errorf("保留的三元组应为清洗后内容: got %+v want %+v", kept[0], Clean(tc.input))
			}

			// 第三层：过程验证（丢弃分类计数正确）
			for k, want := range tc.wantDrops {
				if dropped[k] != want {
					t.Errorf("丢弃分类 %s 计数 = %d，期望 %d", k, dropped[k], want)
				}
			}
			if len(dropped) > len(tc.wantDrops) && len(tc.wantDrops) > 0 {
				t.Errorf("出现预期外的丢弃分类: %+v", dropped)
			}
		})
	}
}

// TestClean 验证清洗行为与幂等性
func TestClean(t *testing.T) {
	cases := []struct {
		name  string
		input Triplet
		want  Triplet
	}{
		{
			name:  "去首尾空白与句号",
			input: Triplet{Head: " 苹果公司。", Relation: " 创立 ", Tail: "乔布斯，"},
			want:  Triplet{Head: "苹果公司", Relation: "创立", Tail: "乔布斯"},
		},
		{
			name:  "去引号与括号",
			input: Triplet{Head: "\"苹果\"", Relation: "'发布'", Tail: "（iPhone）"},
			want:  Triplet{Head: "苹果", Relation: "发布", Tail: "iPhone"},
		},
		{
			name:  "中间标点保留（只清首尾）",
			input: Triplet{Head: "乔布斯.保罗.艾伦", Relation: "共同创立", Tail: "苹果公司"},
			want:  Triplet{Head: "乔布斯.保罗.艾伦", Relation: "共同创立", Tail: "苹果公司"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Clean(tc.input)
			if got != tc.want {
				t.Errorf("Clean() = %+v, want %+v", got, tc.want)
			}
			// 幂等性：清洗结果再清洗不变
			if Clean(got) != got {
				t.Errorf("Clean 不幂等: %+v -> %+v", got, Clean(got))
			}
		})
	}
}
