package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testConfig() *config {
	cfg := defaultConfig()
	cfg.Self = 0
	cfg.Monitors["a"] = &monitor{Name: "A", Enabled: true, Ports: map[string]code{"Mac": 0x0F, "PC": 0x11}}
	cfg.Monitors["b"] = &monitor{Name: "B", Enabled: true, Ports: map[string]code{"Mac": 0x1B}}
	cfg.Hotkeys = map[string]string{"PC": "Ctrl+Alt+P"}
	return cfg
}

func TestRenameAndRemoveComputer(t *testing.T) {
	cfg := testConfig()
	if err := cfg.renameComputer(1, "Mac"); err == nil {
		t.Error("duplicate name accepted")
	}
	if err := cfg.renameComputer(1, "Gaming PC"); err != nil {
		t.Fatal(err)
	}
	if cfg.Monitors["a"].Ports["Gaming PC"] != 0x11 || cfg.Hotkeys["Gaming PC"] != "Ctrl+Alt+P" {
		t.Errorf("rename did not move ports/hotkeys: %+v %v", cfg.Monitors["a"].Ports, cfg.Hotkeys)
	}
	if err := cfg.removeComputer(1); err == nil {
		t.Error("removed down to one computer")
	}
	cfg.addComputer()
	if err := cfg.removeComputer(0); err == nil {
		t.Error("removed self")
	}
	if err := cfg.removeComputer(2); err != nil || len(cfg.Computers) != 2 {
		t.Errorf("remove third: %v %v", err, cfg.Computers)
	}
}

func TestPlan(t *testing.T) {
	cfg := testConfig()
	online := map[string]display{"a": {ID: "a"}, "b": {ID: "b"}}
	if got, err := plan(cfg, online, []string{"a"}, 1); err != nil || got["a"] != 0x11 {
		t.Errorf("plan a→PC = %v, %v", got, err)
	}
	if _, err := plan(cfg, online, []string{"a", "b"}, 1); err == nil || !strings.Contains(err.Error(), "B") {
		t.Errorf("missing port on B not reported: %v", err)
	}
	if _, err := plan(cfg, map[string]display{"b": {ID: "b"}}, []string{"a"}, 1); err == nil {
		t.Error("offline monitor accepted")
	}
	if _, err := plan(cfg, online, nil, 1); err == nil {
		t.Error("empty selection accepted")
	}
}

func TestComputerOf(t *testing.T) {
	cfg := testConfig()
	m := cfg.Monitors["a"]
	if cfg.computerOf(m, 0x11) != 1 || cfg.computerOf(m, 0x0F) != 0 || cfg.computerOf(m, 0x12) != -1 || cfg.computerOf(m, 0) != -1 {
		t.Error("computerOf")
	}
}

func TestWiringOf(t *testing.T) {
	cfg := testConfig()
	cfg.Monitors["c"] = &monitor{Name: "C", Ports: map[string]code{}}
	w := wiringOf(cfg)
	if w["a"] != "Mac → DP, PC → HDMI 1" || w["b"] != "Mac → USB-C" || w["c"] != "(not set)" {
		t.Errorf("wiringOf = %q", w)
	}
}

func TestPickLang(t *testing.T) {
	for _, c := range []struct{ setting, locale, want string }{
		{"", "zh-Hans-CN", "zh"},
		{"", "zh-Hant-TW", "zh"},
		{"", "zh", "zh"}, // Windows 的 uiLocale
		{"", "en-US", "en"},
		{"", "fr-FR", "en"},
		{"en", "zh-Hans-CN", "en"},
		{"zh", "en-US", "zh"},
		{"fr", "zh-Hans-CN", "zh"}, // 读不懂的设置按跟随系统
	} {
		if got := pickLang(c.setting, c.locale); got != c.want {
			t.Errorf("pickLang(%q, %q) = %q, want %q", c.setting, c.locale, got, c.want)
		}
	}
}

// 0.1.0 的设置没有 homekitAs：已经打开 Apple 家庭的照旧当电视，其余用默认的开关；写明了的照写。
func TestHomeKitAsMigration(t *testing.T) {
	dir := t.TempDir()
	for _, c := range []struct{ json, want string }{
		{`{"version":2,"computers":["Mac","PC"],"homekit":true}`, "tv"},
		{`{"version":2,"computers":["Mac","PC"]}`, "switch"},
		{`{"version":2,"computers":["Mac","PC"],"homekit":true,"homekitAs":"switch"}`, "switch"},
		{`{"version":2,"computers":["Mac","PC"],"homekitAs":"tv"}`, "tv"},
		{`not json`, "switch"},
	} {
		path := filepath.Join(dir, "config.json")
		if err := os.WriteFile(path, []byte(c.json), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := readConfig(path, filepath.Join(dir, "none.json")).HomeKitAs; got != c.want {
			t.Errorf("%s: HomeKitAs %q, want %q", c.json, got, c.want)
		}
	}
	if got := readConfig(filepath.Join(dir, "missing.json"), filepath.Join(dir, "none.json")).HomeKitAs; got != "switch" {
		t.Errorf("new install: HomeKitAs %q", got)
	}
}
