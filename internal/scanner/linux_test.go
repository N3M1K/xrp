//go:build linux

package scanner

import "testing"

func TestDecodeProcAddr(t *testing.T) {
	cases := map[string]string{
		"3500007F":                         "127.0.0.53", // 127.0.0.53
		"0100007F":                         "127.0.0.1",  // 127.0.0.1
		"00000000":                         "127.0.0.1",  // 0.0.0.0
		"00000000000000000000000001000000": "::1",        // ::1
		"00000000000000000000000000000000": "127.0.0.1",  // ::
	}
	for in, want := range cases {
		if got := decodeProcAddr(in); got != want {
			t.Errorf("decodeProcAddr(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseProcNet(t *testing.T) {
	data := []byte("  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode\n" +
		"   0: 00000000:0BB8 00000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 12345 1 0000000000000000 100 0 0 10 0\n" +
		"   1: 00000000:1F90 00000000:0000 01 00000000:00000000 00:00000000 00000000  1000        0 99999 1 0000000000000000 100 0 0 10 0\n")
	socks := parseProcNet(data)
	if len(socks) != 1 {
		t.Fatalf("expected 1 LISTEN socket, got %d", len(socks))
	}
	if socks[0].port != 3000 {
		t.Errorf("port = %d, want 3000", socks[0].port)
	}
	if socks[0].inode != "12345" {
		t.Errorf("inode = %q, want 12345", socks[0].inode)
	}
	if socks[0].addr != "127.0.0.1" {
		t.Errorf("addr = %q, want 127.0.0.1", socks[0].addr)
	}
}
