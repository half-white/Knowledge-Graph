package vector

import (
	"fmt"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// 向量配置常量
const (
	// 向量维度（bge-m3模型为1024维）
	VectorDim = 1024
	// 高价值关系节点标签
	TripletLabel = "Triplet"
	// 向量索引名称
	IndexName = "triplet_vector"
	// 向量索引创建语句
	createIndexQuery = "CREATE VECTOR INDEX " + IndexName + " IF NOT EXISTS FOR (n:" + TripletLabel + ") ON (n.vector) OPTIONS {indexConfig: {`vector.dimensions`: 1024, `vector.similarity_function`: 'cosine'}}"
)

// EnsureVectorIndex 创建Triplet节点向量索引（幂等）
func EnsureVectorIndex(driver neo4j.Driver) error {
	// 创建与目标数据库的会话
	session := driver.NewSession(neo4j.SessionConfig{
		AccessMode:   neo4j.AccessModeWrite,
		DatabaseName: "neo4j",
	})
	defer session.Close()

	// 执行索引创建
	_, err := session.Run(createIndexQuery, nil)
	return err
}

// SearchSimilar 向量相似度检索，返回相似三元组文本列表
func SearchSimilar(driver neo4j.Driver, vec []float32, topK int, threshold float64) ([]string, error) {
	// 创建与目标数据库的会话
	session := driver.NewSession(neo4j.SessionConfig{
		AccessMode:   neo4j.AccessModeRead,
		DatabaseName: "neo4j",
	})
	defer session.Close()

	// 使用原生向量索引查询最相似的三元组
	query := `
		CALL db.index.vector.queryNodes($indexName, $topK, $queryVector) YIELD node, score
		WHERE score >= $threshold
		RETURN node.text AS text, score
		ORDER BY score DESC
	`
	result, err := session.Run(query, map[string]interface{}{
		"indexName":   IndexName,
		"topK":        topK,
		"queryVector": vec,
		"threshold":   threshold,
	})
	if err != nil {
		return nil, err
	}

	// 收集检索结果
	var texts []string
	for result.Next() {
		text, ok := result.Record().Values[0].(string)
		if ok {
			texts = append(texts, text)
		}
	}
	if err := result.Err(); err != nil {
		return nil, err
	}

	return texts, nil
}

// SaveTriplet 保存高价值三元组节点及其向量
func SaveTriplet(driver neo4j.Driver, text, uuid string, vec []float32) error {
	// 创建与目标数据库的会话
	session := driver.NewSession(neo4j.SessionConfig{
		AccessMode:   neo4j.AccessModeWrite,
		DatabaseName: "neo4j",
	})
	defer session.Close()

	// 以文本为唯一键写入三元组节点
	query := `
		MERGE (n:Triplet {text: $text})
		SET n.uuid = $uuid, n.vector = $vector
	`
	_, err := session.Run(query, map[string]interface{}{
		"text":   text,
		"uuid":   uuid,
		"vector": vec,
	})
	if err != nil {
		return fmt.Errorf("保存三元组向量失败: %v", err)
	}

	return nil
}
