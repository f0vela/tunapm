package quickadd

import (
	"errors"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		in   string
		want Draft
	}{
		{
			in: "t Diseñar logo @web-acme !alta ^viernes",
			want: Draft{
				Kind: KindTask, Title: "Diseñar logo", Target: "web-acme",
				Priority: 2, Date: "viernes", Currency: "GTQ",
			},
		},
		{
			in: "c Juan Pérez #Acme",
			want: Draft{
				Kind: KindClient, Title: "Juan Pérez", Company: "Acme",
				Priority: -1, Currency: "GTQ",
			},
		},
		{
			in: "p Sitio web @Acme $5000 usd ^2026-12-20",
			want: Draft{
				Kind: KindProject, Title: "Sitio web", Target: "Acme",
				AmountCents: 500000, Currency: "USD", Priority: -1, Date: "2026-12-20",
			},
		},
		{
			in: "$ 2500 @web-acme ^15/12",
			want: Draft{
				Kind: KindPayment, Target: "web-acme", AmountCents: 250000,
				Currency: "GTQ", Priority: -1, Date: "15/12",
			},
		},
		{
			in: "t Revisar sin extras",
			want: Draft{
				Kind: KindTask, Title: "Revisar sin extras", Priority: -1, Currency: "GTQ",
			},
		},
	}

	for _, tt := range tests {
		got, err := Parse(tt.in)
		if err != nil {
			t.Errorf("Parse(%q): %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("Parse(%q)\n got  %+v\n want %+v", tt.in, got, tt.want)
		}
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		in   string
		want error
	}{
		{"", ErrNotQuickAdd},
		{"comando desconocido", ErrNotQuickAdd},
		{"t", ErrMissingTitle},
		{"c #Acme", ErrMissingTitle},
		{"p @Acme", ErrMissingTitle},
		{"$ @web-acme", ErrMissingAmount},
		{"$ abc", ErrInvalidAmount},
		{"p Sitio $abc", ErrInvalidAmount},
		{"t Algo !altísima", ErrInvalidPriority},
		{"t Algo @", ErrInvalidTarget},
		{"c Alguien #", ErrInvalidCompany},
	}
	for _, tt := range tests {
		if _, err := Parse(tt.in); !errors.Is(err, tt.want) {
			t.Errorf("Parse(%q) = %v, esperaba %v", tt.in, err, tt.want)
		}
	}

	if _, err := Parse("t Algo ^"); err == nil {
		t.Error("Parse(\"t Algo ^\") esperaba error por fecha vacía")
	}
}
