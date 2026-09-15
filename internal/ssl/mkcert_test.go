package ssl

import (
	"reflect"
	"testing"
)

func TestNormalizeHosts(t *testing.T) {
	got := normalizeHosts([]string{"B.localhost", "a.localhost", "  A.LOCALHOST ", "a.localhost", ""})
	want := []string{"a.localhost", "b.localhost"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("normalizeHosts = %v, want %v", got, want)
	}
}

func TestHostsHashOrderIndependent(t *testing.T) {
	h1 := HostsHash([]string{"a.localhost", "b.localhost"})
	h2 := HostsHash([]string{"b.localhost", "a.localhost", "a.localhost"})
	if h1 != h2 {
		t.Errorf("hash should not depend on order/dupes: %s != %s", h1, h2)
	}
	if h1 == HostsHash([]string{"a.localhost"}) {
		t.Error("hash should change when the host set changes")
	}
}
