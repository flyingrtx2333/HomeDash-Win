package handlers

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"golang.org/x/net/webdav"
)

// sanitizePath 清理并验证路径，防止路径遍历攻击
func sanitizePath(reqPath string) (string, error) {
	if reqPath == "" {
		reqPath = "/"
	}

	// 规范化路径
	cleanPath := filepath.Clean(reqPath)

	// 检查是否尝试向上遍历
	if strings.Contains(cleanPath, "..") {
		return "", filepath.ErrBadPattern
	}

	// 转换为正斜杠（Web 标准）
	cleanPath = filepath.ToSlash(cleanPath)

	// 确保路径以 / 开头
	if !strings.HasPrefix(cleanPath, "/") {
		cleanPath = "/" + cleanPath
	}

	return cleanPath, nil
}

// GetFileList 获取文件列表
func GetFileList(c *gin.Context) {
	reqPath := c.Query("path")

	// 安全检查：防止路径遍历
	safePath, err := sanitizePath(reqPath)
	if err != nil {
		c.JSON(403, gin.H{"error": "非法路径"})
		return
	}

	fullPath := filepath.Join(webdavRoot, filepath.FromSlash(safePath))

	// 验证完整路径是否在 webdavRoot 下
	absRoot, err := filepath.Abs(webdavRoot)
	if err != nil {
		c.JSON(500, gin.H{"error": "根目录错误"})
		return
	}
	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		c.JSON(500, gin.H{"error": "路径解析失败"})
		return
	}

	if !strings.HasPrefix(absPath, absRoot) {
		c.JSON(403, gin.H{"error": "禁止访问"})
		return
	}

	entries, err := os.ReadDir(fullPath)
	if err != nil {
		c.JSON(500, gin.H{"error": "读取目录失败: " + err.Error()})
		return
	}

	var files []FileInfo
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		relPath := filepath.ToSlash(filepath.Join(safePath, entry.Name()))
		files = append(files, FileInfo{
			Name:    entry.Name(),
			Path:    relPath,
			IsDir:   entry.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime().UnixMilli(),
		})
	}

	// 排序：文件夹在前，然后按名称排序
	sort.Slice(files, func(i, j int) bool {
		if files[i].IsDir != files[j].IsDir {
			return files[i].IsDir
		}
		return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
	})

	c.JSON(200, gin.H{
		"path":  safePath,
		"root":  webdavRoot,
		"files": files,
	})
}

// CreateDirectory 创建文件夹
func CreateDirectory(c *gin.Context) {
	var req struct {
		Path string `json:"path"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "无效的请求"})
		return
	}

	// 安全检查
	safePath, err := sanitizePath(req.Path)
	if err != nil {
		c.JSON(403, gin.H{"error": "非法路径"})
		return
	}

	fullPath := filepath.Join(webdavRoot, filepath.FromSlash(safePath))

	// 验证完整路径
	absRoot, _ := filepath.Abs(webdavRoot)
	absPath, _ := filepath.Abs(fullPath)
	if !strings.HasPrefix(absPath, absRoot) {
		c.JSON(403, gin.H{"error": "禁止访问"})
		return
	}

	// 验证文件夹名，禁止特殊字符
	folderName := filepath.Base(safePath)
	if folderName == "" || strings.ContainsAny(folderName, "/\\:*?\"<>|") {
		c.JSON(400, gin.H{"error": "无效的文件夹名"})
		return
	}

	if err := os.MkdirAll(fullPath, 0755); err != nil {
		c.JSON(500, gin.H{"error": "创建文件夹失败: " + err.Error()})
		return
	}

	c.JSON(200, gin.H{"success": true})
}

// DeleteFile 删除文件/文件夹
func DeleteFile(c *gin.Context) {
	reqPath := c.Query("path")
	if reqPath == "" || reqPath == "/" {
		c.JSON(400, gin.H{"error": "无效的路径"})
		return
	}

	// 安全检查
	safePath, err := sanitizePath(reqPath)
	if err != nil {
		c.JSON(403, gin.H{"error": "非法路径"})
		return
	}

	// 禁止删除根目录
	if safePath == "/" {
		c.JSON(403, gin.H{"error": "禁止删除根目录"})
		return
	}

	fullPath := filepath.Join(webdavRoot, filepath.FromSlash(safePath))

	// 验证完整路径
	absRoot, _ := filepath.Abs(webdavRoot)
	absPath, _ := filepath.Abs(fullPath)
	if !strings.HasPrefix(absPath, absRoot) || absPath == absRoot {
		c.JSON(403, gin.H{"error": "禁止删除"})
		return
	}

	if err := os.RemoveAll(fullPath); err != nil {
		c.JSON(500, gin.H{"error": "删除失败: " + err.Error()})
		return
	}

	c.JSON(200, gin.H{"success": true})
}

// UploadFile 上传文件
func UploadFile(c *gin.Context) {
	targetPath := c.PostForm("path")

	// 安全检查
	safePath, err := sanitizePath(targetPath)
	if err != nil {
		c.JSON(403, gin.H{"error": "非法路径"})
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(400, gin.H{"error": "未找到上传文件"})
		return
	}

	// 验证文件名
	filename := file.Filename
	if filename == "" || strings.ContainsAny(filename, "/\\:*?\"<>|") {
		c.JSON(400, gin.H{"error": "无效的文件名"})
		return
	}

	// 限制文件大小（50MB）
	if file.Size > 50*1024*1024 {
		c.JSON(400, gin.H{"error": "文件过大，最大 50MB"})
		return
	}

	fullPath := filepath.Join(webdavRoot, filepath.FromSlash(safePath), filename)

	// 验证完整路径
	absRoot, _ := filepath.Abs(webdavRoot)
	absPath, _ := filepath.Abs(fullPath)
	if !strings.HasPrefix(absPath, absRoot) {
		c.JSON(403, gin.H{"error": "禁止访问"})
		return
	}

	if err := c.SaveUploadedFile(file, fullPath); err != nil {
		c.JSON(500, gin.H{"error": "保存文件失败: " + err.Error()})
		return
	}

	c.JSON(200, gin.H{"success": true})
}

// DownloadFile 下载文件
func DownloadFile(c *gin.Context) {
	reqPath := c.Query("path")
	if reqPath == "" {
		c.JSON(400, gin.H{"error": "无效的路径"})
		return
	}

	// 安全检查
	safePath, err := sanitizePath(reqPath)
	if err != nil {
		c.JSON(403, gin.H{"error": "非法路径"})
		return
	}

	fullPath := filepath.Join(webdavRoot, filepath.FromSlash(safePath))

	// 验证完整路径
	absRoot, _ := filepath.Abs(webdavRoot)
	absPath, _ := filepath.Abs(fullPath)
	if !strings.HasPrefix(absPath, absRoot) {
		c.JSON(403, gin.H{"error": "禁止访问"})
		return
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		c.JSON(404, gin.H{"error": "文件不存在"})
		return
	}

	if info.IsDir() {
		c.JSON(400, gin.H{"error": "不能下载文件夹"})
		return
	}

	c.FileAttachment(fullPath, filepath.Base(fullPath))
}

// GetWebdavHandler 获取WebDAV处理器
func GetWebdavHandler() *webdav.Handler {
	return &webdav.Handler{
		Prefix:     "/webdav",
		FileSystem: webdav.Dir(webdavRoot),
		LockSystem: webdav.NewMemLS(),
	}
}
