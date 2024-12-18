package graph_api

import (
	"encoding/csv"
	"fmt"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/neo4j/neo4j-go-driver/neo4j"
)

// 生成图谱逻辑
func (this GraphApi) GenGraph(c *gin.Context) {
	// 向后端服务提交csv文件，进行结点导入

	// 图谱展示

}

// 导入csv文件
func ImportCsv(filename string) *csv.Reader {
	// 打开文件
	file, err := os.Open(filename)
	if err != nil {
		fmt.Println("文件打开出错")
		return nil
	}

	//创建csv.Reader
	reader := csv.NewReader(file)

	return reader
}

// 生成知识图谱：新建数据表，插入三元组节点
func InsertNode(reader *csv.Reader, neo4j *neo4j.Duration) {
	//创建一个批量操作

}
