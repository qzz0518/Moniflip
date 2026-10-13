package main

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/brutella/hap"
	"github.com/brutella/hap/accessory"
	"github.com/brutella/hap/characteristic"
	hlog "github.com/brutella/hap/log"
	"github.com/brutella/hap/service"
	"github.com/egoist/mygo"
)

// homekit 把 Moniflip 加入 Apple 家庭，在设置里选样子：
//   - 开关：每台电脑一个开关，打开哪个就把勾选的显示器切到哪台。场景、自动化和配件按钮都能用。
//   - 电视：输入源就是各台电脑，在“家庭”App 里选输入源切换；但场景和自动化只能开关电视，选不了输入源。
//
// 配件的类别在配对时定下，换样子就是换一台新配件（见 setHomeKitAs）。电视配件必须独立配对（不能挂在桥下），
// 开关也照样独立配对。电脑增删后重启服务，hap 会因配置哈希变化递增 c#，让家庭 App 重新读取。
type homekit struct {
	cancel    context.CancelFunc
	server    *hap.Server
	tv        *service.Television // 电视样子；开关样子为 nil
	switches  []*service.Switch   // 开关样子，与 computers 对应
	computers []string            // 启动时的电脑列表
	ids       []int               // 与 computers 对应的编号（输入源的 Identifier、开关的 IID 都由它定）
	pin       string
	setup     string // 配对二维码的内容

	mu   sync.Mutex
	last *http.Request // 最近一次触发切换的请求
}

func init() { hlog.Info.Disable() }

// startHomeKit 按当前电脑列表和样子启动配件；已在运行且都没变则不动。只在主线程调用。
func (a *app) startHomeKit() {
	tv := a.cfg.HomeKitAs == "tv"
	if a.hk != nil && (a.hk.tv != nil) == tv && slices.Equal(a.hk.computers, a.cfg.Computers) {
		return
	}
	a.stopHomeKit()
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		a.notify(err.Error(), true)
		return
	}
	store := hap.NewFsStore(filepath.Join(dir, "homekit"))
	pin, setupID := setupCodes(store)

	h := &homekit{computers: slices.Clone(a.cfg.Computers), pin: pin}
	for _, name := range h.computers {
		h.ids = append(h.ids, a.cfg.inputID(name))
	}
	a.cfg.save() // 新分配的编号
	acc := h.build(tv, mygo.App.Version())
	h.setup = setupURI(pin, acc.Type, setupID)
	if h.tv != nil {
		// 先切显示器再接受新值：切换失败时家庭 App 收到错误，输入源保持原值。
		h.tv.ActiveIdentifier.OnSetRemoteValue(func(v int) error { return a.switchFromHome(slices.Index(h.ids, v)) })
		// 显示器没有电源可控：保持“开机”，家庭 App 才会显示输入源选择器。等写入的响应发出后再复位，
		// 事件才会在响应之后到达发起方。
		h.tv.Active.OnValueRemoteUpdate(func(v int) {
			if v == characteristic.ActiveInactive {
				go func() {
					time.Sleep(300 * time.Millisecond)
					h.tv.Active.SetValue(characteristic.ActiveActive)
				}()
			}
		})
	}
	for i, sw := range h.switches {
		// 同样先切再接受新值。打开就切到这台；两台电脑时关掉一台就是切到另一台，在“家庭”里点一下就能来回切。
		// 更多台时关掉不知道该切去哪：接受这次写入，稍后按实际状态复位。整个配件一起“打开”时一次请求会写好几个开关，
		// 只按第一个切，免得依次切过每一台。
		sw.On.SetValueRequestFunc = func(v any, req *http.Request) (any, int) {
			to := -1
			switch {
			case v.(bool):
				to = i
			case len(h.switches) == 2:
				to = 1 - i
			}
			if to >= 0 {
				h.mu.Lock()
				first := req != h.last
				h.last = req
				h.mu.Unlock()
				if first {
					if err := a.switchFromHome(to); err != nil {
						return nil, hap.JsonStatusServiceCommunicationFailure
					}
					return nil, 0
				}
			}
			go func() {
				time.Sleep(300 * time.Millisecond)
				a.win.Update(a.syncHomeKit)
			}()
			return nil, 0
		}
	}

	h.server, err = hap.NewServer(store, acc)
	if err != nil {
		a.cfg.HomeKit = false // 开关复位；重新打开即重试
		a.notify(fmt.Sprintf(tr("Apple 家庭启动失败：%v", "Apple Home failed to start: %v"), err), true)
		return
	}
	h.server.Pin, h.server.SetupId = pin, setupID
	ctx, cancel := context.WithCancel(context.Background())
	h.cancel = cancel
	// 等上一个服务完全停下再起：它停下时发的 Bonjour 告别与这个同名，晚到的话会把新服务的公告
	// 从 iPhone 的缓存里删掉，“家庭”就找不到配件了。
	prev, done := a.hkDone, make(chan struct{})
	a.hkDone = done
	go func() {
		defer close(done)
		if prev != nil {
			<-prev
		}
		if ctx.Err() != nil {
			return
		}
		if err := h.server.ListenAndServe(ctx); err != nil && ctx.Err() == nil {
			logf("homekit: %v", err)
			a.win.Update(func() {
				if a.hk == h {
					a.stopHomeKit()
					a.cfg.HomeKit = false // 开关复位，设置页不再显示“等待配对”；重新打开即重试
					a.p.HomeKitCode, a.p.HomeKitSetup = "", ""
					a.refreshPanel()
				}
				a.notify(fmt.Sprintf(tr("Apple 家庭出错：%v", "Apple Home error: %v"), err), true)
			})
		}
	}()
	a.hk = h
	a.syncHomeKit()
}

// build 按 computers、ids 搭出配件（电视或开关），不接回调。
func (h *homekit) build(tv bool, firmware string) *accessory.A {
	typ := accessory.TypeSwitch
	if tv {
		typ = accessory.TypeTelevision
	}
	acc := accessory.New(accessory.Info{
		Name: "Moniflip", Manufacturer: "Moniflip", Model: "Moniflip",
		SerialNumber: "MONIFLIP", Firmware: firmware,
	}, typ)
	acc.Id = 1
	if tv {
		h.tv = service.NewTelevision()
		h.tv.Primary = true
		h.tv.Active.SetValue(characteristic.ActiveActive)
		h.tv.ConfiguredName.SetValue("Moniflip")
		h.tv.SleepDiscoveryMode.SetValue(characteristic.SleepDiscoveryModeAlwaysDiscoverable)
		acc.AddS(h.tv.S)
	}
	for i, name := range h.computers {
		if !tv {
			sw := service.NewSwitch()
			// iOS 16 起“家庭”用 ConfiguredName 给配件里的各个服务命名
			n, cn := characteristic.NewName(), characteristic.NewConfiguredName()
			n.SetValue(name)
			cn.SetValue(name)
			sw.AddC(n.C)
			sw.AddC(cn.C)
			// IID 由编号定：自动化和按钮按 IID 记住开关，电脑改名、增删后各开关的 IID 都不变。
			// 配件信息占 1–7，编号从 1 起，乘 100 不会撞上。
			sw.Id = 100 * uint64(h.ids[i])
			for j, c := range sw.Cs {
				c.Id = sw.Id + 1 + uint64(j)
			}
			acc.AddS(sw.S)
			h.switches = append(h.switches, sw)
			continue
		}
		in := service.NewInputSource()
		id := characteristic.NewIdentifier() // hap 生成的 InputSource 没带 Identifier，ActiveIdentifier 靠它对应
		id.SetValue(h.ids[i])
		in.AddC(id.C)
		n := characteristic.NewName()
		n.SetValue(name)
		in.AddC(n.C)
		in.ConfiguredName.SetValue(name)
		in.InputSourceType.SetValue(characteristic.InputSourceTypeHdmi)
		in.IsConfigured.SetValue(characteristic.IsConfiguredConfigured)
		in.CurrentVisibilityState.SetValue(characteristic.CurrentVisibilityStateShown)
		acc.AddS(in.S)
		h.tv.AddS(in.S)
	}
	return acc
}

func (a *app) stopHomeKit() {
	if a.hk != nil {
		a.hk.cancel()
		a.hk = nil
	}
}

// unpairHomeKit 清掉配件的全部存储：配对记录、配对码、Setup ID 和配件身份（设备 ID、密钥）。
// 关掉 Apple 家庭或换样子时调用，再打开就是一台新配件。不清配对记录的话，在“家庭”里离线移除配件后，
// 本机仍以为已配对，不再接受新的配对；不换配对码的话，旧码可能被人看到过，解除配对后它又能用来配对；
// 不换身份的话，“家庭”里还没删掉的旧配件与新配件是同一个设备 ID，重新添加时可能冲突。须在服务停下后调用。
func unpairHomeKit() {
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		return
	}
	if err := os.RemoveAll(filepath.Join(dir, "homekit")); err != nil {
		logf("homekit: %v", err)
	}
}

// switchFromHome 在 hap 的协程里调用：在主线程算出目标并占住“切换中”，再在这里切换并等回读确认。
func (a *app) switchFromHome(to int) error {
	type result struct {
		targets map[string]code
		err     error
	}
	ch := make(chan result, 1)
	mygo.RunOnMain(func() {
		if to < 0 || to >= len(a.cfg.Computers) {
			ch <- result{err: fmt.Errorf(tr("未知的输入源 %d", "Unknown input %d"), to+1)}
			return
		}
		if a.p.Switching {
			ch <- result{err: errors.New(tr("正在切换", "A switch is in progress"))}
			return
		}
		var ids []string
		for _, id := range a.order {
			if m := a.cfg.Monitors[id]; m.Enabled && a.online[id].ID != "" {
				ids = append(ids, id)
			}
		}
		t, err := plan(a.cfg, a.online, ids, to)
		if err == nil {
			a.switchGen++
			a.p.Switching = true
			for i := range a.p.Rows {
				a.p.Rows[i].Busy = t[a.p.Rows[i].ID] != 0
			}
			a.invalidate()
		}
		ch <- result{t, err}
	})
	r := <-ch
	began := r.err == nil
	var o outcome
	if began {
		o, r.err = switchInputs(r.targets)
	}
	a.win.Update(func() {
		if began {
			a.p.Switching = false
			for i := range a.p.Rows {
				a.p.Rows[i].Busy = false
			}
		}
		if r.err != nil {
			logf("homekit switch: %v", r.err)
			a.notify(r.err.Error(), true)
		}
		a.detect(true)
	})
	if r.err == nil {
		go func() { // 跳回了原输入时再检测一次，让磁贴显示真实状态
			if len(bounced(r.targets, o)) > 0 {
				a.win.Update(func() { a.detect(true) })
			}
		}()
	}
	return r.err
}

// syncHomeKit 同步当前电脑：勾选的在线显示器都在同一台电脑上时，电视选中它这个输入源、它的开关打开；
// 否则电视不选任何输入、开关全关。hap 会把与当前值相同的写入当成无操作直接丢掉，所以不一致时
// 不能停在某台电脑上，否则在“家庭”里选那台（打开它的开关）就不会真正切换。
func (a *app) syncHomeKit() {
	if a.hk == nil {
		return
	}
	a.p.HomeKitCode = a.hk.pin[:3] + "-" + a.hk.pin[3:5] + "-" + a.hk.pin[5:]
	a.p.HomeKitSetup = a.hk.setup
	a.p.HomeKitPaired = a.hk.server.IsPaired()
	want, n := 0, 0
	for _, r := range a.p.Rows {
		if !r.Enabled || !r.Online {
			continue
		}
		n++
		id := 0
		if r.Current >= 0 && r.Current < len(a.cfg.Computers) {
			if i := slices.Index(a.hk.computers, a.cfg.Computers[r.Current]); i >= 0 {
				id = a.hk.ids[i]
			}
		}
		if n == 1 {
			want = id
		} else if id != want {
			want = 0
		}
	}
	if a.hk.tv != nil && a.hk.tv.ActiveIdentifier.Value() != want {
		a.hk.tv.ActiveIdentifier.SetValue(want)
	}
	for i, sw := range a.hk.switches {
		sw.On.SetValue(want != 0 && a.hk.ids[i] == want)
	}
}

// setupCodes 返回本机的配对码和 Setup ID：首次随机生成，之后从存储里读，重装或升级都不变；
// 关掉 Apple 家庭时被删掉（见 unpairHomeKit），再打开时重新生成。
func setupCodes(st hap.Store) (pin, setupID string) {
	if b, err := st.Get("moniflip.pin"); err == nil && len(b) == 8 {
		pin = string(b)
	} else {
		for pin == "" || hap.InvalidPins[pin] {
			n, _ := rand.Int(rand.Reader, big.NewInt(1e8))
			pin = fmt.Sprintf("%08d", n)
		}
		st.Set("moniflip.pin", []byte(pin))
	}
	if b, err := st.Get("moniflip.setupid"); err == nil && len(b) == 4 {
		setupID = string(b)
	} else {
		const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
		for range 4 {
			n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
			setupID += string(alphabet[n.Int64()])
		}
		st.Set("moniflip.setupid", []byte(setupID))
	}
	return pin, setupID
}

// setupURI 是“家庭”App 扫的配对二维码内容（HAP 规范）：X-HM:// 后接 9 位 36 进制的载荷——
// 配件类别、只走 IP 的标志、8 位配对码——再接 4 位 Setup ID。配件在 Bonjour 里广播的
// Setup ID 哈希与之对应，“家庭”App 才认得出扫的是哪台。
func setupURI(pin string, category byte, setupID string) string {
	n, _ := strconv.ParseUint(pin, 10, 27)
	s := strings.ToUpper(strconv.FormatUint(uint64(category)<<31|2<<27|n, 36))
	return "X-HM://" + strings.Repeat("0", max(0, 9-len(s))) + s + setupID
}
