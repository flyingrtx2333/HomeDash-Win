package handlers

import (
	"fmt"
	"homedash/internal/ui"

	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// GetDatabases 获取所有数据库列表
func GetDatabases(c *gin.Context) {
	databases := loadDatabases()
	c.JSON(200, databases)
}

// GetDatabase 获取单个数据库详情
func GetDatabase(c *gin.Context) {
	id := c.Param("id")
	databases := loadDatabases()

	for _, db := range databases {
		if db.ID == id {
			c.JSON(200, db)
			return
		}
	}

	c.JSON(404, gin.H{"error": "数据库不存在"})
}

// CreateDatabase 创建新数据库配置
func CreateDatabase(c *gin.Context) {
	var db Database
	if err := c.ShouldBindJSON(&db); err != nil {
		c.JSON(400, gin.H{"error": "无效的请求数据"})
		return
	}

	// 验证配置
	if db.Name == "" {
		c.JSON(400, gin.H{"error": "数据库名称不能为空"})
		return
	}
	if db.Type == "" {
		db.Type = "mysql" // 默认 MySQL
	}
	if db.Host == "" {
		db.Host = "localhost"
	}
	if db.Port == 0 {
		if db.Type == "mysql" {
			db.Port = 3306
		} else if db.Type == "redis" {
			db.Port = 6379
		} else if db.Type == "mongodb" {
			db.Port = 27017
		}
	}
	if db.Username == "" {
		c.JSON(400, gin.H{"error": "用户名不能为空"})
		return
	}

	// 生成ID和时间戳
	db.ID = uuid.New().String()[:8]
	db.CreatedAt = time.Now().UnixMilli()
	db.UpdatedAt = db.CreatedAt

	databases := loadDatabases()
	databases = append(databases, db)

	if err := saveDatabases(databases); err != nil {
		c.JSON(500, gin.H{"error": "保存失败: " + err.Error()})
		return
	}

	c.JSON(200, db)
}

// UpdateDatabase 更新数据库配置
func UpdateDatabase(c *gin.Context) {
	id := c.Param("id")
	var updated Database
	if err := c.ShouldBindJSON(&updated); err != nil {
		c.JSON(400, gin.H{"error": "无效的请求数据"})
		return
	}

	databases := loadDatabases()
	found := false
	for i, db := range databases {
		if db.ID == id {
			// 保留ID和时间戳
			updated.ID = databases[i].ID
			updated.CreatedAt = databases[i].CreatedAt
			updated.UpdatedAt = time.Now().UnixMilli()

			// 设置默认值
			if updated.Host == "" {
				updated.Host = "localhost"
			}
			if updated.Port == 0 {
				if updated.Type == "mysql" {
					updated.Port = 3306
				} else if updated.Type == "redis" {
					updated.Port = 6379
				} else if updated.Type == "mongodb" {
					updated.Port = 27017
				}
			}

			databases[i] = updated
			found = true
			break
		}
	}

	if !found {
		c.JSON(404, gin.H{"error": "数据库不存在"})
		return
	}

	if err := saveDatabases(databases); err != nil {
		c.JSON(500, gin.H{"error": "保存失败: " + err.Error()})
		return
	}

	c.JSON(200, updated)
}

// DeleteDatabase 删除数据库配置
func DeleteDatabase(c *gin.Context) {
	id := c.Param("id")

	databases := loadDatabases()
	newDatabases := make([]Database, 0)
	found := false

	for _, db := range databases {
		if db.ID == id {
			found = true
			continue
		}
		newDatabases = append(newDatabases, db)
	}

	if !found {
		c.JSON(404, gin.H{"error": "数据库不存在"})
		return
	}

	if err := saveDatabases(newDatabases); err != nil {
		c.JSON(500, gin.H{"error": "删除失败: " + err.Error()})
		return
	}

	c.JSON(200, gin.H{"success": true})
}

// TestConnection 测试数据库连接
func TestConnection(c *gin.Context) {
	id := c.Param("id")
	databases := loadDatabases()

	var db *Database
	for i := range databases {
		if databases[i].ID == id {
			db = &databases[i]
			break
		}
	}

	if db == nil {
		c.JSON(404, gin.H{"error": "数据库不存在"})
		return
	}

	if db.Type != "mysql" {
		c.JSON(400, gin.H{"error": "当前仅支持 MySQL 连接测试"})
		return
	}

	// 测试 MySQL 连接
	err := testMySQLConnection(db)
	if err != nil {
		c.JSON(200, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"success": true})
}

// testMySQLConnection 测试 MySQL 连接
func testMySQLConnection(db *Database) error {
	// 使用 mysql 命令测试连接
	args := []string{
		"-h", db.Host,
		"-P", fmt.Sprintf("%d", db.Port),
		"-u", db.Username,
		"-p" + db.Password,
		"-e", "SELECT 1",
	}

	cmd := ui.HideWindow("mysql", args...)
	if runtime.GOOS == "windows" {
		cmd = ui.HideWindow("mysql.exe", args...)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("连接失败: %s", string(output))
	}

	return nil
}

// ChangePassword 修改数据库密码
func ChangePassword(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		NewPassword string `json:"newPassword"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "无效的请求数据"})
		return
	}

	if req.NewPassword == "" {
		c.JSON(400, gin.H{"error": "新密码不能为空"})
		return
	}

	databases := loadDatabases()
	var db *Database
	for i := range databases {
		if databases[i].ID == id {
			db = &databases[i]
			break
		}
	}

	if db == nil {
		c.JSON(404, gin.H{"error": "数据库不存在"})
		return
	}

	if db.Type != "mysql" {
		c.JSON(400, gin.H{"error": "当前仅支持 MySQL 密码修改"})
		return
	}

	// 使用 ALTER USER 修改密码
	err := changeMySQLPassword(db, req.NewPassword)
	if err != nil {
		c.JSON(500, gin.H{"error": "修改密码失败: " + err.Error()})
		return
	}

	// 更新配置中的密码
	for i := range databases {
		if databases[i].ID == id {
			databases[i].Password = req.NewPassword
			databases[i].UpdatedAt = time.Now().UnixMilli()
			break
		}
	}

	if err := saveDatabases(databases); err != nil {
		c.JSON(500, gin.H{"error": "保存配置失败: " + err.Error()})
		return
	}

	c.JSON(200, gin.H{"success": true})
}

// changeMySQLPassword 修改 MySQL 密码
func changeMySQLPassword(db *Database, newPassword string) error {
	// 使用 mysql 命令执行 ALTER USER
	query := fmt.Sprintf("ALTER USER '%s'@'%s' IDENTIFIED BY '%s'", db.Username, db.Host, newPassword)
	args := []string{
		"-h", db.Host,
		"-P", fmt.Sprintf("%d", db.Port),
		"-u", db.Username,
		"-p" + db.Password,
		"-e", query,
	}

	cmd := ui.HideWindow("mysql", args...)
	if runtime.GOOS == "windows" {
		cmd = ui.HideWindow("mysql.exe", args...)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("修改密码失败: %s", string(output))
	}

	return nil
}

// ExportSQL 导出 SQL
func ExportSQL(c *gin.Context) {
	id := c.Param("id")
	databases := loadDatabases()

	var db *Database
	for i := range databases {
		if databases[i].ID == id {
			db = &databases[i]
			break
		}
	}

	if db == nil {
		c.JSON(404, gin.H{"error": "数据库不存在"})
		return
	}

	if db.Type != "mysql" {
		c.JSON(400, gin.H{"error": "当前仅支持 MySQL 导出"})
		return
	}

	// 使用 mysqldump 导出
	sqlData, err := exportMySQL(db)
	if err != nil {
		c.JSON(500, gin.H{"error": "导出失败: " + err.Error()})
		return
	}

	// 设置响应头，触发下载
	filename := fmt.Sprintf("%s_%s_%d.sql", db.Name, db.Username, time.Now().Unix())
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", "application/sql")
	c.Data(200, "application/sql", sqlData)
}

// exportMySQL 导出 MySQL 数据库
func exportMySQL(db *Database) ([]byte, error) {
	args := []string{
		"-h", db.Host,
		"-P", fmt.Sprintf("%d", db.Port),
		"-u", db.Username,
		"-p" + db.Password,
		"--default-character-set=utf8mb4", // 导出为 UTF-8，避免 Windows 下乱码
		db.Name,
	}

	cmd := ui.HideWindow("mysqldump.exe", args...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("mysqldump 执行失败: %s", string(output))
	}

	// Windows 下 mysqldump 可能仍输出 GBK，解码为 UTF-8
	outStr := decodeMySQLOutput(output)
	return []byte(outStr), nil
}

// ImportSQL 导入 SQL（需要严格确认）
func ImportSQL(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		SQL       string `json:"sql"`
		Confirmed bool   `json:"confirmed"` // 必须为 true
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "无效的请求数据"})
		return
	}

	if !req.Confirmed {
		c.JSON(400, gin.H{"error": "必须确认导入操作"})
		return
	}

	if req.SQL == "" {
		c.JSON(400, gin.H{"error": "SQL 内容不能为空"})
		return
	}

	databases := loadDatabases()
	var db *Database
	for i := range databases {
		if databases[i].ID == id {
			db = &databases[i]
			break
		}
	}

	if db == nil {
		c.JSON(404, gin.H{"error": "数据库不存在"})
		return
	}

	if db.Type != "mysql" {
		c.JSON(400, gin.H{"error": "当前仅支持 MySQL 导入"})
		return
	}

	// 导入 SQL
	err := importMySQL(db, req.SQL)
	if err != nil {
		c.JSON(500, gin.H{"error": "导入失败: " + err.Error()})
		return
	}

	c.JSON(200, gin.H{"success": true})
}

// importMySQL 导入 MySQL SQL
func importMySQL(db *Database, sql string) error {
	args := []string{
		"-h", db.Host,
		"-P", fmt.Sprintf("%d", db.Port),
		"-u", db.Username,
		"-p" + db.Password,
		db.Name,
	}

	cmd := ui.HideWindow("mysql", args...)
	if runtime.GOOS == "windows" {
		cmd = ui.HideWindow("mysql.exe", args...)
	}

	// 将 SQL 内容通过 stdin 输入
	cmd.Stdin = strings.NewReader(sql)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("导入失败: %s", string(output))
	}

	return nil
}

// GetBackups 获取备份列表
func GetBackups(c *gin.Context) {
	id := c.Param("id")
	databases := loadDatabases()

	var db *Database
	for i := range databases {
		if databases[i].ID == id {
			db = &databases[i]
			break
		}
	}

	if db == nil {
		c.JSON(404, gin.H{"error": "数据库不存在"})
		return
	}

	backupDir := db.BackupDir
	if backupDir == "" {
		// 默认备份目录：web/backups/{db_id}
		backupDir = filepath.Join(GetWebDir(), "backups", db.ID)
	}

	backups, err := listBackups(backupDir)
	if err != nil {
		c.JSON(500, gin.H{"error": "获取备份列表失败: " + err.Error()})
		return
	}

	c.JSON(200, backups)
}

// listBackups 列出备份文件
func listBackups(backupDir string) ([]BackupInfo, error) {
	var backups []BackupInfo

	if _, err := os.Stat(backupDir); os.IsNotExist(err) {
		return backups, nil
	}

	files, err := os.ReadDir(backupDir)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(file.Name()), ".sql") {
			continue
		}

		info, err := file.Info()
		if err != nil {
			continue
		}

		backups = append(backups, BackupInfo{
			Filename: file.Name(),
			Size:     info.Size(),
			ModTime:  info.ModTime().Unix(),
		})
	}

	return backups, nil
}

// CreateBackup 创建备份
func CreateBackup(c *gin.Context) {
	id := c.Param("id")
	databases := loadDatabases()

	var db *Database
	for i := range databases {
		if databases[i].ID == id {
			db = &databases[i]
			break
		}
	}

	if db == nil {
		c.JSON(404, gin.H{"error": "数据库不存在"})
		return
	}

	if db.Type != "mysql" {
		c.JSON(400, gin.H{"error": "当前仅支持 MySQL 备份"})
		return
	}

	backupDir := db.BackupDir
	if backupDir == "" {
		backupDir = filepath.Join(GetWebDir(), "backups", db.ID)
	}

	// 确保备份目录存在
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		c.JSON(500, gin.H{"error": "创建备份目录失败: " + err.Error()})
		return
	}

	// 导出 SQL
	sqlData, err := exportMySQL(db)
	if err != nil {
		c.JSON(500, gin.H{"error": "备份失败: " + err.Error()})
		return
	}

	// 保存备份文件
	filename := fmt.Sprintf("%s_%s_%d.sql", db.Name, db.Username, time.Now().Unix())
	backupPath := filepath.Join(backupDir, filename)

	if err := os.WriteFile(backupPath, sqlData, 0644); err != nil {
		c.JSON(500, gin.H{"error": "保存备份文件失败: " + err.Error()})
		return
	}

	c.JSON(200, gin.H{"success": true, "filename": filename})
}

// DeleteBackup 删除备份
func DeleteBackup(c *gin.Context) {
	id := c.Param("id")
	filename := c.Param("filename")

	databases := loadDatabases()
	var db *Database
	for i := range databases {
		if databases[i].ID == id {
			db = &databases[i]
			break
		}
	}

	if db == nil {
		c.JSON(404, gin.H{"error": "数据库不存在"})
		return
	}

	backupDir := db.BackupDir
	if backupDir == "" {
		backupDir = filepath.Join(GetWebDir(), "backups", db.ID)
	}

	// 防止路径遍历攻击
	if strings.Contains(filename, "..") || strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
		c.JSON(400, gin.H{"error": "无效的文件名"})
		return
	}

	backupPath := filepath.Join(backupDir, filename)
	if err := os.Remove(backupPath); err != nil {
		c.JSON(500, gin.H{"error": "删除备份失败: " + err.Error()})
		return
	}

	c.JSON(200, gin.H{"success": true})
}

// DownloadBackup 下载备份文件
func DownloadBackup(c *gin.Context) {
	id := c.Param("id")
	filename := c.Param("filename")

	databases := loadDatabases()
	var db *Database
	for i := range databases {
		if databases[i].ID == id {
			db = &databases[i]
			break
		}
	}

	if db == nil {
		c.JSON(404, gin.H{"error": "数据库不存在"})
		return
	}

	backupDir := db.BackupDir
	if backupDir == "" {
		backupDir = filepath.Join(GetWebDir(), "backups", db.ID)
	}

	// 防止路径遍历攻击
	if strings.Contains(filename, "..") || strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
		c.JSON(400, gin.H{"error": "无效的文件名"})
		return
	}

	backupPath := filepath.Join(backupDir, filename)
	c.File(backupPath)
}

// GetBackupConfig 获取备份配置
func GetBackupConfig(c *gin.Context) {
	id := c.Param("id")
	databases := loadDatabases()

	var db *Database
	for i := range databases {
		if databases[i].ID == id {
			db = &databases[i]
			break
		}
	}

	if db == nil {
		c.JSON(404, gin.H{"error": "数据库不存在"})
		return
	}

	c.JSON(200, gin.H{
		"backupDir":  db.BackupDir,
		"autoBackup": db.AutoBackup,
		"backupCron": db.BackupCron,
	})
}

// UpdateBackupConfig 更新备份配置
func UpdateBackupConfig(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		BackupDir  string `json:"backupDir"`
		AutoBackup bool   `json:"autoBackup"`
		BackupCron string `json:"backupCron"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "无效的请求数据"})
		return
	}

	databases := loadDatabases()
	found := false
	for i := range databases {
		if databases[i].ID == id {
			databases[i].BackupDir = req.BackupDir
			databases[i].AutoBackup = req.AutoBackup
			databases[i].BackupCron = req.BackupCron
			databases[i].UpdatedAt = time.Now().UnixMilli()
			found = true
			break
		}
	}

	if !found {
		c.JSON(404, gin.H{"error": "数据库不存在"})
		return
	}

	if err := saveDatabases(databases); err != nil {
		c.JSON(500, gin.H{"error": "保存配置失败: " + err.Error()})
		return
	}

	c.JSON(200, gin.H{"success": true})
}

// GetTables 获取数据库表列表
func GetTables(c *gin.Context) {
	id := c.Param("id")
	databases := loadDatabases()

	var db *Database
	for i := range databases {
		if databases[i].ID == id {
			db = &databases[i]
			break
		}
	}

	if db == nil {
		c.JSON(404, gin.H{"error": "数据库不存在"})
		return
	}

	if db.Type != "mysql" {
		c.JSON(400, gin.H{"error": "当前仅支持 MySQL 表列表查看"})
		return
	}

	// 获取表列表
	tables, err := getMySQLTables(db)
	if err != nil {
		c.JSON(500, gin.H{"error": "获取表列表失败: " + err.Error()})
		return
	}

	c.JSON(200, tables)
}

// getMySQLTables 获取 MySQL 数据库表列表
func getMySQLTables(db *Database) ([]TableInfo, error) {
	// 使用 mysql 命令查询表信息
	query := fmt.Sprintf("SELECT TABLE_NAME, TABLE_ROWS, IFNULL(DATA_LENGTH, 0), IFNULL(INDEX_LENGTH, 0), ENGINE, IFNULL(TABLE_COMMENT, '') FROM information_schema.TABLES WHERE TABLE_SCHEMA = '%s' ORDER BY TABLE_NAME", db.Name)
	args := []string{
		"-h", db.Host,
		"-P", fmt.Sprintf("%d", db.Port),
		"-u", db.Username,
		"-p" + db.Password,
		"--default-character-set=utf8mb4", // 避免 Windows 下中文注释乱码
		"-N",                              // 不输出列名
		"-s",                              // 静默模式，使用制表符分隔
		"-e", query,
	}

	cmd := ui.HideWindow("mysql", args...)
	if runtime.GOOS == "windows" {
		cmd = ui.HideWindow("mysql.exe", args...)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("查询失败: %s", string(output))
	}

	// 解码输出：Windows 下 mysql 客户端可能输出 GBK，需转为 UTF-8
	outputStr := decodeMySQLOutput(output)

	// 解析输出
	lines := strings.Split(strings.TrimSpace(outputStr), "\n")
	var tables []TableInfo

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// 使用制表符分割（mysql -N -s 输出使用制表符）
		parts := strings.Split(line, "\t")
		if len(parts) < 6 {
			continue
		}

		var rows int64
		var dataSize int64
		var indexSize int64

		fmt.Sscanf(parts[1], "%d", &rows)
		fmt.Sscanf(parts[2], "%d", &dataSize)
		fmt.Sscanf(parts[3], "%d", &indexSize)

		tables = append(tables, TableInfo{
			Name:      parts[0],
			Rows:      rows,
			DataSize:  dataSize,
			IndexSize: indexSize,
			Engine:    parts[4],
			Comment:   parts[5],
		})
	}

	return tables, nil
}

// decodeMySQLOutput 将 mysql 命令输出解码为 UTF-8（Windows 下多为 GBK）
func decodeMySQLOutput(raw []byte) string {
	if utf8.Valid(raw) {
		return string(raw)
	}
	if runtime.GOOS != "windows" {
		return string(raw)
	}
	// Windows 下常见为 GBK
	decoder := simplifiedchinese.GBK.NewDecoder()
	utf8Bytes, _, err := transform.Bytes(decoder, raw)
	if err != nil {
		return string(raw)
	}
	return string(utf8Bytes)
}
