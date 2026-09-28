package route

import (
	"net"
	"testing"
)

func TestValidateRouteSpecRequiresMatchingAddressFamilies(t *testing.T) {
	tests := []struct {
		name    string
		spec    RouteSpec
		wantErr bool
	}{
		{name: "IPv4", spec: RouteSpec{IP: "192.0.2.10", Gateway: "192.0.2.1"}},
		{name: "IPv6", spec: RouteSpec{IP: "2001:db8::10", Gateway: "fe80::1%en0"}},
		{name: "IPv4 through IPv6", spec: RouteSpec{IP: "192.0.2.10", Gateway: "fe80::1%en0"}, wantErr: true},
		{name: "IPv6 through IPv4", spec: RouteSpec{IP: "2001:db8::10", Gateway: "192.0.2.1"}, wantErr: true},
		{name: "IPv4 with IPv6 mask", spec: RouteSpec{IP: "192.0.2.0", Gateway: "192.0.2.1", Mask: net.CIDRMask(64, 128)}, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, _, err := validateRouteSpec(test.spec)
			if (err != nil) != test.wantErr {
				t.Fatalf("validateRouteSpec() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}
