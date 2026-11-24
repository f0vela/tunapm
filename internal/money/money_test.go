package money

import "testing"

func TestFormat(t *testing.T) {
	tests := []struct {
		cents    int64
		currency string
		want     string
	}{
		{0, "GTQ", "Q 0.00"},
		{5, "GTQ", "Q 0.05"},
		{1250000, "GTQ", "Q 12,500.00"},
		{120000, "USD", "$ 1,200.00"},
		{999999999, "USD", "$ 9,999,999.99"},
		{-2500, "GTQ", "-Q 25.00"},
		{5000, "", "Q 50.00"},
	}
	for _, tt := range tests {
		if got := Format(tt.cents, tt.currency); got != tt.want {
			t.Errorf("Format(%d, %q) = %q, esperaba %q", tt.cents, tt.currency, got, tt.want)
		}
	}
}

func TestParse(t *testing.T) {
	valid := []struct {
		in   string
		want int64
	}{
		{"5000", 500000},
		{"5,000", 500000},
		{"$ 1,200.50", 120050},
		{"Q12.5", 1250},
		{".5", 50},
		{"-25", -2500},
		{"  1200.00  ", 120000},
	}
	for _, tt := range valid {
		got, err := Parse(tt.in)
		if err != nil || got != tt.want {
			t.Errorf("Parse(%q) = %d, %v; esperaba %d", tt.in, got, err, tt.want)
		}
	}

	for _, in := range []string{"", "abc", "1.234", "1,2.3.4", "$"} {
		if _, err := Parse(in); err == nil {
			t.Errorf("Parse(%q) esperaba error", in)
		}
	}
}

func TestFormatParseRoundTrip(t *testing.T) {
	for _, cents := range []int64{0, 1, 99, 100, 123456789, 99999999999} {
		for _, cur := range []string{"GTQ", "USD"} {
			got, err := Parse(Format(cents, cur))
			if err != nil || got != cents {
				t.Errorf("ida y vuelta %d %s: %d, %v", cents, cur, got, err)
			}
		}
	}
}
