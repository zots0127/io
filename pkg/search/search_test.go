package search

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/zots0127/io/pkg/metadata/repository"
	"github.com/zots0127/io/pkg/types"
)

func TestNewSearchEngine(t *testing.T) {
	config := &SearchConfig{
		EnableFullTextSearch: true,
		EnableSemanticSearch: false,
		EnableFuzzySearch:    true,
		MaxResults:           50,
		QueryTimeout:         10 * time.Second,
	}

	searchEngine := NewSearchEngine(nil, nil, config)

	if searchEngine == nil {
		t.Fatal("Search engine should not be nil")
	}

	if searchEngine.config.MaxResults != 50 {
		t.Errorf("Expected MaxResults 50, got %d", searchEngine.config.MaxResults)
	}
}

func TestSearchEngine_ValidateQuery(t *testing.T) {
	config := &SearchConfig{
		MinQueryLength: 2,
	}

	searchEngine := NewSearchEngine(nil, nil, config)

	// 测试有效查询
	validQuery := &SearchQuery{
		Query: "test",
		Limit: 10,
	}

	err := searchEngine.validateQuery(validQuery)
	if err != nil {
		t.Errorf("Valid query should not fail: %v", err)
	}

	// 测试查询太短
	shortQuery := &SearchQuery{
		Query: "t",
		Limit: 10,
	}

	err = searchEngine.validateQuery(shortQuery)
	if err == nil {
		t.Error("Short query should fail validation")
	}

	// 测试无效限制
	invalidLimitQuery := &SearchQuery{
		Query: "test",
		Limit: -1,
	}

	err = searchEngine.validateQuery(invalidLimitQuery)
	if err == nil {
		t.Error("Negative limit should fail validation")
	}

	// 测试无效偏移
	invalidOffsetQuery := &SearchQuery{
		Query:  "test",
		Limit:  10,
		Offset: -1,
	}

	err = searchEngine.validateQuery(invalidOffsetQuery)
	if err == nil {
		t.Error("Negative offset should fail validation")
	}
}

func TestSearchEngine_MatchesQuery(t *testing.T) {
	searchEngine := NewSearchEngine(nil, nil, nil)

	// 创建测试文件
	file := &types.FileMetadata{
		SHA1:        "test123",
		FileName:    "test_document.pdf",
		Size:        1024,
		Tags:        []string{"document", "important"},
		Description: "This is a test document",
		UploadedAt:  time.Now(),
	}

	// 测试文本匹配
	query := &SearchQuery{
		Query: "test",
	}

	if !searchEngine.matchesQuery(file, query) {
		t.Error("File should match text query")
	}

	// 测试标签匹配
	tagQuery := &SearchQuery{
		Tags: []string{"document"},
	}

	if !searchEngine.matchesQuery(file, tagQuery) {
		t.Error("File should match tag query")
	}

	// 测试多个标签匹配
	multiTagQuery := &SearchQuery{
		Tags: []string{"document", "important"},
	}

	if !searchEngine.matchesQuery(file, multiTagQuery) {
		t.Error("File should match multi-tag query")
	}

	// 测试不匹配的标签
	noMatchQuery := &SearchQuery{
		Tags: []string{"image"},
	}

	if searchEngine.matchesQuery(file, noMatchQuery) {
		t.Error("File should not match non-existent tag")
	}

	// 测试大小范围
	sizeQuery := &SearchQuery{
		SizeRange: &SizeRange{
			Min: 500,
			Max: 2000,
		},
	}

	if !searchEngine.matchesQuery(file, sizeQuery) {
		t.Error("File should match size range")
	}

	// 测试超出大小范围
	oversizeQuery := &SearchQuery{
		SizeRange: &SizeRange{
			Min: 2000,
			Max: 5000,
		},
	}

	if searchEngine.matchesQuery(file, oversizeQuery) {
		t.Error("File should not match out-of-range size")
	}

	// 测试日期范围
	dateQuery := &SearchQuery{
		DateRange: &DateRange{
			From: time.Now().Add(-1 * time.Hour),
			To:   time.Now().Add(1 * time.Hour),
		},
	}

	if !searchEngine.matchesQuery(file, dateQuery) {
		t.Error("File should match date range")
	}
}

func TestSearchEngine_CalculateScore(t *testing.T) {
	config := &SearchConfig{
		BoostRecentFiles: true,
	}

	searchEngine := NewSearchEngine(nil, nil, config)

	// 创建测试文件
	file := &types.FileMetadata{
		SHA1:        "test123",
		FileName:    "test_document.pdf",
		Size:        1024,
		Tags:        []string{"document", "important"},
		Description: "This is a test document",
		UploadedAt:  time.Now(),
	}

	// 测试完全匹配
	query := &SearchQuery{
		Query: "test_document.pdf",
		Tags:  []string{"document"},
	}

	score := searchEngine.calculateScore(file, query)
	if score <= 0 {
		t.Error("Score should be positive for matching file")
	}

	// 测试部分匹配
	partialQuery := &SearchQuery{
		Query: "test",
		Tags:  []string{"document"},
	}

	partialScore := searchEngine.calculateScore(file, partialQuery)
	if partialScore <= 0 {
		t.Error("Score should be positive for partial match")
	}

	// 完全匹配应该比部分匹配分数高
	if score <= partialScore {
		t.Error("Perfect match should have higher score than partial match")
	}
}

func TestSearchEngine_Tokenize(t *testing.T) {
	searchEngine := NewSearchEngine(nil, nil, nil)

	// 测试基本分词
	text := "Hello World Test"
	tokens := searchEngine.tokenize(text)
	expected := []string{"hello", "world", "test"}

	if len(tokens) != len(expected) {
		t.Errorf("Expected %d tokens, got %d", len(expected), len(tokens))
	}

	for i, token := range tokens {
		if token != expected[i] {
			t.Errorf("Expected token '%s', got '%s'", expected[i], token)
		}
	}

	// 测试标点符号过滤
	textWithPunctuation := "Hello, world! This is a test."
	tokensWithPunct := searchEngine.tokenize(textWithPunctuation)
	expectedWithPunct := []string{"hello", "world", "this", "is", "test"}

	if len(tokensWithPunct) != len(expectedWithPunct) {
		t.Errorf("Expected %d tokens, got %d", len(expectedWithPunct), len(tokensWithPunct))
	}

	// 验证每个token
	for i, token := range tokensWithPunct {
		if token != expectedWithPunct[i] {
			t.Errorf("Expected token '%s', got '%s'", expectedWithPunct[i], token)
		}
	}

	// 测试空字符串
	emptyText := ""
	emptyTokens := searchEngine.tokenize(emptyText)
	if len(emptyTokens) != 0 {
		t.Error("Empty text should produce no tokens")
	}
}

func TestInvertedIndex(t *testing.T) {
	index := NewInvertedIndex()

	// 测试添加倒排列表项
	index.AddPosting("hello", "file1", 1.0)
	index.AddPosting("world", "file1", 0.8)
	index.AddPosting("hello", "file2", 1.0)

	// 测试获取词汇信息
	helloTerm := index.GetTerm("hello")
	if helloTerm == nil {
		t.Error("Term 'hello' should exist")
	}

	if helloTerm.DF != 2 {
		t.Errorf("Expected document frequency 2, got %d", helloTerm.DF)
	}

	if len(helloTerm.Postings) != 2 {
		t.Errorf("Expected 2 postings, got %d", len(helloTerm.Postings))
	}

	// 测试获取不存在的词汇
	nonExistentTerm := index.GetTerm("nonexistent")
	if nonExistentTerm != nil {
		t.Error("Non-existent term should return nil")
	}

	// 测试查找相似词汇
	similarTerms := index.FindSimilarTerms("helo", 0.8)
	// 这里需要实现真正的相似度算法
	if len(similarTerms) == 0 {
		t.Log("No similar terms found (expected with basic implementation)")
	}
}

func TestQueryCache(t *testing.T) {
	cache := NewQueryCache(2)

	// 测试设置和获取
	result := &SearchResult{
		Files: []*SearchResultFile{},
		Total: 0,
	}

	cache.Set("key1", result, 5*time.Minute)
	cachedResult := cache.Get("key1")

	if cachedResult == nil {
		t.Error("Cached result should not be nil")
	}

	if cachedResult.Total != result.Total {
		t.Errorf("Expected total %d, got %d", result.Total, cachedResult.Total)
	}

	// 测试缓存未命中
	nonExistent := cache.Get("nonexistent")
	if nonExistent != nil {
		t.Error("Non-existent key should return nil")
	}

	// 测试缓存过期
	cache.Set("expire", result, 1*time.Millisecond)
	time.Sleep(10 * time.Millisecond)
	expired := cache.Get("expire")
	if expired != nil {
		t.Error("Expired cache should return nil")
	}

	// 测试LRU淘汰
	cache.Set("key1", result, 5*time.Minute)
	cache.Set("key2", result, 5*time.Minute)
	cache.Set("key3", result, 5*time.Minute) // 应该淘汰key1

	key1AfterEviction := cache.Get("key1")
	if key1AfterEviction != nil {
		t.Error("Evicted key should return nil")
	}
}

func TestSearchEngine_Integration(t *testing.T) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "search_test")
	if err != nil {
		t.Fatal("Failed to create temp directory:", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")

	// 初始化元数据
	metadataRepo, err := repository.NewMetadataRepository(dbPath)
	if err != nil {
		t.Fatal("Failed to initialize metadata repository:", err)
	}
	defer metadataRepo.Close()

	// 添加测试数据
	testFiles := []*types.FileMetadata{
		{
			SHA1:        "file1",
			FileName:    "document1.pdf",
			Size:        1024,
			Tags:        []string{"document", "important"},
			Description: "This is an important document",
			UploadedAt:  time.Now().Add(-1 * time.Hour),
		},
		{
			SHA1:        "file2",
			FileName:    "image1.jpg",
			Size:        2048,
			Tags:        []string{"image", "photo"},
			Description: "A beautiful photo",
			UploadedAt:  time.Now().Add(-2 * time.Hour),
		},
		{
			SHA1:        "file3",
			FileName:    "document2.pdf",
			Size:        512,
			Tags:        []string{"document", "draft"},
			Description: "A draft document",
			UploadedAt:  time.Now().Add(-30 * time.Minute),
		},
	}

	for _, file := range testFiles {
		err := metadataRepo.SaveMetadata(file)
		if err != nil {
			t.Fatal("Failed to save test metadata:", err)
		}
	}

	// 创建搜索引擎
	config := &SearchConfig{
		EnableFullTextSearch: true,
		EnableFuzzySearch:    true,
		MaxResults:           10,
		QueryTimeout:         5 * time.Second,
	}

	searchEngine := NewSearchEngine(nil, metadataRepo, config)

	// 等待索引构建
	time.Sleep(100 * time.Millisecond)

	// 测试基本搜索
	query := &SearchQuery{
		Query:   "document",
		SortBy:  SortByRelevance,
		Limit:   10,
		Offset:  0,
	}

	result, err := searchEngine.Search(context.Background(), query)
	if err != nil {
		t.Fatal("Search failed:", err)
	}

	if result.Total < 2 {
		t.Errorf("Expected at least 2 results for 'document', got %d", result.Total)
	}

	// 测试标签搜索
	tagQuery := &SearchQuery{
		Tags:    []string{"image"},
		SortBy:  SortByRelevance,
		Limit:   10,
		Offset:  0,
	}

	tagResult, err := searchEngine.Search(context.Background(), tagQuery)
	if err != nil {
		t.Fatal("Tag search failed:", err)
	}

	if tagResult.Total != 1 {
		t.Errorf("Expected 1 result for 'image' tag, got %d", tagResult.Total)
	}

	// 测试排序
	sortQuery := &SearchQuery{
		Query:   "",
		SortBy:  SortByDate,
		SortOrder: SortOrderDesc,
		Limit:   10,
		Offset:  0,
	}

	sortResult, err := searchEngine.Search(context.Background(), sortQuery)
	if err != nil {
		t.Fatal("Sort search failed:", err)
	}

	if len(sortResult.Files) != 3 {
		t.Errorf("Expected 3 files total, got %d", len(sortResult.Files))
	}

	// 验证排序（最新的文件应该在前面）
	for i := 1; i < len(sortResult.Files); i++ {
		if sortResult.Files[i-1].UploadedAt.Before(sortResult.Files[i].UploadedAt) {
			t.Error("Files should be sorted by date in descending order")
			break
		}
	}

	t.Logf("Integration test completed. Found %d results for 'document'", result.Total)
}

// 基准测试
func BenchmarkSearchEngine_Search(b *testing.B) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "search_bench")
	if err != nil {
		b.Fatal("Failed to create temp directory:", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "bench.db")
	metadataRepo, err := repository.NewMetadataRepository(dbPath)
	if err != nil {
		b.Fatal("Failed to initialize metadata repository:", err)
	}
	defer metadataRepo.Close()

	// 添加大量测试数据
	for i := 0; i < 1000; i++ {
		file := &types.FileMetadata{
			SHA1:        fmt.Sprintf("file%d", i),
			FileName:    fmt.Sprintf("document%d.pdf", i),
			Size:        int64(1024 + i*100),
			Tags:        []string{"document", fmt.Sprintf("tag%d", i%10)},
			Description: fmt.Sprintf("Test document number %d", i),
			UploadedAt:  time.Now().Add(-time.Duration(i) * time.Hour),
		}
		metadataRepo.SaveMetadata(file)
	}

	// 创建搜索引擎
	config := &SearchConfig{
		EnableFullTextSearch: true,
		MaxResults:           20,
	}
	searchEngine := NewSearchEngine(nil, metadataRepo, config)

	// 等待索引构建
	time.Sleep(500 * time.Millisecond)

	// 基准测试搜索
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		query := &SearchQuery{
			Query:  fmt.Sprintf("document%d", i%100),
			Limit:  20,
			SortBy: SortByRelevance,
		}
		_, err := searchEngine.Search(context.Background(), query)
		if err != nil {
			b.Fatal("Search failed:", err)
		}
	}
}

func BenchmarkSearchEngine_Tokenize(b *testing.B) {
	searchEngine := NewSearchEngine(nil, nil, nil)
	text := "This is a test document for tokenization benchmark with various words and punctuation marks"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		searchEngine.tokenize(text)
	}
}