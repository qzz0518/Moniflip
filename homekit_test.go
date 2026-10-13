package main

import (
	"slices"
	"strconv"
	"testing"

	"github.com/brutella/hap"
	"github.com/brutella/hap/accessory"
	"github.com/brutella/hap/characteristic"
)

func TestSetupURI(t *testing.T) {
	u := setupURI("31415926", 31, "W3QX")
	if len(u) != 20 || u[:7] != "X-HM://" || u[16:] != "W3QX" {
		t.Fatalf("setupURI = %q", u)
	}
	p, err := strconv.ParseUint(u[7:16], 36, 64)
	if err != nil {
		t.Fatal(err)
	}
	if v, cat, flags, pin := p>>43, p>>31&0xFF, p>>27&0xF, p&(1<<27-1); v != 0 || cat != 31 || flags != 2 || pin != 31415926 {
		t.Fatalf("payload version %d category %d flags %d pin %d", v, cat, flags, pin)
	}
}

// 两种样子都能起服务（IID 不冲突）；开关的 IID 由编号定，电脑增删后不变。
func TestAccessory(t *testing.T) {
	for _, tv := range []bool{false, true} {
		h := &homekit{computers: []string{"Mac", "PC", "Linux"}, ids: []int{1, 2, 5}}
		acc := h.build(tv, "0.1.0")
		if _, err := hap.NewServer(hap.NewMemStore(), acc); err != nil {
			t.Fatalf("tv=%v: %v", tv, err)
		}
		if tv && (acc.Type != accessory.TypeTelevision || h.tv == nil || h.switches != nil) {
			t.Fatalf("tv: type %d", acc.Type)
		}
		if !tv && (acc.Type != accessory.TypeSwitch || h.tv != nil || len(h.switches) != 3) {
			t.Fatalf("switch: type %d, %d switches", acc.Type, len(h.switches))
		}
	}

	iids := func(computers []string, ids []int) map[string][]uint64 {
		h := &homekit{computers: computers, ids: ids}
		acc := h.build(false, "0.1.0")
		if _, err := hap.NewServer(hap.NewMemStore(), acc); err != nil {
			t.Fatal(err)
		}
		m := map[string][]uint64{}
		for _, sw := range h.switches {
			name := sw.C(characteristic.TypeConfiguredName).Value().(string)
			if sw.C(characteristic.TypeName).Value() != name {
				t.Fatalf("names differ on %d", sw.Id)
			}
			m[name] = append(m[name], sw.Id)
			for _, c := range sw.Cs {
				m[name] = append(m[name], c.Id)
			}
		}
		return m
	}
	before := iids([]string{"Mac", "PC"}, []int{1, 2})
	after := iids([]string{"PC", "Linux"}, []int{2, 3}) // 删掉 Mac、加了 Linux，PC 换到第一个
	if len(before["PC"]) != 4 || !slices.Equal(before["PC"], after["PC"]) {
		t.Fatalf("PC switch IIDs changed: %v → %v", before["PC"], after["PC"])
	}
}
