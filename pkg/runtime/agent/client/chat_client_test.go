package client

import (
	"fmt"
	"testing"
	"time"

	"github.com/pingcap-incubator/tinykv/log"
)

// TestChatClient 测试聊天客户端
func TestChatClient(t *testing.T) {
	// 创建客户端
	client := NewChatServerClient("http://localhost:20001")

	fmt.Println("🧪 Starting Chat Client Tests...")
	fmt.Println("=======================================")

	// 测试1: 健康检查
	fmt.Println("\n1. Testing Health Check...")
	testHealthCheck2(client, t)

	// 测试2: 获取模型信息
	fmt.Println("\n2. Testing Model Information...")
	testModelInformation2(client, t)

	// 测试3: 快速聊天
	fmt.Println("\n3. Testing Quick Chat...")
	testQuickChat(client, t)

	// 测试4: 完整聊天补全
	fmt.Println("\n4. Testing Chat Completion...")
	testChatCompletion(client, t)

	// 测试5: 多轮对话
	fmt.Println("\n5. Testing Multi-turn Chat...")
	testMultiTurnChat(client, t)

	// 测试6: 错误情况测试
	fmt.Println("\n6. Testing Error Cases...")
	testErrorCases(client, t)

	fmt.Println("\n✅ All tests completed!")
}

// testHealthCheck 测试健康检查
func testHealthCheck2(client *ChatServerClient, t *testing.T) {
	health, err := client.HealthCheck()
	if err != nil {
		t.Errorf("❌ Health check failed: %v", err)
		return
	}

	fmt.Printf("   ✅ Status: %s\n", health.Status)
	fmt.Printf("   ✅ Model: %s\n", health.Model)
	fmt.Printf("   ✅ Client Initialized: %t\n", health.ClientInitialized)

	if health.Status != "healthy" {
		t.Errorf("❌ Expected status 'healthy', got '%s'", health.Status)
	}
}

// testModelInformation 测试模型信息
func testModelInformation2(client *ChatServerClient, t *testing.T) {
	models, err := client.ListModels()
	if err != nil {
		t.Errorf("❌ Get models failed: %v", err)
		return
	}

	fmt.Printf("   ✅ Current Model: %s\n", models.CurrentModel)
	fmt.Printf("   ✅ Supported Models: %v\n", models.SupportedModels)

	if len(models.SupportedModels) == 0 {
		t.Errorf("❌ No supported models returned")
	}

	// 检查参数配置
	if models.Parameters == nil {
		t.Errorf("❌ Parameters configuration missing")
	} else {
		fmt.Printf("   ✅ Parameters: %v\n", models.Parameters)
	}
}

// testQuickChat 测试快速聊天
func testQuickChat(client *ChatServerClient, t *testing.T) {
	testCases := []struct {
		name         string
		message      string
		systemPrompt string
	}{
		{
			name:         "自我介绍",
			message:      "你好，请介绍一下你自己",
			systemPrompt: "你是一个有用的助手",
		},
		{
			name:    "编程问题",
			message: "用Go语言写一个Hello World程序",
		},
		{
			name:    "知识问答",
			message: "什么是人工智能？",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			startTime := time.Now()
			var resp *QuickChatResponse
			var err error

			if tc.systemPrompt != "" {
				resp, err = client.QuickChat(tc.message, tc.systemPrompt)
			} else {
				resp, err = client.QuickChat(tc.message)
			}
			processingTime := time.Since(startTime)

			if err != nil {
				t.Errorf("❌ Quick chat failed: %v", err)
				return
			}

			fmt.Printf("   ✅ %s: %d chars, time: %v\n",
				tc.name, len(resp.Response), processingTime)

			// 验证响应内容
			if resp.Response == "" {
				t.Errorf("❌ Empty response received")
			}

			if len(resp.Response) < 10 {
				t.Errorf("❌ Response too short: %s", resp.Response)
			}
		})
	}
}

// testChatCompletion 测试完整聊天补全
func testChatCompletion(client *ChatServerClient, t *testing.T) {
	testCases := []struct {
		name      string
		messages  []ChatMessage
		maxTokens int
	}{
		{
			name: "技术问题",
			messages: []ChatMessage{
				{Role: "system", Content: "你是一个技术专家"},
				{Role: "user", Content: "解释一下RESTful API的设计原则"},
			},
			maxTokens: 500,
		},
		{
			name: "创意写作",
			messages: []ChatMessage{
				{Role: "system", Content: "你是一个诗人"},
				{Role: "user", Content: "写一首关于秋天的短诗"},
			},
			maxTokens: 200,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := ChatCompletionRequest{
				Messages:  tc.messages,
				MaxTokens: tc.maxTokens,
			}

			startTime := time.Now()
			resp, err := client.ChatCompletion(req)
			processingTime := time.Since(startTime)

			if err != nil {
				t.Errorf("❌ Chat completion failed: %v", err)
				return
			}

			fmt.Printf("   ✅ %s: %d choices, time: %v\n",
				tc.name, len(resp.Choices), processingTime)

			// 验证响应结构
			if len(resp.Choices) == 0 {
				t.Errorf("❌ No choices in response")
				return
			}

			choice := resp.Choices[0]
			if choice.Message.Content == "" {
				t.Errorf("❌ Empty message content")
			}

			if choice.Message.Role == "" {
				t.Errorf("❌ Empty message role")
			}

			fmt.Printf("   ✅ Response length: %d chars\n", len(choice.Message.Content))
		})
	}
}

// testMultiTurnChat 测试多轮对话
func testMultiTurnChat(client *ChatServerClient, t *testing.T) {
	// 初始对话
	conversation := []ChatMessage{
		{Role: "user", Content: "我喜欢学习编程"},
		{Role: "assistant", Content: "太好了！编程是21世纪的重要技能。你目前在学习什么语言？"},
	}

	fmt.Printf("   ✅ Initial conversation: %d messages\n", len(conversation))

	// 第一轮对话
	t.Run("第一轮对话", func(t *testing.T) {
		resp, err := client.MultiTurnChat(conversation, "我在学习Go语言")
		if err != nil {
			t.Errorf("❌ Multi-turn chat failed: %v", err)
			return
		}

		fmt.Printf("   ✅ First response: %d chars\n", len(resp.Response))
		fmt.Printf("   ✅ Updated conversation: %d messages\n", len(resp.UpdatedConversation))

		if len(resp.UpdatedConversation) != len(conversation)+2 {
			t.Errorf("❌ Expected %d messages, got %d",
				len(conversation)+2, len(resp.UpdatedConversation))
		}

		// 第二轮对话
		t.Run("第二轮对话", func(t *testing.T) {
			resp2, err := client.MultiTurnChat(resp.UpdatedConversation, "有什么学习建议吗？")
			if err != nil {
				t.Errorf("❌ Second round failed: %v", err)
				return
			}

			fmt.Printf("   ✅ Second response: %d chars\n", len(resp2.Response))
			fmt.Printf("   ✅ Final conversation: %d messages\n", len(resp2.UpdatedConversation))

			if len(resp2.UpdatedConversation) != len(resp.UpdatedConversation)+2 {
				t.Errorf("❌ Expected %d messages, got %d",
					len(resp.UpdatedConversation)+2, len(resp2.UpdatedConversation))
			}
		})
	})
}

// testErrorCases 测试错误情况
func testErrorCases(client *ChatServerClient, t *testing.T) {
	testCases := []struct {
		name        string
		testFunc    func() error
		expectError bool
	}{
		{
			name: "空消息",
			testFunc: func() error {
				_, err := client.QuickChat("")
				return err
			},
			expectError: true,
		},
		{
			name: "无效消息格式",
			testFunc: func() error {
				req := ChatCompletionRequest{
					Messages: []ChatMessage{
						{Role: "user"}, // 缺少content字段
					},
				}
				_, err := client.ChatCompletion(req)
				return err
			},
			expectError: true,
		},
		{
			name: "空对话历史",
			testFunc: func() error {
				_, err := client.MultiTurnChat([]ChatMessage{}, "")
				return err
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.testFunc()

			if tc.expectError {
				if err == nil {
					t.Errorf("❌ Expected error but got none")
				} else {
					fmt.Printf("   ✅ %s: Got expected error - %v\n", tc.name, err)
				}
			} else {
				if err != nil {
					t.Errorf("❌ Unexpected error: %v", err)
				}
			}
		})
	}
}

// BenchmarkQuickChat 性能基准测试
func BenchmarkQuickChat(b *testing.B) {
	client := NewChatServerClient("http://localhost:5000")
	testMessage := "这是一条性能测试消息"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.QuickChat(testMessage)
		if err != nil {
			b.Fatalf("Benchmark failed: %v", err)
		}
	}
}

// BenchmarkChatCompletion 聊天补全性能基准测试
func BenchmarkChatCompletion(b *testing.B) {
	client := NewChatServerClient("http://localhost:5000")
	req := ChatCompletionRequest{
		Messages: []ChatMessage{
			{Role: "user", Content: "这是一条性能测试消息"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.ChatCompletion(req)
		if err != nil {
			b.Fatalf("Benchmark failed: %v", err)
		}
	}
}

// ExampleChatServerClient 示例用法
func ExampleChatServerClient() {
	// 创建客户端
	client := NewChatServerClient("http://localhost:5000")

	// 健康检查
	health, err := client.HealthCheck()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Service status: %s\n", health.Status)

	// 快速聊天
	response, err := client.QuickChat("Hello, world!")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Assistant: %s\n", response.Response)
}

// TestStreamingChat 测试流式聊天（可选测试）
func TestStreamingChat(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping streaming test in short mode")
	}

	client := NewChatServerClient("http://localhost:5000")

	req := ChatCompletionRequest{
		Messages: []ChatMessage{
			{Role: "user", Content: "请流式输出数字1到5，每个数字单独输出"},
		},
		Stream: true,
	}

	stream, err := client.StreamChatCompletion(req)
	if err != nil {
		t.Errorf("❌ Stream chat failed: %v", err)
		return
	}
	defer stream.Close()

	fmt.Println("   ✅ Streaming test started...")
	// 注意：这里需要根据实际的流式响应格式进行解析
	// 当前实现中流式响应是纯文本，可能需要调整
}

// TestPerformance 性能测试
func TestPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	client := NewChatServerClient("http://localhost:5000")

	startTime := time.Now()
	successCount := 0
	totalRequests := 10

	for i := 0; i < totalRequests; i++ {
		_, err := client.QuickChat(fmt.Sprintf("性能测试消息 %d", i+1))
		if err == nil {
			successCount++
		}
		time.Sleep(100 * time.Millisecond) // 小延迟避免过于频繁
	}

	duration := time.Since(startTime)
	fmt.Printf("   ✅ Performance: %d/%d requests in %v\n", successCount, totalRequests, duration)
	fmt.Printf("   ✅ Average response time: %v\n", duration/time.Duration(totalRequests))
	fmt.Printf("   ✅ Requests per second: %.2f\n", float64(totalRequests)/duration.Seconds())

	if successCount < totalRequests {
		t.Errorf("❌ Performance test: only %d/%d requests succeeded", successCount, totalRequests)
	}
}
