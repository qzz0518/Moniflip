package main

import (
	"bytes"
	"fmt"
	"time"

	"github.com/ebitengine/purego"
)

// Apple Silicon 的 DDC 通道：IOAVService 私有 API（与 m1ddc、MonitorControl 相同），经 purego 调用。
var (
	ioRegistryGetRootEntry          func(port uint32) uint32
	ioRegistryEntryCreateIterator   func(entry uint32, plane string, options uint32, it *uint32) int32
	ioIteratorNext                  func(it uint32) uint32
	ioObjectRelease                 func(obj uint32) int32
	ioObjectConformsTo              func(obj uint32, class string) int32
	ioRegistryEntryGetName          func(entry uint32, name *byte) int32
	ioRegistryEntryCreateCFProperty func(entry uint32, key, alloc uintptr, options uint32) uintptr
	ioRegistryEntrySearchCFProperty func(entry uint32, plane string, key, alloc uintptr, options uint32) uintptr
	ioAVServiceCreateWithService    func(alloc uintptr, service uint32) uintptr
	ioAVServiceWriteI2C             func(svc uintptr, chip, addr uint32, buf *byte, size uint32) int32
	ioAVServiceReadI2C              func(svc uintptr, chip, offset uint32, buf *byte, size uint32) int32

	cfStringCreateWithCString func(alloc uintptr, s string, encoding uint32) uintptr
	cfStringGetCString        func(s uintptr, buf *byte, size int, encoding uint32) uint8
	cfNumberGetValue          func(n uintptr, typ int, out *int64) uint8
	cfDictionaryGetValue      func(dict, key uintptr) uintptr
	cfGetTypeID               func(v uintptr) uintptr
	cfStringGetTypeID         func() uintptr
	cfNumberGetTypeID         func() uintptr
	cfDictionaryGetTypeID     func() uintptr
	cfRelease                 func(v uintptr)

	keyLocation, keyDisplayAttributes, keyProductAttributes, keyProductName, keyAlnumSerial, keySerial uintptr
)

const (
	cfUTF8             = 0x08000100 // kCFStringEncodingUTF8
	cfNumberSInt64     = 4          // kCFNumberSInt64Type
	iterateRecursively = 1          // kIORegistryIterateRecursively
)

func init() {
	libs := map[string]map[string]any{
		"/System/Library/Frameworks/IOKit.framework/IOKit": {
			"IORegistryGetRootEntry":          &ioRegistryGetRootEntry,
			"IORegistryEntryCreateIterator":   &ioRegistryEntryCreateIterator,
			"IOIteratorNext":                  &ioIteratorNext,
			"IOObjectRelease":                 &ioObjectRelease,
			"IOObjectConformsTo":              &ioObjectConformsTo,
			"IORegistryEntryGetName":          &ioRegistryEntryGetName,
			"IORegistryEntryCreateCFProperty": &ioRegistryEntryCreateCFProperty,
			"IORegistryEntrySearchCFProperty": &ioRegistryEntrySearchCFProperty,
			"IOAVServiceCreateWithService":    &ioAVServiceCreateWithService,
			"IOAVServiceWriteI2C":             &ioAVServiceWriteI2C,
			"IOAVServiceReadI2C":              &ioAVServiceReadI2C,
		},
		"/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation": {
			"CFStringCreateWithCString": &cfStringCreateWithCString,
			"CFStringGetCString":        &cfStringGetCString,
			"CFNumberGetValue":          &cfNumberGetValue,
			"CFDictionaryGetValue":      &cfDictionaryGetValue,
			"CFGetTypeID":               &cfGetTypeID,
			"CFStringGetTypeID":         &cfStringGetTypeID,
			"CFNumberGetTypeID":         &cfNumberGetTypeID,
			"CFDictionaryGetTypeID":     &cfDictionaryGetTypeID,
			"CFRelease":                 &cfRelease,
		},
	}
	for path, fns := range libs {
		lib, err := purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			panic(err)
		}
		for name, fn := range fns {
			purego.RegisterLibFunc(fn, lib, name)
		}
	}
	key := func(s string) uintptr { return cfStringCreateWithCString(0, s, cfUTF8) }
	keyLocation, keyDisplayAttributes, keyProductAttributes = key("Location"), key("DisplayAttributes"), key("ProductAttributes")
	keyProductName, keyAlnumSerial, keySerial = key("ProductName"), key("AlphanumericSerialNumber"), key("SerialNumber")
}

type avDisplay struct {
	id, name string
	svc      uintptr
}

// externalDisplays 返回外接显示器及其 IOAVService，用完要 release。
// 注册表递归遍历时，DCPAVServiceProxy 紧跟在它所属的 IOMobileFramebuffer 之后（m1ddc 同样依赖这一点）。
func externalDisplays() []avDisplay {
	var out []avDisplay
	var it uint32
	if ioRegistryEntryCreateIterator(ioRegistryGetRootEntry(0), "IOService", iterateRecursively, &it) != 0 {
		logf("cannot iterate the IORegistry")
		return nil
	}
	defer ioObjectRelease(it)
	var cur *avDisplay
	for e := ioIteratorNext(it); e != 0; e = ioIteratorNext(it) {
		switch {
		case ioObjectConformsTo(e, "IOMobileFramebuffer") != 0:
			cur = productAttributes(e)
		case cur != nil && entryName(e) == "DCPAVServiceProxy" && location(e) == "External":
			if cur.svc = ioAVServiceCreateWithService(0, e); cur.svc != 0 {
				out = append(out, *cur)
			}
			cur = nil
		}
		ioObjectRelease(e)
	}
	return out
}

func release(ds []avDisplay) {
	for _, d := range ds {
		cfRelease(d.svc)
	}
}

func entryName(e uint32) string {
	var buf [128]byte // io_name_t
	if ioRegistryEntryGetName(e, &buf[0]) != 0 {
		return ""
	}
	return string(buf[:bytes.IndexByte(buf[:], 0)])
}

func location(e uint32) string {
	v := ioRegistryEntryCreateCFProperty(e, keyLocation, 0, 0)
	if v == 0 {
		return ""
	}
	defer cfRelease(v)
	return cfString(v)
}

// productAttributes 取帧缓冲对应显示器的名称和序列号；没有名称（如内建屏）返回 nil。
func productAttributes(e uint32) *avDisplay {
	attrs := ioRegistryEntrySearchCFProperty(e, "IOService", keyDisplayAttributes, 0, iterateRecursively)
	if attrs == 0 {
		return nil
	}
	defer cfRelease(attrs)
	if cfGetTypeID(attrs) != cfDictionaryGetTypeID() {
		return nil
	}
	prod := cfDictionaryGetValue(attrs, keyProductAttributes)
	if prod == 0 || cfGetTypeID(prod) != cfDictionaryGetTypeID() {
		return nil
	}
	d := &avDisplay{name: cfString(cfDictionaryGetValue(prod, keyProductName))}
	if d.name == "" {
		return nil
	}
	serial := cfString(cfDictionaryGetValue(prod, keyAlnumSerial))
	if n := cfDictionaryGetValue(prod, keySerial); serial == "" && n != 0 && cfGetTypeID(n) == cfNumberGetTypeID() {
		var v int64
		if cfNumberGetValue(n, cfNumberSInt64, &v) != 0 && v != 0 {
			serial = fmt.Sprint(v)
		}
	}
	d.id = d.name
	if serial != "" {
		d.id += "#" + serial
	}
	return d
}

func cfString(v uintptr) string {
	var buf [256]byte
	if v == 0 || cfGetTypeID(v) != cfStringGetTypeID() || cfStringGetCString(v, &buf[0], len(buf), cfUTF8) == 0 {
		return ""
	}
	return string(buf[:bytes.IndexByte(buf[:], 0)])
}

// ddcSend 发一个 DDC/CI 包 [0x80|长度, 数据..., 校验]，连发两次（部分显示器会丢掉空闲后的第一个包）。
func ddcSend(svc uintptr, payload ...byte) bool {
	pkt := append([]byte{0x80 | byte(len(payload))}, payload...)
	sum := byte(0x6E ^ 0x51)
	for _, b := range pkt {
		sum ^= b
	}
	pkt = append(pkt, sum)
	ok := false
	for range 2 {
		time.Sleep(10 * time.Millisecond)
		if ioAVServiceWriteI2C(svc, 0x37, 0x51, &pkt[0], uint32(len(pkt))) == 0 {
			ok = true
		}
	}
	return ok
}

// transact 发请求并读回复，返回校验通过、操作码为 op 的回复数据（不含地址、长度和校验字节）。
func transact(svc uintptr, req []byte, op byte, n int) []byte {
	r := make([]byte, n)
	for range 5 {
		if !ddcSend(svc, req...) {
			continue
		}
		time.Sleep(50 * time.Millisecond)
		if ioAVServiceReadI2C(svc, 0x37, 0x51, &r[0], uint32(n)) != 0 {
			continue
		}
		l := int(r[1] & 0x7F)
		if r[0] != 0x6E || r[1]&0x80 == 0 || l == 0 || l+3 > n {
			continue // 空消息或格式不对，重试
		}
		sum := byte(0x50)
		for _, b := range r[:l+2] {
			sum ^= b
		}
		if sum == r[l+2] && r[2] == op {
			return r[2 : l+2]
		}
	}
	return nil
}

// capabilities 分段读取能力字符串。
func capabilities(svc uintptr) string {
	var out []byte
	for off := 0; off < 2048; {
		r := transact(svc, []byte{0xF3, byte(off >> 8), byte(off)}, 0xE3, 38)
		if len(r) <= 3 {
			break
		}
		if int(r[1])<<8|int(r[2]) != off {
			return "" // 显示器回了旧分片：这次放弃，不缓存，下次检测重读
		}
		out = append(out, r[3:]...)
		off += len(r) - 3
	}
	return string(bytes.TrimRight(out, "\x00"))
}

// currentInput 读 VCP 0x60；当前输入在低字节（SL）。
func currentInput(svc uintptr) code {
	r := transact(svc, []byte{0x01, 0x60}, 0x02, 12)
	if len(r) < 8 || r[1] != 0 || r[2] != 0x60 {
		return 0
	}
	return code(r[7])
}

func detectDisplays(known map[string][]code) []display {
	ds := externalDisplays()
	defer release(ds)
	out := make([]display, 0, len(ds))
	for _, d := range ds {
		out = append(out, display{ID: d.id, Name: d.name})
	}
	uniqueIDs(out)
	for i, d := range ds {
		x := &out[i]
		x.Current = currentInput(d.svc)
		if in, ok := known[x.ID]; ok {
			x.Inputs = in
		} else {
			x.Caps = capabilities(d.svc)
			x.Inputs = parseInputs(x.Caps)
		}
	}
	return out
}

// withServices 给出 ID → IOAVService（ID 去重方式与 detectDisplays 一致）。
func withServices(fn func(svcs map[string]uintptr)) {
	ds := externalDisplays()
	defer release(ds)
	tmp := make([]display, len(ds))
	for i, d := range ds {
		tmp[i].ID = d.id
	}
	uniqueIDs(tmp)
	svcs := map[string]uintptr{}
	for i, d := range ds {
		svcs[tmp[i].ID] = d.svc
	}
	fn(svcs)
}

func setInputs(targets map[string]code) (missing, failed []string) {
	withServices(func(svcs map[string]uintptr) {
		for id := range targets {
			if svcs[id] == 0 {
				missing = append(missing, id)
			}
		}
		if len(missing) > 0 {
			return
		}
		for id, c := range targets {
			if !ddcSend(svcs[id], 0x03, 0x60, 0x00, byte(c)) {
				failed = append(failed, id)
			}
		}
	})
	return missing, failed
}

func readInputs(ids []string) map[string]code {
	out := map[string]code{}
	withServices(func(svcs map[string]uintptr) {
		for _, id := range ids {
			if svc := svcs[id]; svc != 0 {
				out[id] = currentInput(svc)
			}
		}
	})
	return out
}
