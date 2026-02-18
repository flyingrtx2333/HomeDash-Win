//go:build windows

package ui

import (
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

// manifestXML 内嵌 Common Controls v6 激活清单
// 运行时写入临时文件并创建 Activation Context，解决 TTM_ADDTOOL failed 问题
// 这样 go run 和 go build 都无需外部 .syso / .manifest 文件
const manifestXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<assembly xmlns="urn:schemas-microsoft-com:asm.v1" manifestVersion="1.0">
  <assemblyIdentity version="1.0.0.0" processorArchitecture="*"
    name="HomeDash.Win" type="win32"/>
  <dependency>
    <dependentAssembly>
      <assemblyIdentity type="win32"
        name="Microsoft.Windows.Common-Controls"
        version="6.0.0.0"
        processorArchitecture="*"
        publicKeyToken="6595b64144ccf1df"
        language="*"/>
    </dependentAssembly>
  </dependency>
</assembly>`

var (
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	procCreateActCtxW  = kernel32.NewProc("CreateActCtxW")
	procActivateActCtx = kernel32.NewProc("ActivateActCtx")
)

// actCtxW 对应 Windows ACTCTXW 结构体
type actCtxW struct {
	cbSize              uint32
	dwFlags             uint32
	lpSource            *uint16
	wProcessorArch      uint16
	wLangId             uint16
	lpAssemblyDirectory *uint16
	lpResourceName      *uint16
	lpApplicationName   *uint16
	hModule             uintptr
}

func init() {
	activateCommonControls()
}

// activateCommonControls 在进程启动时激活 Common Controls v6
// walk 库需要此激活才能正常初始化 ToolTip 等控件
func activateCommonControls() {
	// 将 manifest 写入临时目录
	tmpDir := os.TempDir()
	manifestPath := filepath.Join(tmpDir, "homedash_cc6.manifest")

	if err := os.WriteFile(manifestPath, []byte(manifestXML), 0644); err != nil {
		return
	}

	src, err := syscall.UTF16PtrFromString(manifestPath)
	if err != nil {
		return
	}

	ctx := actCtxW{
		cbSize:   uint32(unsafe.Sizeof(actCtxW{})),
		lpSource: src,
	}

	// CreateActCtxW 返回 activation context handle
	hActCtx, _, _ := procCreateActCtxW.Call(uintptr(unsafe.Pointer(&ctx)))
	const invalidHandleValue = ^uintptr(0) // (HANDLE)-1
	if hActCtx == invalidHandleValue {
		return
	}

	// ActivateActCtx 激活，使后续创建的控件使用 Common Controls v6
	var cookie uintptr
	procActivateActCtx.Call(hActCtx, uintptr(unsafe.Pointer(&cookie)))
	// 故意不调用 DeactivateActCtx，让激活在整个进程生命周期内有效
}