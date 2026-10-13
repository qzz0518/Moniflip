package main

import (
	"bytes"
	"fmt"
	"hash/fnv"
	"syscall"
	"time"
	"unsafe"
)

// Windows 的 DDC 通道：dxva2 SetVCPFeature；显示器名称取自 QueryDisplayConfig（来自 EDID）。
var (
	user32                                  = syscall.NewLazyDLL("user32.dll")
	dxva2                                   = syscall.NewLazyDLL("dxva2.dll")
	procGetDisplayConfigBufferSizes         = user32.NewProc("GetDisplayConfigBufferSizes")
	procQueryDisplayConfig                  = user32.NewProc("QueryDisplayConfig")
	procDisplayConfigGetDeviceInfo          = user32.NewProc("DisplayConfigGetDeviceInfo")
	procEnumDisplayMonitors                 = user32.NewProc("EnumDisplayMonitors")
	procGetMonitorInfoW                     = user32.NewProc("GetMonitorInfoW")
	procGetNumberOfPhysicalMonitorsFromHMON = dxva2.NewProc("GetNumberOfPhysicalMonitorsFromHMONITOR")
	procGetPhysicalMonitorsFromHMONITOR     = dxva2.NewProc("GetPhysicalMonitorsFromHMONITOR")
	procDestroyPhysicalMonitors             = dxva2.NewProc("DestroyPhysicalMonitors")
	procSetVCPFeature                       = dxva2.NewProc("SetVCPFeature")
	procGetCapabilitiesStringLength         = dxva2.NewProc("GetCapabilitiesStringLength")
	procCapabilitiesRequestAndReply         = dxva2.NewProc("CapabilitiesRequestAndCapabilitiesReply")
	procGetVCPFeatureAndReply               = dxva2.NewProc("GetVCPFeatureAndVCPFeatureReply")
)

const (
	qdcOnlyActivePaths  = 0x2
	getSourceName       = 1
	getTargetName       = 2
	errInsufficientBuff = 122
)

type luid struct {
	low  uint32
	high int32
}

// DISPLAYCONFIG_PATH_INFO
type pathInfo struct {
	srcAdapter              luid
	srcID, srcMode, srcFlag uint32
	tgtAdapter              luid
	tgtID, tgtMode          uint32
	outputTech, rotation    uint32
	scaling                 uint32
	refreshNum, refreshDen  uint32
	scanLine                uint32
	available               int32
	tgtFlag                 uint32
	flags                   uint32
}

// DISPLAYCONFIG_MODE_INFO，内容用不到
type modeInfo [64]byte

// DISPLAYCONFIG_DEVICE_INFO_HEADER
type infoHeader struct {
	typ, size uint32
	adapter   luid
	id        uint32
}

// DISPLAYCONFIG_TARGET_DEVICE_NAME
type targetName struct {
	hdr          infoHeader
	flags        uint32
	outputTech   uint32
	manufacturer uint16
	product      uint16
	connector    uint32
	friendlyName [64]uint16
	devicePath   [128]uint16
}

// DISPLAYCONFIG_SOURCE_DEVICE_NAME
type sourceName struct {
	hdr     infoHeader
	gdiName [32]uint16
}

// MONITORINFOEXW
type monitorInfoEx struct {
	size    uint32
	monitor [4]int32
	work    [4]int32
	flags   uint32
	device  [32]uint16
}

// PHYSICAL_MONITOR
type physicalMonitor struct {
	handle uintptr
	desc   [128]uint16
}

// 结构体大小必须与 Windows x64 头文件一致，不一致则编译失败。
var (
	_ [72]byte  = [unsafe.Sizeof(pathInfo{})]byte{}
	_ [64]byte  = [unsafe.Sizeof(modeInfo{})]byte{}
	_ [420]byte = [unsafe.Sizeof(targetName{})]byte{}
	_ [84]byte  = [unsafe.Sizeof(sourceName{})]byte{}
	_ [104]byte = [unsafe.Sizeof(monitorInfoEx{})]byte{}
	_ [264]byte = [unsafe.Sizeof(physicalMonitor{})]byte{}
)

// target 是 QueryDisplayConfig 里的一个显示器。
type target struct {
	name string // EDID 里的名称，如 "DELL G3223Q"
	path string // monitorDevicePath，连接相关但稳定，用来区分同型号显示器
}

// displayTargets 返回 GDI 设备名（\\.\DISPLAY1）→ 显示器。
func displayTargets() (map[string]target, error) {
	var paths []pathInfo
	var modes []modeInfo
	for {
		var np, nm uint32
		if r, _, _ := procGetDisplayConfigBufferSizes.Call(qdcOnlyActivePaths, uintptr(unsafe.Pointer(&np)), uintptr(unsafe.Pointer(&nm))); r != 0 {
			return nil, fmt.Errorf("GetDisplayConfigBufferSizes: %d", r)
		}
		if np == 0 {
			return nil, nil
		}
		paths, modes = make([]pathInfo, np), make([]modeInfo, nm)
		r, _, _ := procQueryDisplayConfig.Call(qdcOnlyActivePaths,
			uintptr(unsafe.Pointer(&np)), uintptr(unsafe.Pointer(&paths[0])),
			uintptr(unsafe.Pointer(&nm)), uintptr(unsafe.Pointer(&modes[0])), 0)
		if r == errInsufficientBuff {
			continue
		}
		if r != 0 {
			return nil, fmt.Errorf("QueryDisplayConfig: %d", r)
		}
		paths = paths[:np]
		break
	}
	out := map[string]target{}
	for _, p := range paths {
		src := sourceName{hdr: infoHeader{typ: getSourceName, size: uint32(unsafe.Sizeof(sourceName{})), adapter: p.srcAdapter, id: p.srcID}}
		tgt := targetName{hdr: infoHeader{typ: getTargetName, size: uint32(unsafe.Sizeof(targetName{})), adapter: p.tgtAdapter, id: p.tgtID}}
		if r, _, _ := procDisplayConfigGetDeviceInfo.Call(uintptr(unsafe.Pointer(&src))); r != 0 {
			continue
		}
		if r, _, _ := procDisplayConfigGetDeviceInfo.Call(uintptr(unsafe.Pointer(&tgt))); r != 0 {
			continue
		}
		switch tgt.outputTech {
		case 6, 11, 13, 0x80000000: // LVDS、内嵌 DP、内嵌 UDI、INTERNAL：笔记本内屏，不走 DDC/CI
			continue
		}
		out[syscall.UTF16ToString(src.gdiName[:])] = target{syscall.UTF16ToString(tgt.friendlyName[:]), syscall.UTF16ToString(tgt.devicePath[:])}
	}
	return out, nil
}

type physical struct {
	id, name string
	handle   uintptr // 唯一的物理显示器句柄（复制模式不列出）
	all      []physicalMonitor
}

// EnumDisplayMonitors 回调的输入输出；回调只建一次（NewCallback 数量有上限），调用方持有 ddc 锁。
var (
	enumTargets map[string]target
	enumOut     []physical
	enumCB      = syscall.NewCallback(func(hmon, hdc, rect, lparam uintptr) uintptr {
		mi := monitorInfoEx{size: uint32(unsafe.Sizeof(monitorInfoEx{}))}
		if r, _, _ := procGetMonitorInfoW.Call(hmon, uintptr(unsafe.Pointer(&mi))); r == 0 {
			return 1
		}
		t := enumTargets[syscall.UTF16ToString(mi.device[:])]
		var n uint32
		if r, _, _ := procGetNumberOfPhysicalMonitorsFromHMON.Call(hmon, uintptr(unsafe.Pointer(&n))); r == 0 || n == 0 || t.name == "" {
			return 1
		}
		if n != 1 { // 复制模式：一个 HMONITOR 对应多台，名称和句柄对不上号，宁可不列出也不能写错显示器
			logf("%s: mirrored displays are not supported", t.name)
			return 1
		}
		pm := make([]physicalMonitor, n)
		if r, _, _ := procGetPhysicalMonitorsFromHMONITOR.Call(hmon, uintptr(n), uintptr(unsafe.Pointer(&pm[0]))); r == 0 {
			return 1
		}
		h := fnv.New32a()
		h.Write([]byte(t.path))
		enumOut = append(enumOut, physical{id: fmt.Sprintf("%s#%08x", t.name, h.Sum32()), name: t.name, handle: pm[0].handle, all: pm})
		return 1
	})
)

func physicalMonitors() []physical {
	targets, err := displayTargets()
	if err != nil {
		logf("enumerate monitors: %v", err)
		return nil
	}
	enumTargets, enumOut = targets, nil
	procEnumDisplayMonitors.Call(0, 0, enumCB, 0)
	return enumOut
}

func release(ps []physical) {
	for _, p := range ps {
		procDestroyPhysicalMonitors.Call(uintptr(len(p.all)), uintptr(unsafe.Pointer(&p.all[0])))
	}
}

func capabilities(h uintptr) string {
	var n uint32
	if r, _, _ := procGetCapabilitiesStringLength.Call(h, uintptr(unsafe.Pointer(&n))); r == 0 || n == 0 {
		return ""
	}
	buf := make([]byte, n)
	if r, _, _ := procCapabilitiesRequestAndReply.Call(h, uintptr(unsafe.Pointer(&buf[0])), uintptr(n)); r == 0 {
		return ""
	}
	return string(bytes.TrimRight(buf, "\x00"))
}

// currentInput 读 VCP 0x60；当前输入在低字节。
func currentInput(h uintptr) code {
	var typ, cur, max uint32
	if r, _, _ := procGetVCPFeatureAndReply.Call(h, 0x60, uintptr(unsafe.Pointer(&typ)), uintptr(unsafe.Pointer(&cur)), uintptr(unsafe.Pointer(&max))); r == 0 {
		return 0
	}
	return code(cur & 0xFF)
}

func toDisplays(ps []physical) []display {
	out := make([]display, len(ps))
	for i, p := range ps {
		out[i] = display{ID: p.id, Name: p.name}
	}
	uniqueIDs(out)
	return out
}

func detectDisplays(known map[string][]code) []display {
	ps := physicalMonitors()
	defer release(ps)
	out := toDisplays(ps)
	for i, p := range ps {
		x := &out[i]
		x.Current = currentInput(p.handle)
		if in, ok := known[x.ID]; ok {
			x.Inputs = in
		} else {
			x.Caps = capabilities(p.handle)
			x.Inputs = parseInputs(x.Caps)
		}
	}
	return out
}

func withHandles(fn func(hs map[string]uintptr)) {
	ps := physicalMonitors()
	defer release(ps)
	hs := map[string]uintptr{}
	for i, d := range toDisplays(ps) {
		hs[d.ID] = ps[i].handle
	}
	fn(hs)
}

func setInputs(targets map[string]code) (missing, failed []string) {
	withHandles(func(hs map[string]uintptr) {
		for id := range targets {
			if hs[id] == 0 {
				missing = append(missing, id)
			}
		}
		if len(missing) > 0 {
			return
		}
		// 两台都发一遍，10ms 后再发一遍；切走后显示器可能不再应答，返回失败不一定代表没切成
		ok := map[string]bool{}
		for range 2 {
			for id, c := range targets {
				if r, _, _ := procSetVCPFeature.Call(hs[id], 0x60, uintptr(c)); r != 0 {
					ok[id] = true
				}
			}
			time.Sleep(10 * time.Millisecond)
		}
		for id := range targets {
			if !ok[id] {
				failed = append(failed, id)
			}
		}
	})
	return missing, failed
}

func readInputs(ids []string) map[string]code {
	out := map[string]code{}
	withHandles(func(hs map[string]uintptr) {
		for _, id := range ids {
			if h := hs[id]; h != 0 {
				out[id] = currentInput(h)
			}
		}
	})
	return out
}
