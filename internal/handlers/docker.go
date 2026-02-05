package handlers

import (
	"os/exec"
	"strings"

	"github.com/gin-gonic/gin"
)

// GetDockerContainers 获取Docker容器列表
func GetDockerContainers(c *gin.Context) {
	containers := getDockerContainers()
	c.JSON(200, containers)
}

// GetDockerImages 获取Docker镜像列表
func GetDockerImages(c *gin.Context) {
	images := getDockerImages()
	c.JSON(200, images)
}

// getDockerContainers 获取Docker容器列表
func getDockerContainers() []DockerContainer {
	var containers []DockerContainer

	cmd := exec.Command("docker", "ps", "-a", "--format", "{{.ID}}|{{.Names}}|{{.Image}}|{{.Status}}|{{.State}}|{{.Ports}}|{{.CreatedAt}}")
	output, err := cmd.Output()
	if err != nil {
		return containers
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 7)
		if len(parts) >= 5 {
			container := DockerContainer{
				ID:     parts[0],
				Name:   parts[1],
				Image:  parts[2],
				Status: parts[3],
				State:  parts[4],
			}
			if len(parts) >= 6 {
				container.Ports = parts[5]
			}
			if len(parts) >= 7 {
				container.Created = parts[6]
			}
			containers = append(containers, container)
		}
	}

	return containers
}

// getDockerImages 获取Docker镜像列表
func getDockerImages() []DockerImage {
	var images []DockerImage

	cmd := exec.Command("docker", "images", "--format", "{{.ID}}|{{.Repository}}|{{.Tag}}|{{.Size}}|{{.CreatedAt}}")
	output, err := cmd.Output()
	if err != nil {
		return images
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 5)
		if len(parts) >= 4 {
			image := DockerImage{
				ID:   parts[0],
				Repo: parts[1],
				Tag:  parts[2],
				Size: parts[3],
			}
			if len(parts) >= 5 {
				image.Created = parts[4]
			}
			images = append(images, image)
		}
	}

	return images
}
