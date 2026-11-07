// Package service 图谱业务层（REQ-008 分层重构）：
// 本文件为 Neo4j 图谱查询与 ECharts 数据组装，函数体自 api/model_api/use_model.go 逐行搬迁。
// JSON 字段名为前端 graphParser.js 依赖的契约，不可变更。
package service

import (
	"SSE/global"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j/dbtype"
)

type Node struct {
	Name      string `json:"name"`
	SymolSize int    `json:"symbolSize"`
	Category  int    `json:"category"`
}

type Link struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Value  string `json:"value"`
}

type Category struct {
	Name string `json:"name"`
}

type Graph struct {
	Nodes      []Node     `json:"nodes"`
	Links      []Link     `json:"links"`
	Categories []Category `json:"categories"`
}

// 从Neo4j数据库中搜索对应id的节点和关系信息（原 displayGraph 逐行搬迁，
// 导出包装见 pipeline.go 的 DisplayGraphData）
func displayGraph(uuid string) (string, error) {
	driver := global.DB

	// 创建与目标数据库的会话
	sessionConfig := neo4j.SessionConfig{
		AccessMode:   neo4j.AccessModeWrite,
		DatabaseName: "neo4j", // 指定要连接的数据库
	}

	// 创建与目标数据库的会话（v5驱动：NewSession直接返回会话）
	session := driver.NewSession(sessionConfig)
	defer session.Close()

	// 查询节点和关系
	query := `
		MATCH (n)-[r]->(m)
		WHERE n.uuid = $uuid
		RETURN n, r, m
	`

	// 执行查询
	result, err := session.Run(query, map[string]interface{}{
		"uuid": uuid, // 使用传入的 UUID
	})
	if err != nil {
		slog.Error("图谱查询失败", "uuid", uuid, "error", err)
	} else {
		slog.Debug("图谱查询成功", "uuid", uuid)
	}

	// 处理result格式
	// 初始化结果结构体
	var graph Graph
	graph.Categories = []Category{{Name: "Entity"}} // 定义 categories 数组
	nodeNames := make(map[string]bool)

	// 处理查询结果
	for result.Next() {
		record := result.Record()

		// 提取节点信息（v5驱动：Record.Values + dbtype类型）
		nNode1 := record.Values[0].(dbtype.Node)
		relationship := record.Values[1].(dbtype.Relationship)
		nNode2 := record.Values[2].(dbtype.Node)

		// 将节点名称添加到 nodes 列表中（去重处理）
		headEntityName := nNode1.Props["name"].(string)
		if _, exists := nodeNames[headEntityName]; !exists {
			nodeNames[headEntityName] = true
			graph.Nodes = append(graph.Nodes, Node{
				Name:      headEntityName,
				SymolSize: 50,
				Category:  0,
			})
		}

		tailEntityName := nNode2.Props["name"].(string)
		if _, exists := nodeNames[tailEntityName]; !exists {
			nodeNames[tailEntityName] = true
			graph.Nodes = append(graph.Nodes, Node{
				Name:      tailEntityName,
				SymolSize: 50,
				Category:  0,
			})
		}

		// 添加关系到 links 列表（v5驱动：Relationship.Type为字段）
		LinkName := relationship.Type

		graph.Links = append(graph.Links, Link{
			Source: headEntityName,
			Target: tailEntityName,
			Value:  LinkName,
		})
	}

	if err := result.Err(); err != nil {
		return "", fmt.Errorf("Result error: %v", err)
	}

	// 将结果转换为 JSON 格式
	resultJSON, err := json.MarshalIndent(graph, "", "  ")
	if err != nil {
		return "", fmt.Errorf("Error marshaling result: %v", err)
	}

	return string(resultJSON), nil
}

// GetProjectRoot 返回项目根目录（包含 go.mod 的目录）
// 从当前工作目录逐级向上查找，确保无论从何处启动服务都能定位到项目根
// （原 api/model_api getProjectRoot 逐行搬迁；GetPdf 文件保存段留在 handler 层，故导出）
func GetProjectRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
