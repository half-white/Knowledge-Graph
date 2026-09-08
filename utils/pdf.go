package utils

import (
	"log/slog"

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
			slog.Debug("PDF行", "position", row.Position)
			for _, word := range row.Content {
				slog.Debug("PDF词", "content", word.S)
			}
		}
	}
	return "", nil
}
