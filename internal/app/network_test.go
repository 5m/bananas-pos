package app

import (
	"net/netip"
	"testing"
)

func TestPreferredHostIPPrefersPrivateIPv4(t *testing.T) {
	addrs := []netip.Addr{
		netip.MustParseAddr("2001:db8::10"),
		netip.MustParseAddr("203.0.113.8"),
		netip.MustParseAddr("192.168.1.24"),
	}

	got := preferredHostIP(addrs)
	if got != "192.168.1.24" {
		t.Fatalf("unexpected preferred IP %q", got)
	}
}

func TestPreferredHostIPFallsBackToPublicIPv4(t *testing.T) {
	addrs := []netip.Addr{
		netip.MustParseAddr("2001:db8::10"),
		netip.MustParseAddr("203.0.113.8"),
	}

	got := preferredHostIP(addrs)
	if got != "203.0.113.8" {
		t.Fatalf("unexpected preferred IP %q", got)
	}
}

func TestPreferredHostIPFallsBackToIPv6(t *testing.T) {
	addrs := []netip.Addr{
		netip.MustParseAddr("2001:db8::10"),
	}

	got := preferredHostIP(addrs)
	if got != "2001:db8::10" {
		t.Fatalf("unexpected preferred IP %q", got)
	}
}

func TestPreferredHostIPIgnoresLoopbackAndInvalidAddrs(t *testing.T) {
	addrs := []netip.Addr{
		netip.MustParseAddr("127.0.0.1"),
		netip.Addr{},
	}

	got := preferredHostIP(addrs)
	if got != "" {
		t.Fatalf("unexpected preferred IP %q", got)
	}
}
