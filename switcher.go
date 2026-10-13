package main

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"
)

// ddc 让检测、切换和回读不同时占用 DDC。
var ddc sync.Mutex

// plan 算出把显示器 ids 切到第 to 台电脑要写的输入；任何一台不满足条件就整体不切。
func plan(cfg *config, online map[string]display, ids []string, to int) (map[string]code, error) {
	if len(ids) == 0 {
		return nil, errors.New(tr("没有勾选要切换的显示器", "No monitors are checked"))
	}
	name := cfg.Computers[to]
	targets := map[string]code{}
	for _, id := range ids {
		m := cfg.Monitors[id]
		switch {
		case m == nil:
			return nil, fmt.Errorf(tr("找不到显示器 %s", "Monitor %s not found"), id)
		case online[id].ID == "":
			return nil, fmt.Errorf(tr("没有检测到 %s", "%s isn’t detected"), m.Name)
		case m.Ports[name] == 0:
			return nil, fmt.Errorf(tr("先在设置里选择 %s 上 %s 接的接口", "%s: choose the input for %s in Settings"), m.Name, name)
		}
		targets[id] = m.Ports[name]
	}
	return targets, nil
}

// outcome 是一次切换的结果，按显示器 ID。
type outcome struct {
	before     map[string]code // 切换前的输入，用于判断是否跳回
	unverified []string        // 写入后没能读回目标值（显示器可能不支持回读，未必失败）
	failed     []string        // 写入失败且读回也不是目标值
}

// switchInputs 写入后在 4 秒内回读确认。显示器切走后通常仍能读到输入（低字节），读不到就算未确认。
func switchInputs(targets map[string]code) (outcome, error) {
	ddc.Lock()
	defer ddc.Unlock()
	ids := slices.Sorted(maps.Keys(targets))
	o := outcome{before: readInputs(ids)}
	missing, failed := setInputs(targets)
	if len(missing) > 0 {
		return o, fmt.Errorf(tr("找不到显示器 %s", "Monitor %s not found"), names(missing, tr("、", ", ")))
	}
	pending := slices.Clone(ids)
	for deadline := time.Now().Add(4 * time.Second); len(pending) > 0 && time.Now().Before(deadline); {
		time.Sleep(400 * time.Millisecond)
		cur := readInputs(pending)
		pending = slices.DeleteFunc(pending, func(id string) bool { return cur[id] == targets[id] })
	}
	for _, id := range pending {
		if slices.Contains(failed, id) {
			o.failed = append(o.failed, id)
		} else {
			o.unverified = append(o.unverified, id)
		}
	}
	if len(o.failed) > 0 {
		return o, fmt.Errorf(tr("%s 没有响应", "%s did not respond"), names(o.failed, tr("、", ", ")))
	}
	if len(o.unverified) > 0 {
		logf("sent, but could not read back: %s", names(o.unverified, ", "))
	}
	return o, nil
}

// bounced 在切换几秒后再读一次：回到了原输入，多半是目标电脑在睡眠、显示器自动选回了有信号的口。
func bounced(targets map[string]code, o outcome) []string {
	time.Sleep(3 * time.Second)
	ddc.Lock()
	defer ddc.Unlock()
	var ids []string
	for id := range targets {
		if !slices.Contains(o.unverified, id) && o.before[id] != 0 && o.before[id] != targets[id] {
			ids = append(ids, id)
		}
	}
	cur := readInputs(ids)
	var out []string
	for _, id := range ids {
		if cur[id] == o.before[id] {
			out = append(out, id)
		}
	}
	return out
}

// names 把显示器 ID 列表转成给人看的名称（ID 是 "名称#序列号"），用 sep 连接：
// 界面上随语言用 tr("、", ", ")，日志固定用 ", "。
func names(ids []string, sep string) string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i], _, _ = strings.Cut(id, "#")
	}
	return strings.Join(out, sep)
}
