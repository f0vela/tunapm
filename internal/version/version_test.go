package version

import (
	"strings"
	"testing"
)

func TestShort(t *testing.T) {
	cases := []struct {
		in   Info
		want string
	}{
		{Info{Version: "1.2.0", Commit: "abc1234"}, "1.2.0"},
		{Info{Version: "dev", Commit: "abc1234"}, "dev abc1234"},
		{Info{Version: "dev", Commit: "abc1234", Modified: true}, "dev abc1234+"},
		{Info{Version: "dev"}, "dev"},
	}
	for _, c := range cases {
		if got := c.in.Short(); got != c.want {
			t.Errorf("Short(%+v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestString(t *testing.T) {
	s := Info{Version: "dev", Commit: "abc1234", Time: "2026-11-26", Modified: true, Go: "go1.27", Platform: "darwin/arm64"}.String()
	for _, want := range []string{"tuna dev abc1234+", "commit abc1234 (2026-11-26), con cambios sin confirmar", "go1.27, darwin/arm64"} {
		if !strings.Contains(s, want) {
			t.Errorf("String() = %q, falta %q", s, want)
		}
	}
	if got := Get().Go; got == "" {
		t.Error("Get() debe informar la versión de Go")
	}
}
