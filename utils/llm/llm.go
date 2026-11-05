package llm

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/schema"
)

// GLMBaseURL 默认 GLM OpenAI 兼容 BaseURL
// （go-openai 会自动拼接 /chat/completions 后缀形成完整端点）
const GLMBaseURL = "https://open.bigmodel.cn/api/paas/v4"

// 默认抽取调用超时与模型名（可通过环境变量覆盖：GLM_TIMEOUT / GLM_MODEL）
const (
	// 抽取比验证归一更慢，默认 60s；超时后调用方按各自降级语义处理
	defaultTimeout = 60 * time.Second
	defaultModel   = "glm-4-flash" // 当前唯一免费可用；如需切换在 .env 配 GLM_MODEL
)

// GLMTimeout 抽取调用超时（环境变量GLM_TIMEOUT，单位秒，默认60s）
func GLMTimeout() time.Duration {
	if v := os.Getenv("GLM_TIMEOUT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return time.Duration(n) * time.Second
		}
	}
	return defaultTimeout
}

// modelName 模型名（环境变量GLM_MODEL覆盖，默认defaultModel）
// 抽取/验证共用同一模型配置，收敛于此避免多处重复读取
func modelName() string {
	if v := os.Getenv("GLM_MODEL"); v != "" {
		return v
	}
	return defaultModel
}

// Chat 单轮 GLM 对话：prompt → 模型文本输出。项目内 chat 调用的统一入口（REQ-006）。
// 超时/调用失败返回 error，降级行为由调用方决定（抽取→空串跳过该块；验证→候选全保留）。
func Chat(ctx context.Context, prompt string, timeout time.Duration) (string, error) {
	return ChatWithURL(ctx, GLMBaseURL, prompt, timeout)
}

// ChatWithURL 指定 OpenAI 兼容 BaseURL 的单轮对话（eino ChatModel 标准组件）。
// baseURL 兼容完整端点写法（自动去除 /chat/completions 后缀），
// 以延续 use_model.go llmURL / verify.go VerifyURL 包变量的"完整端点"测试注入语义。
// 组件每次调用时构造：与旧实现"每次读环境变量"的运行时可配置语义一致。
func ChatWithURL(ctx context.Context, baseURL, prompt string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	endpoint := strings.TrimSuffix(strings.TrimRight(baseURL, "/"), "/chat/completions")
	cm, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		BaseURL: endpoint,
		APIKey:  os.Getenv("GLM_API_KEY"),
		Model:   modelName(),
	})
	if err != nil {
		return "", fmt.Errorf("构造ChatModel失败: %w", err)
	}

	resp, err := cm.Generate(ctx, []*schema.Message{{Role: schema.User, Content: prompt}})
	if err != nil {
		return "", fmt.Errorf("调用LLM失败: %w", err)
	}
	return resp.Content, nil
}
