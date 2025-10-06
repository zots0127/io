package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"
)

// ContentAnalyzer 内容分析器
type ContentAnalyzer struct {
	classifier *Classifier
	config     *AnalyzerConfig
}

// AnalyzerConfig 分析器配置
type AnalyzerConfig struct {
	EnableDeepAnalysis    bool          `json:"enable_deep_analysis" yaml:"enable_deep_analysis"`
	MaxAnalysisSize       int64         `json:"max_analysis_size" yaml:"max_analysis_size"`
	AnalysisTimeout       time.Duration `json:"analysis_timeout" yaml:"analysis_timeout"`
	EnableSimilarityCheck bool          `json:"enable_similarity_check" yaml:"enable_similarity_check"`
	SimilarityThreshold   float64       `json:"similarity_threshold" yaml:"similarity_threshold"`
	EnableContentSummary  bool          `json:"enable_content_summary" yaml:"enable_content_summary"`
	SummaryMaxLength      int           `json:"summary_max_length" yaml:"summary_max_length"`
}

// AnalysisResult 分析结果
type AnalysisResult struct {
	FileInfo       *FileInfo             `json:"file_info"`
	Classification *ClassificationResult `json:"classification"`
	Content        *ContentInfo          `json:"content"`
	Similarity     *SimilarityInfo       `json:"similarity,omitempty"`
	Summary        string                `json:"summary,omitempty"`
	Keywords       []string              `json:"keywords"`
	Metadata       map[string]interface{} `json:"metadata"`
	AnalyzedAt     time.Time             `json:"analyzed_at"`
	ProcessingTime time.Duration         `json:"processing_time"`
}

// FileInfo 文件信息
type FileInfo struct {
	Name         string    `json:"name"`
	Size         int64     `json:"size"`
	Extension    string    `json:"extension"`
	MimeType     string    `json:"mime_type"`
	UploadedAt   time.Time `json:"uploaded_at"`
	LastModified time.Time `json:"last_modified"`
}

// ContentInfo 内容信息
type ContentInfo struct {
	Type         string                 `json:"type"`
	Encoding     string                 `json:"encoding"`
	Language     string                 `json:"language,omitempty"`
	TextContent  string                 `json:"text_content,omitempty"`
	WordCount    int                    `json:"word_count,omitempty"`
	LineCount    int                    `json:"line_count,omitempty"`
	Properties   map[string]interface{} `json:"properties"`
}

// SimilarityInfo 相似性信息
type SimilarityInfo struct {
	SimilarFiles  []SimilarFile `json:"similar_files"`
	Score         float64       `json:"score"`
	Algorithm     string        `json:"algorithm"`
	Threshold     float64       `json:"threshold"`
}

// SimilarFile 相似文件
type SimilarFile struct {
	SHA1     string  `json:"sha1"`
	Filename string  `json:"filename"`
	Score    float64 `json:"score"`
	Reason   string  `json:"reason"`
}

// NewContentAnalyzer 创建内容分析器
func NewContentAnalyzer(classifier *Classifier, config *AnalyzerConfig) *ContentAnalyzer {
	if config == nil {
		config = &AnalyzerConfig{
			EnableDeepAnalysis:    true,
			MaxAnalysisSize:       10 * 1024 * 1024, // 10MB
			AnalysisTimeout:       30 * time.Second,
			EnableSimilarityCheck: false,
			SimilarityThreshold:   0.8,
			EnableContentSummary:  true,
			SummaryMaxLength:      500,
		}
	}

	return &ContentAnalyzer{
		classifier: classifier,
		config:     config,
	}
}

// AnalyzeFile 分析文件
func (a *ContentAnalyzer) AnalyzeFile(ctx context.Context, filePath string, file *multipart.FileHeader) (*AnalysisResult, error) {
	startTime := time.Now()

	// 设置超时上下文
	if a.config.AnalysisTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, a.config.AnalysisTimeout)
		defer cancel()
	}

	result := &AnalysisResult{
		FileInfo: &FileInfo{
			Name:         file.Filename,
			Size:         file.Size,
			Extension:    filepath.Ext(file.Filename),
			MimeType:     file.Header.Get("Content-Type"),
			UploadedAt:   time.Now(),
			LastModified: time.Now(),
		},
		Metadata:   make(map[string]interface{}),
		AnalyzedAt: time.Now(),
	}

	// 1. 文件分类
	classification, err := a.classifier.ClassifyFile(ctx, filePath, file)
	if err != nil {
		return nil, fmt.Errorf("classification failed: %w", err)
	}
	result.Classification = classification

	// 2. 内容分析
	if a.config.EnableDeepAnalysis && file.Size <= a.config.MaxAnalysisSize {
		content, err := a.analyzeContent(ctx, filePath, file)
		if err == nil {
			result.Content = content
		}
	}

	// 3. 关键词提取
	keywords := a.extractKeywords(classification, result.Content)
	result.Keywords = keywords

	// 4. 内容摘要
	if a.config.EnableContentSummary && result.Content != nil {
		summary := a.generateSummary(result.Content)
		result.Summary = summary
	}

	// 5. 相似性检查
	if a.config.EnableSimilarityCheck {
		similarity, err := a.checkSimilarity(ctx, classification)
		if err == nil {
			result.Similarity = similarity
		}
	}

	// 6. 处理时间
	result.ProcessingTime = time.Since(startTime)

	return result, nil
}

// analyzeContent 分析文件内容
func (a *ContentAnalyzer) analyzeContent(ctx context.Context, filePath string, file *multipart.FileHeader) (*ContentInfo, error) {
	content := &ContentInfo{
		Properties: make(map[string]interface{}),
	}

	// 基于MIME类型确定内容类型
	mimeType := file.Header.Get("Content-Type")
	content.Type = a.detectContentType(mimeType)
	content.Encoding = a.detectEncoding(mimeType)

	// 如果是文本文件，尝试读取内容
	if a.isTextFile(mimeType, file.Filename) {
		textContent, err := a.extractTextContent(filePath)
		if err == nil {
			content.TextContent = textContent
			content.WordCount = len(strings.Fields(textContent))
			content.LineCount = len(strings.Split(textContent, "\n"))
			content.Language = a.detectLanguage(textContent)
		}
	}

	// 基于内容类型的特定分析
	switch content.Type {
	case "image":
		a.analyzeImageContent(content, filePath)
	case "document":
		a.analyzeDocumentContent(content, filePath)
	case "code":
		a.analyzeCodeContent(content, filePath)
	}

	return content, nil
}

// detectContentType 检测内容类型
func (a *ContentAnalyzer) detectContentType(mimeType string) string {
	if mimeType == "" {
		return "unknown"
	}

	if strings.HasPrefix(mimeType, "text/") {
		return "text"
	}
	if strings.HasPrefix(mimeType, "image/") {
		return "image"
	}
	if strings.HasPrefix(mimeType, "video/") {
		return "video"
	}
	if strings.HasPrefix(mimeType, "audio/") {
		return "audio"
	}
	if strings.Contains(mimeType, "pdf") {
		return "document"
	}
	if strings.Contains(mimeType, "json") || strings.Contains(mimeType, "xml") {
		return "data"
	}

	return "binary"
}

// detectEncoding 检测编码
func (a *ContentAnalyzer) detectEncoding(mimeType string) string {
	if strings.Contains(mimeType, "utf-8") {
		return "utf-8"
	}
	if strings.Contains(mimeType, "utf-16") {
		return "utf-16"
	}
	if strings.Contains(mimeType, "gbk") || strings.Contains(mimeType, "gb2312") {
		return "gbk"
	}

	return "unknown"
}

// isTextFile 判断是否为文本文件
func (a *ContentAnalyzer) isTextFile(mimeType, filename string) bool {
	if mimeType != "" {
		return strings.HasPrefix(mimeType, "text/") ||
			strings.Contains(mimeType, "json") ||
			strings.Contains(mimeType, "xml") ||
			strings.Contains(mimeType, "javascript") ||
			strings.Contains(mimeType, "css")
	}

	// 基于扩展名判断
	textExtensions := []string{
		".txt", ".md", ".json", ".xml", ".yaml", ".yml",
		".js", ".ts", ".html", ".css", ".go", ".py",
		".java", ".cpp", ".c", ".h", ".sh", ".sql",
	}

	ext := strings.ToLower(filepath.Ext(filename))
	for _, textExt := range textExtensions {
		if ext == textExt {
			return true
		}
	}

	return false
}

// extractTextContent 提取文本内容
func (a *ContentAnalyzer) extractTextContent(filePath string) (string, error) {
	// 这里应该实现真正的文件读取和文本提取
	// 目前返回模拟结果
	return "文件文本内容提取功能需要实现文件读取逻辑", nil
}

// detectLanguage 检测语言
func (a *ContentAnalyzer) detectLanguage(text string) string {
	// 简单的语言检测逻辑
	chineseChars := 0
	englishWords := 0

	for _, r := range text {
		if r >= 0x4e00 && r <= 0x9fff {
			chineseChars++
		} else if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			englishWords++
		}
	}

	if chineseChars > englishWords {
		return "zh"
	} else if englishWords > 0 {
		return "en"
	}

	return "unknown"
}

// analyzeImageContent 分析图像内容
func (a *ContentAnalyzer) analyzeImageContent(content *ContentInfo, filePath string) {
	// 这里应该集成图像处理库
	content.Properties["image_analysis"] = "需要集成图像处理库"
}

// analyzeDocumentContent 分析文档内容
func (a *ContentAnalyzer) analyzeDocumentContent(content *ContentInfo, filePath string) {
	// 这里应该集成文档处理库
	content.Properties["document_analysis"] = "需要集成文档处理库"
}

// analyzeCodeContent 分析代码内容
func (a *ContentAnalyzer) analyzeCodeContent(content *ContentInfo, filePath string) {
	ext := strings.ToLower(filepath.Ext(filePath))
	content.Properties["language"] = a.getLanguageFromExtension(ext)
	content.Properties["syntax_highlightable"] = true
}

// getLanguageFromExtension 从扩展名获取编程语言
func (a *ContentAnalyzer) getLanguageFromExtension(ext string) string {
	languages := map[string]string{
		".go":   "Go",
		".js":   "JavaScript",
		".ts":   "TypeScript",
		".py":   "Python",
		".java": "Java",
		".cpp":  "C++",
		".c":    "C",
		".h":    "C/C++ Header",
		".cs":   "C#",
		".php":  "PHP",
		".rb":   "Ruby",
		".rs":   "Rust",
		".swift": "Swift",
		".kt":   "Kotlin",
		".scala": "Scala",
		".html": "HTML",
		".css":  "CSS",
		".sql":  "SQL",
		".sh":   "Shell",
		".json": "JSON",
		".xml":  "XML",
		".yaml": "YAML",
		".yml":  "YAML",
	}

	if lang, exists := languages[ext]; exists {
		return lang
	}

	return "Unknown"
}

// extractKeywords 提取关键词
func (a *ContentAnalyzer) extractKeywords(classification *ClassificationResult, content *ContentInfo) []string {
	var keywords []string

	// 从分类结果中提取关键词
	if classification != nil {
		keywords = append(keywords, classification.Tags...)
		keywords = append(keywords, classification.Category)
		if classification.Subcategory != "" {
			keywords = append(keywords, classification.Subcategory)
		}
	}

	// 从内容中提取关键词
	if content != nil {
		if content.Language != "" && content.Language != "unknown" {
			keywords = append(keywords, content.Language)
		}
		if content.Type != "" {
			keywords = append(keywords, content.Type)
		}
	}

	// 去重并返回
	return MergeTags(keywords)
}

// generateSummary 生成内容摘要
func (a *ContentAnalyzer) generateSummary(content *ContentInfo) string {
	if content.TextContent == "" {
		return ""
	}

	text := content.TextContent
	if len(text) > a.config.SummaryMaxLength {
		text = text[:a.config.SummaryMaxLength] + "..."
	}

	return fmt.Sprintf("这是一个%s文件，包含%d个单词，语言：%s",
		content.Type, content.WordCount, content.Language)
}

// checkSimilarity 检查相似性
func (a *ContentAnalyzer) checkSimilarity(ctx context.Context, classification *ClassificationResult) (*SimilarityInfo, error) {
	// 这里应该实现真正的相似性检测算法
	// 目前返回模拟结果
	similarity := &SimilarityInfo{
		SimilarFiles: []SimilarFile{
			{
				SHA1:     "example_sha1_1",
				Filename: "similar_file_1.txt",
				Score:    0.85,
				Reason:   "相同内容类型和标签",
			},
		},
		Score:     0.85,
		Algorithm: "tag_similarity",
		Threshold: a.config.SimilarityThreshold,
	}

	return similarity, nil
}

// GetAnalysisConfig 获取分析配置
func (a *ContentAnalyzer) GetAnalysisConfig() *AnalyzerConfig {
	return a.config
}

// UpdateAnalysisConfig 更新分析配置
func (a *ContentAnalyzer) UpdateAnalysisConfig(config *AnalyzerConfig) {
	a.config = config
}

// AnalyzeBatch 批量分析
func (a *ContentAnalyzer) AnalyzeBatch(ctx context.Context, files map[string]*multipart.FileHeader) ([]*AnalysisResult, error) {
	results := make([]*AnalysisResult, 0, len(files))

	for filePath, file := range files {
		result, err := a.AnalyzeFile(ctx, filePath, file)
		if err != nil {
			// 记录错误但继续处理其他文件
			continue
		}
		results = append(results, result)
	}

	return results, nil
}

// ExportAnalysisResult 导出分析结果
func (a *ContentAnalyzer) ExportAnalysisResult(result *AnalysisResult, format string) ([]byte, error) {
	switch format {
	case "json":
		return json.MarshalIndent(result, "", "  ")
	case "summary":
		return a.exportSummary(result), nil
	default:
		return nil, fmt.Errorf("unsupported export format: %s", format)
	}
}

// exportSummary 导出摘要
func (a *ContentAnalyzer) exportSummary(result *AnalysisResult) []byte {
	summary := fmt.Sprintf(`文件分析摘要
文件名: %s
大小: %d bytes
类型: %s / %s
分类: %s
标签: %s
关键词: %s
处理时间: %v
分析时间: %s
`,
		result.FileInfo.Name,
		result.FileInfo.Size,
		result.FileInfo.MimeType,
		result.Content.Type,
		result.Classification.Category,
		strings.Join(result.Classification.Tags, ", "),
		strings.Join(result.Keywords, ", "),
		result.ProcessingTime,
		result.AnalyzedAt.Format(time.RFC3339),
	)

	return []byte(summary)
}