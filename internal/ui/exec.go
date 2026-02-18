//go:build windows

package ui

import (
	"os/exec"
	"syscall"
)

// HideWindow 给任意 exec.Cmd 设置 CREATE_NO_WINDOW 标志
// 在 -H windowsgui 打包后，子进程默认会继承父进程的"无窗口"属性，
// 但某些情况下仍会闪现黑框，显式设置此标志可完全消除
//
// 用法：
//
//	cmd := ui.HideWindow("frpc", "-c", "frpc.toml")
//	ui.HideWindow(cmd)
//	cmd.Start()
func HideWindow(nameOrCmd interface{}, args ...string) *exec.Cmd {
	var cmd *exec.Cmd

	// 判断第一个参数类型
	switch v := nameOrCmd.(type) {
	case *exec.Cmd:
		// 传入 *exec.Cmd，直接设置
		cmd = v
	case string:
		// 传入命令字符串，构建 Cmd
		cmd = exec.Command(v, args...)
	default:
		// 不支持的类型，返回 nil（调用方需检查）
		return nil
	}

	// 设置隐藏窗口标志
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
	cmd.SysProcAttr.CreationFlags = 0x08000000 // CREATE_NO_WINDOW

	return cmd
}
