//go:build windows

package route

import (
	"net"
	"reflect"
	"testing"
)

func TestWindowsRouteArgs(t *testing.T) {
	tests := []struct {
		name string
		spec RouteSpec
		want []string
	}{
		{
			name: "IPv4 network",
			spec: RouteSpec{IP: "192.168.10.0", Gateway: "192.168.1.1", Mask: net.CIDRMask(24, 32), InterfaceIndex: 12},
			want: []string{"-4", "add", "192.168.10.0", "mask", "255.255.255.0", "192.168.1.1", "if", "12"},
		},
		{
			name: "IPv6 host",
			spec: RouteSpec{IP: "2001:db8::10", Gateway: "fe80::1%12", InterfaceIndex: 12},
			want: []string{"-6", "add", "2001:db8::10/128", "fe80::1", "if", "12"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := windowsRouteArgs("add", test.spec)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("windowsRouteArgs() = %v, want %v", got, test.want)
			}
		})
	}
}
