package money

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var ErrInvalidAmount = errors.New("monto inválido")

// Symbols devuelve el símbolo de la moneda; GTQ por defecto.
func Symbol(currency string) string {
	if strings.EqualFold(currency, "USD") {
		return "$"
	}
	return "Q"
}

// Format convierte centavos a "Q 12,500.00" o "$ 1,200.00".
func Format(cents int64, currency string) string {
	sign := ""
	if cents < 0 {
		sign, cents = "-", -cents
	}
	whole := strconv.FormatInt(cents/100, 10)
	return fmt.Sprintf("%s%s %s.%02d", sign, Symbol(currency), group(whole), cents%100)
}

func group(digits string) string {
	if len(digits) <= 3 {
		return digits
	}
	first := len(digits) % 3
	if first == 0 {
		first = 3
	}
	var b strings.Builder
	b.WriteString(digits[:first])
	for i := first; i < len(digits); i += 3 {
		b.WriteString(",")
		b.WriteString(digits[i : i+3])
	}
	return b.String()
}

// Parse acepta "5000", "5,000", "$ 1,200.50", "Q12.5" y devuelve centavos.
func Parse(s string) (int64, error) {
	s = strings.TrimSpace(s)
	negative := false
	if strings.HasPrefix(s, "-") {
		negative, s = true, s[1:]
	}
	s = strings.TrimPrefix(strings.TrimPrefix(s, "$"), "Q")
	s = strings.TrimPrefix(s, "q")
	s = strings.ReplaceAll(s, ",", "")
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "_", "")
	if s == "" {
		return 0, ErrInvalidAmount
	}

	whole, frac, hasFrac := strings.Cut(s, ".")
	if whole == "" {
		whole = "0"
	}
	if len(frac) > 2 {
		return 0, ErrInvalidAmount
	}
	for len(frac) < 2 {
		frac += "0"
	}
	if !digitsOnly(whole) || (hasFrac && !digitsOnly(frac)) {
		return 0, ErrInvalidAmount
	}

	units, err := strconv.ParseInt(whole, 10, 64)
	if err != nil || units > (1<<62)/100 {
		return 0, ErrInvalidAmount
	}
	centsPart, err := strconv.ParseInt(frac, 10, 64)
	if err != nil {
		return 0, ErrInvalidAmount
	}
	cents := units*100 + centsPart
	if negative {
		cents = -cents
	}
	return cents, nil
}

func digitsOnly(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}
