// Moniflip：状态栏小工具，经 DDC/CI（VCP 0x60 输入源）一键把显示器切到另一台电脑。
// 点状态栏图标打开面板，右键是快捷菜单；也支持全局快捷键、moniflip:// 链接和命令行。
package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"
	"github.com/egoist/mygo/plugins/updater/native"
	"github.com/egoist/mygo/ui"
)

const panelWidth = 344

//go:embed assets/tray.png
var trayPNG []byte

var onMac = runtime.GOOS == "darwin"

func logf(format string, a ...any) {
	msg := fmt.Sprintf(format, a...)
	fmt.Fprintln(os.Stderr, msg)
	dir, err := mygo.App.Path(mygo.PathLogs)
	if err != nil {
		return
	}
	f, err := os.OpenFile(filepath.Join(dir, "moniflip.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s %s\n", time.Now().Format(time.DateTime), msg)
}

func main() {
	if len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "--mygo") && !strings.Contains(os.Args[1], "://") {
		attachConsole()
		os.Exit(cli(os.Args[1:]))
	}
	if !mygo.App.RequestSingleInstanceLock() {
		return
	}
	a := &app{cfg: loadConfig(), online: map[string]display{}}
	initLang(a.cfg)
	// 更新窗口的语言在启动时定下，改了界面语言要下次启动才跟上。跟随系统的中文界面交给插件按系统挑简繁体；
	// Windows 上 Locale 是区域格式，与显示语言不符时直接用界面语言。
	ul := lang
	if a.cfg.Language == "" && lang == "zh" && strings.HasPrefix(mygo.App.Locale(), "zh") {
		ul = ""
	}
	mygo.Use(native.New(updater.Options{Language: ul}))
	mygo.App.SetActivationPolicy(mygo.ActivationPolicyAccessory)
	if onMac { // Windows 上菜单栏属于每个窗口，不设
		mygo.App.SetMenu(appMenu())
	}
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) { a.quitting = true })
	mygo.App.OnSecondInstance(func(args []string, _ string) {
		if !slices.ContainsFunc(args, func(s string) bool { return strings.HasPrefix(strings.ToLower(s), "moniflip:") }) {
			mygo.App.WhenReady(a.show) // ready 之前 a.tray、a.win 还是 nil
		}
	})
	mygo.App.OnOpenURL(a.openURL)
	mygo.App.WhenReady(a.ready)
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

// app 是状态栏小工具：状态栏图标 + 无边框面板。字段只在主线程读写。
type app struct {
	cfg           *config
	online        map[string]display // 最近一次检测到的显示器，按 ID
	order         []string           // 面板里显示器的顺序（ID）
	p             panel
	win           *mygo.Window
	tray          *mygo.Tray
	hotkeys       []string     // 已注册的快捷键
	settingsWin   *mygo.Window // 独立的设置窗口，第一次打开时创建
	settingsEcho  bool         // 见 fitSettings
	hk            *homekit     // Apple 家庭配件；未开启为 nil
	hotkeysPaused bool         // 设置页打开时暂停全局快捷键
	pending       func()       // 检测进行中收到的链接，检测完再执行
	redetect      bool         // 检测进行中又有人要求检测
	switchGen     int          // 每次切换加一，过期的跳回提示作废
	detecting     bool
	quitting      bool
}

func (a *app) ready() {
	a.p.Version = mygo.App.Version()
	a.p.OpenAtLogin = mygo.App.OpenAtLogin()
	a.p.SwitchAll = func(to int) { a.switchAll(to, true) }
	a.p.SwitchOne = a.switchOne
	a.p.Refresh = func() { a.detect(false) }
	a.p.Remove = a.remove
	a.p.Changed = a.applyRows
	a.p.SetLogin = a.setLogin
	a.p.SetLanguage = a.setLanguage
	a.p.SetHotkey = a.setHotkey
	a.p.SetHomeKit = a.setHomeKit
	a.p.RenameComputer = func(i int, name string) { a.edit(a.cfg.renameComputer(i, strings.TrimSpace(name))) }
	a.p.AddComputer = func() { a.cfg.addComputer(); a.edit(nil) }
	a.p.RemoveComputer = func(i int) { a.edit(a.cfg.removeComputer(i)) }
	a.p.CheckUpdate = updater.CheckForUpdates
	a.p.CopyDiagnostics = a.copyDiagnostics
	a.p.OpenSettings = a.openSettings
	a.p.Quit = mygo.App.Quit

	var err error
	a.tray, err = mygo.NewTray(mygo.TrayOptions{Icon: trayIcon(), IconIsTemplate: true, ToolTip: "Moniflip"})
	if err != nil {
		log.Fatal("tray: ", err)
	}
	a.win = mygo.NewWindow(mygo.WindowOptions{
		Title:             "Moniflip",
		Width:             panelWidth,
		Height:            panelHeight(2),
		Hidden:            true,
		Frameless:         true,
		DisableResize:     true,
		DisableMinimize:   true,
		DisableMaximize:   true,
		DisableFullScreen: true,
		AlwaysOnTop:       true,
		SkipTaskbar:       true,
		Content:           ui.View(a.view),
	})
	a.win.OnBlur(a.win.Hide)
	a.win.OnClose(func(e *mygo.CloseEvent) {
		if !a.quitting {
			e.PreventDefault()
			a.win.Hide()
		}
	})
	popUp := func() {
		a.win.Hide()
		a.tray.SetMenu(a.menu())
		defer a.tray.SetMenu(nil)
		a.tray.PopUpMenu()
	}
	a.tray.OnClick(func() {
		if controlPressed() {
			popUp()
		} else {
			a.toggle()
		}
	})
	a.tray.OnRightClick(popUp)
	if !onMac {
		mygo.Theme.OnUpdated(func() { _ = a.tray.SetIcon(trayIcon(), true) })
	}
	mygo.Power.OnResume(func() { a.detect(true); a.registerHotkeys() })
	mygo.Screen.OnDisplaysChanged(func() { a.detect(true) })
	mygo.App.OnWillQuit(func(*mygo.QuitEvent) { a.stopHomeKit(); a.tray.Destroy() })
	mygo.App.OnActivate(func(bool) { a.show() }) // 在“应用程序”或聚焦搜索里再次打开

	a.refreshPanel()
	a.registerHotkeys()
	if a.cfg.HomeKit {
		a.startHomeKit()
	}
	// 等待配对期间定时刷新配对状态
	go func() {
		for range time.Tick(3 * time.Second) {
			a.win.Update(func() {
				if a.hk != nil && a.p.HomeKitPaired != a.hk.server.IsPaired() {
					a.refreshPanel()
				}
			})
		}
	}()
	a.detect(false)
	if !mygo.App.WasOpenedAtLogin() {
		// 状态栏图标刚建好时报告的位置不对（约 200ms 后才就位），等位置稳定再弹出面板
		go func() {
			last := a.tray.Bounds()
			for i := range 20 {
				time.Sleep(100 * time.Millisecond)
				b := a.tray.Bounds()
				if b == last && i >= 2 {
					break
				}
				last = b
			}
			a.win.Update(func() {
				if !a.win.IsVisible() {
					a.show()
				}
			})
		}()
	}
}

func (a *app) menu() *mygo.Menu {
	items := []*mygo.MenuItem{
		{Label: tr("打开面板", "Open Panel"), Click: func(*mygo.MenuItem, *mygo.Window) { a.show() }},
		{Label: tr("设置…", "Settings…"), Click: func(*mygo.MenuItem, *mygo.Window) { a.openSettings() }},
		mygo.Separator(),
	}
	for to, name := range a.cfg.Computers {
		items = append(items, &mygo.MenuItem{Label: fmt.Sprintf(tr("全部切到 %s", "Switch All to %s"), name), Click: func(*mygo.MenuItem, *mygo.Window) { a.switchAll(to, false) }})
	}
	return mygo.NewMenu(append(items, mygo.Separator(), &mygo.MenuItem{Role: mygo.RoleQuit, Label: tr("退出 Moniflip", "Quit Moniflip")}))
}

// appMenu 是 macOS 的菜单栏，只在设置窗口打开（普通 App 模式）时看得见。各项保留 Role，行为和快捷键
// 仍由系统处理；标题按界面语言写出（mygo 按 Role 给的默认标题只有英文），中文与 macOS 自己的用词一致。
// 改界面语言后重建。
func appMenu() *mygo.Menu {
	return mygo.NewMenu([]*mygo.MenuItem{
		{Role: mygo.RoleAppMenu, Submenu: []*mygo.MenuItem{
			{Role: mygo.RoleAbout, Label: tr("关于 Moniflip", "About Moniflip")},
			mygo.Separator(),
			{Role: mygo.RoleServices, Label: tr("服务", "Services"), Submenu: []*mygo.MenuItem{}}, // 空的子菜单由系统填上各项服务
			mygo.Separator(),
			{Role: mygo.RoleHide, Label: tr("隐藏 Moniflip", "Hide Moniflip")},
			{Role: mygo.RoleHideOthers, Label: tr("隐藏其他", "Hide Others")},
			{Role: mygo.RoleUnhide, Label: tr("全部显示", "Show All")},
			mygo.Separator(),
			{Role: mygo.RoleQuit, Label: tr("退出 Moniflip", "Quit Moniflip")},
		}},
		{Role: mygo.RoleEditMenu, Label: tr("编辑", "Edit"), Submenu: []*mygo.MenuItem{
			{Role: mygo.RoleUndo, Label: tr("撤销", "Undo")},
			{Role: mygo.RoleRedo, Label: tr("重做", "Redo")},
			mygo.Separator(),
			{Role: mygo.RoleCut, Label: tr("剪切", "Cut")},
			{Role: mygo.RoleCopy, Label: tr("拷贝", "Copy")},
			{Role: mygo.RolePaste, Label: tr("粘贴", "Paste")},
			{Role: mygo.RolePasteAndMatchStyle, Label: tr("粘贴并匹配样式", "Paste and Match Style")},
			{Role: mygo.RoleDelete, Label: tr("删除", "Delete")},
			{Role: mygo.RoleSelectAll, Label: tr("全选", "Select All")},
			mygo.Separator(),
			{Label: tr("语音", "Speech"), Submenu: []*mygo.MenuItem{
				{Role: mygo.RoleStartSpeaking, Label: tr("开始朗读", "Start Speaking")},
				{Role: mygo.RoleStopSpeaking, Label: tr("停止朗读", "Stop Speaking")},
			}},
		}},
		{Role: mygo.RoleWindowMenu, Label: tr("窗口", "Window"), Submenu: []*mygo.MenuItem{
			{Role: mygo.RoleMinimize, Label: tr("最小化", "Minimize")},
			{Role: mygo.RoleZoom, Label: tr("缩放", "Zoom")},
			mygo.Separator(),
			{Role: mygo.RoleFront, Label: tr("前置全部窗口", "Bring All to Front")},
		}},
	})
}

func (a *app) view(c *ui.Context) {
	a.p.view(c)
	if c.Shortcut(0, ui.KeyEscape) {
		a.win.Hide()
	}
}

// openSettings 打开独立的设置窗口。打开期间 macOS 上临时变成普通 App（有 Dock 图标、能 Cmd+Tab），
// 并暂停全局快捷键：否则录制已注册的组合键时，按键先被系统拿走。
func (a *app) openSettings() {
	a.win.Hide()
	if a.settingsWin == nil {
		opts := mygo.WindowOptions{
			Title:         tr("Moniflip 设置", "Moniflip Settings"),
			Width:         600,
			Height:        780, // 高度由 fitSettings 按内容定
			DisableResize: true,
			StateKey:      "settings-v7",
			TitleBarStyle: mygo.TitleBarHidden,
			// 红绿灯放在 52 高的顶栏里垂直居中，和“设置”标题对齐
			TrafficLightPosition: &mygo.Point{X: 20, Y: 19},
			Content:              ui.View(a.settingsView),
		}
		a.settingsWin = mygo.NewWindow(opts)
		a.settingsWin.OnClose(func(e *mygo.CloseEvent) {
			if !a.quitting {
				e.PreventDefault()
				a.settingsWin.Hide()
			}
		})
		a.settingsWin.OnHide(func() {
			if onMac {
				mygo.App.SetActivationPolicy(mygo.ActivationPolicyAccessory)
			}
			a.hotkeysPaused = false
			a.registerHotkeys()
		})
	}
	if onMac {
		mygo.App.SetActivationPolicy(mygo.ActivationPolicyRegular)
	}
	a.hotkeysPaused = true
	a.registerHotkeys()
	a.settingsWin.Show()
	a.settingsWin.Focus()
	mygo.App.Focus()
	a.detect(true)
}

func (a *app) settingsView(c *ui.Context) {
	if c.Shortcut(ui.Cmd, ui.KeyW) {
		a.settingsWin.Hide()
	}
	a.fitSettings(c, a.p.settingsWindow(c))
}

// fitSettings 让设置窗口的高度始终刚好放下全部内容：内容变长变短（开关 Apple 家庭、增删电脑）
// 窗口都跟着变，底边出了屏幕就整体上移，屏幕放不下时才滚动。fit 是按上一帧排版量出的，
// 所以每帧重画后补画一帧，让内容的变化在下一帧落到窗口上。
func (a *app) fitSettings(c *ui.Context, fit float32) {
	if a.settingsEcho = !a.settingsEcho; a.settingsEcho {
		c.AnimationFrame()
	}
	_, h := c.Size()
	if fit <= 0 || math.Abs(float64(fit-h)) < 1 {
		return
	}
	b := a.settingsWin.Bounds()
	wa := mygo.Screen.DisplayMatching(b).WorkArea
	nb := b
	nb.Height = min(b.Height+int(math.Ceil(float64(fit-h))), wa.Height)
	nb.Y = max(wa.Y, min(b.Y, wa.Y+wa.Height-nb.Height))
	if nb != b {
		a.settingsWin.SetBounds(nb)
	}
}

// invalidate 让面板和设置窗口都按最新状态重画。
func (a *app) invalidate() {
	a.win.Invalidate()
	if a.settingsWin != nil {
		a.settingsWin.Invalidate()
	}
}

func (a *app) toggle() {
	if a.win.IsVisible() {
		a.win.Hide()
	} else {
		a.show()
	}
}

// show 把面板放在状态栏图标旁边：macOS 在图标下方，底部任务栏则在图标上方。
func (a *app) show() {
	anchor := a.tray.Bounds()
	b := a.win.Bounds()
	if anchor.Width == 0 || anchor.Height == 0 {
		// 拿不到图标位置（如 Windows 图标藏在溢出区）时，放在主屏右上角或右下角
		area := mygo.Screen.PrimaryDisplay().WorkArea
		y := area.Y + 6
		if !onMac {
			y = area.Y + area.Height - b.Height - 6
		}
		a.win.SetPosition(area.X+area.Width-b.Width-6, y)
	} else {
		area := mygo.Screen.DisplayNearestPoint(mygo.Point{X: anchor.X + anchor.Width/2, Y: anchor.Y + anchor.Height/2}).WorkArea
		x := anchor.X + (anchor.Width-b.Width)/2
		y := anchor.Y + anchor.Height + 6
		if y+b.Height > area.Y+area.Height {
			y = anchor.Y - b.Height - 6
		}
		x = max(area.X, min(x, area.X+max(0, area.Width-b.Width)))
		y = max(area.Y, min(y, area.Y+max(0, area.Height-b.Height)))
		a.win.SetPosition(x, y)
	}
	if !a.p.Switching {
		a.p.Notice, a.p.NoticeError = "", false
	}
	a.p.Page = 0
	a.win.Show()
	a.win.Focus()
	a.detect(true)
}

func (a *app) notify(msg string, isErr bool) {
	a.p.Notice, a.p.NoticeError = msg, isErr
	a.invalidate()
}

// detect 在后台重新检测显示器（含当前输入），结果回到主线程更新面板。silent 时不显示检测状态。
func (a *app) detect(silent bool) {
	if a.detecting {
		a.redetect = true
		a.p.Loading = a.p.Loading || !silent
		return
	}
	a.detecting, a.p.Loading = true, !silent
	known := map[string][]code{}
	for id, m := range a.cfg.Monitors {
		if len(m.Inputs) > 0 {
			known[id] = m.Inputs
		}
	}
	go func() {
		ddc.Lock()
		ds := detectDisplays(known)
		ddc.Unlock()
		a.win.Update(func() {
			a.detecting, a.p.Loading = false, false
			a.online = map[string]display{}
			for _, d := range ds {
				a.online[d.ID] = d
				m := a.cfg.adopt(d)
				if len(d.Inputs) > 0 {
					m.Inputs = d.Inputs
				}
			}
			a.cfg.save()
			a.refreshPanel()
			if f := a.pending; f != nil {
				a.pending = nil
				f()
			}
			if a.redetect {
				a.redetect = false
				a.detect(true)
			}
		})
	}()
}

// refreshPanel 按检测结果和设置生成面板：检测到的显示器在前，设置里有但没检测到的标为离线。
func (a *app) refreshPanel() {
	cfg := a.cfg
	a.p.Computers, a.p.Self = slices.Clone(cfg.Computers), cfg.Self
	a.p.HomeKit, a.p.Language = cfg.HomeKit, cfg.Language
	a.p.Hotkeys = make([]string, len(cfg.Computers))
	for i, name := range cfg.Computers {
		a.p.Hotkeys[i] = cfg.Hotkeys[name]
	}
	var online, offline []string
	for id := range cfg.Monitors {
		if strings.HasPrefix(id, "legacy:") {
			continue
		}
		if _, ok := a.online[id]; ok {
			online = append(online, id)
		} else {
			offline = append(offline, id)
		}
	}
	slices.Sort(online)
	slices.Sort(offline)
	a.order = append(online, offline...)
	a.p.Rows = make([]monitorRow, len(a.order))
	for i, id := range a.order {
		m := cfg.Monitors[id]
		d, ok := a.online[id]
		r := monitorRow{ID: id, Name: m.Name, Portrait: m.Portrait, Online: ok, Enabled: m.Enabled, Current: cfg.computerOf(m, d.Current)}
		inputs := m.Inputs
		if len(inputs) == 0 {
			inputs = commonInputs
		}
		for _, in := range inputs {
			r.Options = append(r.Options, in.String())
		}
		for _, name := range cfg.Computers {
			p := m.Ports[name].String()
			r.Ports = append(r.Ports, p)
			if p != "" && !slices.Contains(r.Options, p) {
				r.Options = append(r.Options, p)
			}
		}
		a.p.Rows[i] = r
	}
	a.syncHomeKit()
	a.resize()
	a.invalidate()
}

// panelHeight 让主页不用滚动：顶栏、按钮和状态行约 138，每台显示器 76，至多 4 台的高度。
func panelHeight(n int) int {
	if n == 0 {
		n = 2 // 检测中的骨架屏是两张卡
	}
	return 138 + 76*min(n, 4)
}

func (a *app) resize() {
	h := panelHeight(len(a.p.Rows))
	if b := a.win.Bounds(); b.Height != h {
		a.win.SetSize(panelWidth, h)
		if a.win.IsVisible() && b.Y < a.tray.Bounds().Y { // 面板在图标上方：保持下沿不动
			a.win.SetPosition(b.X, b.Y+b.Height-h)
		}
	}
}

// applyRows 把面板上改的勾选、接线和“设为本机”写回设置。
func (a *app) applyRows() {
	if a.p.Self >= 0 && a.p.Self < len(a.cfg.Computers) {
		a.cfg.Self = a.p.Self
	}
	for _, r := range a.p.Rows {
		m := a.cfg.Monitors[r.ID]
		if m == nil {
			continue
		}
		m.Enabled = r.Enabled
		for i, name := range a.cfg.Computers {
			if i < len(r.Ports) {
				if c := parseCode(r.Ports[i]); c != 0 {
					m.Ports[name] = c
				} else {
					delete(m.Ports, name)
				}
			}
		}
	}
	a.cfg.save()
	a.refreshPanel()
}

// edit 应用一次设置改动（电脑增删改名），出错则提示。
func (a *app) edit(err error) {
	if err != nil {
		a.notify(err.Error(), true)
	}
	a.cfg.save()
	a.registerHotkeys()
	if a.hk != nil {
		a.startHomeKit() // 电脑列表变了要重建配件
	}
	a.refreshPanel()
}

func (a *app) remove(row int) {
	if row < 0 || row >= len(a.order) {
		return
	}
	delete(a.cfg.Monitors, a.order[row])
	a.cfg.save()
	a.refreshPanel()
}

func (a *app) switchAll(to int, fromPanel bool) {
	var ids []string
	for _, id := range a.order {
		if m := a.cfg.Monitors[id]; m.Enabled && a.online[id].ID != "" {
			ids = append(ids, id)
		}
	}
	a.run(ids, to, fromPanel && to != a.cfg.Self)
}

func (a *app) switchOne(row, to int) {
	if row >= 0 && row < len(a.order) {
		a.run([]string{a.order[row]}, to, false)
	}
}

// run 在后台切换并回读确认；hide 为真时成功后收起面板（把屏幕交给别的电脑时）。
func (a *app) run(ids []string, to int, hide bool) {
	if a.p.Switching {
		return
	}
	targets, err := plan(a.cfg, a.online, ids, to)
	if err != nil {
		if !a.win.IsVisible() {
			a.show() // 从菜单、快捷键触发时面板是收起的，弹出来让原因看得见
		}
		a.notify(err.Error(), true)
		return
	}
	name := a.cfg.Computers[to]
	a.switchGen++
	gen := a.switchGen
	a.p.Switching = true
	for i := range a.p.Rows {
		a.p.Rows[i].Busy = targets[a.p.Rows[i].ID] != 0
	}
	a.notify("", false)
	go func() {
		o, err := switchInputs(targets)
		a.win.Update(func() {
			a.p.Switching = false
			for i := range a.p.Rows {
				a.p.Rows[i].Busy = false
			}
			if err != nil {
				a.notify(err.Error(), true)
			} else {
				a.notify(fmt.Sprintf(tr("已切到 %s", "Switched to %s"), name), false)
				if hide {
					a.win.Hide()
				}
			}
			a.detect(true)
		})
		if err != nil {
			return
		}
		if back := bounced(targets, o); len(back) > 0 {
			logf("bounced back: %s", names(back, ", "))
			a.win.Update(func() {
				if gen != a.switchGen {
					return // 之后又切过，这次回读的结论作废
				}
				if !a.win.IsVisible() {
					a.show()
				}
				// 状态行只有一行：原因放前面，名字多了被省略也看得到
				a.notify(fmt.Sprintf(tr("%[2]s 可能在睡眠，%[1]s 又跳回了原来的输入", "%[2]s may be asleep, so %[1]s switched back"), names(back, tr("、", ", ")), name), true)
			})
		}
	}()
}

func (a *app) setLogin(on bool) {
	if err := mygo.App.SetOpenAtLogin(on); err != nil {
		a.p.OpenAtLogin = !on
		a.notify(tr("开机启动设置失败：", "Could not change Open at Login: ")+err.Error(), true)
		return
	}
	a.p.OpenAtLogin = on
}

// setLanguage 改界面语言（"" 为跟随系统）并立即生效：面板、设置窗口和菜单栏都按新语言重画，
// 右键菜单每次弹出时现建，自然跟上；只有更新窗口要等下次启动。
func (a *app) setLanguage(l string) {
	a.cfg.Language = l
	a.cfg.save()
	initLang(a.cfg)
	if onMac {
		mygo.App.SetMenu(appMenu())
	}
	if a.settingsWin != nil {
		a.settingsWin.SetTitle(tr("Moniflip 设置", "Moniflip Settings"))
	}
	if !a.p.Switching {
		a.p.Notice, a.p.NoticeError = "", false // 旧语言写的提示
	}
	a.refreshPanel()
}

// registerHotkeys 按设置注册全局快捷键：每台电脑一个，“全部切到这台”。
func (a *app) registerHotkeys() {
	for _, acc := range a.hotkeys {
		mygo.GlobalShortcut.Unregister(acc)
	}
	a.hotkeys = nil
	if a.hotkeysPaused {
		return
	}
	for to, name := range a.cfg.Computers {
		acc := a.cfg.Hotkeys[name]
		if acc == "" {
			continue
		}
		if err := mygo.GlobalShortcut.Register(acc, func() { a.switchAll(to, false) }); err != nil {
			logf("register hotkey %s: %v", acc, err)
			continue
		}
		a.hotkeys = append(a.hotkeys, acc)
	}
}

func (a *app) setHotkey(to int, acc string) {
	name := a.cfg.Computers[to]
	if a.cfg.Hotkeys == nil {
		a.cfg.Hotkeys = map[string]string{}
	}
	old, had := a.cfg.Hotkeys[name]
	if acc == "" {
		delete(a.cfg.Hotkeys, name)
	} else {
		a.cfg.Hotkeys[name] = acc
	}
	paused := a.hotkeysPaused
	a.hotkeysPaused = false // 暂停时什么都没注册，无法判断冲突：临时真注册一次
	a.registerHotkeys()
	if acc != "" && !slices.Contains(a.hotkeys, acc) {
		if had {
			a.cfg.Hotkeys[name] = old
		} else {
			delete(a.cfg.Hotkeys, name)
		}
		a.notify(fmt.Sprintf(tr("%s 已被其他程序占用，换一个试试", "%s is already used by another app. Try a different shortcut."), acc), true)
	}
	a.hotkeysPaused = paused
	a.registerHotkeys() // 设置页还开着时会再全部注销，录制框继续收得到按键
	a.cfg.save()
	a.refreshPanel()
}

func (a *app) setHomeKit(on bool) {
	a.cfg.HomeKit = on
	a.cfg.save()
	if on {
		a.startHomeKit()
	} else {
		a.stopHomeKit()
		unpairHomeKit()
	}
	a.refreshPanel()
}

// openURL 处理 moniflip://switch?to=PC[&monitor=名称或 ID 片段]。
func (a *app) openURL(raw string) {
	u, err := url.Parse(raw)
	if err != nil || u.Host != "switch" {
		return
	}
	act := func() {
		q := u.Query()
		to := slices.IndexFunc(a.cfg.Computers, func(n string) bool { return strings.EqualFold(n, q.Get("to")) })
		if to < 0 {
			a.show()
			a.notify(fmt.Sprintf(tr("没有叫 %s 的电脑", "No computer named %s"), q.Get("to")), true)
			return
		}
		part := strings.ToLower(q.Get("monitor"))
		if part == "" {
			a.switchAll(to, false)
			return
		}
		var ids []string
		for _, id := range a.order {
			if strings.Contains(strings.ToLower(id), part) {
				ids = append(ids, id)
			}
		}
		if len(ids) == 0 {
			a.show()
			a.notify(fmt.Sprintf(tr("找不到显示器 %s", "Monitor %s not found"), q.Get("monitor")), true)
			return
		}
		a.run(ids, to, false)
	}
	mygo.App.WhenReady(func() {
		if a.detecting { // 刚启动或正在检测：等检测完，否则显示器都还算离线
			a.pending = act
			return
		}
		act()
	})
}

func (a *app) copyDiagnostics(done func()) {
	wiring := wiringOf(a.cfg) // 在主线程拍快照，后台只读快照
	go func() {
		text := diagnostics(wiring)
		mygo.Clipboard.WriteText(text)
		a.win.Update(done)
	}()
}

// wiringOf 是各显示器接线的文字快照，给诊断信息用：按电脑的顺序写成 "Mac → DP, PC → HDMI 1"。
func wiringOf(cfg *config) map[string]string {
	w := map[string]string{}
	for id, m := range cfg.Monitors {
		var s []string
		for _, name := range cfg.Computers {
			if c := m.Ports[name]; c != 0 {
				s = append(s, name+" → "+c.String())
			}
		}
		w[id] = strings.Join(s, ", ")
		if len(s) == 0 {
			w[id] = "(not set)"
		}
	}
	return w
}

// diagnostics 汇总排查兼容性问题要的信息：显示器型号（序列号遮住大半）、当前输入、capabilities、接线设置。
// 用户会把它贴进公开的 issue，所以固定用英文、不带完整序列号。
func diagnostics(wiring map[string]string) string {
	ddc.Lock()
	ds := detectDisplays(nil)
	ddc.Unlock()
	var b strings.Builder
	fmt.Fprintf(&b, "Moniflip %s · %s/%s\n", mygo.App.Version(), runtime.GOOS, runtime.GOARCH)
	if len(ds) == 0 {
		b.WriteString("No DDC/CI-capable monitors found\n")
	}
	for _, d := range ds {
		cur, inputs, caps := d.Current.String(), joinCodes(d.Inputs), d.Caps
		if cur == "" {
			cur = "unknown"
		}
		if inputs == "" {
			inputs = "(none)"
		}
		if caps == "" {
			caps = "(unreadable)"
		}
		fmt.Fprintf(&b, "\n%s\n  current input: %s (0x%02X)\n  inputs: %s\n  capabilities: %s\n", maskID(d.ID), cur, byte(d.Current), inputs, caps)
		if w, ok := wiring[d.ID]; ok {
			fmt.Fprintf(&b, "  wiring: %s\n", w)
		}
	}
	return b.String()
}

// cli 实现命令行：list、switch、diagnose、version。
func cli(args []string) int {
	cfg := loadConfig()
	initLang(cfg)
	switch args[0] {
	case "list":
		ddc.Lock()
		ds := detectDisplays(nil)
		ddc.Unlock()
		if slices.Contains(args, "--json") {
			b, _ := json.MarshalIndent(ds, "", "  ")
			fmt.Println(string(b))
			return 0
		}
		for _, d := range ds {
			where := d.Current.String()
			if where == "" {
				where = tr("未知", "unknown")
			}
			if m := cfg.Monitors[d.ID]; m != nil {
				if i := cfg.computerOf(m, d.Current); i >= 0 {
					where += fmt.Sprintf(tr("（%s）", " (%s)"), cfg.Computers[i])
				}
			}
			fmt.Printf(tr("%s\t当前 %s\t可选 %s\n", "%s\tcurrent %s\tinputs %s\n"), d.ID, where, joinCodes(d.Inputs))
		}
		if len(ds) == 0 {
			fmt.Fprintln(os.Stderr, tr("没有检测到能用 DDC/CI 控制的显示器", "No DDC/CI-capable monitors found"))
			return 1
		}
		return 0
	case "switch":
		if len(args) < 2 {
			break
		}
		to := slices.IndexFunc(cfg.Computers, func(n string) bool { return strings.EqualFold(n, args[1]) })
		if to < 0 {
			fmt.Fprintf(os.Stderr, tr("没有叫 %s 的电脑（设置里有：%s）\n", "No computer named %s (configured: %s)\n"), args[1], strings.Join(cfg.Computers, ", "))
			return 2
		}
		known := map[string][]code{}
		for id, m := range cfg.Monitors {
			known[id] = m.Inputs
		}
		ddc.Lock()
		ds := detectDisplays(known)
		ddc.Unlock()
		online := map[string]display{}
		var ids []string
		for _, d := range ds {
			online[d.ID] = d
			m := cfg.adopt(d)
			if len(args) > 2 && strings.Contains(strings.ToLower(d.ID), strings.ToLower(args[2])) || len(args) == 2 && m.Enabled {
				ids = append(ids, d.ID)
			}
		}
		if len(args) > 2 && len(ids) == 0 {
			fmt.Fprintf(os.Stderr, tr("找不到显示器 %s\n", "Monitor %s not found\n"), args[2])
			return 2
		}
		targets, err := plan(cfg, online, ids, to)
		if err == nil {
			_, err = switchInputs(targets)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	case "diagnose":
		fmt.Print(diagnostics(wiringOf(cfg)))
		return 0
	case "version", "--version":
		fmt.Println("Moniflip", mygo.App.Version())
		return 0
	}
	fmt.Fprint(os.Stderr, tr(`用法：
  moniflip list [--json]             列出能用 DDC/CI 控制的显示器
  moniflip switch <电脑> [显示器]    把勾选的显示器（或 ID 含 <显示器> 的显示器）切到 <电脑>
  moniflip diagnose                  输出诊断信息，反馈问题时附上
  moniflip version                   显示版本号
也可以打开链接 moniflip://switch?to=PC
`, `Usage:
  moniflip list [--json]                list monitors that support DDC/CI
  moniflip switch <computer> [monitor]  switch the checked monitors (or those whose ID contains <monitor>) to <computer>
  moniflip diagnose                     print diagnostics for bug reports
  moniflip version                      print the version
You can also open moniflip://switch?to=PC
`))
	return 2
}

// trayIcon 是状态栏图标（assets/tray.svg 渲染的 32px 黑色模板图）：macOS 由系统着色；Windows 深色任务栏改成白色。
func trayIcon() []byte {
	if onMac || lightTaskbar() {
		return trayPNG
	}
	src, err := png.Decode(bytes.NewReader(trayPNG))
	if err != nil {
		return trayPNG
	}
	r := src.Bounds()
	img := image.NewNRGBA(r)
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			_, _, _, a := src.At(x, y).RGBA()
			img.SetNRGBA(x, y, color.NRGBA{R: 255, G: 255, B: 255, A: uint8(a >> 8)})
		}
	}
	var b bytes.Buffer
	_ = png.Encode(&b, img)
	return b.Bytes()
}
