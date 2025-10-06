package ai

import (
	"context"
	"fmt"
	"mime/multipart"
	"testing"
	"time"
)

// MockAIModel 模拟AI模型
type MockAIModel struct {
	name     string
	modelType string
}

func (m *MockAIModel) Analyze(ctx context.Context, filePath string, file *multipart.FileHeader) (*ClassificationResult, error) {
	return &ClassificationResult{
		ContentType:  ContentTypeDocument,
		Category:     "文档",
		Subcategory:  "测试文档",
		Confidence:   0.9,
		Tags:         []string{"测试", "文档"},
		ProcessedAt:  time.Now(),
		ModelVersion: "v1.0.0",
	}, nil
}

func (m *MockAIModel) ClassifyContent(ctx context.Context, data []byte, mimeType string) (*ClassificationResult, error) {
	return &ClassificationResult{
		ContentType:  ContentTypeData,
		Category:     "数据",
		Confidence:   0.8,
		Tags:         []string{"测试", "数据"},
		ProcessedAt:  time.Now(),
		ModelVersion: "v1.0.0",
	}, nil
}

func (m *MockAIModel) ExtractText(ctx context.Context, filePath string) (string, error) {
	return "提取的测试文本内容", nil
}

func (m *MockAIModel) DetectObjects(ctx context.Context, filePath string) ([]string, error) {
	return []string{"测试对象1", "测试对象2"}, nil
}

func (m *MockAIModel) GetModelInfo() ModelInfo {
	return ModelInfo{
		Name:        m.name,
		Version:     "v1.0.0",
		Type:        m.modelType,
		Accuracy:    0.9,
		LastUpdated: time.Now(),
		Status:      "active",
	}
}

// createTestFileHeader 创建测试文件头
func createTestFileHeader(filename string, size int64, mimeType string) *multipart.FileHeader {
	header := &multipart.FileHeader{
		Filename: filename,
		Size:     size,
	}
	if mimeType != "" {
		header.Header = make(map[string][]string)
		header.Header.Set("Content-Type", mimeType)
	}
	return header
}

func TestNewClassifier(t *testing.T) {
	config := &ClassifierConfig{
		EnableCache:           true,
		CacheExpiration:       time.Hour,
		MaxCacheSize:          100,
		EnableParallel:        false,
		MaxParallelWorkers:    2,
		DefaultConfidence:     0.8,
		EnableOCR:             false,
		EnableObjectDetection: false,
	}

	classifier := NewClassifier(config)
	if classifier == nil {
		t.Fatal("Failed to create classifier")
	}

	if classifier.config != config {
		t.Error("Config not set correctly")
	}

	if len(classifier.models) == 0 {
		t.Error("Default models not initialized")
	}
}

func TestClassifierRegisterModel(t *testing.T) {
	classifier := NewClassifier(nil)
	mockModel := &MockAIModel{
		name:     "test_model",
		modelType: "test",
	}

	classifier.RegisterModel("test", mockModel)

	models := classifier.GetSupportedModels()
	if _, exists := models["test"]; !exists {
		t.Error("Model not registered correctly")
	}

	if models["test"].Name != "test_model" {
		t.Error("Model info not correct")
	}
}

func TestContentTypeModel(t *testing.T) {
	model := NewContentTypeModel()

	tests := []struct {
		name     string
		filename string
		mimeType string
		expected ContentType
	}{
		{
			name:     "PDF file",
			filename: "document.pdf",
			mimeType: "application/pdf",
			expected: ContentTypeDocument,
		},
		{
			name:     "JPEG image",
			filename: "photo.jpg",
			mimeType: "image/jpeg",
			expected: ContentTypeImage,
		},
		{
			name:     "Text file",
			filename: "notes.txt",
			mimeType: "text/plain",
			expected: ContentTypeDocument,
		},
		{
			name:     "Unknown type",
			filename: "unknown.xyz",
			mimeType: "application/octet-stream",
			expected: ContentTypeOther,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := createTestFileHeader(tt.filename, 1024, tt.mimeType)
			result, err := model.Analyze(context.Background(), "test_path", file)

			if err != nil {
				t.Errorf("Analyze failed: %v", err)
				return
			}

			if result.ContentType != tt.expected {
				t.Errorf("Expected content type %v, got %v", tt.expected, result.ContentType)
			}

			if result.Confidence <= 0 {
				t.Error("Confidence should be positive")
			}

			if len(result.Tags) == 0 {
				t.Error("Tags should not be empty")
			}
		})
	}
}

func TestTaggerModel(t *testing.T) {
	config := &ClassifierConfig{
		EnableOCR:             false,
		EnableObjectDetection: false,
	}
	model := NewTaggerModel(config)

	tests := []struct {
		name     string
		filename string
		size     int64
		expected []string
	}{
		{
			name:     "Business report",
			filename: "business_report_2024.pdf",
			size:     1024 * 1024,
			expected: []string{"大文件", "报告", "报表", "2024年", "新文件", "商务", "工作"},
		},
		{
			name:     "Small text file",
			filename: "notes.txt",
			size:     512,
			expected: []string{"小文件", "笔记", "备注"},
		},
		{
			name:     "Large video file",
			filename: "presentation_video.mp4",
			size:     500 * 1024 * 1024,
			expected: []string{"超大文件"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := createTestFileHeader(tt.filename, tt.size, "")
			result, err := model.Analyze(context.Background(), "test_path", file)

			if err != nil {
				t.Errorf("Analyze failed: %v", err)
				return
			}

			if len(result.Tags) == 0 {
				t.Error("Tags should not be empty")
			}

			// 检查是否包含期望的标签
			for _, expectedTag := range tt.expected {
				found := false
				for _, tag := range result.Tags {
					if tag == expectedTag {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected tag %s not found in result tags %v", expectedTag, result.Tags)
				}
			}
		})
	}
}

func TestClassifierClassifyFile(t *testing.T) {
	config := &ClassifierConfig{
		EnableCache:           false, // 禁用缓存以简化测试
		EnableParallel:        false,
		EnableOCR:             false,
		EnableObjectDetection: false,
	}
	classifier := NewClassifier(config)

	file := createTestFileHeader("test_document.pdf", 1024*1024, "application/pdf")
	result, err := classifier.ClassifyFile(context.Background(), "test_path", file)

	if err != nil {
		t.Errorf("ClassifyFile failed: %v", err)
		return
	}

	if result.ContentType != ContentTypeDocument {
		t.Errorf("Expected content type %v, got %v", ContentTypeDocument, result.ContentType)
	}

	if result.Category == "" {
		t.Error("Category should not be empty")
	}

	if len(result.Tags) == 0 {
		t.Error("Tags should not be empty")
	}

	if result.ProcessedAt.IsZero() {
		t.Error("ProcessedAt should be set")
	}
}

func TestClassificationCache(t *testing.T) {
	cache := NewClassificationCache(2)

	// 测试设置和获取
	result := &ClassificationResult{
		ContentType: ContentTypeDocument,
		Category:    "测试",
		ProcessedAt: time.Now(),
	}

	cache.Set("key1", 1024, result, time.Hour)
	cached := cache.Get("key1", 1024)

	if cached == nil {
		t.Error("Cached result should not be nil")
		return
	}

	if cached.ContentType != result.ContentType {
		t.Error("Cached result doesn't match original")
	}

	// 测试过期
	expiredResult := &ClassificationResult{
		ContentType: ContentTypeImage,
		Category:    "过期测试",
		ProcessedAt: time.Now(),
	}

	cache.Set("expired_key", 1024, expiredResult, -time.Hour) // 立即过期
	cachedExpired := cache.Get("expired_key", 1024)

	if cachedExpired != nil {
		t.Error("Expired result should be nil")
	}

	// 测试缓存大小限制
	cache.Set("key2", 1024, result, time.Hour)
	cache.Set("key3", 1024, result, time.Hour) // 应该移除key1

	cachedKey1 := cache.Get("key1", 1024)
	if cachedKey1 != nil {
		t.Error("key1 should be evicted due to size limit")
	}

	cachedKey3 := cache.Get("key3", 1024)
	if cachedKey3 == nil {
		t.Error("key3 should be in cache")
	}
}

func TestDetectContentTypeFromExtension(t *testing.T) {
	tests := []struct {
		filename string
		expected ContentType
	}{
		{"document.pdf", ContentTypeDocument},
		{"image.jpg", ContentTypeImage},
		{"video.mp4", ContentTypeVideo},
		{"audio.mp3", ContentTypeAudio},
		{"archive.zip", ContentTypeArchive},
		{"script.go", ContentTypeCode},
		{"data.json", ContentTypeData},
		{"unknown.xyz", ContentTypeOther},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := DetectContentTypeFromExtension(tt.filename)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v for %s", tt.expected, result, tt.filename)
			}
		})
	}
}

func TestGetCategoryFromContentType(t *testing.T) {
	tests := []struct {
		contentType ContentType
		expected    string
	}{
		{ContentTypeDocument, "文档"},
		{ContentTypeImage, "图片"},
		{ContentTypeVideo, "视频"},
		{ContentTypeAudio, "音频"},
		{ContentTypeArchive, "压缩包"},
		{ContentTypeCode, "代码"},
		{ContentTypeData, "数据"},
		{ContentTypeOther, "其他"},
	}

	for _, tt := range tests {
		t.Run(string(tt.contentType), func(t *testing.T) {
			result := GetCategoryFromContentType(tt.contentType)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestMergeTags(t *testing.T) {
	tagSets := [][]string{
		{"tag1", "tag2"},
		{"tag2", "tag3"},
		{"tag4", "tag1"},
	}

	result := MergeTags(tagSets...)

	expected := []string{"tag1", "tag2", "tag3", "tag4"}

	if len(result) != len(expected) {
		t.Errorf("Expected %d tags, got %d", len(expected), len(result))
	}

	for _, expectedTag := range expected {
		found := false
		for _, tag := range result {
			if tag == expectedTag {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected tag %s not found", expectedTag)
		}
	}
}

func TestContentAnalyzer(t *testing.T) {
	classifier := NewClassifier(nil)
	config := &AnalyzerConfig{
		EnableDeepAnalysis:    true,
		MaxAnalysisSize:       10 * 1024 * 1024,
		AnalysisTimeout:       30 * time.Second,
		EnableSimilarityCheck: false,
		EnableContentSummary:  true,
		SummaryMaxLength:      500,
	}

	analyzer := NewContentAnalyzer(classifier, config)

	file := createTestFileHeader("test.txt", 1024, "text/plain")
	result, err := analyzer.AnalyzeFile(context.Background(), "test_path", file)

	if err != nil {
		t.Errorf("AnalyzeFile failed: %v", err)
		return
	}

	if result.FileInfo == nil {
		t.Error("FileInfo should not be nil")
	}

	if result.Classification == nil {
		t.Error("Classification should not be nil")
	}

	if result.AnalyzedAt.IsZero() {
		t.Error("AnalyzedAt should be set")
	}

	if result.ProcessingTime <= 0 {
		t.Error("ProcessingTime should be positive")
	}
}

func TestAIServiceImpl(t *testing.T) {
	// 注意：这里使用nil作为metadataRepo，在实际使用中需要真实的repository
	config := &AIServiceConfig{
		EnableClassification: true,
		EnableAnalysis:       true,
		EnableBatchProcess:   true,
		MaxBatchSize:         10,
		ProcessingTimeout:    10 * time.Second,
		EnableSimilarity:     false,
		EnableAutoTagging:    false,
		EnableInsights:       false,
	}

	service := NewAIService(nil, config)

	if service == nil {
		t.Fatal("Failed to create AI service")
	}

	// 测试配置
	serviceConfig := service.GetConfig()
	if serviceConfig == nil {
		t.Error("Config should not be nil")
	}

	// 测试统计
	stats := service.GetStats()
	if stats == nil {
		t.Error("Stats should not be nil")
	}

	// 测试健康检查
	err := service.Health()
	// 健康检查应该通过，因为classifier和analyzer都已初始化
	if err != nil {
		t.Errorf("Health check should pass: %v", err)
	}
}

// 基准测试

func BenchmarkContentTypeModelAnalyze(b *testing.B) {
	model := NewContentTypeModel()
	file := createTestFileHeader("benchmark.pdf", 1024*1024, "application/pdf")
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := model.Analyze(ctx, "test_path", file)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTaggerModelAnalyze(b *testing.B) {
	config := &ClassifierConfig{}
	model := NewTaggerModel(config)
	file := createTestFileHeader("benchmark_report_2024.pdf", 1024*1024, "")
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := model.Analyze(ctx, "test_path", file)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkClassifierClassifyFile(b *testing.B) {
	config := &ClassifierConfig{
		EnableCache:           false,
		EnableParallel:        false,
		EnableOCR:             false,
		EnableObjectDetection: false,
	}
	classifier := NewClassifier(config)
	file := createTestFileHeader("benchmark.pdf", 1024*1024, "application/pdf")
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := classifier.ClassifyFile(ctx, "test_path", file)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkClassificationCache(b *testing.B) {
	cache := NewClassificationCache(1000)
	result := &ClassificationResult{
		ContentType: ContentTypeDocument,
		ProcessedAt: time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key_%d", i%100) // 循环使用100个不同的key
		cache.Set(key, 1024, result, time.Hour)
		cached := cache.Get(key, 1024)
		if cached == nil {
			b.Fatal("Cached result should not be nil")
		}
	}
}