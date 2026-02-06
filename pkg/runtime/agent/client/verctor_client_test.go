package client

import (
	"fmt"
	"testing"
	"time"

	"github.com/pingcap-incubator/tinykv/log"
)

// TestEmbeddingClient 测试嵌入客户端
func TestEmbeddingClient(t *testing.T) {
	// 创建客户端
	client := NewEmbeddingServerClient("http://localhost:20002")

	fmt.Println("🧪 Starting Embedding Client Tests...")
	fmt.Println("=======================================")

	// 测试1: 健康检查
	fmt.Println("\n1. Testing Health Check...")
	testHealthCheck(client, t)

	// 测试2: 获取模型信息
	fmt.Println("\n2. Testing Model Information...")
	testModelInformation(client, t)

	// 测试3: 单文本嵌入
	fmt.Println("\n3. Testing Single Text Embedding...")
	testSingleTextEmbedding(client, t)

	// 测试4: 批量文本嵌入
	fmt.Println("\n4. Testing Batch Text Embedding...")
	testBatchTextEmbedding(client, t)

	// 测试5: 不同维度测试
	fmt.Println("\n5. Testing Different Dimensions...")
	testDifferentDimensions(client, t)

	// 测试7: 批量测试
	fmt.Println("\n7. Testing Batch Testing...")
	testBatchTesting(client, t)

	fmt.Println("\n✅ All tests completed!")
}

// testHealthCheck 测试健康检查
func testHealthCheck(client *EmbeddingServerClient, t *testing.T) {
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
func testModelInformation(client *EmbeddingServerClient, t *testing.T) {
	models, err := client.GetModels()
	if err != nil {
		t.Errorf("❌ Get models failed: %v", err)
		return
	}

	fmt.Printf("   ✅ Current Model: %s\n", models.CurrentModel)
	fmt.Printf("   ✅ Default Dimensions: %d\n", models.DefaultDimensions)
	fmt.Printf("   ✅ Batch Size Limit: %d\n", models.BatchSizeLimit)
	fmt.Printf("   ✅ Supported Dimensions: %v\n", models.SupportedDimensions)

	if len(models.SupportedModels) == 0 {
		t.Errorf("❌ No supported models returned")
	}
}

// testSingleTextEmbedding 测试单文本嵌入
func testSingleTextEmbedding(client *EmbeddingServerClient, t *testing.T) {
	testText := "This is a test sentence for embedding generation"

	// 测试不同维度
	dimensions := []int{256, 512, 1024}

	for _, dim := range dimensions {
		startTime := time.Now()
		embedding, err := client.EmbedSingle(testText, dim)
		processingTime := time.Since(startTime)

		if err != nil {
			t.Errorf("❌ Single embedding failed for dimension %d: %v", dim, err)
			continue
		}

		fmt.Printf("   ✅ Dimension %d: %d elements, time: %v\n",
			dim, len(embedding), processingTime)

		// 验证向量维度
		if len(embedding) != dim {
			t.Errorf("❌ Expected dimension %d, got %d", dim, len(embedding))
		}
		// 验证向量值范围（大致检查）
		for i, val := range embedding {
			if i >= 5 { // 只检查前5个值
				break
			}
			if val < -10 || val > 10 {
				t.Errorf("❌ Unexpected embedding value at index %d: %f", i, val)
			}
		}
	}
}

// testBatchTextEmbedding 测试批量文本嵌入
func testBatchTextEmbedding(client *EmbeddingServerClient, t *testing.T) {
	testTexts := []string{
		"I really like this product, it's amazing!",
		"The quality is good and delivery was fast",
		"Not satisfied with the customer service",
		"This is exactly what I was looking for",
		"The price is reasonable for the quality",
		"Will definitely recommend to my friends",
		"Package arrived damaged, very disappointed",
		"Easy to use and works perfectly",
		"Better than I expected, great value",
		"Customer support was very helpful",
	}

	startTime := time.Now()
	embeddings, err := client.Embed(testTexts, 512)
	processingTime := time.Since(startTime)

	if err != nil {
		t.Errorf("❌ Batch embedding failed: %v", err)
		return
	}

	fmt.Printf("   ✅ Processed %d texts in %v\n", len(testTexts), processingTime)
	fmt.Printf("   ✅ Generated %d embedding vectors\n", len(embeddings))

	// 验证返回数量
	if len(embeddings) != len(testTexts) {
		t.Errorf("❌ Expected %d embeddings, got %d", len(testTexts), len(embeddings))
	}

	// 验证每个向量的维度
	for i, embedding := range embeddings {
		if len(embedding) != 512 {
			t.Errorf("❌ Text %d: expected dimension 512, got %d", i, len(embedding))
		}
	}
}

// testDifferentDimensions 测试不同维度
func testDifferentDimensions(client *EmbeddingServerClient, t *testing.T) {
	testText := "Testing different embedding dimensions"
	dimensions := []int{128, 256, 512, 768, 1024}

	fmt.Println("   Testing dimensions:", dimensions)

	for _, dim := range dimensions {
		embedding, err := client.EmbedSingle(testText, dim)
		if err != nil {
			t.Errorf("❌ Embedding failed for dimension %d: %v", dim, err)
			continue
		}

		if len(embedding) != dim {
			t.Errorf("❌ Dimension %d: expected %d, got %d", dim, dim, len(embedding))
		} else {
			fmt.Printf("   ✅ Dimension %d: PASS\n", dim)
		}
	}
}

// testBatchTesting 测试批量测试功能
func testBatchTesting(client *EmbeddingServerClient, t *testing.T) {
	customTexts := []string{
		"Excellent product with great features",
		"Poor quality and bad customer service",
		"Average product, nothing special",
		"Outstanding performance and value",
	}

	results, err := client.BatchTest(customTexts)
	if err != nil {
		t.Errorf("❌ Batch test failed: %v", err)
		return
	}

	fmt.Printf("   ✅ Batch test completed successfully\n")

	// 检查返回结果结构
	if testResults, exists := results["test_results"]; exists {
		fmt.Printf("   ✅ Test results structure: OK\n")
		_ = testResults // 可以进一步解析和验证
	} else {
		t.Errorf("❌ Missing test_results in batch test response")
	}

	if testTexts, exists := results["test_texts"]; exists {
		if texts, ok := testTexts.([]interface{}); ok {
			fmt.Printf("   ✅ Test texts count: %d\n", len(texts))
		}
	}
}

// BenchmarkEmbedding 性能基准测试
func BenchmarkEmbedding(b *testing.B) {
	client := NewEmbeddingServerClient("http://localhost:5000")
	testText := "This is a benchmark test sentence for performance testing"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.EmbedSingle(testText, 512)
		if err != nil {
			b.Fatalf("Benchmark failed: %v", err)
		}
	}
}

// Example usage 示例用法
func ExampleEmbeddingServerClient() {
	// 创建客户端
	client := NewEmbeddingServerClient("http://localhost:5000")

	// 健康检查
	health, err := client.HealthCheck()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Service status: %s\n", health.Status)

	// 生成嵌入向量
	embeddings, err := client.Embed([]string{"Hello, world!"}, 256)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Generated %d embedding vectors\n", len(embeddings))

}
