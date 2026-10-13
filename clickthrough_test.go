package main

import (
	"fmt"
	"testing"

	"github.com/egoist/mygo/ui"
)

// 把面板上每个交互点一遍：先在不接任何回调的 sample() 上点（只要求不 panic、页面照常切换），
// 再接上全部回调，逐个核对调到了哪个、参数对不对。

// wire 接上所有回调，返回调用记录。
func wire(p *panel) *[]string {
	log := &[]string{}
	rec := func(f string, a ...any) { *log = append(*log, fmt.Sprintf(f, a...)) }
	p.SwitchAll = func(to int) { rec("SwitchAll(%d)", to) }
	p.SwitchOne = func(row, to int) { rec("SwitchOne(%d,%d)", row, to) }
	p.Refresh = func() { rec("Refresh") }
	p.Remove = func(row int) { rec("Remove(%d)", row) }
	p.Changed = func() { rec("Changed") }
	p.SetLogin = func(on bool) { rec("SetLogin(%v)", on) }
	p.SetHotkey = func(to int, acc string) { rec("SetHotkey(%d,%q)", to, acc) }
	p.SetHomeKit = func(on bool) { rec("SetHomeKit(%v)", on) }
	p.RenameComputer = func(i int, name string) { rec("RenameComputer(%d,%q)", i, name) }
	p.AddComputer = func() { rec("AddComputer") }
	p.RemoveComputer = func(i int) { rec("RemoveComputer(%d)", i) }
	p.CheckUpdate = func() { rec("CheckUpdate") }
	p.CopyDiagnostics = func(done func()) { rec("CopyDiagnostics"); done() }
	p.Open = func(url string) { rec("Open(%s)", url) }
	p.Quit = func() { rec("Quit") }
	return log
}

type walker struct {
	t   *testing.T
	tt  *ui.Tester
	p   *panel
	log *[]string // nil：没接回调
}

func (w *walker) click(s string) {
	w.t.Helper()
	if err := w.tt.Click(s); err != nil {
		w.t.Fatalf("%v; on screen: %q", err, w.tt.Texts())
	}
	w.tt.Frame()
}

// expect 核对最近一次回调；没接回调时不查。
func (w *walker) expect(want string) {
	w.t.Helper()
	if w.log == nil {
		return
	}
	if n := len(*w.log); n == 0 || (*w.log)[n-1] != want {
		w.t.Fatalf("want %s, calls %q", want, *w.log)
	}
}

func (w *walker) page(want int) {
	w.t.Helper()
	if w.p.Page != want {
		w.t.Fatalf("page %d, want %d", w.p.Page, want)
	}
}

func clickThrough(t *testing.T, wired bool) {
	p := sample()
	w := &walker{t: t, p: p}
	if wired {
		w.log = wire(p)
	}
	tt := ui.NewTester(p.view, width, height)
	w.tt = tt
	tt.Frame()

	// ── 主页 ──
	w.click("重新检测")
	w.expect("Refresh")
	w.click("PC") // 第一台显示器的 PC 段
	w.expect("SwitchOne(0,1)")
	if p.Rows[0].Current != 0 {
		t.Fatal("view must not change Current itself")
	}
	w.click("全部切到 PC") // 主按钮
	w.expect("SwitchAll(1)")
	w.click("全部切到 Mac") // 旁边的次要按钮
	w.expect("SwitchAll(0)")
	w.click("一起切换 DELL G3223Q") // 勾选圈
	w.expect("Changed")
	if p.Rows[0].Enabled {
		t.Fatal("check should toggle Enabled")
	}
	w.click("一起切换 DELL G3223Q")

	// 离线的那台可以移除。
	p.Rows[1].Online = false
	tt.Frame()
	w.click("移除 DELL U2723QX")
	w.expect("Remove(1)")
	p.Rows[1].Online = true

	// 没设接口时，“设置接口”直达设置页。
	p.Rows[0].Ports[1] = ""
	tt.Frame()
	w.click("设置接口")
	w.page(1)
	p.Rows[0].Ports[1] = "HDMI 1"
	w.click("返回")
	w.page(0)

	// ── 设置页（拉高，免得目标滚在视口外） ──
	tt.SetSize(width, 1200)
	w.click("设置")
	w.page(1)

	// 接线：选 G3223Q 上 Mac 的口。
	w.click("DELL G3223Q 上 Mac 的接口")
	w.click("HDMI 2")
	if p.Rows[0].Ports[0] != "HDMI 2" {
		t.Fatalf("port %q", p.Rows[0].Ports[0])
	}
	w.expect("Changed")

	// 改名：点 PC 那一行，全选，输入，回车。
	r, ok := tt.Find("电脑名")
	if !ok {
		t.Fatal("no name input")
	}
	tt.ClickAt(r.X+r.W/2, r.Y+r.H/2+39)
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("Win")
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if p.Computers[1] != "Win" {
		t.Fatalf("computers %q", p.Computers)
	}
	w.expect(`RenameComputer(1,"Win")`)

	// 悬停出现“设为本机”。
	tt.Move(r.X+r.W/2, r.Y+r.H/2+39)
	tt.Frame()
	w.click("设为本机")
	if p.Self != 1 {
		t.Fatalf("self %d", p.Self)
	}
	w.expect("Changed")
	p.Self = 0

	// 添加电脑；第三台出现后可以移除（本机不行）。
	w.click("添加电脑")
	w.expect("AddComputer")
	p.Computers = append(p.Computers, "NAS")
	tt.Frame()
	w.click("移除 NAS")
	w.expect("RemoveComputer(2)")
	if tt.HasText("移除 Mac") {
		t.Fatal("self must not be removable")
	}
	p.Computers = p.Computers[:2]
	tt.Frame()

	// 快捷键：录一组，再清掉。
	w.click("点击录制")
	if !tt.HasText("按下快捷键…") {
		t.Fatalf("not recording: %q", tt.Texts())
	}
	tt.Key(ui.Ctrl|ui.Alt|ui.Super, ui.KeyM)
	tt.Frame()
	w.expect(`SetHotkey(0,"Ctrl+Alt+Cmd+M")`)
	if p.Hotkeys[0] != "Ctrl+Alt+Cmd+M" {
		t.Fatalf("hotkeys %q", p.Hotkeys)
	}
	w.click("清除快捷键")
	w.expect(`SetHotkey(0,"")`)
	if p.Hotkeys[0] != "" {
		t.Fatalf("hotkeys %q", p.Hotkeys)
	}

	// Apple 家庭：打开后露出配对码。
	w.click("加入 Apple 家庭")
	w.expect("SetHomeKit(true)")
	if !p.HomeKit || !tt.HasText(p.HomeKitCode) {
		t.Fatalf("pairing code not shown: %q", tt.Texts())
	}

	// 通用。
	w.click("开机时启动")
	w.expect("SetLogin(true)")
	w.click("检查更新")
	w.expect("CheckUpdate")
	w.click("X @zerah_eth")
	w.expect("Open(https://x.com/zerah_eth)")
	w.click("GitHub")
	w.expect("Open(https://github.com/qzz0518/Moniflip)")
	w.click("复制诊断信息")
	w.expect("CopyDiagnostics")
	if wired && !tt.HasText("已复制") {
		t.Fatal("copy should confirm")
	}
	w.click("退出 Moniflip")
	w.expect("Quit")

	w.click("返回")
	w.page(0)

	// ── 没有显示器：说明卡片里的“重新检测” ──
	p.Rows = nil
	tt.SetSize(width, height)
	tt.Frame()
	hint, ok := tt.Find("确认显示器已开启 DDC/CI")
	if !ok {
		t.Fatalf("no empty state: %q", tt.Texts())
	}
	tt.ClickAt(hint.X+hint.W/2, hint.Y+hint.H+12+12) // 说明下方 12 点处的按钮
	tt.Frame()
	w.expect("Refresh") // 上一次调用是 Quit，这里只能是新的一次
	if wired {
		t.Logf("calls: %q", *w.log)
	}
}

func TestClickThroughUnwired(t *testing.T) { clickThrough(t, false) }
func TestClickThroughWired(t *testing.T)   { clickThrough(t, true) }
