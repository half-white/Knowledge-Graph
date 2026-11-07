// Package dao 图谱数据访问层（REQ-008 分层重构）：
// Neo4j/MySQL 的直接读写，函数体自 api/model_api/use_model.go 逐行搬迁（仅导出改名），
// 不含业务编排；编排逻辑见 internal/service。
package dao

import (
	"SSE/global"
	"SSE/models"
	"SSE/utils/metrics"
	"SSE/utils/rag"
	"SSE/utils/triplet"
	"log/slog"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// SaveToNeo4j 向Neo4j中插入节点（原 api/model_api saveToNeo4j 逐行搬迁）
func SaveToNeo4j(triplets []triplet.Triplet, id string, trace *metrics.Trace) {
	driver := global.DB

	// 创建与目标数据库的会话
	sessionConfig := neo4j.SessionConfig{
		AccessMode:   neo4j.AccessModeWrite,
		DatabaseName: "neo4j", // 指定要连接的数据库
	}

	// 创建与目标数据库的会话（v5驱动：NewSession直接返回会话）
	session := driver.NewSession(sessionConfig)
	defer session.Close()

	// 在业务层面确保操作的原子性情况下，选择不开启事务
	// 循环处理每个三元组
	for _, record := range triplets {

		headEntity := record.Head
		relationship := record.Relation
		tailEntity := record.Tail

		// 创建节点并添加关系的查询语句（关系类型用反引号包裹，
		// 避免特殊字符导致 Cypher 语法错误、三元组静默丢失）
		query := `
		MERGE (e1:Entity {name:$headEntity,uuid:$id})
		MERGE (e2:Entity {name:$tailEntity,uuid:$id})
		MERGE (e1)-[:` + "`" + relationship + "`" + `]->(e2)
		`

		// 执行查询
		_, err := session.Run(query, map[string]interface{}{
			"headEntity":   headEntity,
			"tailEntity":   tailEntity,
			"relationship": relationship,
			"id":           id,
		})
		if err != nil {
			slog.Error("三元组写入Neo4j失败", "head", headEntity, "relationship", relationship, "tail", tailEntity, "uuid", id, "error", err)
			continue // 出错时继续处理下一个三元组
		} else {
			slog.Debug("三元组写入成功", "head", headEntity, "relationship", relationship, "tail", tailEntity)
		}

		// RAG：原子项（头实体/关系/尾实体）保留判定与新项向量入库（embedding不可用时自动降级跳过）
		rag.SaveIfNew(driver, headEntity, relationship, tailEntity, id, trace)
	}

	slog.Info("三元组全部处理完成", "count", len(triplets), "uuid", id)
}

// SaveGraph 自动保存图谱（原 api/model_api saveGraph 逐行搬迁）
func SaveGraph(uuid string) {
	// 生成图谱名称
	currentDate := time.Now().Format("2006-01-02")
	var title string
	title = currentDate + "知识图谱"

	// 数据库保存逻辑
	mysql := global.Mysql

	err := mysql.Create(&models.GraphModel{
		Title: title,
		UUID:  uuid,
	}).Error
	if err != nil {
		slog.Error("图谱保存到数据库失败", "uuid", uuid, "title", title, "error", err)
		return
	}
	return
}

// DeleteGraphNodes 依据uuid删除Neo4j数据库中的图谱节点，删除成功返回true
// （原 DeleteGraph handler 内联 Neo4j 段逐行搬迁；失败仅记录日志，与原行为一致）
func DeleteGraphNodes(uuid string) bool {
	driver := global.DB
	sessionConfig := neo4j.SessionConfig{
		AccessMode:   neo4j.AccessModeWrite,
		DatabaseName: "neo4j", // 指定要连接的数据库
	}

	// 创建与目标数据库的会话（v5驱动：NewSession直接返回会话）
	session := driver.NewSession(sessionConfig)
	defer session.Close()

	// 查询节点和关系
	query := `
		MATCH (n {uuid: $uuid})
		DETACH DELETE n
	`
	// 执行删除
	_, err := session.Run(query, map[string]interface{}{
		"uuid": uuid, // 使用传入的 UUID
	})
	if err != nil {
		slog.Error("删除图谱节点失败", "uuid", uuid, "error", err)
		return false
	}
	return true
}

// FindAndDeleteGraph 依据uuid删除Mysql数据库中的图谱记录，
// 找到并删除返回true；未找到记录时仅告警并返回false（原 DeleteGraph handler 内联 MySQL 段逐行搬迁）
func FindAndDeleteGraph(uuid string) bool {
	// 依据uuid删除Mysql数据库信息
	mysql := global.Mysql
	var graph models.GraphModel
	count := mysql.Debug().Select("uuid").Find(&graph, uuid).RowsAffected
	if count == 0 {
		slog.Warn("未找到待删除的图谱", "uuid", uuid)
		return false
	}
	global.Mysql.Delete(&graph)
	return true
}
