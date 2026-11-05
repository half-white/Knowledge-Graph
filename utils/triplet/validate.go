package triplet

import (
	"regexp"
	"strings"
)

// Triplet 知识图谱三元组：头实体、关系、尾实体（json tag 与 LLM 抽取输出对齐）
type Triplet struct {
	Head     string `json:"head"`
	Relation string `json:"relation"`
	Tail     string `json:"tail"`
}

// 硬校验丢弃分类常量（漏斗计数 key，随 trace meta 上报）
const (
	DroppedEmpty     = "dropped_empty"     // 空壳三元组：字段为空串/纯空白
	DroppedLen       = "dropped_len"       // 超长：实体/关系超出长度上限
	DroppedBlacklist = "dropped_blacklist" // 空洞关系词：命中黑名单
	DroppedFormat    = "dropped_format"    // 格式非法：关系词含白名单外字符
)

// 实体/关系长度上限（rune 计）
const (
	MaxEntityLen   = 30
	MaxRelationLen = 12
)

// emptyRelations 关系词黑名单（精确匹配，防误杀：如「相关性」不会命中「相关」）
var emptyRelations = map[string]bool{
	"相关": true,
	"有关": true,
	"有":  true,
	"是":  true,
	"涉及": true,
	"包括": true,
	"拥有": true,
	"属于": true,
	"存在": true,
	"进行": true,
	"关联": true,
}

// validRelationRe 关系词字符白名单：仅中文、字母、数字、下划线，
// 拦截空格/标点/引号等会导致 Cypher 关系类型非法或污染图谱的字符
var validRelationRe = regexp.MustCompile(`^[\p{Han}\w]+$`)

// trimChars 实体/关系首尾需要清除的标点、引号与空白
const trimChars = " \t　。．.、,，;；:：!！?？\"'‘’“”`()（）[]【】"

// Clean 清洗三元组：trim 空白、去首尾标点/引号（幂等）
func Clean(t Triplet) Triplet {
	t.Head = strings.Trim(strings.TrimSpace(t.Head), trimChars)
	t.Relation = strings.Trim(strings.TrimSpace(t.Relation), trimChars)
	t.Tail = strings.Trim(strings.TrimSpace(t.Tail), trimChars)
	return t
}

// Validate 表驱动硬校验：内部先清洗再逐条校验（幂等，可对验证器输出复检）。
// 返回通过校验的三元组（已清洗）与按分类的丢弃计数。
func Validate(ts []Triplet) (kept []Triplet, dropped map[string]int) {
	dropped = make(map[string]int)
	for _, t := range ts {
		t = Clean(t)
		if reason, ok := check(t); !ok {
			dropped[reason]++
			continue
		}
		kept = append(kept, t)
	}
	return kept, dropped
}

// check 单条校验，返回丢弃分类；通过时 ok=true
func check(t Triplet) (string, bool) {
	// 非空校验
	if t.Head == "" || t.Relation == "" || t.Tail == "" {
		return DroppedEmpty, false
	}
	// 长度上限（半截短语、整句被当实体的主要拦截点）
	if len([]rune(t.Head)) > MaxEntityLen || len([]rune(t.Tail)) > MaxEntityLen ||
		len([]rune(t.Relation)) > MaxRelationLen {
		return DroppedLen, false
	}
	// 关系词黑名单（空洞关系）
	// if emptyRelations[t.Relation] {
	// 	return DroppedBlacklist, false
	// }
	// 关系词字符白名单
	if !validRelationRe.MatchString(t.Relation) {
		return DroppedFormat, false
	}
	return "", true
}
