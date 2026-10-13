package main

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

func TestParseInputs(t *testing.T) {
	for _, tc := range []struct {
		caps string
		want []code
	}{
		{"vcp(02 04 10 12 60(0F 11 12 ) 62)", []code{0x0F, 0x11, 0x12}},      // Dell G3223Q
		{"vcp(10 12 60( 1B 0F 11) AA(01 02 04))", []code{0x1B, 0x0F, 0x11}},  // Dell U2723QX
		{"vcp(16181A5260(3033 04))", nil},                                    // 没有分隔的 vcp 码：60 前不是空白或括号，不误判
		{"vcp(10 60 ( 0f 11 ) 62)", []code{0x0F, 0x11}},                      // 括号前有空格、小写
		{"vcp(60(11 12 0F FF 11 12 0F 31))", []code{0x11, 0x12, 0x0F, 0x31}}, // Lenovo：重复值和 0xFF
		{"vcp(60(0f1112))", []code{0x0F, 0x11, 0x12}},                        // 值之间没有分隔
		{"(prot(monitor)type(lcd)model(X)cmds(01 02)vcp(10 12))", nil},       // 没有 0x60
	} {
		if got := parseInputs(tc.caps); !slices.Equal(got, tc.want) {
			t.Errorf("parseInputs(%q) = %v, want %v", tc.caps, got, tc.want)
		}
	}
}

func TestCodeNamesRoundTrip(t *testing.T) {
	for _, c := range []code{0x0F, 0x11, 0x1B, 0x31} {
		if got := parseCode(c.String()); got != c {
			t.Errorf("parseCode(%q) = %v, want %v", c.String(), got, c)
		}
	}
	if code(0).String() != "" || parseCode("") != 0 || parseCode("49") != 0x31 {
		t.Error("zero/decimal handling")
	}
	b, _ := json.Marshal(map[string]code{"PC": 0x11})
	if string(b) != `{"PC":"0x11"}` {
		t.Errorf("marshal = %s", b)
	}
	var m map[string]code
	if err := json.Unmarshal([]byte(`{"Mac":"0x1B"}`), &m); err != nil || m["Mac"] != 0x1B {
		t.Errorf("unmarshal = %v %v", m, err)
	}
}

func TestUniqueIDs(t *testing.T) {
	ds := []display{{ID: "DELL U2723QX"}, {ID: "DELL U2723QX"}, {ID: "LG#1"}}
	uniqueIDs(ds)
	if ds[0].ID != "DELL U2723QX" || ds[1].ID != "DELL U2723QX#2" || ds[2].ID != "LG#1" {
		t.Errorf("uniqueIDs = %v", ds)
	}
}

func TestMaskID(t *testing.T) {
	for _, id := range []string{"DELL G3223Q#2", "DELL G3223Q"} { // 没有序列号的照原样
		if got := maskID(id); got != id {
			t.Errorf("maskID(%q) = %q", id, got)
		}
	}
	// 序列号（macOS）和路径哈希（Windows）都看不到，末两位相同的两台也分得开，同型号编号保留
	a, b := maskID("DELL G3223Q#AB12CN3"), maskID("DELL G3223Q#XY98ZN3")
	if strings.Contains(a+b, "AB12") || strings.Contains(a+b, "XY98") || a == b || !strings.HasPrefix(a, "DELL G3223Q#•") {
		t.Errorf("maskID: %q, %q", a, b)
	}
	if got := maskID("DELL G3223Q#AB12CN3#2"); got != a+"#2" {
		t.Errorf("maskID with duplicate suffix = %q, want %q", got, a+"#2")
	}
	if got := maskID("DELL G3223Q#1a2b3c4d"); strings.Contains(got, "1a2b") {
		t.Errorf("maskID(windows) = %q", got)
	}
	if got := joinCodes([]code{0x0F, 0x11, 0x31}); got != "DP, HDMI 1, 0x31" {
		t.Errorf("joinCodes = %q", got)
	}
}
