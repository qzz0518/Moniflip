package main

import (
	"fmt"
	"math"
	"time"

	"github.com/egoist/mygo/ui"
)

const (
	pad  = 14 // 面板左右留白
	rowH = 68 // 每台显示器卡片的高度
)

// palette 是面板的配色：在主题之上收得更克制，浅色、深色各一套。
type palette struct {
	bg, card, menu, line, hover ui.Color
	text, muted, faint          ui.Color
	accent, onAccent            ui.Color
	ok, bad, warn               ui.Color
	bezel                       ui.Color
	screen                      [2]ui.Color // 亮屏渐变（显示本机）：上、下
	away                        [2]ui.Color // 显示别的电脑时的屏幕：灰一些
	knob                        ui.Color    // 分段控件里选中的那段
	track                       ui.Color    // 关着的开关
	lift                        ui.Color    // 主按钮下沿的投影
	dark                        bool
}

func colors(t *ui.Theme) palette {
	white, black := ui.RGB(255, 255, 255), ui.RGB(0, 0, 0)
	if t.Dark {
		return palette{
			bg: ui.Hex("#1c1c1e"), card: ui.Hex("#2c2c2e"), menu: ui.Hex("#323234"),
			line: ui.RGBA(255, 255, 255, 0.08), hover: ui.RGBA(255, 255, 255, 0.08),
			text: ui.Hex("#f5f5f7"), muted: ui.Hex("#98989d"), faint: ui.Hex("#636366"),
			accent: t.Accent, onAccent: t.AccentText,
			ok: ui.Hex("#30d158"), bad: ui.Hex("#ff453a"), warn: ui.Hex("#ff9f0a"),
			bezel: ui.Hex("#8e8e93"), track: ui.RGBA(255, 255, 255, 0.2), lift: ui.RGBA(0, 0, 0, 0.4),
			screen: [2]ui.Color{t.Accent.Mix(black, 0.1), t.Accent.Mix(black, 0.5)},
			away:   [2]ui.Color{ui.Hex("#5a5a5f"), ui.Hex("#3a3a3d")},
			knob:   ui.Hex("#5c5c61"),
			dark:   true,
		}
	}
	return palette{
		bg: ui.Hex("#f5f5f3"), card: white, menu: white,
		line: ui.RGBA(0, 0, 0, 0.07), hover: ui.RGBA(0, 0, 0, 0.05),
		text: ui.Hex("#1d1d1f"), muted: ui.Hex("#6e6e73"), faint: ui.Hex("#aeaeb2"),
		accent: t.Accent, onAccent: t.AccentText,
		ok: ui.Hex("#34c759"), bad: ui.Hex("#ff3b30"), warn: ui.Hex("#ff9500"),
		bezel: ui.Hex("#3a3a3c"), track: ui.RGBA(0, 0, 0, 0.12), lift: ui.RGBA(0, 0, 0, 0.14),
		screen: [2]ui.Color{t.Accent.Mix(white, 0.6), t.Accent.Mix(white, 0.22)},
		away:   [2]ui.Color{ui.Hex("#e3e3e6"), ui.Hex("#c4c4c9")},
		knob:   white,
	}
}

func (p *panel) view(c *ui.Context) {
	k := colors(c.Theme())
	ui.Column(c).Fill().Background(k.bg).TextColor(k.text).FontSize(13).Children(func() {
		if p.Page == 1 {
			p.settings(c.Key("settings"), k)
			return
		}
		p.home(c.Key("home"), k)
	})
}

// openSettings 打开设置：真实程序里是独立窗口，测试和设计稿里翻到面板内的设置页。
func (p *panel) openSettings() {
	if p.OpenSettings != nil {
		p.OpenSettings()
		return
	}
	p.Page = 1
}

// 翻页：设置页从右边轻轻滑进来，主页从左边回来；离开的一页只淡出。
var (
	toSettings = ui.ElementTransition{Enter: &ui.Motion{X: 16}, Exit: &ui.Motion{}, Duration: 180 * time.Millisecond}
	toHome     = ui.ElementTransition{Enter: &ui.Motion{X: -16}, Exit: &ui.Motion{}, Duration: 180 * time.Millisecond}
)

func (p *panel) home(c *ui.Context, k palette) {
	ui.Column(c).Grow(1).Padding(10, pad, 0, pad).Transition(toHome).Children(func() {
		p.header(c, k)
		if len(p.Rows) == 0 && !p.Loading {
			// 没有显示器可切：不放主按钮，说明卡片占满，有结果时才留状态行。
			p.empty(c, k)
			if p.Notice != "" {
				p.status(c, k)
			}
			return
		}
		p.screens(c, k)
		p.actions(c, k)
		p.status(c, k)
	})
}

func (p *panel) header(c *ui.Context, k palette) {
	// 右侧留 7，让设置按钮与下方勾选圈对齐在同一条竖线上。
	ui.Row(c).Height(30).Gap(2).Padding(0, 7, 0, 0).Margin(0, 0, 6, 0).Children(func() {
		ui.Text(c, "Moniflip").FontSize(14).FontWeight(600).Grow(1)
		b := iconButton(c.Key("refresh"), k).Label(tr("重新检测", "Refresh")).Tooltip(tr("重新检测显示器", "Detect monitors again"))
		b.Children(func() {
			ic := ui.Icon(c, iconRefresh).FontSize(14)
			if p.Loading {
				ic.Rotate(ic.Loop("spin", time.Second, ui.Linear) * 360)
			}
		})
		b.OnClick(func() {
			if p.Refresh != nil && !p.Loading && !p.Switching {
				p.Refresh()
			}
		})
		g := iconButton(c.Key("settings"), k).Label(tr("设置", "Settings")).Tooltip(tr("设置", "Settings"))
		g.Children(func() { ui.Icon(c, iconGear).FontSize(15) })
		g.OnClick(func() { p.openSettings() })
	})
}

func (p *panel) screens(c *ui.Context, k palette) {
	// 两台正好放下；多于两台时卡片区自己滚，主按钮始终露在下面。
	// 左右借出 pad 的留白，卡片的阴影不会被滚动区裁掉。
	sc := ui.Scroll(c).Shrink(1).Margin(0, -pad).Padding(1, pad, 2, pad).Gap(8)
	st := ui.Local(sc, "scroll", func() ui.ScrollState { return ui.ScrollState{} })
	// 还有卡片藏在上下边外时：边缘淡出成底色，不硬切出半张卡片；再画一条细线，
	// 和设置页顶栏一样，表示内容还在线那边。
	sc.TrackScroll(st).DrawOver(func(pt *ui.Painter, r ui.Rect) {
		const fade = 16
		edge := ui.Rect{X: r.X + pad, W: r.W - 2*pad, H: 1}
		if st.Y > 0.5 {
			pt.FillGradient(ui.Rect{X: r.X, Y: r.Y, W: r.W, H: fade}, ui.LinearGradient{From: k.bg, To: k.bg.Alpha(0), Angle: 180}, 0)
			edge.Y = r.Y
			pt.Fill(edge, k.line, 0)
		}
		if st.Y < st.MaxY-0.5 {
			pt.FillGradient(ui.Rect{X: r.X, Y: r.Y + r.H - fade, W: r.W, H: fade}, ui.LinearGradient{From: k.bg.Alpha(0), To: k.bg, Angle: 180}, 0)
			edge.Y = r.Y + r.H - 1
			pt.Fill(edge, k.line, 0)
		}
	})
	sc.Children(func() {
		if len(p.Rows) == 0 {
			for i := range 2 {
				skeleton(c.Key(i), k, i == 1)
			}
			return
		}
		for i := range p.Rows {
			p.screen(c, k, i)
		}
	})
}

func (p *panel) empty(c *ui.Context, k palette) {
	ui.Column(c).Grow(1).Margin(0, 0, 14, 0).Center().Gap(4).Radius(12).Border(1, k.line).Children(func() {
		// 两台淡色显示器，底座落在同一条线上。
		ui.Row(c).AlignItems(ui.End).Gap(4).Margin(0, 0, 10, 0).Children(func() {
			for _, portrait := range []bool{false, true} {
				w, h := monitorSize(portrait)
				ui.Box(c).Size(w+6, h+standH).Draw(func(pt *ui.Painter, r ui.Rect) {
					drawMonitor(pt, r, k, portrait, glyphGhost)
				})
			}
		})
		ui.Text(c, tr("没有检测到显示器", "No monitors found")).FontWeight(600)
		ui.Text(c, tr("确认显示器已开启 DDC/CI", "Make sure DDC/CI is turned on")).FontSize(12).TextColor(k.muted)
		b := quietButton(c.Key("retry"), k).Margin(12, 0, 0, 0)
		b.Children(func() { ui.Text(c, tr("重新检测", "Try Again")).FontSize(12).FontWeight(500) })
		b.OnClick(func() {
			if p.Refresh != nil {
				p.Refresh()
			}
		})
	})
}

func card(c *ui.Context, k palette) ui.Element {
	e := ui.Row(c).Height(rowH).Padding(0, 10, 0, 8).Gap(10).Radius(12).Background(k.card)
	if k.dark {
		return e.InsetShadow(0, 0, 0, 1, k.line)
	}
	return e.Border(1, k.line).Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.04))
}

func (p *panel) screen(c *ui.Context, k palette, i int) {
	row := &p.Rows[i]
	// 按序号作键：接两台同型号显示器也不会撞键。
	card(c.Key(i), k).Children(func() {
		ui.Box(c).Size(44, 48).Draw(func(pt *ui.Painter, r ui.Rect) {
			drawMonitor(pt, r, k, row.Portrait, p.glyph(row))
		})
		ui.Column(c).Grow(1).MinWidth(0).Gap(6).Children(func() {
			ui.Row(c).Gap(6).Children(func() {
				name := ui.Text(c, row.Name).FontWeight(600).SingleLine().MinWidth(0)
				if !row.Online {
					name.TextColor(k.muted)
					ui.Text(c, tr("离线", "Offline")).FontSize(10.5).FontWeight(600).TextColor(k.bad).
						Padding(1, 5).Radius(4).Background(k.bad.Alpha(0.12))
				}
			})
			if !row.Online {
				ui.Text(c, tr("检测不到这台显示器", "This monitor isn’t detected")).FontSize(12).TextColor(k.muted)
				return
			}
			ui.Row(c).Gap(8).Children(func() {
				p.picker(c, k, i)
				p.hint(c, k, row)
			})
		})
		switch {
		case !row.Online:
			b := quietButton(c.Key("remove"), k).Label(fmt.Sprintf(tr("移除 %s", "Remove %s"), row.Name)).Tooltip(tr("不再显示这台显示器", "Stop showing this monitor"))
			b.Children(func() { ui.Text(c, tr("移除", "Remove")).FontSize(12).FontWeight(500) })
			b.OnClick(func() {
				if p.Remove != nil {
					p.Remove(i)
				}
			})
		case row.Busy:
			ui.Box(c).Size(20, 20).Justify(ui.Center).Children(func() { spinner(c, 16, k.accent) })
		default:
			b := check(c.Key("on"), k, &row.Enabled).Label(fmt.Sprintf(tr("一起切换 %s", "Include %s when switching all"), row.Name)).Tooltip(tr("参与“全部切到”", "Include when switching all"))
			if b.Changed() && p.Changed != nil {
				p.Changed()
			}
		}
	})
}

// glyph 让显示器图形自己说话：显示本机时屏幕亮起强调色，显示别的电脑时是灰的。
func (p *panel) glyph(row *monitorRow) glyphState {
	switch {
	case !row.Online:
		return glyphOff
	case row.Busy:
		return glyphBusy
	case row.Current == p.Self:
		return glyphOn
	case row.Current >= 0:
		return glyphAway
	}
	return glyphBlank
}

// port 是第 to 台电脑接在这台显示器的哪个口，没设置时为 ""。
func (row *monitorRow) port(to int) string {
	if to < 0 || to >= len(row.Ports) {
		return ""
	}
	return row.Ports[to]
}

// picker 是这台显示器的电脑分段：选中的那段就是它此刻显示的电脑，点另一段把它切过去。
func (p *panel) picker(c *ui.Context, k palette, i int) {
	row := &p.Rows[i]
	sel := row.Current // 本帧的副本：选中态只跟着回读走，视图不自己改 Current
	seg := ui.SegmentedBase(c.Key("to"), &sel, len(p.Computers))
	// 切换进行中整组停用：点了也不会切，就别装作能点。名字太长时各段一起收窄、省略。
	track := seg.Track.Height(24).MinWidth(0).Shrink(1).Padding(2).Gap(2).Radius(7).Background(k.hover).
		Label(fmt.Sprintf(tr("%s 显示的电脑", "Computer shown on %s"), row.Name)).Disabled(row.Busy || p.Switching)
	track.Children(func() {
		for j, name := range p.Computers {
			s := seg.Segment(j).Height(20).MinWidth(44).Shrink(1).Padding(0, 9).Radius(5).Justify(ui.Center)
			port := row.port(j)
			ink, weight := k.muted, 500
			switch {
			case j == row.Current:
				s.Background(k.knob).Tooltip(fmt.Sprintf(tr("正在显示 %s · %s", "Showing %s · %s"), name, port))
				if !k.dark {
					s.Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.12)).Shadow(0, 0, 0, 0.5, ui.RGBA(0, 0, 0, 0.06))
				}
				ink, weight = k.text, 600
			case port == "":
				s.Disabled(true).Tooltip(fmt.Sprintf(tr("还没设置 %s 接在哪个口", "Input for %s isn’t set"), name))
				ink = k.faint
			default:
				s.Tooltip(fmt.Sprintf(tr("切到 %s · %s", "Switch to %s · %s"), name, port))
				if s.Hovered() && !p.Switching {
					s.Background(k.hover)
					ink = k.text
				}
			}
			s.Children(func() {
				ui.Text(c, name).FontSize(12).FontWeight(weight).TextColor(ink).SingleLine().MinWidth(0)
			})
		}
	})
	if track.Changed() && sel != row.Current && !p.Switching && p.SwitchOne != nil {
		p.SwitchOne(i, sel)
	}
}

// hint 在分段旁补一句：接口没设置时引去设置页；当前输入认不出来时轻轻标出。
func (p *panel) hint(c *ui.Context, k palette, row *monitorRow) {
	for j := range p.Computers {
		if row.port(j) == "" {
			b := ui.ButtonBase(c.Key("wire")).Padding(2, 4).Margin(0, 0, 0, -4).Radius(5).TextColor(k.warn).
				Tooltip(tr("去设置里选好每台电脑接的口", "Choose each computer’s input in Settings"))
			if b.Hovered() {
				b.Background(k.warn.Alpha(0.12))
			}
			b.Children(func() { ui.Text(c, tr("设置接口", "Set Inputs")).FontSize(11.5).FontWeight(500) })
			b.OnClick(func() { p.openSettings() })
			return
		}
	}
	if row.Current < 0 && !row.Busy {
		ui.Text(c, tr("未识别", "Unknown")).FontSize(11.5).TextColor(k.muted).
			Tooltip(tr("当前输入不是任何一台电脑的接口，或者读不出来", "The current input doesn’t match any computer, or couldn’t be read"))
	}
}

type glyphState int

const (
	glyphOn    glyphState = iota // 显示本机
	glyphAway                    // 显示别的电脑
	glyphBlank                   // 在线，但认不出当前输入
	glyphOff                     // 离线
	glyphBusy                    // 正在切换：屏幕暗下去再亮起，像真的在换输入源
	glyphGhost                   // 检测中的占位
)

const standH = 6 // 屏幕下方露出的支架与底座

// monitorSize 是屏幕的大小：横屏 16:9，竖屏 9:16。
func monitorSize(portrait bool) (w, h float32) {
	if portrait {
		return 18, 32
	}
	return 38, 21.5
}

// drawMonitor 画一台显示器：真实比例的屏幕、细边框、支架，右上角是状态点。
// 整台（屏幕加支架）在 r 里居中，横屏不会显得偏低。
func drawMonitor(pt *ui.Painter, r ui.Rect, k palette, portrait bool, s glyphState) {
	w, h := monitorSize(portrait)
	cx, top := r.X+r.W/2, r.Y+(r.H-h-standH)/2
	base := top + h + 4
	scr := ui.Rect{X: cx - w/2, Y: top, W: w, H: h}
	in := ui.Rect{X: scr.X + 1.5, Y: scr.Y + 1.5, W: w - 3, H: h - 3}
	ink := k.bezel
	switch s {
	case glyphOff:
		ink = k.faint
	case glyphGhost:
		ink = k.faint.Alpha(0.6)
	}
	pt.Fill(ui.Rect{X: cx - 1.5, Y: scr.Y + h - 1, W: 3, H: 5}, ink, 0)
	pt.Fill(ui.Rect{X: cx - 7, Y: base, W: 14, H: 2}, ink, 1)
	pt.Fill(scr, ink, 3.5)
	switch s {
	case glyphOn:
		pt.FillGradient(in, ui.LinearGradient{From: k.screen[0], To: k.screen[1], Angle: 160, Oklab: true}, 2)
	case glyphAway:
		pt.FillGradient(in, ui.LinearGradient{From: k.away[0], To: k.away[1], Angle: 160, Oklab: true}, 2)
	case glyphBusy:
		pt.AnimationFrame()
		t := float64(pt.Now().UnixMilli()%1400) / 1400
		lit := float32(0.5 + 0.5*math.Cos(2*math.Pi*t))
		pt.Fill(in, k.bezel, 2)
		pt.FillGradient(in, ui.LinearGradient{From: k.screen[0].Alpha(lit), To: k.screen[1].Alpha(lit), Angle: 160, Oklab: true}, 2)
	default:
		pt.Fill(in, k.card, 2)
	}
	if s == glyphGhost {
		return
	}
	dot := k.ok
	if s == glyphOff {
		dot = k.faint
	}
	dx, dy := scr.X+w-1, scr.Y+1
	pt.Fill(ui.Rect{X: dx - 5, Y: dy - 5, W: 10, H: 10}, k.card, 5)
	pt.Fill(ui.Rect{X: dx - 3.5, Y: dy - 3.5, W: 7, H: 7}, dot, 3.5)
}

func skeleton(c *ui.Context, k palette, portrait bool) {
	e := card(c, k)
	pulse := 0.45 + 0.55*e.Loop("pulse", 1400*time.Millisecond, ui.Bounce(ui.EaseInOut))
	bar := k.text.Alpha(0.07)
	e.Children(func() {
		ui.Box(c).Size(44, 48).Opacity(pulse).Draw(func(pt *ui.Painter, r ui.Rect) {
			drawMonitor(pt, r, k, portrait, glyphGhost)
		})
		ui.Column(c).Grow(1).Gap(10).Opacity(pulse).Children(func() {
			ui.Box(c).Size(104, 11).Radius(5.5).Background(bar)
			ui.Box(c).Size(92, 20).Radius(6).Background(bar)
		})
	})
}

// ready 是会参与“全部切到”的显示器数量。
func (p *panel) ready() (n int) {
	for _, r := range p.Rows {
		if r.Enabled && r.Online {
			n++
		}
	}
	return n
}

// target 是主按钮要去的电脑：先找勾选的显示器大多停在哪台（平手算本机），
// 停在别处就把它们叫回本机，停在本机就送去下一台。两台电脑时就是“另一台”。
func (p *panel) target() int {
	n := len(p.Computers)
	if n == 0 || p.Self < 0 || p.Self >= n {
		return 0
	}
	count := make([]int, n)
	for _, r := range p.Rows {
		if r.Enabled && r.Online && r.Current >= 0 && r.Current < n {
			count[r.Current]++
		}
	}
	most := p.Self
	for j := range count {
		if count[j] > count[most] {
			most = j
		}
	}
	if most != p.Self {
		return p.Self
	}
	return (p.Self + 1) % n
}

// actions 是一键切换：主按钮去建议的那台，旁边是去其他电脑的安静按钮。
// 切换进行中只有带进度的那个按钮保持原样，其余一律停用（淡一半），不再装作能点。
func (p *panel) actions(c *ui.Context, k palette) {
	if len(p.Computers) == 0 {
		return
	}
	to := p.target()
	can := p.ready() > 0 && !p.Switching && !p.Loading
	bar := ui.Row(c).Margin(12, 0, 0, 0).Gap(8)
	pending := ui.Local(bar, "to", func() int { return -1 }) // 这次切换是从这里点出去的，去哪台
	if !p.Switching {
		*pending = -1
	}
	go_ := func(j int) {
		if can && p.SwitchAll != nil {
			*pending = j
			p.SwitchAll(j)
		}
	}
	bar.Children(func() {
		p.hero(c, k, to, can, *pending, go_)
		for j, name := range p.Computers {
			if j == to {
				continue
			}
			mine := p.Switching && *pending == j
			// 次要去处：和卡片同一种白底，名字加箭头，读作“（全部）去 Mac”。
			// 三台以上电脑时并排放不下箭头，只留名字（悬停提示写全）。
			arrow := len(p.Computers) == 2
			b := ui.ButtonBase(c.Key(j)).Height(40).MinWidth(0).Shrink(1).Padding(0, 12).Gap(6).Radius(12).
				Justify(ui.Center).Background(k.card).Disabled(!can && !mine).
				Label(fmt.Sprintf(tr("全部切到 %s", "Switch to %s"), name)).Tooltip(fmt.Sprintf(tr("全部切到 %s", "Switch to %s"), name))
			if arrow {
				b.Padding(0, 12, 0, 14)
			}
			if k.dark {
				b.InsetShadow(0, 0, 0, 1, k.line)
			} else {
				b.Border(1, k.line).Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.04))
			}
			hover := can && b.Hovered()
			if hover {
				b.Background(k.card.Mix(k.text, 0.04))
			}
			b.Children(func() {
				if mine {
					spinner(c, 13, k.muted)
					return
				}
				ui.Text(c, name).FontSize(13).FontWeight(600).SingleLine().MinWidth(0)
				if !arrow {
					return
				}
				ink, nudge := k.faint, float32(0)
				if hover {
					ink, nudge = k.muted, 2
				}
				ui.Icon(c, iconArrow).FontSize(13).TextColor(ink).Translate(b.AnimateTo("nudge", nudge, ui.Snappy), 0)
			})
			b.OnClick(func() { go_(j) })
		}
	})
}

// hero 是主按钮，无障碍名字与看得见的文字相同。切换中由它报进度：从这里点出去的写“正在切到 X…”，
// 快捷键、Apple 家庭或单台切换发起的不知道去哪台，只写“正在切换…”；
// 进度在旁边的次要按钮上时，它跟着停用。
func (p *panel) hero(c *ui.Context, k palette, to int, can bool, pending int, go_ func(int)) {
	name := p.Computers[to]
	busy := p.Switching && (pending == to || pending < 0)
	label := fmt.Sprintf(tr("全部切到 %s", "Switch to %s"), name)
	if busy { // 读屏读到的就是看得见的字
		label = tr("正在切换…", "Switching…")
		if pending == to {
			label = fmt.Sprintf(tr("正在切到 %s…", "Switching to %s…"), name)
		}
	}
	b := ui.ButtonBase(c.Key("all")).Grow(1).MinWidth(150).Height(40).Padding(0, 14).Radius(12).Justify(ui.Center).Gap(8).
		Label(label).Disabled(!can && !busy)
	white := ui.RGB(255, 255, 255)
	if can || p.Switching {
		top := k.accent.Mix(white, 0.14)
		if b.Pressed() {
			top = k.accent
		}
		b.LinearGradient(ui.LinearGradient{From: top, To: k.accent, Angle: 180}).
			InsetShadow(0, 1, 0, 0, white.Alpha(0.18)).
			Shadow(0, 1, 2, 0, k.lift).
			TextColor(k.onAccent)
	} else {
		b.Background(k.hover).TextColor(k.muted)
	}
	b.Children(func() {
		if busy {
			spinner(c, 14, k.onAccent)
			ui.Text(c, label).FontSize(14).FontWeight(600).SingleLine().MinWidth(0)
			return
		}
		ui.Text(c, fmt.Sprintf(tr("全部切到 %s", "Switch to %s"), name)).FontSize(14).FontWeight(600).SingleLine().MinWidth(0)
		nudge := float32(0)
		if can && b.Hovered() {
			nudge = 3
		}
		ui.Icon(c, iconArrow).FontSize(15).Translate(b.AnimateTo("nudge", nudge, ui.Snappy), 0)
	})
	b.OnClick(func() { go_(to) })
}

func (p *panel) status(c *ui.Context, k palette) {
	ui.Row(c).Height(36).Justify(ui.Center).Gap(5).FontSize(12).TextColor(k.muted).Children(func() {
		switch {
		case p.Notice != "" && !p.Switching:
			ic, col := iconDone, k.ok
			if p.NoticeError {
				ic, col = iconAlert, k.bad
			}
			ui.Icon(c, ic).FontSize(13).TextColor(col)
			t := ui.Text(c, p.Notice).SingleLine().MinWidth(0).Tooltip(p.Notice)
			if p.NoticeError {
				t.TextColor(col)
			}
		case p.Loading:
			ui.Text(c, tr("正在检测显示器…", "Detecting monitors…"))
		case p.Switching:
			// 进度在卡片和按钮上，这里留白。
		case p.ready() == 0:
			msg := tr("勾选要一起切换的显示器", "Check the monitors to switch together")
			for _, r := range p.Rows {
				if r.Enabled {
					msg = tr("勾选的显示器都离线了", "All checked monitors are offline")
					break
				}
			}
			ui.Text(c, msg)
		default:
			// 主按钮有快捷键时顺手教一下；没有就提一句卡片上的电脑名也能点。
			to := p.target()
			if to < len(p.Hotkeys) && p.Hotkeys[to] != "" {
				ui.Text(c, tr("快捷键", "Shortcut"))
				keycap(c, k, hotkeyLabel(p.Hotkeys[to]))
				break
			}
			ui.Text(c, tr("点卡片上的电脑名，可单独切换", "Click a computer name to switch one monitor"))
		}
	})
}

// keycap 把快捷键画成一枚小键帽。
func keycap(c *ui.Context, k palette, label string) ui.Element {
	return ui.Text(c, label).FontSize(11.5).FontWeight(500).TextColor(k.text).
		Padding(1, 6).Radius(5).Background(k.card).Border(1, k.line)
}

func miniSwitch(c *ui.Context, k palette, on bool) {
	track := ui.Box(c).Size(26, 16).Radius(8)
	x, bg := float32(0), k.track
	if on {
		x, bg = 10, k.accent
	}
	x = track.Animate("x", x, 160*time.Millisecond)
	track.Background(bg).Children(func() {
		ui.Box(c).Size(12, 12).Radius(6).Background(ui.RGB(255, 255, 255)).
			Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.2)).Absolute().Top(2).Left(2 + x)
	})
}

func iconButton(c *ui.Context, k palette) ui.Element {
	b := ui.ButtonBase(c).Size(26, 26).Radius(7).Justify(ui.Center).TextColor(k.muted)
	if b.Hovered() {
		b.Background(k.hover).TextColor(k.text)
	}
	return b
}

// quietButton 是次要按钮：浅底，悬停时深一点。
func quietButton(c *ui.Context, k palette) ui.Element {
	b := ui.ButtonBase(c).Padding(4, 10).Radius(7).Gap(5).Background(k.hover).TextColor(k.text)
	if b.Hovered() {
		b.Background(k.text.Alpha(0.1))
	}
	return b
}

func check(c *ui.Context, k palette, on *bool) ui.Element {
	b := ui.CheckboxBase(c, on).Size(20, 20).Radius(10).Justify(ui.Center)
	if *on {
		b.Background(k.accent).TextColor(k.onAccent)
	} else {
		b.Border(1.5, k.faint)
	}
	return b.Children(func() {
		if *on {
			ui.Icon(c, iconCheckBold).FontSize(12)
		}
	})
}

// spinner 是一段转动的圆弧，用在强调色上（系统 Spinner 是灰色辐条）。
func spinner(c *ui.Context, size float32, color ui.Color) {
	s := ui.Box(c).Size(size, size)
	s.Rotate(s.Loop("spin", 800*time.Millisecond, ui.Linear) * 360)
	s.Draw(func(pt *ui.Painter, r ui.Rect) {
		cx, cy, rad := r.X+r.W/2, r.Y+r.H/2, r.W/2-1
		pt.StrokePath((&ui.Path{}).Circle(cx, cy, rad), 1.75, color.Alpha(0.3))
		pt.StrokePath((&ui.Path{}).Circle(cx, cy, rad).Trim(0, 0.28), 1.75, color)
	})
}
