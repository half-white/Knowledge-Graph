package flag

import (
	"flag"

	"github.com/fatih/structs"
)

type Option struct {
	DB bool
}

// Parse 解析命令行参数
func Parse() Option {
	db := flag.Bool("db", false, "初始化数据库")
	flag.Parse()
	return Option{
		DB: *db,
	}
}

// IsWebStop 是否停止web项目
func IsWebStop(option Option) (f bool) {
	maps := structs.Map(&option)
	for _, v := range maps {
		switch val := v.(type) {
		case string:
			if val != "" {
				f = true
			}
		case bool:
			if val == true {
				f = true
			}
		}
	}
	return f
}

func UseOption(option Option) {
	if option.DB {
		Makemigrations()
		return
	}
}
