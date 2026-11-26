package dates

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var ErrInvalidDate = errors.New("fecha inválida")

const Layout = "2006-01-02"

// Today devuelve la fecha local en formato YYYY-MM-DD.
func Today() string {
	return time.Now().Format(Layout)
}

var weekdays = map[string]time.Weekday{
	"dom": time.Sunday, "domingo": time.Sunday,
	"lun": time.Monday, "lunes": time.Monday,
	"mar": time.Tuesday, "martes": time.Tuesday,
	"mié": time.Wednesday, "mie": time.Wednesday, "miércoles": time.Wednesday, "miercoles": time.Wednesday,
	"jue": time.Thursday, "jueves": time.Thursday,
	"vie": time.Friday, "viernes": time.Friday,
	"sáb": time.Saturday, "sab": time.Saturday, "sábado": time.Saturday, "sabado": time.Saturday,
}

// Parse interpreta fechas rápidas relativas a today (que también puede venir
// con hora; solo se usa su día) y devuelve YYYY-MM-DD.
func Parse(input string, today time.Time) (string, error) {
	s := strings.ToLower(strings.TrimSpace(input))
	if s == "" {
		return "", ErrInvalidDate
	}
	base := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)

	switch s {
	case "hoy":
		return base.Format(Layout), nil
	case "mañana", "manana":
		return base.AddDate(0, 0, 1).Format(Layout), nil
	case "ayer":
		return base.AddDate(0, 0, -1).Format(Layout), nil
	}

	if wd, ok := weekdays[s]; ok {
		delta := (int(wd) - int(base.Weekday()) + 7) % 7
		return base.AddDate(0, 0, delta).Format(Layout), nil
	}

	if strings.HasPrefix(s, "+") || strings.HasPrefix(s, "-") {
		unit := s[len(s)-1]
		n, err := strconv.Atoi(s[:len(s)-1])
		if err != nil || (unit != 'd' && unit != 's' && unit != 'm') {
			return "", ErrInvalidDate
		}
		switch unit {
		case 'd':
			return base.AddDate(0, 0, n).Format(Layout), nil
		case 's':
			return base.AddDate(0, 0, 7*n).Format(Layout), nil
		default:
			return base.AddDate(0, n, 0).Format(Layout), nil
		}
	}

	if t, err := time.Parse(Layout, s); err == nil {
		return t.Format(Layout), nil
	}

	if parts := strings.Split(s, "/"); len(parts) == 2 || len(parts) == 3 {
		day, errD := strconv.Atoi(parts[0])
		month, errM := strconv.Atoi(parts[1])
		year := base.Year()
		if len(parts) == 3 {
			year, _ = strconv.Atoi(parts[2])
		}
		if errD != nil || errM != nil || year < 1 {
			return "", ErrInvalidDate
		}
		date := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
		// "15/12" escrito después de esa fecha se refiere al año siguiente.
		if len(parts) == 2 && date.Before(base) {
			date = date.AddDate(1, 0, 0)
		}
		if date.Day() != day || int(date.Month()) != month {
			return "", ErrInvalidDate
		}
		return date.Format(Layout), nil
	}

	return "", fmt.Errorf("%w: %q", ErrInvalidDate, input)
}

// Human muestra una fecha YYYY-MM-DD relativa a today, para tablas y detalles.
func Human(date, today string) string {
	switch date {
	case today:
		return "hoy"
	}
	if t, err := time.Parse(Layout, date); err == nil {
		if base, err := time.Parse(Layout, today); err == nil {
			days := int(t.Sub(base).Hours() / 24)
			switch days {
			case 1:
				return "mañana"
			case -1:
				return "ayer"
			}
			if t.Year() == base.Year() {
				return t.Format("02/01")
			}
		}
	}
	return date
}

// Short es como Human pero nunca pasa de 8 caracteres: en otro año usa "02/01/06".
// Sirve para columnas angostas.
func Short(date, today string) string {
	h := Human(date, today)
	if h == date {
		if t, err := time.Parse(Layout, date); err == nil {
			return t.Format("02/01/06")
		}
	}
	return h
}
