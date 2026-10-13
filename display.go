package main

import (
	"fmt"
	"hash/fnv"
	"regexp"
	"strconv"
	"strings"
)

// display 是一台能用 DDC/CI 控制的显示器。
type display struct {
	ID      string // macOS：名称#序列号（没有序列号时只用名称，同型号按检测顺序加 #2）；Windows：名称#连接路径哈希
	Name    string // 如 "DELL G3223Q"
	Inputs  []code // capabilities 里 VCP 0x60 的可选值；读不到为空
	Caps    string // 本次读到的 capabilities 原文（用了缓存则为空），诊断用
	Current code   // 当前输入（VCP 0x60 低字节）；读不到为 0
}

// code 是 VCP 0x60 的输入源编码，JSON 里写成 "0x0F" 方便手改。
type code byte

func (c code) MarshalText() ([]byte, error) { return []byte(fmt.Sprintf("0x%02X", byte(c))), nil }

func (c *code) UnmarshalText(b []byte) error {
	v, err := strconv.ParseUint(string(b), 0, 8)
	*c = code(v)
	return err
}

// MCCS 标准编码的简称，外加厂商常用的 USB-C（Dell 等用 0x1B）。
var portNames = map[code]string{
	0x01: "VGA", 0x02: "VGA 2", 0x03: "DVI", 0x04: "DVI 2",
	0x0F: "DP", 0x10: "DP 2", 0x11: "HDMI 1", 0x12: "HDMI 2", 0x1B: "USB-C",
}

// commonInputs 是读不到 capabilities 时给用户的候选。
var commonInputs = []code{0x0F, 0x10, 0x11, 0x12, 0x1B}

func (c code) String() string {
	if c == 0 {
		return ""
	}
	if n, ok := portNames[c]; ok {
		return n
	}
	return fmt.Sprintf("0x%02X", byte(c))
}

// joinCodes 把输入列表写成 "DP, HDMI 1, USB-C"：名字本身带空格，用逗号才分得清。
func joinCodes(cs []code) string {
	s := make([]string, len(cs))
	for i, c := range cs {
		s[i] = c.String()
	}
	return strings.Join(s, ", ")
}

// parseCode 是 String 的逆运算，也接受用户手填的 "0x31"、"49"。
func parseCode(s string) code {
	s = strings.TrimSpace(s)
	for c, n := range portNames {
		if strings.EqualFold(n, s) {
			return c
		}
	}
	if v, err := strconv.ParseUint(s, 0, 8); err == nil {
		return code(v)
	}
	return 0
}

var vcp60 = regexp.MustCompile(`[\s(]60\s*\(([0-9A-Fa-f\s]*)\)`)

// parseInputs 从 capabilities 字符串里取出 VCP 0x60 的可选值：容忍多余空格、
// 值之间没有分隔（按两位切分，同 ddcutil）、重复值，去掉 0x00 和 0xFF。
func parseInputs(caps string) []code {
	m := vcp60.FindStringSubmatch(caps)
	if m == nil {
		return nil
	}
	var out []code
	seen := map[code]bool{}
	for _, tok := range strings.Fields(m[1]) {
		if len(tok)%2 != 0 {
			tok = "0" + tok
		}
		for i := 0; i < len(tok); i += 2 {
			v, err := strconv.ParseUint(tok[i:i+2], 16, 8)
			if c := code(v); err == nil && c != 0 && c != 0xFF && !seen[c] {
				seen[c] = true
				out = append(out, c)
			}
		}
	}
	return out
}

// maskID 遮住 ID 里的序列号，给要贴到公开 issue 的诊断信息用：名称照留，"#" 后面长于两个字符的段
// （macOS 的序列号、Windows 的路径哈希）换成它的 4 位短哈希——推不回原值，同型号的几台仍分得清
// （序列号的末几位常常相同）；同型号的编号（"#2"）不动。
func maskID(id string) string {
	parts := strings.Split(id, "#")
	for i := 1; i < len(parts); i++ {
		if len([]rune(parts[i])) > 2 {
			h := fnv.New32a()
			h.Write([]byte(parts[i]))
			parts[i] = fmt.Sprintf("•%04x", h.Sum32()&0xffff)
		}
	}
	return strings.Join(parts, "#")
}

// uniqueIDs 给重复的 ID 加 "#2"、"#3"，保证同一次检测里 ID 唯一。
func uniqueIDs(ds []display) {
	n := map[string]int{}
	for i := range ds {
		n[ds[i].ID]++
		if k := n[ds[i].ID]; k > 1 {
			ds[i].ID = fmt.Sprintf("%s#%d", ds[i].ID, k)
		}
	}
}
