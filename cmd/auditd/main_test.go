package main

import "testing"

func TestLoopbackAddressPolicy(t *testing.T) {
	for _, address := range []string{"127.0.0.1:8777", "[::1]:8777"} {
		if !isLoopbackAddress(address) {
			t.Errorf("loopback rejected: %s", address)
		}
	}
	for _, address := range []string{"0.0.0.0:8777", ":8777", "localhost:8777", "192.0.2.1:8777", "invalid"} {
		if isLoopbackAddress(address) {
			t.Errorf("public address accepted as loopback: %s", address)
		}
	}
}
