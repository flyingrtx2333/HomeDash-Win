package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// GetComfyUIConfig 获取ComfyUI配置
func GetComfyUIConfig(c *gin.Context) {
	settings := loadSettings()
	c.JSON(200, gin.H{
		"serverUrl": settings.ComfyUIServerURL,
	})
}

// UpdateComfyUIConfig 更新ComfyUI配置
func UpdateComfyUIConfig(c *gin.Context) {
	var config struct {
		ServerURL string `json:"serverUrl"`
	}
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(400, gin.H{"error": "无效的请求数据"})
		return
	}

	settings := loadSettings()
	settings.ComfyUIServerURL = config.ServerURL
	saveSettings(settings)

	c.JSON(200, gin.H{"success": true})
}

// ExecuteComfyUIWorkflow 执行ComfyUI工作流
func ExecuteComfyUIWorkflow(c *gin.Context) {
	var req struct {
		Workflow map[string]interface{} `json:"workflow"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "无效的请求数据"})
		return
	}

	settings := loadSettings()
	if settings.ComfyUIServerURL == "" {
		c.JSON(400, gin.H{"error": "请先配置ComfyUI服务器地址"})
		return
	}

	promptId, err := submitComfyUIWorkflow(settings.ComfyUIServerURL, req.Workflow)
	if err != nil {
		c.JSON(500, gin.H{"error": "提交工作流失败: " + err.Error()})
		return
	}

	c.JSON(200, gin.H{"success": true, "promptId": promptId})
}

// GetComfyUIWorkflowStatus 获取ComfyUI工作流状态
func GetComfyUIWorkflowStatus(c *gin.Context) {
	promptId := c.Param("id")
	settings := loadSettings()
	if settings.ComfyUIServerURL == "" {
		c.JSON(400, gin.H{"error": "请先配置ComfyUI服务器地址"})
		return
	}

	status, err := getComfyUIWorkflowStatus(settings.ComfyUIServerURL, promptId)
	if err != nil {
		c.JSON(500, gin.H{"error": "查询状态失败: " + err.Error()})
		return
	}

	c.JSON(200, status)
}

// submitComfyUIWorkflow 提交ComfyUI工作流
func submitComfyUIWorkflow(serverURL string, workflow map[string]interface{}) (string, error) {
	// 构建请求体
	reqBody := map[string]interface{}{
		"prompt": workflow,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	// 发送POST请求
	resp, err := http.Post(serverURL+"/prompt", "application/json", strings.NewReader(string(jsonData)))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	// 提取prompt_id
	promptId, ok := result["prompt_id"].(string)
	if !ok {
		return "", fmt.Errorf("无法获取prompt_id")
	}

	return promptId, nil
}

// getComfyUIWorkflowStatus 查询ComfyUI工作流状态
func getComfyUIWorkflowStatus(serverURL, promptId string) (map[string]interface{}, error) {
	// 1. 先查询队列状态
	queueResp, err := http.Get(serverURL + "/queue")
	if err == nil {
		defer queueResp.Body.Close()
		if queueResp.StatusCode == http.StatusOK {
			var queueData map[string]interface{}
			if json.NewDecoder(queueResp.Body).Decode(&queueData) == nil {
				// 检查运行队列
				running, _ := queueData["queue_running"].([]interface{})
				for _, item := range running {
					itemData, ok := item.([]interface{})
					if !ok || len(itemData) < 2 {
						continue
					}
					// 队列格式: [序号, prompt_id, workflow, extra_info, output_nodes]
					itemPromptId, ok := itemData[1].(string)
					if ok && itemPromptId == promptId {
						return map[string]interface{}{
							"completed": false,
							"progress":  50,
							"message":   "执行中",
						}, nil
					}
				}

				// 检查等待队列
				pending, _ := queueData["queue_pending"].([]interface{})
				for _, item := range pending {
					itemData, ok := item.([]interface{})
					if !ok || len(itemData) < 2 {
						continue
					}
					itemPromptId, ok := itemData[1].(string)
					if ok && itemPromptId == promptId {
						return map[string]interface{}{
							"completed": false,
							"progress":  10,
							"message":   "等待执行",
						}, nil
					}
				}
			}
		}
	}

	// 2. 如果不在队列中，查询历史记录
	historyResp, err := http.Get(serverURL + "/history?max_items=64")
	if err != nil {
		return map[string]interface{}{
			"completed": false,
			"progress":  0,
			"message":   "查询失败",
		}, nil
	}
	defer historyResp.Body.Close()

	if historyResp.StatusCode != http.StatusOK {
		return map[string]interface{}{
			"completed": false,
			"progress":  0,
			"message":   "等待执行",
		}, nil
	}

	var history map[string]interface{}
	if err := json.NewDecoder(historyResp.Body).Decode(&history); err != nil {
		return map[string]interface{}{
			"completed": false,
			"progress":  0,
			"message":   "解析失败",
		}, nil
	}

	// 检查历史记录中是否有该prompt_id
	promptData, ok := history[promptId].(map[string]interface{})
	if !ok {
		return map[string]interface{}{
			"completed": false,
			"progress":  0,
			"message":   "等待执行",
		}, nil
	}

	// 检查状态
	statusData, ok := promptData["status"].(map[string]interface{})
	if ok {
		completed, _ := statusData["completed"].(bool)
		if completed {
			// 执行完成，提取图片
			outputs, _ := promptData["outputs"].(map[string]interface{})
			images := []map[string]string{}

			// 遍历所有节点的输出
			for nodeId, nodeOutput := range outputs {
				nodeData, ok := nodeOutput.(map[string]interface{})
				if !ok {
					continue
				}
				imagesData, ok := nodeData["images"].([]interface{})
				if !ok {
					continue
				}
				for _, imgData := range imagesData {
					img, ok := imgData.(map[string]interface{})
					if !ok {
						continue
					}
					filename, _ := img["filename"].(string)
					subfolder, _ := img["subfolder"].(string)
					imgType, _ := img["type"].(string)
					if filename != "" {
						// 构建图片URL
						imgURL := fmt.Sprintf("%s/view?filename=%s", serverURL, filename)
						if subfolder != "" {
							imgURL += fmt.Sprintf("&subfolder=%s", subfolder)
						}
						if imgType != "" {
							imgURL += fmt.Sprintf("&type=%s", imgType)
						}
						images = append(images, map[string]string{
							"url":      imgURL,
							"filename": filename,
							"nodeId":   nodeId,
						})
					}
				}
			}

			return map[string]interface{}{
				"completed": true,
				"progress":  100,
				"message":   "执行完成",
				"images":    images,
			}, nil
		}
	}

	return map[string]interface{}{
		"completed": false,
		"progress":  0,
		"message":   "等待执行",
	}, nil
}
