package standard_executor

import (
	"NucleusMem/pkg/configs"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/pingcap-incubator/tinykv/log"
	"io"
	"net/http"
	"net/url"
	_ "net/url"
	"time"
)

type HTTPStandardExecutor struct {
	client *http.Client
}

func NewHTTPStandardExecutor() *HTTPStandardExecutor {
	return &HTTPStandardExecutor{
		client: &http.Client{
			Timeout: 60 * time.Second, // 默认超时
		},
	}
}

// Execute performs the HTTP request based on the tool definition
func (e *HTTPStandardExecutor) Execute(ctx context.Context, tool *configs.StandardToolDefinition, params map[string]interface{}) (map[string]interface{}, error) {
	cfg := tool.HTTPConfig
	if cfg == nil {
		return nil, fmt.Errorf("http config is missing for tool %s", tool.Name)
	}
	// 1. 构建 URL (支持简单的参数替换，或者直接使用配置好的 URL)
	targetURL := cfg.URL
	// 如果 ParamLocation 是 query，我们可以尝试将 params 拼接到 URL
	if cfg.ParamLocation == "query" && len(params) > 0 {
		u, err := url.Parse(targetURL)
		if err != nil {
			return nil, fmt.Errorf("invalid URL: %w", err)
		}
		q := u.Query()
		for k, v := range params {
			q.Set(k, fmt.Sprintf("%v", v))
		}
		u.RawQuery = q.Encode()
		targetURL = u.String()
	}
	// 2. 准备 Request Body
	var bodyReader io.Reader
	if cfg.Method == http.MethodPost || cfg.Method == http.MethodPut || cfg.Method == http.MethodPatch {
		jsonBody, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal params: %w", err)
		}
		bodyReader = bytes.NewBuffer(jsonBody)
	}
	// 3. 创建 HTTP Request
	req, err := http.NewRequestWithContext(ctx, cfg.Method, targetURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	// 4. 设置 Headers
	req.Header.Set("Content-Type", "application/json")
	for k, v := range cfg.Headers {
		req.Header.Set(k, v)
	}
	// 5. 执行请求
	log.Infof("HTTP Executor calling: %s %s", cfg.Method, targetURL)
	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()
	// 6. 读取响应
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	// 7. 检查状态码
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return map[string]interface{}{
			"status":     "error",
			"statusCode": resp.StatusCode,
			"body":       string(bodyBytes),
		}, fmt.Errorf("API returned status: %d", resp.StatusCode)
	}
	// 8. 尝试解析 JSON 响应
	var jsonResponse interface{}
	if err := json.Unmarshal(bodyBytes, &jsonResponse); err != nil {
		// 如果不是 JSON，返回原始字符串
		return map[string]interface{}{
			"status": "success",
			"data":   string(bodyBytes),
		}, nil
	}
	return map[string]interface{}{
		"status": "success",
		"data":   jsonResponse,
	}, nil
}
