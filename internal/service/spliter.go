// Package service 图谱业务层（REQ-008 分层重构）：
// 业务编排与业务逻辑，不依赖 gin；数据访问下沉 internal/dao。
// 本文件为文本切分逻辑，函数体自 api/model_api/use_model.go 逐行搬迁。
package service

import "strings"

// splitContent 将文本按语义边界拆解为约 maxLength 字符的片段。
// 切分优先级：段落（行）> 句子 > 硬截断，相邻语义单元贪心合并，
// 使每个片段尽量完整、长度贴近 maxLength，供下游模型逐块抽取。
func splitContent(content string) []string {
	const maxLength = 1000
	content = strings.TrimSpace(content)
	if content == "" {
		return nil
	}

	// 1. 以行为单位拆出语义原子（段落/标题/列表项），超长行退化为句子级切分
	var units []string
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if len([]rune(line)) <= maxLength {
			units = append(units, line)
			continue
		}
		units = append(units, splitBySentence(line, maxLength)...)
	}

	// 2. 贪心合并相邻语义单元，片段长度接近但不轻易超过 maxLength
	var parts []string
	var buf strings.Builder
	bufLen := 0
	flush := func() {
		if buf.Len() > 0 {
			parts = append(parts, strings.TrimSpace(buf.String()))
			buf.Reset()
			bufLen = 0
		}
	}
	for _, u := range units {
		uLen := len([]rune(u))
		if bufLen > 0 && bufLen+uLen+1 > maxLength {
			flush()
		}
		if bufLen > 0 {
			buf.WriteString("\n")
			bufLen++
		}
		buf.WriteString(u)
		bufLen += uLen
	}
	flush()
	return parts
}

// splitBySentence 将超长段落按句子边界切分（保留句末标点与后随引号/括号），
// 单句仍超长时按 maxLength 硬截断。
func splitBySentence(text string, maxLength int) []string {
	runes := []rune(text)

	// 先按句末标点切成句子
	var sentences []string
	start := 0
	for i := 0; i < len(runes); i++ {
		if isSentenceEnd(runes[i]) {
			// 吸收紧跟的结束引号/括号，避免把引号留在下一句开头
			for i+1 < len(runes) && strings.ContainsRune("”’」』）)\"", runes[i+1]) {
				i++
			}
			sentences = append(sentences, string(runes[start:i+1]))
			start = i + 1
		}
	}
	if start < len(runes) {
		sentences = append(sentences, string(runes[start:]))
	}

	// 句子贪心合并；单句超长时硬截断
	var units []string
	var buf strings.Builder
	bufLen := 0
	flush := func() {
		if buf.Len() > 0 {
			units = append(units, strings.TrimSpace(buf.String()))
			buf.Reset()
			bufLen = 0
		}
	}
	for _, s := range sentences {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		r := []rune(s)
		if len(r) > maxLength {
			flush()
			for len(r) > maxLength {
				units = append(units, string(r[:maxLength]))
				r = r[maxLength:]
			}
			s = strings.TrimSpace(string(r))
			if s == "" {
				continue
			}
		}
		if bufLen > 0 && bufLen+len([]rune(s)) > maxLength {
			flush()
		}
		buf.WriteString(s)
		bufLen += len([]rune(s))
	}
	flush()
	return units
}

// isSentenceEnd 判断是否为句末标点
func isSentenceEnd(r rune) bool {
	switch r {
	case '。', '！', '？', '；', '…', '!', '?', ';', '.':
		return true
	}
	return false
}
