package templates

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// TemplateLoader 模板加载器
type TemplateLoader struct {
	templates    map[string]string
	templateDir  string
	mu           sync.RWMutex
}

var (
	instance *TemplateLoader
	once     sync.Once
)

// GetTemplateLoader 获取单例模板加载器
func GetTemplateLoader() *TemplateLoader {
	once.Do(func() {
		instance = &TemplateLoader{
			templates: make(map[string]string),
		}
		instance.findTemplateDir()
		instance.loadTemplates()
	})
	return instance
}

// findTemplateDir 查找模板目录
func (tl *TemplateLoader) findTemplateDir() {
	// 尝试多个可能的模板目录位置
	possibleDirs := []string{
		"templates",
		"../templates",
		"../../templates",
		"./templates",
	}

	// 获取可执行文件所在目录
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		possibleDirs = append(possibleDirs, filepath.Join(exeDir, "templates"))
	}

	// 获取 home 目录下的 templates
	homeDir, _ := os.UserHomeDir()
	if homeDir != "" {
		possibleDirs = append(possibleDirs, filepath.Join(homeDir, ".neoray", "templates"))
	}

	for _, dir := range possibleDirs {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			// 检查是否有模板文件
			if files, err := filepath.Glob(filepath.Join(dir, "*.md")); err == nil && len(files) > 0 {
				tl.templateDir = dir
				return
			}
			// 检查子目录
			if files, err := filepath.Glob(filepath.Join(dir, "**", "*.md")); err == nil && len(files) > 0 {
				tl.templateDir = dir
				return
			}
		}
	}

	// 默认使用当前目录下的 templates
	tl.templateDir = "templates"
}

// loadTemplates 加载所有模板（获取锁）
func (tl *TemplateLoader) loadTemplates() {
	tl.mu.Lock()
	defer tl.mu.Unlock()
	tl.loadTemplatesLocked()
}

// loadTemplatesLocked 加载所有模板（调用者必须持有写锁）
func (tl *TemplateLoader) loadTemplatesLocked() {

	if tl.templateDir == "" {
		return
	}

	// 递归加载所有 .md 文件
	err := filepath.Walk(tl.templateDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if filepath.Ext(path) == ".md" {
			// 读取模板内容
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			// 计算相对路径作为 key
			relPath, err := filepath.Rel(tl.templateDir, path)
			if err != nil {
				relPath = filepath.Base(path)
			}
			// 使用正斜杠作为路径分隔符
			key := filepath.ToSlash(relPath)
			tl.templates[key] = string(content)
		}
		return nil
	})
	if err != nil {
		fmt.Printf("Warning: failed to load templates: %v\n", err)
	}
}

// GetTemplate 获取模板
func (tl *TemplateLoader) GetTemplate(name string) (string, bool) {
	tl.mu.RLock()
	defer tl.mu.RUnlock()
	content, ok := tl.templates[name]
	return content, ok
}

// RenderTemplate 渲染模板（简单的变量替换）
func (tl *TemplateLoader) RenderTemplate(name string, vars map[string]string) (string, error) {
	templateContent, ok := tl.GetTemplate(name)
	if !ok {
		return "", fmt.Errorf("template not found: %s", name)
	}

	// 简单的变量替换：{{ var }} -> value
	result := templateContent
	for key, value := range vars {
		placeholder := fmt.Sprintf("{{ %s }}", key)
		result = strings.ReplaceAll(result, placeholder, value)
		// 也支持 {{.key}} 格式
		placeholder = fmt.Sprintf("{{.%s}}", key)
		result = strings.ReplaceAll(result, placeholder, value)
	}

	// 处理简单的 include 指令：{% include 'path' %}
	result = tl.processIncludes(result)

	return result, nil
}

// processIncludes 处理 include 指令
func (tl *TemplateLoader) processIncludes(content string) string {
	// 简单的 include 处理
	lines := strings.Split(content, "\n")
	var result []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "{% include") && strings.HasSuffix(trimmed, "%}") {
			// 提取路径
			includePart := strings.TrimPrefix(trimmed, "{% include")
			includePart = strings.TrimSuffix(includePart, "%}")
			includePath := strings.Trim(strings.TrimSpace(includePart), "'\"")
			if includedContent, ok := tl.GetTemplate(includePath); ok {
				result = append(result, includedContent)
			}
		} else {
			result = append(result, line)
		}
	}
	return strings.Join(result, "\n")
}

// ListTemplates 列出所有可用的模板
func (tl *TemplateLoader) ListTemplates() []string {
	tl.mu.RLock()
	defer tl.mu.RUnlock()
	var templates []string
	for name := range tl.templates {
		templates = append(templates, name)
	}
	return templates
}

// SetTemplateDir 手动设置模板目录
func (tl *TemplateLoader) SetTemplateDir(dir string) {
	tl.mu.Lock()
	defer tl.mu.Unlock()
	tl.templateDir = dir
	tl.templates = make(map[string]string)
	tl.loadTemplatesLocked()
}
