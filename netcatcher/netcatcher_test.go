package netcatcher

import (
	"context"
	"net"
	"testing"
	"time"

	"netcatcher/config"
)

func TestNewNetCatcher(t *testing.T) {
	cfg := config.Interface{Name: "lo0", Routes: []string{}}
	nc := NewNetCatcher(cfg, nil)
	if nc == nil {
		t.Fatal("expected non-nil NetCatcher")
	}
	status := nc.GetStatus()
	if status.InterfaceName != "lo0" {
		t.Fatalf("expected lo0, got %s", status.InterfaceName)
	}
	if status.Connected {
		t.Fatal("expected disconnected initially")
	}
}

func TestResolveRoutesMatchesGatewayAddressFamily(t *testing.T) {
	n := NewNetCatcher(config.Interface{
		Name:   "test",
		Routes: []string{"192.0.2.10", "10.0.0.0/8", "2001:db8::10", "2001:db8:1::/64"},
	}, nil)
	n.ipv4Gateway = "192.0.2.1"
	n.ipv6Gateway = "fe80::1%test"
	n.resolveRoutes(nil)

	if len(n.routes) != 4 {
		t.Fatalf("expected 4 routes, got %d", len(n.routes))
	}
	for _, entry := range n.routes {
		ip := net.ParseIP(entry.ip)
		if ip == nil {
			t.Fatalf("invalid resolved route IP %q", entry.ip)
		}
		want := n.ipv6Gateway
		if ip.To4() != nil {
			want = n.ipv4Gateway
		}
		if entry.gateway != want {
			t.Fatalf("route %s gateway = %q, want %q", entry.ip, entry.gateway, want)
		}
	}
}

func TestResolveRoutesSkipsFamilyWithoutGateway(t *testing.T) {
	n := NewNetCatcher(config.Interface{
		Name:   "test",
		Routes: []string{"192.0.2.10", "2001:db8::10"},
	}, nil)
	n.ipv4Gateway = "192.0.2.1"
	n.resolveRoutes(nil)

	if len(n.routes) != 1 || n.routes[0].ip != "192.0.2.10" {
		t.Fatalf("expected only the IPv4 route, got %#v", n.routes)
	}
}

func TestConfiguredGatewayScopeMustMatchInterface(t *testing.T) {
	iface := &net.Interface{Name: "en0", Index: 12}
	tests := []struct {
		name    string
		gateway string
		want    string
	}{
		{name: "interface name", gateway: "fe80::1%en0", want: "fe80::1%en0"},
		{name: "interface index", gateway: "fe80::1%12", want: "fe80::1%12"},
		{name: "wrong interface", gateway: "fe80::1%en1"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			n := NewNetCatcher(config.Interface{Name: iface.Name, IPv6Gateway: test.gateway}, nil)
			if got := n.resolveGateway(iface, true); got != test.want {
				t.Fatalf("resolveGateway() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestWatchCancellation(t *testing.T) {
	cfg := config.Interface{Name: "nonexistent_iface_xyz", Routes: []string{}}
	nc := NewNetCatcher(cfg, nil)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		nc.Watch(ctx)
		close(done)
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// Watch returned after cancel — correct
	case <-time.After(2 * time.Second):
		t.Fatal("Watch did not return after context cancellation")
	}
}

func TestHasDomainRoutes(t *testing.T) {
	tests := []struct {
		name   string
		routes []string
		want   bool
	}{
		{name: "empty", routes: nil, want: false},
		{name: "only IP and CIDR", routes: []string{"192.168.1.10", "10.0.0.0/8"}, want: false},
		{name: "domain", routes: []string{"192.168.1.10", "git.example.com"}, want: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			n := NewNetCatcher(config.Interface{Name: "test", Routes: test.routes}, nil)
			if got := n.hasDomainRoutes(); got != test.want {
				t.Fatalf("hasDomainRoutes() = %v, want %v", got, test.want)
			}
		})
	}
}
