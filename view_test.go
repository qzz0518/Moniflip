package main

import (
	"image/png"
	"os"
	"testing"
	"unicode"

	"github.com/egoist/mygo/ui"
)

// 额外状态的设计稿（out/extra/），go test 时顺带出图。
func extra(t *testing.T, name string, p *panel, dark bool, h int, act func(tt *ui.Tester)) *ui.Tester {
	t.Helper()
	tt := ui.NewTester(p.view, width, h)
	tt.SetScale(2)
	tt.SetDark(dark)
	tt.Frame()
	if act != nil {
		act(tt)
	}
	tt.Frame()
	tt.Frame()
	if os.Getenv("MONIFLIP_SHOTS") == "" {
		return tt // 只在 MONIFLIP_SHOTS=1 go test 时出图到 out/extra/
	}
	os.MkdirAll("out/extra", 0o755)
	f, err := os.Create("out/extra/" + name + ".png")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	png.Encode(f, tt.Image())
	return tt
}

func TestExtraStates(t *testing.T) {
	// MONIFLIP_LANG=en 时出英文稿。
	if l := os.Getenv("MONIFLIP_LANG"); l != "" {
		defer func(old string) { lang = old }(lang)
		lang = l
	}
	full := sample()
	full.Page = 1
	full.HomeKit, full.HomeKitPaired = true, true
	extra(t, "settings-full-light", full, false, 1100, nil)
	extra(t, "settings-full-dark", full, true, 1100, nil)

	three := sample()
	three.Computers = append(three.Computers, "NAS")
	three.Hotkeys = append(three.Hotkeys, "")
	three.Rows[0].Ports = append(three.Rows[0].Ports, "HDMI 2")
	three.Rows[1].Ports = append(three.Rows[1].Ports, "")
	three.Rows = append(three.Rows, monitorRow{ID: "x", Name: "LG 27UK850-W", Online: true, Enabled: false, Current: -1,
		Ports: []string{"HDMI 1", "DP", "USB-C"}, Options: []string{"HDMI 1", "HDMI 2", "DP", "USB-C"}})
	extra(t, "three-light", three, false, height, nil)
	three.Page = 1
	extra(t, "three-settings-dark", three, true, 1100, nil)

	empty := sample()
	empty.Rows = nil
	extra(t, "empty-light", empty, false, height, nil)

	all := sample()
	all.SwitchAll = func(to int) {
		all.Switching = true
		for i := range all.Rows {
			all.Rows[i].Busy = all.Rows[i].Enabled && all.Rows[i].Online
		}
	}
	extra(t, "switch-all-light", all, false, height, func(tt *ui.Tester) { tt.Click(tr("全部切到 PC", "Switch to PC")) })

	done := sample()
	done.Rows[1].Current = 0
	done.Notice = "已全部切到 Mac"
	extra(t, "done-dark", done, true, height, nil)

	none := sample()
	none.Rows[0].Enabled, none.Rows[1].Enabled = false, false
	none.Rows[1].Current = -1
	none.Rows[0].Ports[1] = ""
	extra(t, "unchecked-light", none, false, height, nil)

	hover := sample()
	hover.Page = 1
	extra(t, "settings-hover-light", hover, false, height, func(tt *ui.Tester) {
		if r, ok := tt.Find(tr("电脑名", "Computer name")); ok {
			tt.Move(r.X+r.W/2, r.Y+r.H/2+39)
		}
	})

	rec := sample()
	rec.Page = 1
	extra(t, "recording-light", rec, false, 560, func(tt *ui.Tester) { tt.Click(tr("点击录制", "Record Shortcut")) })

	// 名字很长、三台电脑：分段和底部按钮都要收得住，不能挤出卡片。
	long := sample()
	long.Computers = []string{"MacBook Pro", "Gaming PC", "Mac mini"}
	long.Hotkeys = append(long.Hotkeys, "")
	long.Rows[0].Name = "DELL U3224KB UltraSharp 6K"
	long.Rows[0].Ports = append(long.Rows[0].Ports, "HDMI 2")
	long.Rows[1].Ports = append(long.Rows[1].Ports, "")
	extra(t, "long-names-light", long, false, height, nil)

	// 主按钮那台没设快捷键：状态行改提示卡片上的电脑名也能点。
	nokey := sample()
	nokey.Hotkeys = nil
	extra(t, "no-hotkey-light", nokey, false, height, nil)
}

// 英文界面：主页、设置页按真实尺寸渲染，文案都换成英文，不漏中文。
func TestEnglish(t *testing.T) {
	defer func(old string) { lang = old }(lang)
	lang = "en"
	expect := func(tt *ui.Tester, want ...string) {
		t.Helper()
		for _, s := range want {
			if !tt.HasText(s) {
				t.Errorf("no %q on screen: %q", s, tt.Texts())
			}
		}
		for _, s := range tt.Texts() {
			if s == "简体中文" { // 语言名用它自己的文字写
				continue
			}
			for _, r := range s {
				if unicode.Is(unicode.Han, r) {
					t.Errorf("untranslated %q", s)
					break
				}
			}
		}
	}

	home := sample()
	home.Rows[1].Online = false
	expect(extra(t, "en-home-light", home, false, height, nil), "Switch to PC", "Offline", "Remove DELL U2723QX", "Refresh", "Settings")

	set := sample()
	set.Page = 1
	set.HomeKit = true
	tt := extra(t, "en-settings-light", set, false, height, nil)
	expect(tt, "Settings", "Computers", "Add Computer", "This Computer")
	tt.SetSize(width, 1200) // 下面几组滚在视口外，拉高一并检查
	tt.Frame()
	expect(tt, "Inputs", "Shortcuts", "Record Shortcut", "Add to Apple Home", "Show in Home as", "Switches", "TV", "Waiting to pair", "Language", "System", "English", "Open at Login", "Quit Moniflip")
}

// 语言分段：点哪段就把对应的取值交给 SetLanguage。
func TestLanguagePicker(t *testing.T) {
	p := sample()
	p.Page = 1
	got := "unset"
	p.SetLanguage = func(l string) { got = l }
	tt := ui.NewTester(p.view, width, 1200)
	tt.Frame()
	for _, c := range []struct{ seg, want string }{{"English", "en"}, {"简体中文", "zh"}, {"跟随系统", ""}} {
		if err := tt.Click(c.seg); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		if got != c.want || p.Language != c.want {
			t.Errorf("click %s: SetLanguage(%q), Language %q; want %q", c.seg, got, p.Language, c.want)
		}
	}
}

// Apple 家庭的样子：接了回调时只交给 SetHomeKitAs（已配对时要先确认），没接时视图自己改。
func TestHomeKitAsPicker(t *testing.T) {
	p := sample()
	p.Page = 1
	p.HomeKit = true
	got := ""
	p.SetHomeKitAs = func(as string) { got = as }
	tt := ui.NewTester(p.view, width, 1200)
	tt.Frame()
	if !tt.HasText("每台电脑一个开关，可用于自动化和配件按钮") {
		t.Fatalf("switch hint missing: %q", tt.Texts())
	}
	if err := tt.Click("电视"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got != "tv" || p.HomeKitAs != "" {
		t.Fatalf("SetHomeKitAs(%q), HomeKitAs %q", got, p.HomeKitAs)
	}
	p.SetHomeKitAs = nil
	for _, c := range []struct{ seg, want, hint string }{{"电视", "tv", "一台电视，输入源就是各台电脑"}, {"开关", "switch", "每台电脑一个开关，可用于自动化和配件按钮"}} {
		if err := tt.Click(c.seg); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		if p.HomeKitAs != c.want || !tt.HasText(c.hint) {
			t.Errorf("click %s: HomeKitAs %q, texts %q", c.seg, p.HomeKitAs, tt.Texts())
		}
	}
}

func TestSwitchOne(t *testing.T) {
	p := sample()
	got := [2]int{-1, -1}
	p.SwitchOne = func(row, to int) { got = [2]int{row, to} }
	tt := ui.NewTester(p.view, width, height)
	tt.Frame()
	// 第一台显示的是 Mac，点它那一行的 PC。
	if err := tt.Click("PC"); err != nil { // 第一个 "PC" 是第一台的分段
		t.Fatal(err)
	}
	if got != [2]int{0, 1} {
		t.Fatalf("SwitchOne got %v", got)
	}
	if p.Rows[0].Current != 0 {
		t.Fatal("view must not change Current itself")
	}
}

func TestPrimaryTarget(t *testing.T) {
	p := sample()
	if to := p.target(); to != 1 { // 平手：送去另一台
		t.Fatalf("tie target %d", to)
	}
	p.Rows[0].Current, p.Rows[1].Current = 1, 1
	if to := p.target(); to != 0 { // 都在 PC：叫回本机
		t.Fatalf("all on PC target %d", to)
	}
	p.Computers = append(p.Computers, "NAS")
	p.Rows[0].Current, p.Rows[1].Current = 0, 0
	if to := p.target(); to != 1 { // 都在本机：下一台
		t.Fatalf("three, all here target %d", to)
	}
}

func TestSettingsAndRecorder(t *testing.T) {
	p := sample()
	var set []string
	p.SetHotkey = func(i int, acc string) { set = append(set, acc) }
	tt := ui.NewTester(p.view, width, height)
	tt.Frame()
	if err := tt.Click("设置"); err != nil {
		t.Fatal(err)
	}
	if p.Page != 1 {
		t.Fatal("gear should open settings")
	}
	tt.SetSize(width, 900) // 设置页要滚动，这里直接拉高
	tt.Frame()
	if err := tt.Click("点击录制"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("按下快捷键…") {
		t.Fatalf("not recording: %q", tt.Texts())
	}
	tt.Key(ui.Ctrl|ui.Alt|ui.Super, ui.KeyM)
	tt.Frame()
	if p.Hotkeys[0] != "Ctrl+Alt+Cmd+M" {
		t.Fatalf("hotkey %q (set %q)", p.Hotkeys[0], set)
	}
	// 同一组键给另一台：从 Mac 上拿掉。
	if err := tt.Click("全部切到 PC 的快捷键"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Key(ui.Ctrl|ui.Alt|ui.Super, ui.KeyM)
	tt.Frame()
	if p.Hotkeys[0] != "" || p.Hotkeys[1] != "Ctrl+Alt+Cmd+M" {
		t.Fatalf("hotkeys %q", p.Hotkeys)
	}
	if err := tt.Click("返回"); err != nil {
		t.Fatal(err)
	}
	if p.Page != 0 {
		t.Fatal("back should return")
	}
}

func TestRename(t *testing.T) {
	p := sample()
	var renamed string
	p.RenameComputer = func(i int, name string) { renamed = name }
	p.Page = 1
	tt := ui.NewTester(p.view, width, height)
	tt.Frame()
	r, ok := tt.Find("电脑名")
	if !ok {
		t.Fatal("no name input")
	}
	tt.ClickAt(r.X+r.W/2, r.Y+r.H/2+39) // 第二行：PC
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("Windows")
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if renamed != "Windows" || p.Computers[1] != "Windows" {
		t.Fatalf("renamed %q, computers %q", renamed, p.Computers)
	}
}

func TestAccelerator(t *testing.T) {
	cases := []struct {
		ev   ui.InputEvent
		want string
	}{
		{ui.InputEvent{Key: ui.KeyP, PhysicalKey: ui.KeyP, Mods: ui.Ctrl | ui.Alt | ui.Super}, "Ctrl+Alt+Cmd+P"},
		{ui.InputEvent{Key: ui.Key1, Mods: ui.Shift | ui.Alt}, "Alt+Shift+1"},
		{ui.InputEvent{Key: ui.KeyF5}, "F5"},
		{ui.InputEvent{Key: ui.KeyP}, ""},                 // 不带修饰键会吞掉打字
		{ui.InputEvent{Key: ui.KeyP, Mods: ui.Shift}, ""}, // 只有 Shift 也不行
		{ui.InputEvent{Key: ui.KeyUnknown, Mods: ui.Ctrl}, ""},
		{ui.InputEvent{Key: ui.KeyUnknown, PhysicalKey: ui.KeyS, Mods: ui.Ctrl}, "Ctrl+S"},
	}
	for _, c := range cases {
		if got := accelerator(c.ev); got != c.want {
			t.Errorf("accelerator(%+v) = %q, want %q", c.ev, got, c.want)
		}
	}
	if got := hotkeyLabel("Ctrl+Alt+Cmd+P"); got != "⌃⌥⌘P" && got != "Ctrl+Alt+Win+P" {
		t.Errorf("label %q", got)
	}
}
