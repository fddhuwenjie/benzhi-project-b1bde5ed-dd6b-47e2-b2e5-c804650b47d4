package main

import "testing"

func TestValidateAddrOnlyAllowsLoopback(t *testing.T) {
	for _, addr := range []string{"127.0.0.1:19081", "[::1]:19081", "localhost:19081"} {
		if err := validateAddr(addr); err != nil {
			t.Fatalf("应允许 %s: %v", addr, err)
		}
	}
	for _, addr := range []string{"0.0.0.0:19081", "127.0.0.1:0", ":8080", "bad"} {
		if err := validateAddr(addr); err == nil {
			t.Fatalf("应拒绝 %s", addr)
		}
	}
}
