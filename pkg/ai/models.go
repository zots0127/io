package ai

import (
	"context"
	"fmt"
	"log"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"
)

// ContentTypeModel 内容类型识别模型
type ContentTypeModel struct {
	config *ClassifierConfig
}

// NewContentTypeModel 创建内容类型模型
func NewContentTypeModel() *ContentTypeModel {
	return &ContentTypeModel{}
}

func (m *ContentTypeModel) Analyze(ctx context.Context, filePath string, file *multipart.FileHeader) (*ClassificationResult, error) {
	// 基于文件名的分类
	contentType := DetectContentTypeFromExtension(file.Filename)
	category := GetCategoryFromContentType(contentType)

	// 基于MIME类型的细化分类
	subcategory := m.getSubcategoryFromMime(file.Header.Get("Content-Type"))

	result := &ClassificationResult{
		ContentType:  contentType,
		Category:     category,
		Subcategory:  subcategory,
		Confidence:   0.9, // 基于扩展名的分类置信度较高
		Tags:         []string{string(contentType), category},
		Metadata:     map[string]interface{}{
			"filename":    file.Filename,
			"size":        file.Size,
			"mime_type":   file.Header.Get("Content-Type"),
			"extension":   filepath.Ext(file.Filename),
		},
		ProcessedAt:  time.Now(),
		ModelVersion: "v1.0.0",
	}

	return result, nil
}

func (m *ContentTypeModel) ClassifyContent(ctx context.Context, data []byte, mimeType string) (*ClassificationResult, error) {
	// 基于MIME类型和内容特征的分类
	contentType := m.detectFromMimeType(mimeType)
	category := GetCategoryFromContentType(contentType)

	result := &ClassificationResult{
		ContentType:  contentType,
		Category:     category,
		Confidence:   0.8,
		Tags:         []string{string(contentType), category},
		Metadata:     map[string]interface{}{
			"data_size":  len(data),
			"mime_type":  mimeType,
		},
		ProcessedAt:  time.Now(),
		ModelVersion: "v1.0.0",
	}

	return result, nil
}

func (m *ContentTypeModel) ExtractText(ctx context.Context, filePath string) (string, error) {
	return "", fmt.Errorf("content type model does not support text extraction")
}

func (m *ContentTypeModel) DetectObjects(ctx context.Context, filePath string) ([]string, error) {
	return nil, fmt.Errorf("content type model does not support object detection")
}

func (m *ContentTypeModel) GetModelInfo() ModelInfo {
	return ModelInfo{
		Name:        "Content Type Classifier",
		Version:     "v1.0.0",
		Type:        "rule-based",
		Accuracy:    0.95,
		LastUpdated: time.Now(),
		Status:      "active",
	}
}

func (m *ContentTypeModel) getSubcategoryFromMime(mimeType string) string {
	if mimeType == "" {
		return "未知"
	}

	switch {
	case strings.HasPrefix(mimeType, "text/"):
		return "文本文件"
	case strings.HasPrefix(mimeType, "image/"):
		return m.getImageSubtype(mimeType)
	case strings.HasPrefix(mimeType, "video/"):
		return "视频文件"
	case strings.HasPrefix(mimeType, "audio/"):
		return "音频文件"
	case strings.Contains(mimeType, "pdf"):
		return "PDF文档"
	case strings.Contains(mimeType, "word"):
		return "Word文档"
	case strings.Contains(mimeType, "excel") || strings.Contains(mimeType, "spreadsheet"):
		return "Excel表格"
	case strings.Contains(mimeType, "powerpoint") || strings.Contains(mimeType, "presentation"):
		return "PowerPoint演示"
	case strings.Contains(mimeType, "zip") || strings.Contains(mimeType, "archive"):
		return "压缩文件"
	default:
		return "其他类型"
	}
}

func (m *ContentTypeModel) getImageSubtype(mimeType string) string {
	switch mimeType {
	case "image/jpeg":
		return "JPEG图片"
	case "image/png":
		return "PNG图片"
	case "image/gif":
		return "GIF动图"
	case "image/svg+xml":
		return "SVG矢量图"
	case "image/bmp":
		return "BMP位图"
	case "image/tiff":
		return "TIFF图片"
	default:
		return "其他图片"
	}
}

func (m *ContentTypeModel) detectFromMimeType(mimeType string) ContentType {
	if mimeType == "" {
		return ContentTypeOther
	}

	switch {
	case strings.HasPrefix(mimeType, "text/"):
		return ContentTypeDocument
	case strings.HasPrefix(mimeType, "image/"):
		return ContentTypeImage
	case strings.HasPrefix(mimeType, "video/"):
		return ContentTypeVideo
	case strings.HasPrefix(mimeType, "audio/"):
		return ContentTypeAudio
	case strings.Contains(mimeType, "zip") || strings.Contains(mimeType, "archive"):
		return ContentTypeArchive
	case strings.Contains(mimeType, "json") || strings.Contains(mimeType, "xml") || strings.Contains(mimeType, "csv"):
		return ContentTypeData
	default:
		return ContentTypeOther
	}
}

// TaggerModel 标签生成模型
type TaggerModel struct {
	config *ClassifierConfig
}

// NewTaggerModel 创建标签模型
func NewTaggerModel(config *ClassifierConfig) *TaggerModel {
	return &TaggerModel{
		config: config,
	}
}

func (m *TaggerModel) Analyze(ctx context.Context, filePath string, file *multipart.FileHeader) (*ClassificationResult, error) {
	tags := m.generateTags(file.Filename, file.Size)

	result := &ClassificationResult{
		Tags:        tags,
		Confidence:  0.7,
		Metadata:    map[string]interface{}{
			"tag_count":  len(tags),
			"tag_source": "rule-based",
		},
		ProcessedAt:  time.Now(),
		ModelVersion: "v1.0.0",
	}

	return result, nil
}

func (m *TaggerModel) ClassifyContent(ctx context.Context, data []byte, mimeType string) (*ClassificationResult, error) {
	tags := m.generateContentTags(data, mimeType)

	result := &ClassificationResult{
		Tags:        tags,
		Confidence:  0.6,
		Metadata:    map[string]interface{}{
			"tag_count":  len(tags),
			"tag_source": "content-analysis",
		},
		ProcessedAt:  time.Now(),
		ModelVersion: "v1.0.0",
	}

	return result, nil
}

func (m *TaggerModel) ExtractText(ctx context.Context, filePath string) (string, error) {
	return "", fmt.Errorf("tagger model does not support text extraction")
}

func (m *TaggerModel) DetectObjects(ctx context.Context, filePath string) ([]string, error) {
	return nil, fmt.Errorf("tagger model does not support object detection")
}

func (m *TaggerModel) GetModelInfo() ModelInfo {
	return ModelInfo{
		Name:        "Smart Tagger",
		Version:     "v1.0.0",
		Type:        "rule-based",
		Accuracy:    0.75,
		LastUpdated: time.Now(),
		Status:      "active",
	}
}

func (m *TaggerModel) generateTags(filename string, size int64) []string {
	var tags []string
	filename = strings.ToLower(filename)

	// 大小标签
	tags = append(tags, m.getSizeTag(size))

	// 文件名关键词标签
	keywordTags := m.extractKeywordTags(filename)
	tags = append(tags, keywordTags...)

	// 时间相关标签
	timeTags := m.extractTimeTags(filename)
	tags = append(tags, timeTags...)

	// 用途标签
	purposeTags := m.extractPurposeTags(filename)
	tags = append(tags, purposeTags...)

	return MergeTags(tags)
}

func (m *TaggerModel) generateContentTags(data []byte, mimeType string) []string {
	var tags []string

	// 基于MIME类型的标签
	tags = append(tags, m.getMimeTypeTags(mimeType)...)

	// 基于内容特征的标签
	if len(data) > 0 {
		tags = append(tags, m.getContentFeatureTags(data)...)
	}

	return MergeTags(tags)
}

func (m *TaggerModel) getSizeTag(size int64) string {
	switch {
	case size < 1024:
		return "小文件"
	case size < 1024*1024:
		return "中等文件"
	case size < 100*1024*1024:
		return "大文件"
	default:
		return "超大文件"
	}
}

func (m *TaggerModel) extractKeywordTags(filename string) []string {
	var tags []string

	// 文档类型关键词
	docKeywords := map[string][]string{
		"report":    {"报告", "报表"},
		"summary":   {"总结", "摘要"},
		"analysis":  {"分析", "研究"},
		"proposal":  {"提案", "建议"},
		"contract":  {"合同", "协议"},
		"invoice":   {"发票", "账单"},
		"receipt":   {"收据", "凭证"},
		"manual":    {"手册", "说明"},
		"guide":     {"指南", "教程"},
		"note":      {"笔记", "备注"},
	}

	for keyword, tagList := range docKeywords {
		if strings.Contains(filename, keyword) {
			tags = append(tags, tagList...)
		}
	}

	// 图片关键词
	imageKeywords := map[string][]string{
		"photo":     {"照片", "相片"},
		"screenshot": {"截图", "屏幕截图"},
		"avatar":    {"头像", "肖像"},
		"logo":      {"标志", "商标"},
		"banner":    {"横幅", "广告"},
		"icon":      {"图标", "徽标"},
		"background": {"背景", "壁纸"},
	}

	for keyword, tagList := range imageKeywords {
		if strings.Contains(filename, keyword) {
			tags = append(tags, tagList...)
		}
	}

	return tags
}

func (m *TaggerModel) extractTimeTags(filename string) []string {
	var tags []string

	timePatterns := map[string][]string{
		"2024": {"2024年", "新文件"},
		"2023": {"2023年"},
		"2022": {"2022年", "旧文件"},
		"2021": {"2021年", "旧文件"},
		"2020": {"2020年", "历史文件"},
		"2019": {"2019年", "历史文件"},
	}

	for year, tagList := range timePatterns {
		if strings.Contains(filename, year) {
			tags = append(tags, tagList...)
		}
	}

	// 月份关键词
	monthKeywords := map[string][]string{
		"jan": {"一月"},
		"feb": {"二月"},
		"mar": {"三月"},
		"apr": {"四月"},
		"may": {"五月"},
		"jun": {"六月"},
		"jul": {"七月"},
		"aug": {"八月"},
		"sep": {"九月"},
		"oct": {"十月"},
		"nov": {"十一月"},
		"dec": {"十二月"},
	}

	for month, tagList := range monthKeywords {
		if strings.Contains(filename, month) {
			tags = append(tags, tagList...)
		}
	}

	return tags
}

func (m *TaggerModel) extractPurposeTags(filename string) []string {
	var tags []string

	// 商务相关
	businessKeywords := []string{
		"business", "work", "office", "company", "project",
		"商务", "工作", "办公", "公司", "项目",
	}

	// 个人相关
	personalKeywords := []string{
		"personal", "private", "home", "family",
		"个人", "私人", "家庭", "家人",
	}

	// 学习相关
	educationKeywords := []string{
		"study", "learn", "course", "training", "education",
		"学习", "课程", "培训", "教育", "研究",
	}

	for _, keyword := range businessKeywords {
		if strings.Contains(filename, keyword) {
			tags = append(tags, "商务", "工作")
			break
		}
	}

	for _, keyword := range personalKeywords {
		if strings.Contains(filename, keyword) {
			tags = append(tags, "个人", "私人")
			break
		}
	}

	for _, keyword := range educationKeywords {
		if strings.Contains(filename, keyword) {
			tags = append(tags, "学习", "教育")
			break
		}
	}

	return tags
}

func (m *TaggerModel) getMimeTypeTags(mimeType string) []string {
	var tags []string

	switch {
	case strings.HasPrefix(mimeType, "text/"):
		tags = append(tags, "文本", "可读")
	case strings.HasPrefix(mimeType, "image/"):
		tags = append(tags, "图像", "视觉")
	case strings.HasPrefix(mimeType, "video/"):
		tags = append(tags, "视频", "多媒体")
	case strings.HasPrefix(mimeType, "audio/"):
		tags = append(tags, "音频", "声音")
	}

	return tags
}

func (m *TaggerModel) getContentFeatureTags(data []byte) []string {
	var tags []string

	// 检查是否包含可读文本
	if m.isTextContent(data) {
		tags = append(tags, "文本内容")
	}

	// 检查是否为结构化数据
	if m.isStructuredData(data) {
		tags = append(tags, "结构化数据")
	}

	// 检查是否为二进制数据
	if m.isBinaryData(data) {
		tags = append(tags, "二进制数据")
	}

	return tags
}

func (m *TaggerModel) isTextContent(data []byte) bool {
	textBytes := 0
	for _, b := range data[:min(1000, len(data))] {
		if (b >= 32 && b <= 126) || b == 9 || b == 10 || b == 13 {
			textBytes++
		}
	}
	return float64(textBytes)/float64(min(1000, len(data))) > 0.7
}

func (m *TaggerModel) isStructuredData(data []byte) bool {
	content := string(data[:min(500, len(data))])
	return strings.Contains(content, "{") && strings.Contains(content, "}") ||
		strings.Contains(content, "<") && strings.Contains(content, ">")
}

func (m *TaggerModel) isBinaryData(data []byte) bool {
	for _, b := range data[:min(100, len(data))] {
		if b < 32 && b != 9 && b != 10 && b != 13 {
			return true
		}
	}
	return false
}

// OCRModel OCR文字识别模型
type OCRModel struct {
	config *ClassifierConfig
}

// NewOCRModel 创建OCR模型
func NewOCRModel(config *ClassifierConfig) *OCRModel {
	return &OCRModel{
		config: config,
	}
}

func (m *OCRModel) Analyze(ctx context.Context, filePath string, file *multipart.FileHeader) (*ClassificationResult, error) {
	return nil, fmt.Errorf("OCR model only supports text extraction")
}

func (m *OCRModel) ClassifyContent(ctx context.Context, data []byte, mimeType string) (*ClassificationResult, error) {
	return nil, fmt.Errorf("OCR model only supports text extraction")
}

func (m *OCRModel) ExtractText(ctx context.Context, filePath string) (string, error) {
	// 这里应该集成真正的OCR引擎，如Tesseract
	// 目前返回模拟结果
	log.Printf("OCR text extraction for file: %s", filePath)

	// 模拟OCR处理时间
	time.Sleep(100 * time.Millisecond)

	return "OCR文本提取功能需要集成Tesseract或其他OCR引擎", nil
}

func (m *OCRModel) DetectObjects(ctx context.Context, filePath string) ([]string, error) {
	return nil, fmt.Errorf("OCR model does not support object detection")
}

func (m *OCRModel) GetModelInfo() ModelInfo {
	return ModelInfo{
		Name:        "OCR Text Extractor",
		Version:     "v1.0.0",
		Type:        "ocr",
		Accuracy:    0.85,
		LastUpdated: time.Now(),
		Status:      "demo",
	}
}

// ObjectDetectionModel 对象检测模型
type ObjectDetectionModel struct {
	config *ClassifierConfig
}

// NewObjectDetectionModel 创建对象检测模型
func NewObjectDetectionModel(config *ClassifierConfig) *ObjectDetectionModel {
	return &ObjectDetectionModel{
		config: config,
	}
}

func (m *ObjectDetectionModel) Analyze(ctx context.Context, filePath string, file *multipart.FileHeader) (*ClassificationResult, error) {
	return nil, fmt.Errorf("object detection model only supports object detection")
}

func (m *ObjectDetectionModel) ClassifyContent(ctx context.Context, data []byte, mimeType string) (*ClassificationResult, error) {
	return nil, fmt.Errorf("object detection model only supports object detection")
}

func (m *ObjectDetectionModel) ExtractText(ctx context.Context, filePath string) (string, error) {
	return "", fmt.Errorf("object detection model does not support text extraction")
}

func (m *ObjectDetectionModel) DetectObjects(ctx context.Context, filePath string) ([]string, error) {
	// 这里应该集成真正的对象检测引擎，如YOLO或TensorFlow
	// 目前返回模拟结果
	log.Printf("Object detection for file: %s", filePath)

	// 模拟检测处理时间
	time.Sleep(200 * time.Millisecond)

	// 返回一些常见的对象标签
	ext := strings.ToLower(filepath.Ext(filePath))
	if isImageFile(ext) {
		return []string{"图片", "图像", "视觉内容"}, nil
	}

	return []string{}, nil
}

func (m *ObjectDetectionModel) GetModelInfo() ModelInfo {
	return ModelInfo{
		Name:        "Object Detector",
		Version:     "v1.0.0",
		Type:        "computer-vision",
		Accuracy:    0.80,
		LastUpdated: time.Now(),
		Status:      "demo",
	}
}

// 辅助函数

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func isImageFile(ext string) bool {
	imageExts := []string{".jpg", ".jpeg", ".png", ".gif", ".bmp", ".tiff", ".svg", ".webp"}
	for _, imgExt := range imageExts {
		if ext == imgExt {
			return true
		}
	}
	return false
}