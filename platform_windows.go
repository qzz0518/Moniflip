package main

import (
	"os"
	"syscall"
	"unsafe"
)

var (
	advapi32          = syscall.NewLazyDLL("advapi32.dll")
	kernel32          = syscall.NewLazyDLL("kernel32.dll")
	procRegGetValueW  = advapi32.NewProc("RegGetValueW")
	procAttachConsole = kernel32.NewProc("AttachConsole")
	procGetUILanguage = kernel32.NewProc("GetUserDefaultUILanguage")
)

func controlPressed() bool { return false }

// lightTaskbar 报告任务栏是否为浅色（系统主题，与应用主题分开设置）。
func lightTaskbar() bool {
	key, _ := syscall.UTF16PtrFromString(`Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`)
	name, _ := syscall.UTF16PtrFromString("SystemUsesLightTheme")
	var v, n uint32 = 0, 4
	r, _, _ := procRegGetValueW.Call(0x80000001, uintptr(unsafe.Pointer(key)), uintptr(unsafe.Pointer(name)), 0x10, 0, // HKEY_CURRENT_USER, RRF_RT_REG_DWORD
		uintptr(unsafe.Pointer(&v)), uintptr(unsafe.Pointer(&n)))
	return r == 0 && v == 1
}

// attachConsole 让命令行模式的输出出现在启动它的 cmd/PowerShell 里（程序是 GUI 子系统，默认没有控制台）。
func attachConsole() {
	if r, _, _ := procAttachConsole.Call(uintptr(^uint32(0))); r == 0 { // ATTACH_PARENT_PROCESS
		return
	}
	f, err := os.OpenFile("CONOUT$", os.O_RDWR, 0)
	if err != nil {
		return
	}
	// 已重定向到文件或管道的保留原样，只有没有可用句柄的才接到控制台
	redirected := func(g *os.File) bool {
		if g == nil {
			return false
		}
		t, err := syscall.GetFileType(syscall.Handle(g.Fd()))
		return err == nil && (t == syscall.FILE_TYPE_DISK || t == syscall.FILE_TYPE_PIPE)
	}
	if !redirected(os.Stdout) {
		os.Stdout = f
	}
	if !redirected(os.Stderr) {
		os.Stderr = f
	}
}

// uiLocale 是 Windows 的显示语言，只分 "zh" 和 "en"：mygo.App.Locale() 在 Windows 上给的是区域格式，
// 英文系统把区域设成中国时也会是 "zh-CN"。
func uiLocale() string {
	if r, _, _ := procGetUILanguage.Call(); uint16(r)&0x3FF == 0x04 { // PRIMARYLANGID == LANG_CHINESE
		return "zh"
	}
	return "en"
}
