package main

import (
	"os"
	"testing"
)

// 测试里的面板尺寸（点），与两台显示器时的窗口一致。
const width, height = 344, 290

// 测试按中文文案点击、断言；英文另有 TestEnglish。
func TestMain(m *testing.M) {
	lang = "zh"
	os.Exit(m.Run())
}

// sample 是测试用的面板数据（两台电脑、两台显示器）。
func sample() *panel {
	return &panel{
		Self: 0, Computers: []string{"Mac", "PC"},
		Rows: []monitorRow{
			{ID: "g", Name: "DELL G3223Q", Online: true, Enabled: true, Current: 0, Ports: []string{"DP", "HDMI 1"}, Options: []string{"DP", "HDMI 1", "HDMI 2"}},
			{ID: "u", Name: "DELL U2723QX", Portrait: true, Online: true, Enabled: true, Current: 1, Ports: []string{"USB-C", "DP"}, Options: []string{"USB-C", "DP", "HDMI 1"}},
		},
		Hotkeys:      []string{"", "Ctrl+Alt+Cmd+P"},
		Version:      "0.1.0",
		HomeKitCode:  "482-19-305",
		HomeKitSetup: setupURI("48219305", 31, "AB12"),
	}
}
