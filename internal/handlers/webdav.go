package handlers

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"golang.org/x/net/webdav"
)

// GetFileList 获取文件列表
func GetFileList(c *gin.Context) {
	reqPath := c.Query("path")
	if reqPath == "" {
		reqPath = "/"
	}

	// 安全检查：防止路径遍历
	fullPath := filepath.Join(webdavRoot, filepath.Clean(reqPath))
	if !strings.HasPrefix(fullPath, webdavRoot) {
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
		relPath := filepath.Join(reqPath, entry.Name())
		files = append(files, FileInfo{
			Name:    entry.Name(),
			Path:    filepath.ToSlash(relPath),
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
		"path":  reqPath,
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

	fullPath := filepath.Join(webdavRoot, filepath.Clean(req.Path))
	if !strings.HasPrefix(fullPath, webdavRoot) {
		c.JSON(403, gin.H{"error": "禁止访问"})
		return
	}

	if err := os.MkdirAll(fullPath, 0755); err != nil {
		c.JSON(500, gin.H{"error": "创建文件夹失败"})
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

	fullPath := filepath.Join(webdavRoot, filepath.Clean(reqPath))
	if !strings.HasPrefix(fullPath, webdavRoot) || fullPath == webdavRoot {
		c.JSON(403, gin.H{"error": "禁止删除"})
		return
	}

	if err := os.RemoveAll(fullPath); err != nil {
		c.JSON(500, gin.H{"error": "删除失败"})
		return
	}

	c.JSON(200, gin.H{"success": true})
}

// UploadFile 上传文件
func UploadFile(c *gin.Context) {
	targetPath := c.PostForm("path")
	if targetPath == "" {
		targetPath = "/"
	}

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(400, gin.H{"error": "未找到上传文件"})
		return
	}

	fullPath := filepath.Join(webdavRoot, filepath.Clean(targetPath), file.Filename)
	if !strings.HasPrefix(fullPath, webdavRoot) {
		c.JSON(403, gin.H{"error": "禁止访问"})
		return
	}

	if err := c.SaveUploadedFile(file, fullPath); err != nil {
		c.JSON(500, gin.H{"error": "保存文件失败"})
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

	fullPath := filepath.Join(webdavRoot, filepath.Clean(reqPath))
	if !strings.HasPrefix(fullPath, webdavRoot) {
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
