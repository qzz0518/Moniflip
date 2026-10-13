package main

import (
	"strconv"
	"testing"
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
