package route

import (
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseDarwinGateway(t *testing.T) {
	tests := []struct {
		name      string
		output    string
		iface     string
		ipv6      bool
		want      string
		wantError bool
	}{
		{
			name:   "IPv4",
			output: "gateway: 192.168.1.1\ninterface: en0\n",
			iface:  "en0",
			want:   "192.168.1.1",
		},
		{
			name:   "scoped IPv6",
			output: "gateway: fe80::1%en0\ninterface: en0\n",
			iface:  "en0",
			ipv6:   true,
			want:   "fe80::1%en0",
		},
		{
			name:   "add missing IPv6 scope",
			output: "gateway: fe80::1\ninterface: en0\n",
			iface:  "en0",
			ipv6:   true,
			want:   "fe80::1%en0",
		},
		{
			name:      "wrong interface",
			output:    "gateway: 192.168.1.1\ninterface: en1\n",
			iface:     "en0",
			wantError: true,
		},
		{
			name:      "wrong family",
			output:    "gateway: fe80::1%en0\ninterface: en0\n",
			iface:     "en0",
			wantError: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseDarwinGateway([]byte(test.output), test.iface, test.ipv6)
			if (err != nil) != test.wantError {
				t.Fatalf("parseDarwinGateway() error = %v, wantError %v", err, test.wantError)
			}
			if got != test.want {
				t.Fatalf("parseDarwinGateway() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestParseDarwinPeer(t *testing.T) {
	output := []byte("inet 10.0.0.2 --> 10.0.0.1 netmask 0xffffffff\ninet6 fe80::2%ppp0 --> fe80::1%ppp0 prefixlen 64\n")
	if got := parseDarwinPeer(output, false); got != "10.0.0.1" {
		t.Fatalf("IPv4 peer = %q", got)
	}
	if got := parseDarwinPeer(output, true); got != "fe80::1%ppp0" {
		t.Fatalf("IPv6 peer = %q", got)
	}
}

func TestDarwinRouteArgs(t *testing.T) {
	tests := []struct {
		name string
		spec RouteSpec
		want []string
	}{
		{
			name: "IPv4 network",
			spec: RouteSpec{IP: "192.168.10.0", Gateway: "192.168.1.1", Mask: net.CIDRMask(24, 32), InterfaceName: "en0"},
			want: []string{"add", "-inet", "-net", "-ifscope", "en0", "192.168.10.0", "192.168.1.1", "255.255.255.0"},
		},
		{
			name: "IPv6 network",
			spec: RouteSpec{IP: "2001:db8:1::", Gateway: "fe80::1%en0", Mask: net.CIDRMask(64, 128), InterfaceName: "en0"},
			want: []string{"add", "-inet6", "-net", "-ifscope", "en0", "2001:db8:1::", "-prefixlen", "64", "fe80::1%en0"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := darwinRouteArgs("add", test.spec)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("darwinRouteArgs() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestResolverHelperScriptSyntax(t *testing.T) {
	helper := filepath.Join(t.TempDir(), "netcatcher-resolver-helper")
	if err := os.WriteFile(helper, []byte(resolverHelperScript), 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("/bin/sh", "-n", helper).CombinedOutput(); err != nil {
		t.Fatalf("helper script syntax is invalid: %v: %s", err, out)
	}
}

func TestAuthorizationUpToDate(t *testing.T) {
	helper := filepath.Join(t.TempDir(), "netcatcher-resolver-helper")
	if err := os.WriteFile(helper, []byte(resolverHelperScript), 0o755); err != nil {
		t.Fatal(err)
	}

	var calls [][]string
	run := func(name string, args ...string) error {
		calls = append(calls, append([]string{name}, args...))
		return nil
	}

	if !authorizationUpToDate(helper, run) {
		t.Fatal("expected current helper and sudo capabilities to be accepted")
	}
	want := [][]string{
		{"sudo", "-n", "/sbin/route", "-n", "get", "127.0.0.1"},
		{"sudo", "-n", helper, "remove"},
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("unexpected authorization probes: got %v, want %v", calls, want)
	}
}

func TestAuthorizationUpToDateRejectsStaleHelper(t *testing.T) {
	helper := filepath.Join(t.TempDir(), "netcatcher-resolver-helper")
	if err := os.WriteFile(helper, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	called := false
	if authorizationUpToDate(helper, func(string, ...string) error {
		called = true
		return nil
	}) {
		t.Fatal("expected stale helper to be rejected")
	}
	if called {
		t.Fatal("sudo probes should not run for a stale helper")
	}
}

func TestAuthorizationUpToDateRequiresEverySudoCapability(t *testing.T) {
	helper := filepath.Join(t.TempDir(), "netcatcher-resolver-helper")
	if err := os.WriteFile(helper, []byte(resolverHelperScript), 0o755); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name        string
		failingCall int
	}{
		{name: "route", failingCall: 1},
		{name: "helper", failingCall: 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			run := func(string, ...string) error {
				calls++
				if calls == test.failingCall {
					return errors.New("not allowed")
				}
				return nil
			}
			if authorizationUpToDate(helper, run) {
				t.Fatalf("expected sudo probe %d failure to invalidate authorization", test.failingCall)
			}
		})
	}
}
