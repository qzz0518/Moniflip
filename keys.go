package main

import (
	"runtime"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
)

// 全局快捷键：录下的按键存成 mygo accelerator（"Ctrl+Alt+Cmd+P"），显示时换成平台习惯的写法。

var keyNames = map[ui.Key]string{
	ui.KeySpace: "Space", ui.KeyEnter: "Enter", ui.KeyTab: "Tab",
	ui.KeyBackspace: "Backspace", ui.KeyDelete: "Delete", ui.KeyInsert: "Insert",
	ui.KeyHome: "Home", ui.KeyEnd: "End", ui.KeyPageUp: "PageUp", ui.KeyPageDown: "PageDown",
	ui.KeyLeft: "Left", ui.KeyRight: "Right", ui.KeyUp: "Up", ui.KeyDown: "Down",
	ui.KeyMinus: "-", ui.KeyEqual: "=", ui.KeyComma: ",", ui.KeyPeriod: ".", ui.KeySlash: "/",
	ui.KeySemicolon: ";", ui.KeyQuote: "'", ui.KeyBracketLeft: "[", ui.KeyBracketRight: "]",
	ui.KeyBackslash: `\`, ui.KeyBackquote: "`",
}

func keyName(k ui.Key) string {
	switch {
	case k >= ui.KeyA && k <= ui.KeyZ:
		return string(rune('A' + int(k-ui.KeyA)))
	case k >= ui.Key0 && k <= ui.Key9:
		return string(rune('0' + int(k-ui.Key0)))
	case k >= ui.KeyF1 && k <= ui.KeyF12:
		return "F" + strconv.Itoa(int(k-ui.KeyF1)+1)
	}
	return keyNames[k]
}

// accelerator 把一次按键写成 accelerator；不适合做全局快捷键（只有修饰键、
// 不带 Ctrl/Alt/Cmd 的普通键会吞掉打字）时返回 ""。
func accelerator(ev ui.InputEvent) string {
	k := ev.PhysicalKey // 按位置记，换输入法、键盘布局也认得
	if keyName(k) == "" {
		k = ev.Key
	}
	name := keyName(k)
	fn := k >= ui.KeyF1 && k <= ui.KeyF12
	if name == "" || (ev.Mods&(ui.Ctrl|ui.Alt|ui.Super) == 0 && !fn) {
		return ""
	}
	var b strings.Builder
	for _, m := range []struct {
		mod  ui.Modifiers
		name string
	}{{ui.Ctrl, "Ctrl"}, {ui.Alt, "Alt"}, {ui.Shift, "Shift"}, {ui.Super, "Cmd"}} {
		if ev.Mods&m.mod != 0 {
			b.WriteString(m.name + "+")
		}
	}
	return b.String() + name
}

var macKeys = map[string]string{
	"ctrl": "⌃", "control": "⌃", "alt": "⌥", "option": "⌥", "altgr": "⌥", "shift": "⇧",
	"cmd": "⌘", "command": "⌘", "super": "⌘", "meta": "⌘", "win": "⌘",
	"cmdorctrl": "⌘", "commandorcontrol": "⌘",
	"enter": "↩", "return": "↩", "backspace": "⌫", "delete": "⌦", "del": "⌦", "tab": "⇥",
	"left": "←", "right": "→", "up": "↑", "down": "↓", "escape": "⎋", "esc": "⎋", "plus": "+",
}

var pcKeys = map[string]string{
	"control": "Ctrl", "cmdorctrl": "Ctrl", "commandorcontrol": "Ctrl", "option": "Alt", "altgr": "Alt",
	"cmd": "Win", "command": "Win", "super": "Win", "meta": "Win", "plus": "+",
}

// hotkeyLabel 是给人看的写法：macOS 上 "⌃⌥⌘P"，其他平台 "Ctrl+Alt+Win+P"。
func hotkeyLabel(acc string) string {
	if acc == "" {
		return ""
	}
	parts := strings.Split(acc, "+")
	mac := runtime.GOOS == "darwin"
	for i, s := range parts {
		table := pcKeys
		if mac {
			table = macKeys
		}
		if v, ok := table[strings.ToLower(s)]; ok {
			parts[i] = v
		} else if len(s) == 1 {
			parts[i] = strings.ToUpper(s)
		}
	}
	if mac {
		return strings.Join(parts, "")
	}
	return strings.Join(parts, "+")
}

// heldLabel 是录制中已经按住的修饰键，让人看到按键被接住了。
func heldLabel(m ui.Modifiers) string {
	var parts []string
	for _, x := range []struct {
		mod  ui.Modifiers
		name string
	}{{ui.Ctrl, "Ctrl"}, {ui.Alt, "Alt"}, {ui.Shift, "Shift"}, {ui.Super, "Cmd"}} {
		if m&x.mod != 0 {
			parts = append(parts, x.name)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	l := hotkeyLabel(strings.Join(parts, "+"))
	if runtime.GOOS != "darwin" {
		l += "+"
	}
	return l
}
