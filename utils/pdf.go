package utils

import (
	"fmt"

	"github.com/ledongthuc/pdf"
)

func readPdf(path string) (string, error) {
	f, r, err := pdf.Open(path)
	defer func() {
		_ = f.Close()
	}()
	if err != nil {
		return "", err
	}
	totalPage := r.NumPage()

	for pageIndex := 1; pageIndex <= totalPage; pageIndex++ {
		p := r.Page(pageIndex)
		if p.V.IsNull() {
			continue
		}

		rows, _ := p.GetTextByRow()
		for _, row := range rows {
			println(">>>> row: ", row.Position)
			for _, word := range row.Content {
				fmt.Println(word.S)
			}
		}
	}
	return "", nil
}

// func main() {
// 	filePath := "1.pdf"
// 	fmt.Println("Trying to open PDF file at:", filePath)

// 	str, err := readPdf(filePath)
// 	fmt.Println(str, err)
// }
