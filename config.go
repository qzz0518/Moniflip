package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/egoist/mygo"
)

const configVersion = 2

// config 存在用户数据目录的 config.json，只在主线程读写。
type config struct {
	Version   int                 `json:"version"`
	Computers []string            `json:"computers"`           // 共用这些显示器的电脑，如 ["Mac", "PC"]
	Self      int                 `json:"self"`                // 本机是 Computers 的第几个
	Monitors  map[string]*monitor `json:"monitors"`            // 键是显示器 ID
	Hotkeys   map[string]string   `json:"hotkeys,omitempty"`   // 电脑名 → 全局快捷键
	HomeKit   bool                `json:"homekit,omitempty"`   // 作为 Apple 家庭配件
	HomeKitAs string              `json:"homekitAs,omitempty"` // 在 Apple 家庭里的样子："switch" 每台电脑一个开关，"tv" 一台电视
	InputIDs  map[string]int      `json:"inputIds,omitempty"`  // 电脑名 → Apple 家庭里的编号（输入源、开关），只增不复用
	Language  string              `json:"language,omitempty"`  // "zh"、"en"；空为跟随系统

	path   string
	broken bool   // 文件存在但读不懂：不覆盖它，免得毁掉用户的设置
	saved  []byte // 上次写下的内容，没变就不再写
}

// monitor 是一台显示器的设置。
type monitor struct {
	Name     string          `json:"name"`
	Enabled  bool            `json:"enabled"`         // 参与“全部切到 X”
	Ports    map[string]code `json:"ports,omitempty"` // 电脑名 → 它接在这台显示器上的输入
	Inputs   []code          `json:"inputs,omitempty"`
	Portrait bool            `json:"portrait,omitempty"`
}

func defaultConfig() *config {
	cfg := &config{Version: configVersion, Computers: []string{"Mac", "PC"}, Monitors: map[string]*monitor{}, HomeKitAs: "switch"}
	if !onMac {
		cfg.Self = 1
	}
	return cfg
}

func loadConfig() *config {
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		logf("user data directory: %v", err)
		return defaultConfig()
	}
	return readConfig(filepath.Join(dir, "config.json"), filepath.Join(filepath.Dir(dir), "DellSwitch", "config.json"))
}

// readConfig 读 path 处的设置；文件不存在时迁移 legacy 处改名前的设置。
func readConfig(path, legacy string) *config {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		cfg := migrateDellSwitch(legacy)
		cfg.path = path
		return cfg
	}
	cfg := defaultConfig()
	cfg.path = path
	if err == nil {
		cfg.HomeKitAs = "" // 0.1.0 的设置里没有这一项，见下
		err = json.Unmarshal(b, cfg)
	}
	if err != nil || cfg.Version > configVersion || len(cfg.Computers) < 2 {
		logf("cannot read %s (%v); using defaults and leaving the file untouched", path, err)
		cfg = defaultConfig()
		cfg.path, cfg.broken = path, true
	}
	if cfg.Monitors == nil {
		cfg.Monitors = map[string]*monitor{}
	}
	for id, m := range cfg.Monitors {
		if m == nil {
			delete(cfg.Monitors, id)
		} else if m.Ports == nil {
			m.Ports = map[string]code{} // ports 为空时 omitempty 不落盘，读回来是 nil
		}
	}
	cfg.Self = min(max(cfg.Self, 0), len(cfg.Computers)-1)
	if cfg.HomeKitAs == "" {
		// 0.1.0 只有电视：已经打开 Apple 家庭的照旧当电视，免得配好的配件换了样子；其余用默认的开关
		cfg.HomeKitAs = "switch"
		if cfg.HomeKit {
			cfg.HomeKitAs = "tv"
		}
	}
	return cfg
}

// migrateDellSwitch 读取改名前（DellSwitch）的设置：以显示器名称为键，固定 Mac/PC 两台。
// 迁移后的条目键为 "legacy:<名称>"，检测到同名显示器时由 adopt 认领。
func migrateDellSwitch(path string) *config {
	cfg := defaultConfig()
	b, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}
	var old map[string]struct {
		Enabled  bool `json:"enabled"`
		Mac      byte `json:"mac"`
		PC       byte `json:"pc"`
		Portrait bool `json:"portrait"`
	}
	if json.Unmarshal(b, &old) != nil {
		return cfg
	}
	for name, o := range old {
		m := &monitor{Name: name, Enabled: o.Enabled, Portrait: o.Portrait, Ports: map[string]code{}}
		if o.Mac != 0 {
			m.Ports["Mac"] = code(o.Mac)
		}
		if o.PC != 0 {
			m.Ports["PC"] = code(o.PC)
		}
		cfg.Monitors["legacy:"+name] = m
	}
	logf("migrated DellSwitch settings (%d monitors)", len(old))
	return cfg
}

func (cfg *config) save() {
	if cfg.broken || cfg.path == "" {
		return
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err == nil && bytes.Equal(b, cfg.saved) {
		return // 定时检测每次都会存一下
	}
	if err == nil {
		tmp := cfg.path + ".tmp"
		if err = os.WriteFile(tmp, b, 0o644); err == nil {
			err = os.Rename(tmp, cfg.path)
		}
	}
	if err != nil {
		logf("save settings: %v", err)
		return
	}
	cfg.saved = b
}

// adopt 返回显示器的设置，没有就新建：先认领同名的旧版条目，否则只预填本机接口（当前输入），默认参与切换。
func (cfg *config) adopt(d display) *monitor {
	if m, ok := cfg.Monitors[d.ID]; ok {
		return m
	}
	m, ok := cfg.Monitors["legacy:"+d.Name]
	if ok {
		delete(cfg.Monitors, "legacy:"+d.Name)
	} else {
		m = &monitor{Name: d.Name, Enabled: true, Ports: map[string]code{}}
		if d.Current != 0 {
			m.Ports[cfg.self()] = d.Current // 首次检测时多半正对着本机；猜错可在设置里改
		}
	}
	if m.Ports == nil {
		m.Ports = map[string]code{}
	}
	cfg.Monitors[d.ID] = m
	return m
}

func (cfg *config) self() string { return cfg.Computers[cfg.Self] }

// computerOf 返回接在输入 c 上的电脑下标；没有返回 -1。
func (cfg *config) computerOf(m *monitor, c code) int {
	if c == 0 {
		return -1
	}
	for i, name := range cfg.Computers {
		if m.Ports[name] == c {
			return i
		}
	}
	return -1
}

func (cfg *config) renameComputer(i int, name string) error {
	old := cfg.Computers[i]
	if name == "" || name == old {
		return nil
	}
	if slices.Contains(cfg.Computers, name) {
		return fmt.Errorf(tr("已经有一台叫 %s 的电脑", "There is already a computer named %s"), name)
	}
	cfg.Computers[i] = name
	for _, m := range cfg.Monitors {
		if c, ok := m.Ports[old]; ok {
			delete(m.Ports, old)
			m.Ports[name] = c
		}
	}
	if h, ok := cfg.Hotkeys[old]; ok {
		delete(cfg.Hotkeys, old)
		cfg.Hotkeys[name] = h
	}
	if id, ok := cfg.InputIDs[old]; ok {
		delete(cfg.InputIDs, old)
		cfg.InputIDs[name] = id
	}
	return nil
}

// inputID 是电脑在 Apple 家庭里的编号，定了输入源的 Identifier 和开关的 IID：场景和自动化按它们记住电脑，
// 所以电脑改名、删掉别的电脑后编号都不变；删掉的编号也不再给别人用（不从 InputIDs 里删）。
func (cfg *config) inputID(name string) int {
	if cfg.InputIDs == nil {
		cfg.InputIDs = map[string]int{}
		for i, n := range cfg.Computers {
			cfg.InputIDs[n] = i + 1
		}
	}
	if id, ok := cfg.InputIDs[name]; ok {
		return id
	}
	next := 1
	for _, id := range cfg.InputIDs {
		next = max(next, id+1)
	}
	cfg.InputIDs[name] = next
	return next
}

func (cfg *config) addComputer() {
	for n := len(cfg.Computers) + 1; ; n++ {
		if name := fmt.Sprintf(tr("电脑 %d", "Computer %d"), n); !slices.Contains(cfg.Computers, name) {
			cfg.Computers = append(cfg.Computers, name)
			return
		}
	}
}

func (cfg *config) removeComputer(i int) error {
	if i == cfg.Self || len(cfg.Computers) <= 2 {
		return errors.New(tr("至少保留本机和另一台电脑", "Keep this computer and at least one other"))
	}
	name := cfg.Computers[i]
	cfg.Computers = slices.Delete(cfg.Computers, i, i+1)
	if i < cfg.Self {
		cfg.Self--
	}
	for _, m := range cfg.Monitors {
		delete(m.Ports, name)
	}
	delete(cfg.Hotkeys, name)
	return nil
}
