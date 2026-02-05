package handlers

import (
	"html/template"
	"path/filepath"
	"sync"

	"github.com/gin-gonic/gin"
)

var (
	templateCache *template.Template
	templateOnce  sync.Once
	templateDir   string
)

// InitTemplates 初始化模板
func InitTemplates(templatePath string) {
	templateDir = templatePath
}

// LoadTemplates 加载模板文件
func LoadTemplates() (*template.Template, error) {
	var err error
	templateOnce.Do(func() {
		// 解析所有模板文件
		allFiles := []string{
			filepath.Join(templateDir, "layouts", "master.html"),
			filepath.Join(templateDir, "partials", "navbar.html"),
			filepath.Join(templateDir, "partials", "footer.html"),
			filepath.Join(templateDir, "pages", "all.html"),
			filepath.Join(templateDir, "pages", "home.html"),
			filepath.Join(templateDir, "pages", "monitor.html"),
			filepath.Join(templateDir, "pages", "process.html"),
			filepath.Join(templateDir, "pages", "webdav.html"),
			filepath.Join(templateDir, "pages", "terminal.html"),
			filepath.Join(templateDir, "pages", "docker.html"),
			filepath.Join(templateDir, "pages", "comfyui.html"),
			filepath.Join(templateDir, "pages", "settings.html"),
		}

		templateCache, err = template.New("master.html").
			Funcs(template.FuncMap{}).
			ParseFiles(allFiles...)
	})
	return templateCache, err
}

// RenderPage 渲染页面（已废弃，直接使用gin的HTML方法）
func RenderPage(c *gin.Context, pageName string, data interface{}) {
	c.HTML(200, pageName+".html", data)
}
