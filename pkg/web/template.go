package web

import (
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// TemplateManager manages HTML templates
type TemplateManager struct {
	templates map[string]*template.Template
	funcMap   template.FuncMap
}

// NewTemplateManager creates a new template manager
func NewTemplateManager(templatePath string) (*TemplateManager, error) {
	tm := &TemplateManager{
		templates: make(map[string]*template.Template),
		funcMap:   createFuncMap(),
	}

	// Load templates
	err := tm.loadTemplates(templatePath)
	if err != nil {
		return nil, err
	}

	return tm, nil
}

// createFuncMap creates template function map
func createFuncMap() template.FuncMap {
	return template.FuncMap{
		"formatBytes":     formatBytes,
		"formatTime":      formatTime,
		"formatDuration":  formatDuration,
		"truncate":        truncate,
		"capitalize":      capitalize,
		"lower":           strings.ToLower,
		"upper":           strings.ToUpper,
		"contains":        strings.Contains,
		"hasPrefix":       strings.HasPrefix,
		"hasSuffix":       strings.HasSuffix,
		"replace":         strings.Replace,
		"join":            strings.Join,
		"split":           strings.Split,
		"safeHTML":        safeHTML,
		"safeCSS":         safeCSS,
		"safeJS":          safeJS,
		"safeURL":         safeURL,
		"add":             func(a, b int) int { return a + b },
		"sub":             func(a, b int) int { return a - b },
		"mul":             func(a, b int) int { return a * b },
		"div":             func(a, b int) int { return a / b },
		"eq":              func(a, b interface{}) bool { return a == b },
		"ne":              func(a, b interface{}) bool { return a != b },
		"lt":              func(a, b interface{}) bool { return a.(int) < b.(int) },
		"le":              func(a, b interface{}) bool { return a.(int) <= b.(int) },
		"gt":              func(a, b interface{}) bool { return a.(int) > b.(int) },
		"ge":              func(a, b interface{}) bool { return a.(int) >= b.(int) },
		"and":             func(a, b bool) bool { return a && b },
		"or":              func(a, b bool) bool { return a || b },
		"not":             func(a bool) bool { return !a },
		"dict":            dict,
		"list":            list,
		"first":           first,
		"last":            last,
		"len":             func(v interface{}) int { return len(v.([]interface{})) },
		"index":           index,
		"slice":           slice,
		"range":           createRange,
	}
}

// loadTemplates loads all templates from the template directory
func (tm *TemplateManager) loadTemplates(templatePath string) error {
	// Define template files
	templateFiles := []string{
		"layouts/base.html",
		"layouts/header.html",
		"layouts/footer.html",
		"layouts/sidebar.html",
		"dashboard.html",
		"files.html",
		"upload.html",
		"batch.html",
		"batch-upload.html",
		"monitoring.html",
		"config.html",
		"about.html",
		"partials/pagination.html",
		"partials/file-list.html",
		"partials/batch-progress.html",
		"partials/stats-card.html",
		"partials/alert.html",
	}

	// Load each template
	for _, file := range templateFiles {
		templateName := filepath.Base(file)
		templateName = strings.TrimSuffix(templateName, filepath.Ext(templateName))

		templatePath := filepath.Join(templatePath, file)
		tmpl, err := template.New(templateName).Funcs(tm.funcMap).ParseFiles(templatePath)
		if err != nil {
			// If file doesn't exist, create a basic template
			tmpl = template.New(templateName).Funcs(tm.funcMap)
			tmpl, _ = tmpl.Parse(getDefaultTemplate(templateName))
		}

		tm.templates[templateName] = tmpl
	}

	return nil
}

// Render renders a template with the given data
func (tm *TemplateManager) Render(c *gin.Context, templateName string, data gin.H) {
	tmpl, exists := tm.templates[templateName]
	if !exists {
		c.String(http.StatusInternalServerError, "Template not found: %s", templateName)
		return
	}

	// Add common data
	tm.addCommonData(c, data)

	err := tmpl.Execute(c.Writer, data)
	if err != nil {
		c.String(http.StatusInternalServerError, "Template execution error: %v", err)
		return
	}
}

// addCommonData adds common data to all templates
func (tm *TemplateManager) addCommonData(c *gin.Context, data gin.H) {
	// Add request context
	data["request"] = c.Request
	data["baseURL"] = tm.getBaseURL(c)
	data["currentPath"] = c.Request.URL.Path
	data["queryParams"] = c.Request.URL.Query()
	data["userAgent"] = c.Request.UserAgent()
	data["remoteAddr"] = c.ClientIP()

	// Add common template data
	if data["title"] == nil {
		data["title"] = "IO Storage System"
	}

	if data["basePath"] == nil {
		data["basePath"] = tm.getBasePath(c.Request.URL.Path)
	}

	// Add flash messages if available
	if flashes := tm.getFlashMessages(c); len(flashes) > 0 {
		data["flashes"] = flashes
	}
}

// getBaseURL gets the base URL for the request
func (tm *TemplateManager) getBaseURL(c *gin.Context) string {
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + c.Request.Host
}

// getBasePath gets the base path for templates
func (tm *TemplateManager) getBasePath(path string) string {
	if strings.HasPrefix(path, "/") {
		return ""
	}
	return "../"
}

// getFlashMessages gets flash messages from the session
func (tm *TemplateManager) getFlashMessages(c *gin.Context) []gin.H {
	// This would integrate with a session store
	// For now, return empty
	return []gin.H{}
}

// Helper functions for templates

// formatBytes formats bytes into human readable format
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return "< 1 KB"
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// formatTime formats time in a human readable format
func formatTime(t interface{}) string {
	switch v := t.(type) {
	case string:
		// Parse string time and format
		return v
	case interface{ String() string }:
		return v.String()
	default:
		return ""
	}
}

// formatDuration formats duration in human readable format
func formatDuration(seconds int64) string {
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	if seconds < 3600 {
		return fmt.Sprintf("%dm %ds", seconds/60, seconds%60)
	}
	if seconds < 86400 {
		hours := seconds / 3600
		minutes := (seconds % 3600) / 60
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	days := seconds / 86400
	hours := (seconds % 86400) / 3600
	return fmt.Sprintf("%dd %dh", days, hours)
}

// truncate truncates a string to the specified length
func truncate(s string, length int) string {
	if len(s) <= length {
		return s
	}
	if length <= 3 {
		return s[:length]
	}
	return s[:length-3] + "..."
}

// capitalize capitalizes the first letter of a string
func capitalize(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// safeHTML marks a string as safe HTML
func safeHTML(s string) template.HTML {
	return template.HTML(s)
}

// safeCSS marks a string as safe CSS
func safeCSS(s string) template.CSS {
	return template.CSS(s)
}

// safeJS marks a string as safe JavaScript
func safeJS(s string) template.JS {
	return template.JS(s)
}

// safeURL marks a string as safe URL
func safeURL(s string) template.URL {
	return template.URL(s)
}

// dict creates a dictionary for template use
func dict(values ...interface{}) (map[string]interface{}, error) {
	if len(values)%2 != 0 {
		return nil, fmt.Errorf("invalid dict call")
	}
	dict := make(map[string]interface{}, len(values)/2)
	for i := 0; i < len(values); i += 2 {
		key, ok := values[i].(string)
		if !ok {
			return nil, fmt.Errorf("dict keys must be strings")
		}
		dict[key] = values[i+1]
	}
	return dict, nil
}

// list creates a list for template use
func list(values ...interface{}) []interface{} {
	return values
}

// first returns the first element of a list
func first(list interface{}) interface{} {
	switch v := list.(type) {
	case []interface{}:
		if len(v) > 0 {
			return v[0]
		}
	case []string:
		if len(v) > 0 {
			return v[0]
		}
	}
	return nil
}

// last returns the last element of a list
func last(list interface{}) interface{} {
	switch v := list.(type) {
	case []interface{}:
		if len(v) > 0 {
			return v[len(v)-1]
		}
	case []string:
		if len(v) > 0 {
			return v[len(v)-1]
		}
	}
	return nil
}

// index returns the element at the specified index
func index(list interface{}, i int) interface{} {
	switch v := list.(type) {
	case []interface{}:
		if i >= 0 && i < len(v) {
			return v[i]
		}
	case []string:
		if i >= 0 && i < len(v) {
			return v[i]
		}
	}
	return nil
}

// slice returns a slice of the list
func slice(list interface{}, start, end int) interface{} {
	switch v := list.(type) {
	case []interface{}:
		if start >= 0 && end <= len(v) && start <= end {
			return v[start:end]
		}
	case []string:
		if start >= 0 && end <= len(v) && start <= end {
			return v[start:end]
		}
	}
	return nil
}

// createRange creates a range of numbers
func createRange(start, end int) []int {
	result := make([]int, end-start+1)
	for i := range result {
		result[i] = start + i
	}
	return result
}

// getDefaultTemplate returns a default template for missing templates
func getDefaultTemplate(templateName string) string {
	templates := map[string]string{
		"base": `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.title}}</title>
    <link href="{{.basePath}}/static/css/style.css" rel="stylesheet">
    <link href="https://cdn.jsdelivr.net/npm/bootstrap@5.1.3/dist/css/bootstrap.min.css" rel="stylesheet">
</head>
<body>
    {{template "header" .}}
    <div class="container-fluid">
        <div class="row">
            {{template "sidebar" .}}
            <main class="col-md-9 ms-sm-auto col-lg-10 px-md-4">
                {{template "content" .}}
            </main>
        </div>
    </div>
    {{template "footer" .}}
    <script src="https://cdn.jsdelivr.net/npm/bootstrap@5.1.3/dist/js/bootstrap.bundle.min.js"></script>
    <script src="{{.basePath}}/static/js/app.js"></script>
</body>
</html>
`,
		"dashboard": `
<div class="d-flex justify-content-between flex-wrap flex-md-nowrap align-items-center pt-3 pb-2 mb-3 border-bottom">
    <h1 class="h2">Dashboard</h1>
</div>
<div class="row">
    <div class="col-md-3">
        <div class="card text-white bg-primary">
            <div class="card-body">
                <h5 class="card-title">Total Files</h5>
                <p class="card-text fs-2">{{.stats.totalFiles}}</p>
            </div>
        </div>
    </div>
    <div class="col-md-3">
        <div class="card text-white bg-success">
            <div class="card-body">
                <h5 class="card-title">Storage Used</h5>
                <p class="card-text fs-2">{{.stats.totalSize}}</p>
            </div>
        </div>
    </div>
    <div class="col-md-3">
        <div class="card text-white bg-info">
            <div class="card-body">
                <h5 class="card-title">Today's Uploads</h5>
                <p class="card-text fs-2">{{.stats.todayUploads}}</p>
            </div>
        </div>
    </div>
    <div class="col-md-3">
        <div class="card text-white bg-warning">
            <div class="card-body">
                <h5 class="card-title">Active Batches</h5>
                <p class="card-text fs-2">{{.stats.activeBatches}}</p>
            </div>
        </div>
    </div>
</div>
`,
	}

	if template, exists := templates[templateName]; exists {
		return template
	}
	return fmt.Sprintf("<h1>%s</h1><p>Template not found</p>", strings.Title(templateName))
}