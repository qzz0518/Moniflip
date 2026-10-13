package main

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/skip2/go-qrcode"
)

// 设置页：顶栏固定，下面各组设置一起滚动。
func (p *panel) settings(c *ui.Context, k palette) {
	col := ui.Column(c).Grow(1).Transition(toSettings)
	st := ui.Local(col, "scroll", func() ui.ScrollState { return ui.ScrollState{} })
	col.Children(func() {
		top := ui.Row(c).Height(46).Padding(10, pad, 6, pad-6).Gap(2)
		if st.Y > 0 {
			top.BorderWidth(0, 0, 1, 0).BorderColor(k.line) // 内容滚到顶栏下面时才出现分隔线
		}
		top.Children(func() {
			b := iconButton(c.Key("back"), k).Label(tr("返回", "Back")).Tooltip(tr("返回", "Back"))
			b.Children(func() { ui.Icon(c, iconBack).FontSize(16) })
			b.OnClick(func() { p.Page = 0 })
			ui.Text(c, tr("设置", "Settings")).FontSize(14).FontWeight(600)
		})
		if p.NoticeError && p.Notice != "" {
			ui.Row(c).Padding(0, pad+12, 6, pad+12).Gap(5).FontSize(12).TextColor(k.bad).Children(func() {
				ui.Icon(c, iconAlert).FontSize(13)
				ui.Text(c, p.Notice).SingleLine().MinWidth(0).Tooltip(p.Notice)
			})
		}
		ui.Scroll(c).Grow(1).TrackScroll(st).Padding(2, pad, 16, pad).Gap(18).Children(func() {
			p.computersSection(c.Key("computers"), k)
			p.wiringSection(c.Key("wiring"), k)
			p.hotkeysSection(c.Key("hotkeys"), k)
			p.homeSection(c.Key("home"), k)
			p.generalSection(c.Key("general"), k)
		})
	})
}

// settingsWindow 是独立的设置窗口：隐藏标题栏，顶部一行放标题（与红绿灯同高、可拖动窗口），
// 下面各组设置竖直排成一列。返回刚好放下全部内容的视图高度（按上一帧的排版量，还没排过时为 0），
// 窗口据此调整高度。
func (p *panel) settingsWindow(c *ui.Context) (fit float32) {
	k := colors(c.Theme())
	bar := c.TitleBar()
	ui.Column(c).Fill().Background(k.bg).TextColor(k.text).FontSize(13).Children(func() {
		ui.Row(c).Height(52).Padding(0, 24+bar.Right, 0, max(bar.Left+10, 24)).Gap(12).DragWindow().Children(func() {
			ui.Text(c, tr("设置", "Settings")).FontSize(15).FontWeight(700)
			if p.NoticeError && p.Notice != "" {
				ui.Row(c).Grow(1).MinWidth(0).Gap(5).FontSize(12).TextColor(k.bad).Children(func() {
					ui.Icon(c, iconAlert).FontSize(13)
					ui.Text(c, p.Notice).SingleLine().MinWidth(0).Tooltip(p.Notice)
				})
			}
		})
		scroll := ui.Scroll(c).Grow(1)
		scroll.Children(func() {
			content := ui.Column(c).Padding(2, 24, 20, 24).Gap(20)
			content.Children(func() {
				p.computersSection(c.Key("computers"), k)
				p.wiringSection(c.Key("wiring"), k)
				p.hotkeysSection(c.Key("hotkeys"), k)
				p.homeSection(c.Key("home"), k)
				p.generalSection(c.Key("general"), k)
			})
			// 都取同一帧的排版：滚动区的顶边加内容的高度。不能拿此刻的窗口高度去算，
			// 窗口刚改过尺寸时它和上一帧的排版对不上，算出来会来回振荡。
			if s, in := scroll.LayoutBounds(), content.LayoutBounds(); in.H > 0 {
				fit = s.Y + in.H
			}
		})
	})
	return
}

// section 是一组设置：小标题 + 圆角分组，组内各行之间一条细线。
func section(c *ui.Context, k palette, title string, body func()) ui.Element {
	s := ui.Column(c)
	s.Children(func() {
		sectionTitle(c, k, title).Padding(0, 12, 6, 12)
		group(c, k).Children(body)
	})
	return s
}

func sectionTitle(c *ui.Context, k palette, title string) ui.Element {
	return ui.Text(c, title).FontSize(12).FontWeight(600).TextColor(k.muted)
}

func group(c *ui.Context, k palette) ui.Element {
	e := ui.Column(c).Radius(12).Background(k.card).Gap(1).Dividers(1, k.line)
	if k.dark {
		return e.InsetShadow(0, 0, 0, 1, k.line)
	}
	return e.Border(1, k.line)
}

func item(c *ui.Context) ui.Element {
	return ui.Row(c).Height(38).Padding(0, 12).Gap(8)
}

func note(c *ui.Context, k palette, s string) {
	ui.Text(c, s).FontSize(11.5).TextColor(k.muted).Padding(6, 12, 0, 12)
}

// ── 电脑 ──

func (p *panel) computersSection(c *ui.Context, k palette) {
	section(c, k, tr("电脑", "Computers"), func() {
		for i := range p.Computers {
			p.computerRow(c.Key(i), k, i)
		}
		add := ui.ButtonBase(c.Key("add")).Height(38).Padding(0, 12).Gap(6).Justify(ui.Start).TextColor(k.accent).Label(tr("添加电脑", "Add Computer"))
		if add.Hovered() {
			add.Background(k.hover)
		}
		add.Children(func() {
			ui.Icon(c, iconPlus).FontSize(14)
			ui.Text(c, tr("添加电脑", "Add Computer"))
		})
		add.OnClick(func() {
			if p.AddComputer != nil {
				p.AddComputer()
			}
		})
	})
}

// computerRow 就地改名：输入框看着像一行字，回车或离开时才提交。
func (p *panel) computerRow(c *ui.Context, k palette, i int) {
	r := item(c)
	hovered := r.Hovered()
	draft := ui.Local(r, "draft", func() string { return p.Computers[i] })
	was := ui.Local(r, "focused", func() bool { return false })
	r.Children(func() {
		in := ui.TextInputBase(c.Key("name"), draft).Grow(1).MinWidth(0).Height(26).Padding(0, 6).
			Margin(0, 0, 0, -6).Radius(6).Placeholder(tr("电脑名", "Computer name")).Label(tr("电脑名", "Computer name")).Tooltip(tr("点击改名", "Click to rename"))
		focused := in.Focused()
		switch {
		case focused:
			in.Background(k.bg).InsetShadow(0, 0, 0, 1.5, k.accent)
		case in.Hovered():
			in.Background(k.hover)
		}
		if in.Submitted() || (*was && !focused) {
			p.rename(i, *draft)
		}
		*was = focused
		if !focused {
			*draft = p.Computers[i]
		}
		if i == p.Self {
			ui.Text(c, tr("本机", "This Computer")).FontSize(11).FontWeight(600).TextColor(k.accent).
				Padding(1, 6).Radius(4).Background(k.accent.Alpha(0.12)).Tooltip(tr("Moniflip 正运行在这台电脑上", "Moniflip is running on this computer"))
		} else if hovered {
			b := ui.ButtonBase(c.Key("self")).Padding(2, 6).Radius(5).TextColor(k.muted).Label(tr("设为本机", "Set as This Computer"))
			if b.Hovered() {
				b.Background(k.hover).TextColor(k.text)
			}
			b.Children(func() { ui.Text(c, tr("设为本机", "Set as This Computer")).FontSize(11.5) })
			b.OnClick(func() {
				p.Self = i
				if p.Changed != nil {
					p.Changed()
				}
			})
		}
		if len(p.Computers) > 2 && i != p.Self {
			b := iconButton(c.Key("remove"), k).Size(22, 22).Margin(0, -5, 0, 0).Label(fmt.Sprintf(tr("移除 %s", "Remove %s"), p.Computers[i])).Tooltip(tr("移除", "Remove"))
			if b.Hovered() {
				b.TextColor(k.bad)
			}
			b.Children(func() { ui.Icon(c, iconMinus).FontSize(15) })
			b.OnClick(func() {
				if p.RemoveComputer != nil {
					p.RemoveComputer(i)
				}
			})
		}
	})
}

func (p *panel) rename(i int, name string) {
	name = strings.TrimSpace(name)
	if name == "" || name == p.Computers[i] {
		return
	}
	p.Computers[i] = name
	if p.RenameComputer != nil {
		p.RenameComputer(i, name)
	}
}

// ── 接线 ──

const (
	colW  = 80  // 接线表每列（每台电脑）的宽度
	listW = 112 // 三台以上电脑时，接线改成分组列表，接口框的宽度
)

// wiringSection 是一张小表：每台显示器一行，每台电脑一列，格子里选它接的口。
// 三台以上电脑时列太窄，改成每台显示器一小组、每台电脑一行。
func (p *panel) wiringSection(c *ui.Context, k palette) {
	n := len(p.Computers)
	table := n <= 2
	ui.Column(c).Children(func() {
		ui.Row(c).Padding(0, 12, 6, 12).Gap(6).Children(func() {
			sectionTitle(c, k, tr("接线", "Inputs")).Grow(1)
			if table {
				for _, name := range p.Computers {
					ui.Text(c, name).Width(colW).Padding(0, 0, 0, 8).FontSize(11.5).TextColor(k.muted).SingleLine()
				}
			}
		})
		group(c, k).Children(func() {
			if len(p.Rows) == 0 {
				item(c).Children(func() {
					ui.Text(c, tr("还没有检测到显示器", "No monitors detected yet")).FontSize(12).TextColor(k.muted)
				})
				return
			}
			for i := range p.Rows {
				row := &p.Rows[i]
				for len(row.Ports) < n {
					row.Ports = append(row.Ports, "")
				}
				port := func(j int, w float32) {
					label := fmt.Sprintf(tr("%s 上 %s 的接口", "%s input for %s"), row.Name, p.Computers[j])
					if portSelect(c.Key(j), k, &row.Ports[j], row.Options, w, label) && p.Changed != nil {
						p.Changed()
					}
				}
				name := func() {
					t := ui.Text(c, row.Name).Grow(1).MinWidth(0).SingleLine()
					if !row.Online {
						t.TextColor(k.muted).Tooltip(fmt.Sprintf(tr("%s（离线）", "%s (offline)"), row.Name))
					}
				}
				if table {
					item(c.Key(i)).Gap(6).Children(func() {
						name()
						for j := range p.Computers {
							port(j, colW)
						}
					})
					continue
				}
				ui.Column(c.Key(i)).Padding(9, 12, 6, 12).Gap(2).Children(func() {
					ui.Row(c).Height(22).FontWeight(600).Children(name)
					for j, comp := range p.Computers {
						ui.Row(c.Key(j)).Height(32).Gap(8).Children(func() {
							ui.Text(c, comp).Grow(1).MinWidth(0).SingleLine().TextColor(k.muted)
							port(j, listW)
						})
					}
				})
			}
		})
		note(c, k, tr("按显示器背面接口旁的标识选。", "Match the labels next to the monitor’s ports."))
	})
}

// portSelect 是表格里的一格：浅底小胶囊，没设置时是橙色的“未设置”。
func portSelect(c *ui.Context, k palette, value *string, options []string, w float32, label string) bool {
	sel := ui.SelectBase(c, value)
	trig := sel.Trigger.Width(w).Height(26).Padding(0, 4, 0, 8).Gap(2).Radius(6).Background(k.hover).
		Disabled(len(options) == 0).Label(label)
	if trig.Hovered() {
		trig.Background(k.text.Alpha(0.09))
	}
	text, ink := *value, k.text
	if text == "" {
		text, ink = tr("未设置", "Not Set"), k.warn
	}
	trig.Children(func() {
		ui.Text(c, text).FontSize(12).FontWeight(500).TextColor(ink).SingleLine().Grow(1).MinWidth(0)
		ui.Icon(c, iconChevron).FontSize(11).TextColor(k.faint)
	})
	sel.Popup(func(panel ui.Element) {
		panel.Margin(4, 0, 0, 0).Padding(4).MinWidth(w).Radius(9).Background(k.menu).
			Border(1, k.line).Shadow(0, 6, 20, 0, ui.RGBA(0, 0, 0, 0.16))
		for _, o := range options {
			it := sel.Item(o).Gap(6).Padding(4, 8, 4, 6).Radius(5)
			if it.Highlighted() {
				it.Background(k.accent).TextColor(k.onAccent)
			}
			it.Children(func() {
				mark := ui.Icon(c, iconCheck).FontSize(12)
				if o != *value {
					mark.Opacity(0)
				}
				ui.Text(c, o).FontSize(12)
			})
		}
	})
	return trig.Changed()
}

// ── 快捷键 ──

func (p *panel) hotkeysSection(c *ui.Context, k palette) {
	ui.Column(c).Children(func() {
		sectionTitle(c, k, tr("快捷键", "Shortcuts")).Padding(0, 12, 6, 12)
		group(c, k).Children(func() {
			for i, name := range p.Computers {
				item(c.Key(i)).Children(func() {
					ui.Text(c, fmt.Sprintf(tr("全部切到 %s", "Switch to %s"), name)).Grow(1).MinWidth(0).SingleLine()
					p.recorder(c.Key("rec"), k, i)
				})
			}
		})
		note(c, k, tr("在任何地方按下，就把勾选的显示器切过去。", "Press from any app to switch the checked monitors."))
	})
}

// recorder 是快捷键录制框：点一下进入录制，接住下一组按键；Esc 取消，⌫（Windows 上叫 Backspace）清除。
// 宽度和接线那组的接口框对齐（两台电脑时盖住表格两列），两组控件的左缘在一条线上。
func (p *panel) recorder(c *ui.Context, k palette, i int) {
	acc := ""
	if i < len(p.Hotkeys) {
		acc = p.Hotkeys[i]
	}
	name := p.Computers[i]
	w := float32(2*colW + 6)
	if len(p.Computers) > 2 {
		w = listW
	}
	f := ui.Row(c).Width(w).Height(26).Padding(0, 3, 0, 9).Gap(4).Radius(6).Focusable().
		Role(ui.RoleButton).Label(fmt.Sprintf(tr("全部切到 %s 的快捷键", "Shortcut for Switch to %s"), name)).Value(hotkeyLabel(acc))
	rec := ui.Local(f, "rec", func() bool { return false })
	had := ui.Local(f, "focused", func() bool { return false })
	if f.Clicked() {
		*rec = !*rec
	} else if *rec && *had && !f.Focused() {
		*rec = false // 点到别处就算了
	}
	*had = f.Focused()
	f.HandleInput(func(ev ui.InputEvent) bool {
		if !*rec {
			if ev.Kind == ui.InputKeyDown && ev.Mods == 0 && (ev.Key == ui.KeyEnter || ev.Key == ui.KeySpace) {
				*rec = true
				return true
			}
			return false
		}
		switch ev.Kind {
		case ui.InputKeyDown:
		case ui.InputKeyUp, ui.InputText, ui.InputCompose:
			return true // 录制时按键不打字、不触发别的
		default:
			return false
		}
		switch {
		case ev.Repeat:
		case ev.Mods == 0 && ev.Key == ui.KeyEscape:
			*rec = false
		case ev.Mods == 0 && (ev.Key == ui.KeyBackspace || ev.Key == ui.KeyDelete):
			p.setHotkey(i, "")
			*rec = false
		case ev.Mods&^ui.Shift == 0 && ev.Key == ui.KeyTab:
			*rec = false
			return false // Tab 照常移走焦点
		default:
			if a := accelerator(ev); a != "" {
				p.setHotkey(i, a)
				*rec = false
			}
		}
		return true
	})
	switch {
	case *rec:
		del := "⌫"
		if !onMac {
			del = "Backspace"
		}
		f.Background(k.accent.Alpha(0.1)).InsetShadow(0, 0, 0, 1.5, k.accent).Tooltip(fmt.Sprintf(tr("Esc 取消，%s 清除", "Esc to cancel, %s to clear"), del))
	case f.Hovered():
		f.Background(k.text.Alpha(0.09))
	default:
		f.Background(k.hover)
	}
	f.Children(func() {
		switch {
		case *rec:
			held := heldLabel(c.Modifiers())
			if held == "" {
				held = tr("按下快捷键", "Type Shortcut")
			}
			ui.Text(c, held+"…").FontSize(12).FontWeight(500).TextColor(k.accent).SingleLine().Grow(1)
		case acc != "":
			ui.Text(c, hotkeyLabel(acc)).FontSize(13).FontWeight(500).LetterSpacing(1).SingleLine().Grow(1)
			x := iconButton(c.Key("clear"), k).Size(20, 20).Radius(10).TextColor(k.faint).Label(tr("清除快捷键", "Clear Shortcut")).Tooltip(tr("清除", "Clear"))
			x.Children(func() { ui.Icon(c, iconClose).FontSize(12) })
			x.OnClick(func() { p.setHotkey(i, "") })
		default:
			ui.Text(c, tr("点击录制", "Record Shortcut")).FontSize(12).TextColor(k.muted).Grow(1)
		}
	})
}

// setHotkey 记下快捷键；同一组键原先给了别的电脑的，从那台上拿掉。
func (p *panel) setHotkey(i int, acc string) {
	for len(p.Hotkeys) < len(p.Computers) {
		p.Hotkeys = append(p.Hotkeys, "")
	}
	for j, a := range p.Hotkeys {
		if j != i && acc != "" && strings.EqualFold(a, acc) {
			p.Hotkeys[j] = ""
			if p.SetHotkey != nil {
				p.SetHotkey(j, "")
			}
		}
	}
	p.Hotkeys[i] = acc
	if p.SetHotkey != nil {
		p.SetHotkey(i, acc)
	}
}

// ── Apple 家庭 ──

func (p *panel) homeSection(c *ui.Context, k palette) {
	s := ui.Column(c)
	// 打开了但还没配对：代码就是此刻要看的东西，进页面或刚打开时把这一组滚到顶栏下面。
	// 滚进视野的是一把比视口还高的隐形标尺（从小标题上方 10 点起），这样整组顶端对齐，
	// 而不是刚好露出底边、让上一组的说明切一半挂在顶栏下。
	shown := ui.Local(s, "shown", func() bool { return false })
	want := p.HomeKit && !p.HomeKitPaired
	s.Children(func() {
		mark := ui.Box(c.Key("mark")).Absolute().Top(-10).Left(0).Size(1, 2000).Invisible()
		if want && !*shown {
			mark.ScrollIntoView()
		}
		sectionTitle(c, k, tr("Apple 家庭", "Apple Home")).Padding(0, 12, 6, 12)
		group(c, k).Children(func() {
			sw := ui.SwitchBase(c.Key("on"), &p.HomeKit).Height(52).Padding(0, 12).Gap(10).Label(tr("加入 Apple 家庭", "Add to Apple Home"))
			sw.Children(func() {
				ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
					ui.Text(c, tr("加入 Apple 家庭", "Add to Apple Home"))
					if p.HomeKit && p.HomeKitPaired {
						ui.Row(c).Gap(4).Children(func() {
							ui.Icon(c, iconDone).FontSize(12).TextColor(k.ok)
							ui.Text(c, tr("已添加到“家庭”", "Added to Home")).FontSize(11.5).TextColor(k.ok).NoWrap()
						})
						return
					}
					ui.Text(c, tr("用“家庭”App 和 Siri 切换显示器", "Switch monitors with the Home app or Siri")).FontSize(11.5).TextColor(k.muted).SingleLine()
				})
				miniSwitch(c, k, p.HomeKit)
			})
			if sw.Changed() && p.SetHomeKit != nil {
				p.SetHomeKit(p.HomeKit)
			}
			if p.HomeKit {
				p.homeKitAs(c.Key("as"), k)
			}
			// 配对码只在等待配对时有用；配好以后收成开关下的一行状态，不再占一大块。
			if p.HomeKit && !p.HomeKitPaired {
				p.pairing(c.Key("code"), k)
			}
		})
	})
	*shown = want
}

// homeKitAs 选在“家庭”里的样子，说明随所选的样子变。
func (p *panel) homeKitAs(c *ui.Context, k palette) {
	kinds := []string{"switch", "tv"} // 与 HomeKitAs 的取值对应
	sel := max(slices.Index(kinds, p.HomeKitAs), 0)
	ui.Row(c).Height(52).Padding(0, 12).Gap(10).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			ui.Text(c, tr("在“家庭”中显示为", "Show in Home as"))
			hint := tr("每台电脑一个开关，可用于自动化和配件按钮", "One switch per computer, for automations and buttons")
			if sel == 1 {
				hint = tr("一台电视，输入源就是各台电脑", "A TV whose inputs are your computers")
			}
			ui.Text(c, hint).FontSize(11.5).TextColor(k.muted).SingleLine()
		})
		was := sel
		if segments(c.Key("pick"), k, &sel, []string{tr("开关", "Switches"), tr("电视", "TV")}, tr("显示为", "Show as")) && sel != was {
			if p.SetHomeKitAs != nil {
				p.SetHomeKitAs(kinds[sel])
			} else {
				p.HomeKitAs = kinds[sel]
			}
		}
	})
}

// pairing 照 HomeKit 配对标签的样子：左边二维码，右边房子图标加八位配对码；只在等待配对时出现。
func (p *panel) pairing(c *ui.Context, k palette) {
	ui.Row(c).Padding(16, 12, 16, 12).Gap(20).Justify(ui.Center).AlignItems(ui.Center).Children(func() {
		if p.HomeKitCode == "" {
			ui.Row(c).Gap(6).Children(func() {
				spinner(c, 12, k.muted)
				ui.Text(c, tr("正在准备配对码…", "Preparing setup code…")).FontSize(12).TextColor(k.muted)
			})
			return
		}
		if qr := qrSVG(p.HomeKitSetup); qr != nil {
			// 白底留出静区，深色模式下也扫得出
			ui.Box(c).Padding(8).Radius(10).Background(ui.RGB(255, 255, 255)).Children(func() {
				ui.Image(c, qr).Size(105, 105).Label(tr("配对二维码", "Setup QR code"))
			})
		}
		ui.Column(c).Gap(10).Children(func() {
			ui.Row(c).Gap(8).Padding(6, 14, 6, 10).Radius(9).Border(1.5, k.text).AlignSelf(ui.Start).Children(func() {
				ui.Icon(c, iconHome).FontSize(20)
				ui.Text(c, p.HomeKitCode).FontSize(20).FontWeight(600).FontFeatures("tnum").LetterSpacing(1.2).
					Label(fmt.Sprintf(tr("配对码 %s", "Setup code %s"), p.HomeKitCode))
			})
			ui.Column(c).Gap(2).FontSize(11.5).TextColor(k.muted).Children(func() {
				ui.Text(c, tr("在“家庭”App 中添加配件，扫描二维码", "In the Home app, tap Add Accessory and scan")).NoWrap()
				ui.Text(c, tr("扫不了时点“更多选项…”，输入配对码", "Or tap More options… and enter the code")).NoWrap()
			})
			ui.Row(c).Gap(5).FontSize(12).Children(func() {
				dot := ui.Box(c).Size(7, 7).Radius(3.5).Background(k.warn)
				dot.Opacity(0.4 + 0.6*dot.Loop("wait", 1600*time.Millisecond, ui.Bounce(ui.EaseInOut)))
				ui.Text(c, tr("等待配对", "Waiting to pair")).TextColor(k.muted).NoWrap()
			})
		})
	})
}

// qrSVG 把文字画成二维码（不带白边，每个模块一个单位方块，同一行相连的并成一段），按文字缓存。
var qrCache struct {
	text string
	svg  *ui.SVG
}

func qrSVG(text string) *ui.SVG {
	if text == "" || text == qrCache.text {
		return qrCache.svg
	}
	q, err := qrcode.New(text, qrcode.Medium)
	if err != nil {
		return nil
	}
	q.DisableBorder = true
	bm := q.Bitmap()
	var d strings.Builder
	for y, row := range bm {
		for x := 0; x < len(row); x++ {
			if row[x] {
				x0 := x
				for x < len(row) && row[x] {
					x++
				}
				fmt.Fprintf(&d, "M%d %dh%dv1h-%dz", x0, y, x-x0, x-x0)
			}
		}
	}
	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d"><path fill="#000" d="%s"/></svg>`, len(bm), len(bm), d.String())
	qrCache.text, qrCache.svg = text, ui.MustParseSVG([]byte(svg))
	return qrCache.svg
}

// ── 通用 ──

func (p *panel) generalSection(c *ui.Context, k palette) {
	section(c, k, tr("通用", "General"), func() {
		item(c.Key("lang")).Children(func() {
			ui.Text(c, tr("语言", "Language")).Grow(1)
			p.languagePicker(c.Key("pick"), k)
		})
		login := ui.SwitchBase(c.Key("login"), &p.OpenAtLogin).Height(38).Padding(0, 12).Gap(10).Label(tr("开机时启动", "Open at Login"))
		login.Children(func() {
			ui.Text(c, tr("开机时启动", "Open at Login")).Grow(1)
			miniSwitch(c, k, p.OpenAtLogin)
		})
		if login.Changed() && p.SetLogin != nil {
			p.SetLogin(p.OpenAtLogin)
		}
		item(c.Key("version")).Children(func() {
			ui.Text(c, tr("版本", "Version")).Grow(1)
			ui.Text(c, p.Version).TextColor(k.muted).FontFeatures("tnum")
			b := smallButton(c.Key("update"), k)
			b.Children(func() { ui.Text(c, tr("检查更新", "Check for Updates")) })
			b.OnClick(func() {
				if p.CheckUpdate != nil {
					p.CheckUpdate()
				}
			})
		})
		diag := item(c.Key("diag")).Height(48)
		copied := ui.Local(diag, "copied", func() bool { return false })
		diag.Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
				ui.Text(c, tr("诊断信息", "Diagnostics"))
				ui.Text(c, tr("反馈问题时附上，便于定位", "Attach when reporting an issue")).FontSize(11.5).TextColor(k.muted).SingleLine()
			})
			b := smallButton(c.Key("copy"), k).Label(tr("复制诊断信息", "Copy Diagnostics"))
			b.Children(func() {
				if *copied {
					ui.Icon(c, iconCheck).FontSize(12).TextColor(k.ok)
					ui.Text(c, tr("已复制", "Copied"))
					return
				}
				ui.Text(c, tr("复制", "Copy"))
			})
			b.OnClick(func() {
				if p.CopyDiagnostics != nil {
					p.CopyDiagnostics(func() { *copied = true })
				}
			})
		})
	})
	q := ui.ButtonBase(c.Key("quit")).Height(38).Margin(-8, 0, 0, 0).Radius(12).Justify(ui.Center).Gap(6).
		TextColor(k.muted).Label(tr("退出 Moniflip", "Quit Moniflip"))
	if q.Hovered() {
		q.Background(k.hover).TextColor(k.bad)
	}
	q.Children(func() {
		ui.Icon(c, iconPower).FontSize(13)
		ui.Text(c, tr("退出 Moniflip", "Quit Moniflip"))
	})
	q.OnClick(func() {
		if p.Quit != nil {
			p.Quit()
		}
	})
}

// languagePicker 是界面语言的分段。语言名用它自己的文字写，看不懂当前界面的人也认得出。
func (p *panel) languagePicker(c *ui.Context, k palette) {
	langs := []string{"", "zh", "en"} // 与 Language 的取值对应
	sel := max(slices.Index(langs, p.Language), 0)
	if segments(c, k, &sel, []string{tr("跟随系统", "System"), "简体中文", "English"}, tr("语言", "Language")) && langs[sel] != p.Language {
		p.Language = langs[sel]
		if p.SetLanguage != nil {
			p.SetLanguage(p.Language)
		}
	}
}

// segments 是设置行尾的一组分段，样子同卡片上的电脑分段；各段一样宽、不收窄，两种界面下都写得全。
// 返回这一帧是否点了某一段（点到的下标写回 *sel）。
func segments(c *ui.Context, k palette, sel *int, names []string, label string) bool {
	seg := ui.SegmentedBase(c, sel, len(names))
	track := seg.Track.Height(24).Padding(2).Gap(2).Radius(7).Background(k.hover).Label(label)
	track.Children(func() {
		for j, name := range names {
			s := seg.Segment(j).Height(20).MinWidth(72).Padding(0, 10).Radius(5).Justify(ui.Center)
			ink, weight := k.muted, 500
			switch {
			case j == *sel:
				s.Background(k.knob)
				if !k.dark {
					s.Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.12)).Shadow(0, 0, 0, 0.5, ui.RGBA(0, 0, 0, 0.06))
				}
				ink, weight = k.text, 600
			case s.Hovered():
				s.Background(k.hover)
				ink = k.text
			}
			s.Children(func() {
				ui.Text(c, name).FontSize(12).FontWeight(weight).TextColor(ink).SingleLine()
			})
		}
	})
	return track.Changed()
}

// smallButton 是设置行尾的小按钮。
func smallButton(c *ui.Context, k palette) ui.Element {
	b := ui.ButtonBase(c).Height(24).Padding(0, 10).Gap(4).Radius(6).FontSize(12).FontWeight(500).
		Background(k.hover).Border(1, k.line)
	if b.Hovered() {
		b.Background(k.text.Alpha(0.09))
	}
	return b
}
