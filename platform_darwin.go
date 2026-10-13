package main

import (
	"github.com/ebitengine/purego"
	"github.com/egoist/mygo"
)

var cgEventSourceFlagsState func(state int32) uint64

func init() {
	lib, err := purego.Dlopen("/System/Library/Frameworks/CoreGraphics.framework/CoreGraphics", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		panic(err)
	}
	purego.RegisterLibFunc(&cgEventSourceFlagsState, lib, "CGEventSourceFlagsState")
}

// controlPressed 报告 Control 键是否按着：macOS 上 Control+点击等同右键，mygo 只认真正的右键。
func controlPressed() bool {
	return cgEventSourceFlagsState(0)&0x40000 != 0 // kCGEventSourceStateCombinedSessionState, kCGEventFlagMaskControl
}

// lightTaskbar 只在 Windows 上有意义；macOS 的状态栏图标是模板图，由系统着色。
func lightTaskbar() bool { return false }

// attachConsole 只在 Windows 上需要（GUI 程序默认没有控制台）。
func attachConsole() {}

// uiLocale 是系统的首选语言，如 "zh-Hans-CN"。
func uiLocale() string { return mygo.App.Locale() }
