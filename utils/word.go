package utils

import (
	"fmt"
	"strings"

	"baliance.com/gooxml/document"
)

func extractTextFromWord(filePath string) (string, error) {
	doc, err := document.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("error opening document: %w", err)
	}

	var str strings.Builder
	// 遍历文档中的所有段落
	for _, para := range doc.Paragraphs() {
		// 遍历每个段落中的每个格式片段（run）
		for _, run := range para.Runs() {
			str.WriteString(run.Text() + "\n")
		}
	}

	return str.String(), nil
}
