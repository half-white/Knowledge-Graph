// Package service 图谱业务层（REQ-008 分层重构）：
// 本文件为业务编排方法（包级导出函数；v1.1 取消 interface 抽象，
// 新增 svc 方法 = 新增一个包级函数 + handler 一行调用），
// 函数体自 api/model_api/use_model.go 对应 handler 的内联业务段逐行搬迁（含原注释）。
package service

import (
	"SSE/common"
	"SSE/internal/dao"
	"SSE/models"
	"SSE/utils/metrics"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// UseModel 文本抽取三元组并生成知识图谱
//（原 UseModel handler 内联业务逐行搬迁；trace 创建与 Finish 留在 handler）
func UseModel(title string, trace *metrics.Trace) (string, string) {
	trace.SetMeta("title_len", len([]rune(title)))

	// 将文本拆解为较短片段
	splitStart := time.Now()
	parts := splitContent(title)
	trace.Observe(metrics.StageSplit, time.Since(splitStart), false)
	trace.SetMeta("chunk_count", len(parts))

	// 分块抽取管道：并发抽取→解析→硬校验→验证归一→汇总去重（UseModel/GetPdf 共用）
	slog.Info("文本已拆解", "chunk_count", len(parts))
	triplets, text := runExtractionPipeline(parts, trace)

	// 形成唯一标识的uuid（使用当前时间戳 + UUID）
	ID := fmt.Sprintf("%d-%s", time.Now().Unix(), uuid.New().String())
	slog.Info("三元组抽取完成", "uuid", ID, "triplet_count", len(triplets))

	// 将三元组插入Neo4j数据库，实体使用统一的uuid进行标记
	storeStart := time.Now()
	dao.SaveToNeo4j(triplets, ID, trace)
	trace.Observe(metrics.StageGraphStore, time.Since(storeStart), false)

	// 将neo4j数据库中的相关数据展示成图谱
	queryStart := time.Now()
	result, _ := displayGraph(ID)
	trace.Observe(metrics.StageGraphQuery, time.Since(queryStart), result == "")
	// fmt.Println(string(result))
	slog.Debug("图谱展示结果类型", "type", fmt.Sprintf("%T", result))

	// 自动保留每次生成的知识图谱，以便管理
	dao.SaveGraph(ID)

	return result, text
}

// ProcessPdf OCR 提取 PDF 文本后走与文本生成相同的抽取管线
//（原 GetPdf handler 内联业务逐行搬迁；文件接收/保存/错误响应留在 handler）
func ProcessPdf(savePath, filename string, trace *metrics.Trace) (string, string) {
	// 调用python代码识别pdf文本
	// 注：与原实现一致，OCR 参数为文件名而非保存路径（REQ-008 D6 疑似行为不修正）；
	// 项目根目录按原逻辑重新定位（确定性结果，与 handler 侧取值一致）
	ocrStart := time.Now()
	root := GetProjectRoot()
	cmd := exec.Command("python", filepath.Join(root, "utils", "ocr.py"), filename)
	title, err := cmd.Output()
	trace.Observe(metrics.StageOCR, time.Since(ocrStart), err != nil)
	trace.SetMeta("filename", filename)
	if err != nil {
		slog.Error("OCR解析PDF文本失败", "filename", filename, "error", err)
	}
	// fmt.Println(string(title))

	// 将文本拆解为较短片段
	splitStart := time.Now()
	parts := splitContent(string(title))
	trace.Observe(metrics.StageSplit, time.Since(splitStart), false)
	trace.SetMeta("chunk_count", len(parts))

	// 分块抽取管道：并发抽取→解析→硬校验→验证归一→汇总去重（UseModel/GetPdf 共用）
	slog.Info("PDF文本已拆解", "chunk_count", len(parts))
	triplets, text := runExtractionPipeline(parts, trace)

	// 形成唯一标识的uuid（使用当前时间戳 + UUID）
	ID := fmt.Sprintf("%d-%s", time.Now().Unix(), uuid.New().String())
	slog.Info("三元组抽取完成", "uuid", ID, "triplet_count", len(triplets))

	// 将三元组插入Neo4j数据库，实体使用统一的uuid进行标记
	storeStart := time.Now()
	dao.SaveToNeo4j(triplets, ID, trace)
	trace.Observe(metrics.StageGraphStore, time.Since(storeStart), false)

	// 将neo4j数据库中的相关数据展示成图谱
	queryStart := time.Now()
	result, _ := displayGraph(ID)
	trace.Observe(metrics.StageGraphQuery, time.Since(queryStart), result == "")
	// fmt.Println(string(result))
	// fmt.Printf("Result Type: %T\n", result)

	// 自动保留每次生成的知识图谱，以便管理
	dao.SaveGraph(ID)

	// 删除本地pdf
	err = os.Remove(savePath)
	if err != nil {
		slog.Warn("删除临时PDF失败", "save_path", savePath, "error", err)
	}

	return result, text
}

// GraphList 分页查询图谱列表
//（原 GraphList handler 内联业务逐行搬迁；common.ComList 通用查询层保持顶层不动）
func GraphList(pageInfo models.PageInfo) ([]models.GraphModel, int64, error) {
	return common.ComList(models.GraphModel{}, common.Option{
		PageInfo: pageInfo,
		Debug:    true,
	})
}

// DisplayGraphData 查询 uuid 对应的图谱数据
//（原 DisplayGraph handler 内联业务逐行搬迁；错误与原 `result, _ :=` 丢弃语义一致）
func DisplayGraphData(uuid string) string {
	result, _ := displayGraph(uuid)
	return result
}

// DeleteGraph 删除图谱：Neo4j 节点与 Mysql 记录
//（原 DeleteGraph handler 内联业务逐行搬迁；响应时序由 handler 复现——
// 原实现 Neo4j 成功响应一次、Mysql 成功再响应一次的双重响应行为保持不变，REQ-008 D6）
func DeleteGraph(uuid string) (neo4jOK bool, mysqlFound bool) {
	// 依据uuid删除Neo4j数据库信息
	neo4jOK = dao.DeleteGraphNodes(uuid)

	// 依据uuid删除Mysql数据库信息
	mysqlFound = dao.FindAndDeleteGraph(uuid)
	return neo4jOK, mysqlFound
}
