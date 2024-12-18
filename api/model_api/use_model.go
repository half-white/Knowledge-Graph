package model_api

import (
	"SSE/common"
	"SSE/global"
	"SSE/models"
	"SSE/res"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/neo4j/neo4j-go-driver/neo4j"
)

// 定义需要输入的请求数据
type UseModelRequest struct {
	Title string `json:"title" binding:"required" msg:"请输入内容"`
}

func (ModelApi) UseModel(c *gin.Context) {
	//向模型中传递参数的逻辑
	var cr UseModelRequest
	err := c.ShouldBindJSON(&cr)
	if err != nil {
		res.FailWithError(err, &cr, c)
		return
	}
	// 将文本拆解为较短片段
	parts := splitContent(cr.Title)

	// 利用go多线程进行大语言模型调用逻辑
	// 创建一个 slice 存储返回的结果
	fmt.Printf("文本被拆解成 %d 块 \n", len(parts))
	results := make([]string, len(parts))
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
			result := TypeInModel(part)

			// 使用 Mutex 保护对共享资源的操作
			mu.Lock()
			results[i] = result
			mu.Unlock()

		}(i, part)
	}
	wg.Wait()

	text := ""
	for _, result := range results {
		text += result + "\n"
	}

	// 提取三元组，形成唯一的uuid
	triplets, uuid := ExtractTriplets(text)

	// 将三元组插入Neo4j数据库，实体使用统一的uuid进行标记
	saveToNeo4j(triplets, uuid)

	// 将neo4j数据库中的相关数据展示成图谱
	result, _ := displayGraph(uuid)
	// fmt.Println(string(result))
	fmt.Printf("Result Type: %T\n", result)

	// 自动保留每次生成的知识图谱，以便管理
	saveGraph(uuid)

	// res.OkWithMessage(text, c)
	res.Ok(result, text, c)
}

// 获取文心一言大模型token
func get_access_token() string {
	// 构建请求URL
	rawURL := "https://aip.baidubce.com/oauth/2.0/token"
	params := url.Values{}
	params.Add("grant_type", "client_credentials")
	params.Add("client_id", "bSDjp8L2eCEiS3aa8CMU3jQx")             // 请确保使用你的实际Client ID
	params.Add("client_secret", "72Pdlol1fdKVNczdnRIhZxNJFkjyfi0v") // 请确保使用你的实际Client Secret
	encodedParams := params.Encode()
	fullURL := rawURL + "?" + encodedParams

	// 创建HTTP请求
	resp, err := http.Get(fullURL)
	if err != nil {
		fmt.Println("Error making GET request:", err)
		return "error"
	}
	defer resp.Body.Close()

	// 读取响应体
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response body:", err)
		return "error"
	}

	// 解析JSON响应
	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	if err != nil {
		fmt.Println("Error unmarshaling JSON:", err)
		return "error"
	}

	// 获取access_token
	accessToken, ok := result["access_token"].(string)
	if !ok {
		fmt.Println("Error retrieving access_token")
		return "error"
	}

	return accessToken
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type RequestData struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature"`
	TopP        float64   `json:"top_p"`
	MaxTokens   int       `json:"max_tokens"`
	Stream      bool      `json:"stream"`
}

type ResponseData struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func TypeInModel(content string) string {
	// // 文心一言http调用逻辑
	// // 获取access_token（这里应该处理错误）
	// accessToken := get_access_token()
	// if accessToken == "error" {
	// 	fmt.Println("Error getting access_token")
	// 	return ""
	// }

	// // 构建请求URL
	// url := "https://aip.baidubce.com/rpc/2.0/ai_custom/v1/wenxinworkshop/chat/ernie_speed?access_token=" + accessToken

	// // 准备JSON格式的payload
	// payload := map[string]interface{}{
	// 	"messages": []map[string]interface{}{
	// 		{
	// 			"role":    "user",
	// 			"content": content + "帮我把上面这段文字的关键内容提炼成多个三元组，输出格式为（头实体,关系,尾实体），不要有任何额外文字，前后招呼语也去除",
	// 		},
	// 	},
	// }

	// // 将payload编码为JSON格式的字节切片
	// jsonPayload, _ := json.Marshal(payload)

	// // 创建HTTP请求
	// req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonPayload))

	// // 设置请求头
	// req.Header.Set("Content-Type", "application/json")

	// // 发送请求并获取响应
	// client := &http.Client{}
	// resp, _ := client.Do(req)
	// defer resp.Body.Close()

	// // 读取响应体
	// body, _ := ioutil.ReadAll(resp.Body)

	// // 解析JSON响应
	// var result map[string]interface{}
	// _ = json.Unmarshal(body, &result)

	// return result["result"].(string)

	// // 本地GLM3处理逻辑
	// prompt := "以下是需要处理的文本内容，请筛选出其中比较重要的知识转化成三元组，输出成（头实体，关系，尾实体）的格式，不要包含其他文字，输出前请严格遵守要求。 \n"
	// fullPrompt := prompt + content

	// url := "http://172.16.20.10:8006/v1/chat/completions"
	// // 构造请求数据
	// payload := RequestData{
	// 	Model:       "chatglm3-6b",
	// 	Messages:    []Message{{Role: "user", Content: fullPrompt}},
	// 	Temperature: 0.7,
	// 	TopP:        0.9,
	// 	MaxTokens:   2000,
	// 	Stream:      false,
	// }
	// // 将payload编码为JSON格式的字节切片
	// jsonPayload, err := json.Marshal(payload)
	// if err != nil {
	// 	log.Fatalf("Error marshaling payload: %v", err)
	// }

	// // 创建HTTP请求
	// req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonPayload))
	// if err != nil {
	// 	log.Fatalf("Error creating HTTP request: %v", err)
	// }

	// // 设置请求头
	// req.Header.Set("Content-Type", "application/json")

	// // 发送请求并获取响应
	// client := &http.Client{}
	// resp, err := client.Do(req)
	// if err != nil {
	// 	log.Fatalf("Error making HTTP request: %v", err)
	// }
	// defer resp.Body.Close()

	// if resp.StatusCode != http.StatusOK {
	// 	log.Printf("Error: Status Code %d", resp.StatusCode)
	// 	return ""
	// }

	// // 读取响应体
	// responseBody, err := ioutil.ReadAll(resp.Body)
	// if err != nil {
	// 	log.Fatalf("Error reading response body: %v", err)
	// }

	// // 打印返回数据调试
	// fmt.Println("Response Body:", string(responseBody))

	// var responseData ResponseData
	// if err := json.Unmarshal(responseBody, &responseData); err != nil {
	// 	log.Printf("Error unmarshaling response: %v", err)
	// 	return ""
	// }

	// if len(responseData.Choices) == 0 {
	// 	log.Println("Error: Choices is empty")
	// 	return ""
	// }

	// responseText := responseData.Choices[0].Message.Content
	// return responseText

	// GLM4 http调用逻辑
	apiKey := "a7e362b275e0d1d93693e95b69cb1fba.E0wWKXSyGxX94Hde"
	prompt := "以下是需要处理的文本内容，请筛选出其中你认为最重要的知识点,将有用的知识点转化成三元组,按照'主','谓','宾'的格式(没有则依据上下文补全),全部严格输出成（头实体，关系，尾实体）的格式，只输出最重要的8个三元组,一个一行,不要包含序号和任何其它文字,输出前请严格遵守要求。 \n"
	url := "https://open.bigmodel.cn/api/paas/v4/chat/completions"

	payload := map[string]interface{}{
		"model": "glm-4-flash",
		"messages": []map[string]interface{}{
			{
				"role":    "user",
				"content": prompt + content,
			},
		},
	}
	// 将payload编码为JSON格式的字节切片
	jsonPayload, _ := json.Marshal(payload)

	// 创建HTTP请求
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonPayload))

	// 设置请求头
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	// 发送请求并获取响应
	client := &http.Client{}
	resp, _ := client.Do(req)
	defer resp.Body.Close()

	// 读取响应体
	responseBody, _ := ioutil.ReadAll(resp.Body)

	// 打印返回数据调试
	// fmt.Println("Response Body:", string(responseBody))

	var responseData ResponseData
	if err := json.Unmarshal(responseBody, &responseData); err != nil {
		log.Printf("Error unmarshaling response: %v", err)
		return ""
	}

	if len(responseData.Choices) == 0 {
		log.Println("Error: Choices is empty")
		return ""
	}

	responseText := responseData.Choices[0].Message.Content
	return responseText

}

// 将文本内容中的三元组信息匹配到三元组形式的格式中
func ExtractTriplets(text string) ([][]string, string) {
	// 匹配括号中的三元组
	re := regexp.MustCompile(`[（(]([^，,]+)[，,]\s*([^，,]+)[，,]\s*([^)）]+)[）)]`)
	matches := re.FindAllStringSubmatch(text, -1)

	var triplets [][]string
	for _, match := range matches {
		// match[1], match[2], match[3] 分别是头实体、关系、尾实体
		triplets = append(triplets, []string{strings.TrimSpace(match[1]), strings.TrimSpace(match[2]), strings.TrimSpace(match[3])})
	}

	// 生成唯一标识的uuid（使用当前时间戳 + UUID）
	ID := fmt.Sprintf("%d-%s", time.Now().Unix(), uuid.New().String())
	fmt.Println(ID)

	return triplets, ID
}

// 向Neo4j中插入节点
func saveToNeo4j(triplets [][]string, id string) {
	driver := global.DB

	// 创建与目标数据库的会话
	sessionConfig := neo4j.SessionConfig{
		AccessMode:   neo4j.AccessModeWrite,
		DatabaseName: "neo4j", // 指定要连接的数据库
	}

	session, err := driver.NewSession(sessionConfig)
	if err != nil {
		log.Printf("Failed to create session: %v \n", err)
		return
	}
	defer session.Close()

	// 在业务层面确保操作的原子性情况下，选择不开启事务
	// 循环处理每个三元组
	for _, record := range triplets {

		// 假设每一行数据是（实体1，关系，实体2）
		headEntity := record[0]
		relationship := record[1]
		tailEntity := record[2]

		// 创建节点并添加关系的查询语句
		query := `
		MERGE (e1:Entity {name:$headEntity,uuid:$id})
		MERGE (e2:Entity {name:$tailEntity,uuid:$id})
		MERGE (e1)-[:` + relationship + `]->(e2)
		`

		// 执行查询
		_, err := session.Run(query, map[string]interface{}{
			"headEntity":   headEntity,
			"tailEntity":   tailEntity,
			"relationship": relationship,
			"id":           id,
		})
		if err != nil {
			log.Printf("Failed to run query for triple (%s,%s,%s): %v \n", headEntity, relationship, tailEntity, err)
			continue // 出错时继续处理下一个三元组
		} else {
			fmt.Printf("Inserted triple (%s,%s,%s) \n", headEntity, relationship, tailEntity)
		}
	}

	fmt.Printf("All %d triplets have been processed.", len(triplets))
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

	session, err := driver.NewSession(sessionConfig)
	if err != nil {
		log.Printf("Failed to create session: %v \n", err)
		return "", err
	}
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
		log.Printf("Failed to run query: %v \n", err)
	} else {
		fmt.Println("查询成功！")
	}

	// 处理result格式
	// 初始化结果结构体
	var graph Graph
	graph.Categories = []Category{{Name: "Entity"}} // 定义 categories 数组
	nodeNames := make(map[string]bool)

	// 处理查询结果
	for result.Next() {
		record := result.Record()

		// 提取节点信息
		nNode1 := record.GetByIndex(0).(neo4j.Node)
		relationship := record.GetByIndex(1).(neo4j.Relationship)
		nNode2 := record.GetByIndex(2).(neo4j.Node)

		// 将节点名称添加到 nodes 列表中（去重处理）
		headEntityName := nNode1.Props()["name"].(string)
		if _, exists := nodeNames[headEntityName]; !exists {
			nodeNames[headEntityName] = true
			graph.Nodes = append(graph.Nodes, Node{
				Name:      headEntityName,
				SymolSize: 50,
				Category:  0,
			})
		}

		tailEntityName := nNode2.Props()["name"].(string)
		if _, exists := nodeNames[tailEntityName]; !exists {
			nodeNames[tailEntityName] = true
			graph.Nodes = append(graph.Nodes, Node{
				Name:      tailEntityName,
				SymolSize: 50,
				Category:  0,
			})
		}

		// 添加关系到 links 列表
		LinkName := relationship.Type()

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
		fmt.Println("图谱保存失败！", err)
		return
	}
	return
}

// splitContent 将文本拆解为较短字段
func splitContent(content string) []string {
	maxLength := 1000
	var parts []string
	runes := []rune(content) // 转换为字符切片，避免多字节字符问题

	for len(runes) > maxLength {
		splitIndex := maxLength
		// 找到分割范围内的最后一个空格或标点符号
		for i := maxLength - 1; i >= 0; i-- {
			if runes[i] == ' ' || runes[i] == '，' || runes[i] == '。' || runes[i] == '；' {
				splitIndex = i + 1 // 包括分隔符
				break
			}
		}
		parts = append(parts, string(runes[:splitIndex]))
		runes = runes[splitIndex:]
	}
	// 添加最后一部分
	if len(runes) > 0 {
		parts = append(parts, string(runes))
	}
	return parts
}

// GraphList 展示知识图谱列表
// 展示列表需要前端传入正确的表格格式，否则无法查询到结果且没有报错
func (ModelApi) GraphList(c *gin.Context) {
	var cr models.PageInfo
	err := c.ShouldBindJSON(&cr)
	if err != nil {
		res.FailWithCode(res.ArgumentError, c)
		return
	}

	list, count, err := common.ComList(models.GraphModel{}, common.Option{
		PageInfo: cr,
		Debug:    true,
	})
	if err != nil {
		res.FailWithMessage("查询图谱失败！", c)
		return
	}

	res.OkWithList(list, count, c)

	return
}

// DisplayGraph 展示知识图谱
func (ModelApi) DisplayGraph(c *gin.Context) {
	// 接收前端传递而来的uuid
	var cr models.QueryRequest
	err := c.ShouldBindJSON(&cr)
	if err != nil {
		res.FailWithCode(res.ArgumentError, c)
		return
	}

	result, _ := displayGraph(cr.UUID)

	// 将图谱json数据传递给前端
	res.Ok(result, "", c)
}

// DeleteGraph 删除知识图谱逻辑
func (ModelApi) DeleteGraph(c *gin.Context) {
	var cr models.RemoveRequest
	err := c.ShouldBindJSON(&cr)
	if err != nil {
		res.FailWithCode(res.ArgumentError, c)
		return
	}

	// 依据uuid删除Neo4j数据库信息
	driver := global.DB
	sessionConfig := neo4j.SessionConfig{
		AccessMode:   neo4j.AccessModeWrite,
		DatabaseName: "neo4j", // 指定要连接的数据库
	}

	session, err := driver.NewSession(sessionConfig)
	if err != nil {
		res.FailWithMessage("创建 Neo4j 会话失败", c)
		return
	}
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
		log.Printf("Failed to run query: %v \n", err)
	} else {
		res.OkWithMessage(fmt.Sprint("删除图谱节点成功"), c)
	}

	// 依据uuid删除Mysql数据库信息
	mysql := global.Mysql
	var graph models.GraphModel
	count := mysql.Debug().Select("uuid").Find(&graph, cr.UUID).RowsAffected
	if count == 0 {
		fmt.Println("没有找到该图谱")
		return
	}
	global.Mysql.Delete(&graph)
	res.OkWithMessage(fmt.Sprint("删除图谱成功"), c)

}

// GetPdf 接收前端传递过来的pdf并且传递给大模型识别
func (ModelApi) GetPdf(c *gin.Context) {
	// upload 保存文件路径
	upload := "C:/Users/xieenping/Desktop/实习工作/SSE/utils/"
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
	fmt.Println(file.Filename)

	// 拼接目标文件路径
	savePath := filepath.Join(upload, file.Filename)

	// 保存文件到指定目录
	if err := c.SaveUploadedFile(file, savePath); err != nil {
		fmt.Println("文件保存失败")
		return
	}

	// 调用python代码识别pdf文本
	cmd := exec.Command("python", "C:/Users/xieenping/Desktop/实习工作/SSE/utils/ocr.py", file.Filename)
	title, err := cmd.Output()
	if err != nil {
		fmt.Println("解析文本失败")
	}
	// fmt.Println(string(title))

	// 将文本拆解为较短片段
	parts := splitContent(string(title))

	// 利用go多线程进行大语言模型调用逻辑
	// 创建一个 slice 存储返回的结果
	fmt.Printf("文本被拆解成 %d 块 \n", len(parts))
	results := make([]string, len(parts))
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
			result := TypeInModel(part)

			// 使用 Mutex 保护对共享资源的操作
			mu.Lock()
			results[i] = result
			mu.Unlock()

		}(i, part)
	}
	wg.Wait()

	text := ""
	for _, result := range results {
		text += result + "\n"
	}

	// 提取三元组，形成唯一的uuid
	triplets, uuid := ExtractTriplets(text)

	// 将三元组插入Neo4j数据库，实体使用统一的uuid进行标记
	saveToNeo4j(triplets, uuid)

	// 将neo4j数据库中的相关数据展示成图谱
	result, _ := displayGraph(uuid)
	// fmt.Println(string(result))
	// fmt.Printf("Result Type: %T\n", result)

	// 自动保留每次生成的知识图谱，以便管理
	saveGraph(uuid)

	// 删除本地pdf
	err = os.Remove(savePath)
	if err != nil {
		fmt.Printf("文件删除失败: %v \n", err)
	}

	// res.OkWithMessage(text, c)
	res.Ok(result, text, c)
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
