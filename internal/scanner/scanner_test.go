package scanner

import (
	"testing"
)

func TestExpandCIDR_SingleIP(t *testing.T) {
	ips, err := expandCIDR("192.168.1.1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ips) != 1 || ips[0] != "192.168.1.1" {
		t.Errorf("got %v, want [192.168.1.1]", ips)
	}
}

func TestExpandCIDR_Slash30(t *testing.T) {
	// /30 has 4 addresses: network, 2 hosts, broadcast → expandCIDR strips first and last → 2 hosts
	ips, err := expandCIDR("10.0.0.0/30")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ips) != 2 {
		t.Errorf("got %d IPs, want 2: %v", len(ips), ips)
	}
	if ips[0] != "10.0.0.1" || ips[1] != "10.0.0.2" {
		t.Errorf("got %v, want [10.0.0.1 10.0.0.2]", ips)
	}
}

func TestExpandCIDR_Slash29(t *testing.T) {
	// /29 = 8 addresses, strip 2 → 6 hosts
	ips, err := expandCIDR("192.168.1.0/29")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ips) != 6 {
		t.Errorf("got %d IPs, want 6: %v", len(ips), ips)
	}
}

func TestExpandCIDR_InvalidInput(t *testing.T) {
	_, err := expandCIDR("not-an-ip")
	if err == nil {
		t.Error("expected error for invalid input, got nil")
	}
}

func TestExpandCIDR_Slash32(t *testing.T) {
	// /32 is a single host — no strip (len <= 2)
	ips, err := expandCIDR("10.0.0.1/32")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ips) != 1 {
		t.Errorf("got %d IPs, want 1: %v", len(ips), ips)
	}
}

func TestIncrementIP(t *testing.T) {
	tests := []struct {
		input []byte
		want  []byte
	}{
		{[]byte{10, 0, 0, 1}, []byte{10, 0, 0, 2}},
		{[]byte{10, 0, 0, 255}, []byte{10, 0, 1, 0}},
		{[]byte{255, 255, 255, 255}, []byte{0, 0, 0, 0}}, // wraparound
	}

	for _, tc := range tests {
		ip := make([]byte, len(tc.input))
		copy(ip, tc.input)
		incrementIP(ip)
		for i, b := range ip {
			if b != tc.want[i] {
				t.Errorf("incrementIP(%v) = %v, want %v", tc.input, ip, tc.want)
				break
			}
		}
	}
}
