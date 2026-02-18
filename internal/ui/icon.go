//go:build windows

package ui

import (
	"github.com/lxn/walk"
)

// loadIcon 加载应用图标，保证返回非 nil 的 *walk.Icon
// walk 只支持 .ico 格式，不支持 PNG/JPG
// 后备链：app.ico → shell32 图标 → walk.IconApplication()（永不失败）
func loadIcon() *walk.Icon {
	// 1. 尝试加载项目根目录下的 app.ico
	if icon, err := walk.NewIconFromFile("logo.ico"); err == nil {
		return icon
	}

	// 2. 从 shell32.dll 取网络图标（index=14）
	if icon, err := walk.NewIconFromSysDLL("shell32.dll", 14); err == nil {
		return icon
	}

	// 3. 最终保底：walk 内置标准应用程序图标，永远不会返回 nil
	return walk.IconApplication()
}
