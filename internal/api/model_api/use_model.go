// Package model_api 图谱接口层（REQ-008 分层重构）：
// 仅保留 gin handler（参数绑定、调用 service、组装响应、swag 注解），
// 业务逻辑已下沉 internal/service，数据访问已下沉 internal/dao。
package model_api

import (
	"SSE/internal/service"
	"SSE/models"
	"SSE/response"
	"SSE/utils/metrics"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

// 定义需要输入的请求数据
type UseModelRequest struct {
	Title string `json:"title" binding:"required" msg:"请输入内容"`
}

// UseModel 输入文本，抽取三元组并生成知识图谱
// @Summary      文本生成图谱
// @Description  文本按语义分块并发抽取三元组（LLM），验证归一去重后写入 Neo4j，返回 ECharts 图谱数据
// @Description  data 为图谱 JSON 的字符串形式（结构 {nodes,links,categories}，前端 JSON.parse 后渲染），msg 为三元组抽取文本明细
// @Tags         图谱生成
// @Accept       json
// @Produce      json
// @Param        request body UseModelRequest true "待抽取文本"
// @Success      200 {object} response.Response{data=string,msg=string} "code=0 成功；code=7 失败"
// @Router       /api/model [post]
func (ModelApi) UseModel(c *gin.Context) {
	//向模型中传递参数的逻辑
	var cr UseModelRequest
	err := c.ShouldBindJSON(&cr)
	if err != nil {
		response.FailWithError(err, &cr, c)
		return
	}

	// 性能监控：创建请求级追踪器，函数结束时自动汇总输出（REQ-003）
	trace := metrics.StartTrace("text")
	defer trace.Finish()

	// 业务逻辑下沉 service 层（REQ-008 分层重构，行为不变）
	result, text := service.UseModel(cr.Title, trace)

	// response.OkWithMessage(text, c)
	response.Ok(result, text, c)
}

// GraphList 展示知识图谱列表
// 展示列表需要前端传入正确的表格格式，否则无法查询到结果且没有报错
func (ModelApi) GraphList(c *gin.Context) {
	var cr models.PageInfo
	err := c.ShouldBindJSON(&cr)
	if err != nil {
		response.FailWithCode(response.ArgumentError, c)
		return
	}

	list, count, err := service.GraphList(cr)
	if err != nil {
		response.FailWithMessage("查询图谱失败！", c)
		return
	}

	response.OkWithList(list, count, c)

	return
}

// DisplayGraph 展示知识图谱
func (ModelApi) DisplayGraph(c *gin.Context) {
	// 接收前端传递而来的uuid
	var cr models.QueryRequest
	err := c.ShouldBindJSON(&cr)
	if err != nil {
		response.FailWithCode(response.ArgumentError, c)
		return
	}

	result := service.DisplayGraphData(cr.UUID)

	// 将图谱json数据传递给前端
	response.Ok(result, "", c)
}

// DeleteGraph 删除知识图谱逻辑
func (ModelApi) DeleteGraph(c *gin.Context) {
	var cr models.RemoveRequest
	err := c.ShouldBindJSON(&cr)
	if err != nil {
		response.FailWithCode(response.ArgumentError, c)
		return
	}

	// 删除逻辑下沉 service 层；响应时序按原实现复现
	//（REQ-008 D6：Neo4j 成功响应一次、Mysql 成功再响应一次的双重响应行为保持不变）
	neo4jOK, mysqlFound := service.DeleteGraph(cr.UUID)
	if neo4jOK {
		response.OkWithMessage(fmt.Sprint("删除图谱节点成功"), c)
	}
	if !mysqlFound {
		return
	}
	response.OkWithMessage(fmt.Sprint("删除图谱成功"), c)

}

// GetPdf 接收前端传递过来的pdf并且传递给大模型识别
// @Summary      PDF 上传生成图谱
// @Description  上传 PDF，经 python OCR 提取文本后走与文本生成相同的抽取管线，返回 ECharts 图谱数据
// @Description  data/msg 含义与 /api/model 相同；上传与 OCR 环节失败时 code=7 并给出中文提示
// @Tags         图谱生成
// @Accept       multipart/form-data
// @Produce      json
// @Param        file formData file true "PDF 文件"
// @Success      200 {object} response.Response{data=string,msg=string} "code=0 成功；code=7 失败"
// @Router       /api/upload [post]
func (ModelApi) GetPdf(c *gin.Context) {
	// 性能监控：创建请求级追踪器，函数结束时自动汇总输出（REQ-003）
	trace := metrics.StartTrace("pdf")
	defer trace.Finish()

	// upload 保存文件路径：项目根目录下的 resource/doc 文件夹
	root := service.GetProjectRoot()
	upload := filepath.Join(root, "resource", "doc")

	// 若文件夹不存在则自动创建
	if err := os.MkdirAll(upload, os.ModePerm); err != nil {
		response.FailWithMessage("创建上传目录失败", c)
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		response.FailWithMessage("未找到文件", c)
		return
	}

	// 检查文件名是否为空
	if file.Filename == "" {
		response.FailWithMessage("文件名为空", c)
		return
	}
	slog.Info("接收到PDF上传", "filename", file.Filename)

	// 拼接目标文件路径
	savePath := filepath.Join(upload, file.Filename)

	// 保存文件到指定目录
	if err := c.SaveUploadedFile(file, savePath); err != nil {
		slog.Error("保存上传文件失败", "filename", file.Filename, "save_path", savePath, "error", err)
		response.FailWithMessage("文件保存失败", c)
		return
	}

	// OCR识别与抽取管线下沉 service 层（REQ-008 分层重构，行为不变；
	// 含 OCR 失败日志、抽取、入库、图谱查询、图谱记录保存与临时 PDF 清理）
	result, text := service.ProcessPdf(savePath, file.Filename, trace)

	// response.OkWithMessage(text, c)
	response.Ok(result, text, c)
}

// 保存三元组到 CSV 文件
// func saveToCSV(filename string, triplets [][]string) error {
// 	// 检测文件是否存在
// 	var file *os.File
// 	var err error
// 	if _, err = os.Stat(filename); os.IsNotExist(err) {
// 		// 文件不存在，创建文件
// 		file, err = os.Create(filename)
// 		if err != nil {
// 			return err
// 		}
// 		// 写入 UTF-8 BOM 和表头
// 		file.WriteString("\xEF\xBB\xBF")
// 		writer := csv.NewWriter(file)
// 		defer writer.Flush()
// 		writer.Write([]string{"头实体", "关系", "尾实体"})
// 	} else {
// 		// 文件存在，打开文件以追加模式
// 		file, err = os.OpenFile(filename, os.O_APPEND|os.O_WRONLY, os.ModeAppend)
// 		if err != nil {
// 			return err
// 		}
// 	}
// 	defer file.Close()

// 	// 创建 CSV writer 并写入数据
// 	writer := csv.NewWriter(file)
// 	defer writer.Flush()

// 	// 写入三元组数据
// 	for _, triplet := range triplets {
// 		if err := writer.Write(triplet); err != nil {
// 			return err
// 		}
// 	}
// 	return nil
// }
