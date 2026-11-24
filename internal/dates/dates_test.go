package dates

import (
	"errors"
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	today := time.Date(2026, 11, 22, 15, 30, 0, 0, time.UTC) // domingo

	tests := []struct {
		in   string
		want string
	}{
		{"hoy", "2026-11-22"},
		{"mañana", "2026-11-23"},
		{"manana", "2026-11-23"},
		{"ayer", "2026-11-21"},
		{"dom", "2026-11-22"},
		{"domingo", "2026-11-22"},
		{"lun", "2026-11-23"},
		{"vie", "2026-11-27"},
		{"mié", "2026-11-25"},
		{"sábado", "2026-11-28"},
		{"+3d", "2026-11-25"},
		{"+2s", "2026-12-06"},
		{"+1m", "2026-12-22"},
		{"-2d", "2026-11-20"},
		{"15/12", "2026-12-15"},
		{"15/11", "2027-11-15"},
		{"15/12/2026", "2026-12-15"},
		{"2026-12-15", "2026-12-15"},
		{"  HOY ", "2026-11-22"},
	}
	for _, tt := range tests {
		got, err := Parse(tt.in, today)
		if err != nil || got != tt.want {
			t.Errorf("Parse(%q) = %q, %v; esperaba %q", tt.in, got, err, tt.want)
		}
	}

	for _, in := range []string{"", "xyz", "32/01", "2026-13-01", "+3x", "+d", "1/2/3/4"} {
		if _, err := Parse(in, today); !errors.Is(err, ErrInvalidDate) {
			t.Errorf("Parse(%q) esperaba ErrInvalidDate, obtuve %v", in, err)
		}
	}
}

func TestHuman(t *testing.T) {
	tests := []struct{ date, today, want string }{
		{"2026-11-22", "2026-11-22", "hoy"},
		{"2026-11-23", "2026-11-22", "mañana"},
		{"2026-11-21", "2026-11-22", "ayer"},
		{"2026-11-25", "2026-11-22", "25/11"},
		{"2027-01-05", "2026-11-22", "2027-01-05"},
	}
	for _, tt := range tests {
		if got := Human(tt.date, tt.today); got != tt.want {
			t.Errorf("Human(%q, %q) = %q, esperaba %q", tt.date, tt.today, got, tt.want)
		}
	}
}
