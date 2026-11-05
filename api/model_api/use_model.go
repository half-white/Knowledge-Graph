package model_api

import (
	"SSE/common"
	"SSE/global"
	"SSE/models"
	"SSE/response"
	"SSE/utils/llm"
	"SSE/utils/metrics"
	"SSE/utils/rag"
	"SSE/utils/triplet"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j/dbtype"
)

// llmURL 三元组抽取接口地址（完整端点，包级变量，测试可替换为 mock 地址；
// 实际请求经 llm.ChatWithURL 派生 OpenAI 兼容 BaseURL 走 eino ChatModel）
var llmURL = "https://open.bigmodel.cn/api/paas/v4/chat/completions"

// 定义需要输入的请求数据
type UseModelRequest struct {
	Title string `json:"title" binding:"required" msg:"请输入内容"`
}

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
	trace.SetMeta("title_len", len([]rune(cr.Title)))

	// 将文本拆解为较短片段
	splitStart := time.Now()
	parts := splitContent(cr.Title)
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
	saveToNeo4j(triplets, ID, trace)
	trace.Observe(metrics.StageGraphStore, time.Since(storeStart), false)

	// 将neo4j数据库中的相关数据展示成图谱
	queryStart := time.Now()
	result, _ := displayGraph(ID)
	trace.Observe(metrics.StageGraphQuery, time.Since(queryStart), result == "")
	// fmt.Println(string(result))
	slog.Debug("图谱展示结果类型", "type", fmt.Sprintf("%T", result))

	// 自动保留每次生成的知识图谱，以便管理
	saveGraph(ID)

	// response.OkWithMessage(text, c)
	response.Ok(result, text, c)
}

func TypeInModel(content string) string {
	prompt := `你是知识图谱构建专家。请从下面的文本中抽取知识三元组。

抽取规则：
1. 只抽取事实性知识点，忽略问候、评论、过渡句等非事实内容；
2. 头实体和尾实体使用文本中的规范名称，遇到代词（如"他"、"该公司"）必须还原为所指的具体实体；
3. 同一实体在不同句子中出现时使用完全相同的名称，不要有时写全称有时写简称；
4. 关系用一个简短的词或短语概括（如"创始人"、"位于"、"提出"）；
5. 实体和关系的名称中不要包含逗号；
6. 最多输出8个最重要的三元组，不足8个则有多少输出多少；
7. 头实体和尾实体必须是文本中出现的完整规范名称，不能是代词、截断短语或你自己缩写的简称。

输出格式（严格遵守，输出 JSON 数组，不要有任何额外文字、序号或代码块标记）：
[{"head":"头实体","relation":"关系","tail":"尾实体"}]

示例：
输入：乔布斯于1976年创立了苹果公司，该公司后来发布了风靡全球的iPhone。
输出：
[{"head":"乔布斯","relation":"创立","tail":"苹果公司"},{"head":"苹果公司","relation":"发布","tail":"iPhone"}]

以下是需要处理的文本内容：
`

	// RAG向量检索已移除：长文本与精简三元组的向量相似度对比意义不大，
	// 改为通过提示词工程提升抽取质量（实体规范化、few-shot示例等）

	// REQ-006：经 eino ChatModel 调用（utils/llm 统一封装）；
	// llmURL 保持完整端点语义（测试可替换为 mock 地址，由 ChatWithURL 派生 BaseURL）；
	// 超时默认 60s（GLM_TIMEOUT 可配），失败/超时返回空串 → 该块跳过（降级①）
	respText, err := llm.ChatWithURL(context.Background(), llmURL, prompt+content, llm.GLMTimeout())
	if err != nil {
		slog.Error("调用LLM接口失败", "error", err)
		return ""
	}
	return respText

}

// parseTriplets 解析 LLM 抽取输出为候选三元组：JSON 数组优先（容忍 markdown 围栏与前后杂文字），
// 失败降级正则匹配（头，关系，尾）自由文本格式（历史兼容路径）
func parseTriplets(text string) []triplet.Triplet {
	trimmed := strings.TrimSpace(text)
	if trimmed != "" {
		if items, err := parseJSONArray(trimmed); err == nil {
			return items
		}
	}

	// 正则降级：匹配括号中的三元组（match[1..3] 分别是头实体、关系、尾实体）
	re := regexp.MustCompile(`[（(]([^，,]+)[，,]\s*([^，,]+)[，,]\s*([^)）]+)[）)]`)
	matches := re.FindAllStringSubmatch(text, -1)
	var triplets []triplet.Triplet
	for _, match := range matches {
		triplets = append(triplets, triplet.Triplet{
			Head:     strings.TrimSpace(match[1]),
			Relation: strings.TrimSpace(match[2]),
			Tail:     strings.TrimSpace(match[3]),
		})
	}
	return triplets
}

// parseJSONArray 解析 JSON 数组形式的三元组（直接解析失败时截取首 [ 至末 ] 再解析）
func parseJSONArray(text string) ([]triplet.Triplet, error) {
	var items []triplet.Triplet
	if err := json.Unmarshal([]byte(text), &items); err != nil {
		start := strings.Index(text, "[")
		end := strings.LastIndex(text, "]")
		if start < 0 || end <= start {
			return nil, fmt.Errorf("未找到 JSON 数组")
		}
		if err := json.Unmarshal([]byte(text[start:end+1]), &items); err != nil {
			return nil, err
		}
	}
	return items, nil
}

// renderTriplets 将三元组集合渲染为（头，关系，尾）文本格式（前端展示用，与历史展示格式一致）
func renderTriplets(triplets []triplet.Triplet) string {
	var lines []string
	for _, t := range triplets {
		lines = append(lines, fmt.Sprintf("（%s，%s，%s）", t.Head, t.Relation, t.Tail))
	}
	return strings.Join(lines, "\n")
}

// chunkResult 单块抽取结果：候选/校验计数（漏斗用）+ 通过验证的三元组
type chunkResult struct {
	candidates int
	validated  int
	verified   []triplet.Triplet
	dropped    map[string]int
}

// runExtractionPipeline 分块抽取公共管道（UseModel/GetPdf 共用）：
// 并发抽取→解析→硬校验→验证归一→汇总去重。
// 每块在现有并发 goroutine 内顺序执行各环节；验证失败降级为该块候选全部保留（可用性优先）。
// 返回最终入库的三元组集合（规范名、跨块已去重）与前端展示文本，漏斗计数写入 trace。
func runExtractionPipeline(parts []string, trace *metrics.Trace) ([]triplet.Triplet, string) {
	results := make([]chunkResult, len(parts))
	ch := make(chan struct{}, 5) // 生成一个线程池

	var wg sync.WaitGroup
	var mu sync.Mutex

	for i, part := range parts {
		wg.Add(1)
		ch <- struct{}{} // 阻塞直到有空闲 Goroutine
		go func(i int, part string) {
			defer wg.Done()
			defer func() {
				<-ch // 释放go线程
			}()

			// 抽取（子环节计时：LLM）
			llmStart := time.Now()
			raw := TypeInModel(part)
			trace.Observe(metrics.StageLLM, time.Since(llmStart), raw == "")

			// 解析候选三元组 + 硬校验（子环节计时：triplet）
			parseStart := time.Now()
			candidates := parseTriplets(raw)
			validated, dropped := triplet.Validate(candidates)
			trace.Observe(metrics.StageTriplet, time.Since(parseStart), false)

			// 验证归一（子环节计时：verify）；失败降级为全部保留，不阻断主流程
			verifyStart := time.Now()
			verified, err := triplet.VerifyTriplets(part, validated)
			trace.Observe(metrics.StageVerify, time.Since(verifyStart), err != nil)
			if err != nil {
				slog.Warn("三元组验证失败，降级为全部保留", "chunk_index", i, "count", len(validated), "error", err)
				verified = validated
			}

			// 使用 Mutex 保护对共享资源的操作
			mu.Lock()
			results[i] = chunkResult{
				candidates: len(candidates),
				validated:  len(validated),
				verified:   verified,
				dropped:    dropped,
			}
			mu.Unlock()

		}(i, part)
	}
	wg.Wait()

	// 聚合漏斗计数
	candidateTotal, validatedTotal, verifiedTotal := 0, 0, 0
	droppedTotal := map[string]int{}
	var merged []triplet.Triplet
	for _, r := range results {
		candidateTotal += r.candidates
		validatedTotal += r.validated
		verifiedTotal += len(r.verified)
		for k, v := range r.dropped {
			droppedTotal[k] += v
		}
		merged = append(merged, r.verified...)
	}

	// 跨块精确去重：归一后同 head+relation+tail 仅保留一条
	// （图谱层 MERGE 本身幂等不重复画边，去重收益为省冗余写入与原子 embedding 调用）
	seen := make(map[string]bool)
	var final []triplet.Triplet
	for _, t := range merged {
		key := t.Head + "\x00" + t.Relation + "\x00" + t.Tail
		if seen[key] {
			continue
		}
		seen[key] = true
		final = append(final, t)
	}

	// 漏斗四级计数与丢弃分类写入监控（triplet_count 键名保留，兼容既有看板）
	trace.SetMeta("candidate_count", candidateTotal)
	trace.SetMeta("validated_count", validatedTotal)
	trace.SetMeta("verified_count", verifiedTotal)
	trace.SetMeta("dropped_empty", droppedTotal[triplet.DroppedEmpty])
	trace.SetMeta("dropped_len", droppedTotal[triplet.DroppedLen])
	trace.SetMeta("dropped_blacklist", droppedTotal[triplet.DroppedBlacklist])
	trace.SetMeta("dropped_format", droppedTotal[triplet.DroppedFormat])
	trace.SetMeta("triplet_count", len(final))

	return final, renderTriplets(final)
}

// 向Neo4j中插入节点
func saveToNeo4j(triplets []triplet.Triplet, id string, trace *metrics.Trace) {
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

// 从Neo4j数据库中搜索对应id的节点和关系信息
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

// saveGraph 自动保存图谱
func saveGraph(uuid string) {
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

// splitContent 将文本按语义边界拆解为约 maxLength 字符的片段。
// 切分优先级：段落（行）> 句子 > 硬截断，相邻语义单元贪心合并，
// 使每个片段尽量完整、长度贴近 maxLength，供下游模型逐块抽取。
func splitContent(content string) []string {
	const maxLength = 1000
	content = strings.TrimSpace(content)
	if content == "" {
		return nil
	}

	// 1. 以行为单位拆出语义原子（段落/标题/列表项），超长行退化为句子级切分
	var units []string
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if len([]rune(line)) <= maxLength {
			units = append(units, line)
			continue
		}
		units = append(units, splitBySentence(line, maxLength)...)
	}

	// 2. 贪心合并相邻语义单元，片段长度接近但不轻易超过 maxLength
	var parts []string
	var buf strings.Builder
	bufLen := 0
	flush := func() {
		if buf.Len() > 0 {
			parts = append(parts, strings.TrimSpace(buf.String()))
			buf.Reset()
			bufLen = 0
		}
	}
	for _, u := range units {
		uLen := len([]rune(u))
		if bufLen > 0 && bufLen+uLen+1 > maxLength {
			flush()
		}
		if bufLen > 0 {
			buf.WriteString("\n")
			bufLen++
		}
		buf.WriteString(u)
		bufLen += uLen
	}
	flush()
	return parts
}

// splitBySentence 将超长段落按句子边界切分（保留句末标点与后随引号/括号），
// 单句仍超长时按 maxLength 硬截断。
func splitBySentence(text string, maxLength int) []string {
	runes := []rune(text)

	// 先按句末标点切成句子
	var sentences []string
	start := 0
	for i := 0; i < len(runes); i++ {
		if isSentenceEnd(runes[i]) {
			// 吸收紧跟的结束引号/括号，避免把引号留在下一句开头
			for i+1 < len(runes) && strings.ContainsRune("”’」』）)\"", runes[i+1]) {
				i++
			}
			sentences = append(sentences, string(runes[start:i+1]))
			start = i + 1
		}
	}
	if start < len(runes) {
		sentences = append(sentences, string(runes[start:]))
	}

	// 句子贪心合并；单句超长时硬截断
	var units []string
	var buf strings.Builder
	bufLen := 0
	flush := func() {
		if buf.Len() > 0 {
			units = append(units, strings.TrimSpace(buf.String()))
			buf.Reset()
			bufLen = 0
		}
	}
	for _, s := range sentences {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		r := []rune(s)
		if len(r) > maxLength {
			flush()
			for len(r) > maxLength {
				units = append(units, string(r[:maxLength]))
				r = r[maxLength:]
			}
			s = strings.TrimSpace(string(r))
			if s == "" {
				continue
			}
		}
		if bufLen > 0 && bufLen+len([]rune(s)) > maxLength {
			flush()
		}
		buf.WriteString(s)
		bufLen += len([]rune(s))
	}
	flush()
	return units
}

// isSentenceEnd 判断是否为句末标点
func isSentenceEnd(r rune) bool {
	switch r {
	case '。', '！', '？', '；', '…', '!', '?', ';', '.':
		return true
	}
	return false
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

	list, count, err := common.ComList(models.GraphModel{}, common.Option{
		PageInfo: cr,
		Debug:    true,
	})
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

	result, _ := displayGraph(cr.UUID)

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

	// 依据uuid删除Neo4j数据库信息
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
	_, err = session.Run(query, map[string]interface{}{
		"uuid": cr.UUID, // 使用传入的 UUID
	})
	if err != nil {
		slog.Error("删除图谱节点失败", "uuid", cr.UUID, "error", err)
	} else {
		response.OkWithMessage(fmt.Sprint("删除图谱节点成功"), c)
	}

	// 依据uuid删除Mysql数据库信息
	mysql := global.Mysql
	var graph models.GraphModel
	count := mysql.Debug().Select("uuid").Find(&graph, cr.UUID).RowsAffected
	if count == 0 {
		slog.Warn("未找到待删除的图谱", "uuid", cr.UUID)
		return
	}
	global.Mysql.Delete(&graph)
	response.OkWithMessage(fmt.Sprint("删除图谱成功"), c)

}

// getProjectRoot 返回项目根目录（包含 go.mod 的目录）
// 从当前工作目录逐级向上查找，确保无论从何处启动服务都能定位到项目根
func getProjectRoot() string {
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

// GetPdf 接收前端传递过来的pdf并且传递给大模型识别
func (ModelApi) GetPdf(c *gin.Context) {
	// 性能监控：创建请求级追踪器，函数结束时自动汇总输出（REQ-003）
	trace := metrics.StartTrace("pdf")
	defer trace.Finish()

	// upload 保存文件路径：项目根目录下的 resource/doc 文件夹
	root := getProjectRoot()
	upload := filepath.Join(root, "resource", "doc")

	// 若文件夹不存在则自动创建
	if err := os.MkdirAll(upload, os.ModePerm); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "创建上传目录失败",
		})
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "未找到文件",
		})
		return
	}

	// 检查文件名是否为空
	if file.Filename == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "文件名为空",
		})
		return
	}
	slog.Info("接收到PDF上传", "filename", file.Filename)

	// 拼接目标文件路径
	savePath := filepath.Join(upload, file.Filename)

	// 保存文件到指定目录
	if err := c.SaveUploadedFile(file, savePath); err != nil {
		slog.Error("保存上传文件失败", "filename", file.Filename, "save_path", savePath, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "文件保存失败",
		})
		return
	}

	// 调用python代码识别pdf文本
	ocrStart := time.Now()
	cmd := exec.Command("python", filepath.Join(root, "utils", "ocr.py"), file.Filename)
	title, err := cmd.Output()
	trace.Observe(metrics.StageOCR, time.Since(ocrStart), err != nil)
	trace.SetMeta("filename", file.Filename)
	if err != nil {
		slog.Error("OCR解析PDF文本失败", "filename", file.Filename, "error", err)
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
	saveToNeo4j(triplets, ID, trace)
	trace.Observe(metrics.StageGraphStore, time.Since(storeStart), false)

	// 将neo4j数据库中的相关数据展示成图谱
	queryStart := time.Now()
	result, _ := displayGraph(ID)
	trace.Observe(metrics.StageGraphQuery, time.Since(queryStart), result == "")
	// fmt.Println(string(result))
	// fmt.Printf("Result Type: %T\n", result)

	// 自动保留每次生成的知识图谱，以便管理
	saveGraph(ID)

	// 删除本地pdf
	err = os.Remove(savePath)
	if err != nil {
		slog.Warn("删除临时PDF失败", "save_path", savePath, "error", err)
	}

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
