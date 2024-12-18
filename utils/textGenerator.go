package utils

import (
	"fmt"
	"strings"
)

func TextGenerator(filePath string) (string, error) {
	// 根据文件扩展名选择提取方法
	if strings.HasSuffix(filePath, ".pdf") {
		// return extractTextFromPDF(filePath) // 使用 PDF 提取函数
	} else if strings.HasSuffix(filePath, ".docx") {
		return extractTextFromWord(filePath) // 使用 Word 提取函数
	}
	return "", fmt.Errorf("unsupported file format")
}
