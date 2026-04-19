package netcatcher

import (
	"encoding/binary"
	"testing"
)

func buildQuery(name string) []byte {
	pkt := make([]byte, 0, 64)
	// Header: ID=0x1234, flags=RD, QD=1, AN=0, NS=0, AR=0
	pkt = append(pkt, 0x12, 0x34, 0x01, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00)
	// QNAME: length-prefixed labels, null terminator
	labels := []byte{}
	start := 0
	s := name + "."
	for i := 0; i < len(s); i++ {
		if s[i] == '.' {
			labels = append(labels, byte(i-start))
			labels = append(labels, s[start:i]...)
			start = i + 1
		}
	}
	labels = append(labels, 0)
	pkt = append(pkt, labels...)
	// QTYPE=A, QCLASS=IN
	qtype := make([]byte, 4)
	binary.BigEndian.PutUint16(qtype[0:2], 1)
	binary.BigEndian.PutUint16(qtype[2:4], 1)
	pkt = append(pkt, qtype...)
	return pkt
}

func TestExtractQName(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"git.hzwxbz2.cn", "git.hzwxbz2.cn"},
		{"a.b.c.example.com", "a.b.c.example.com"},
	}
	for _, c := range cases {
		got, err := extractQName(buildQuery(c.in))
		if err != nil {
			t.Fatalf("extractQName(%q) error: %v", c.in, err)
		}
		if got != c.want {
			t.Errorf("extractQName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestMatchRoute(t *testing.T) {
	f := NewDNSForwarder()
	// Inject fake routes with nil iface — we only test matching logic.
	f.routes["hzwxbz2.cn"] = forwarderRoute{ifaceName: "ppp0", dnsServers: []string{"1.1.1.1:53"}}
	f.routes["example.com"] = forwarderRoute{ifaceName: "utun9", dnsServers: []string{"2.2.2.2:53"}}

	cases := []struct {
		q, wantIface string
		wantOK       bool
	}{
		{"git.hzwxbz2.cn", "ppp0", true},
		{"a.b.c.hzwxbz2.cn", "ppp0", true},
		{"hzwxbz2.cn", "ppp0", true},
		{"example.com", "utun9", true},
		{"google.com", "", false},
		{"myhzwxbz2.cn", "", false},
	}
	for _, c := range cases {
		r, ok := f.matchRoute(c.q)
		if ok != c.wantOK {
			t.Errorf("matchRoute(%q) ok=%v want %v", c.q, ok, c.wantOK)
			continue
		}
		if ok && r.ifaceName != c.wantIface {
			t.Errorf("matchRoute(%q) iface=%q want %q", c.q, r.ifaceName, c.wantIface)
		}
	}
}

func TestNormalizeDNSServers(t *testing.T) {
	in := []string{"1.1.1.1", "8.8.8.8:5353", " ", "9.9.9.9"}
	got := normalizeDNSServers(in)
	want := []string{"1.1.1.1:53", "8.8.8.8:5353", "9.9.9.9:53"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d] got %q want %q", i, got[i], want[i])
		}
	}
}
