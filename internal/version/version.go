// Package version dice qué versión de tuna se está ejecutando.
package version

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"
)

// Version se fija al compilar:
//
//	go build -ldflags "-X tunapm/internal/version.Version=$(git describe --tags --always --dirty)" ./cmd/tuna
//
// Sin eso vale "dev" y String completa el dato con la información de git que
// Go incrusta solo en el binario.
var Version = "dev"

// Info es lo que se sabe de la compilación.
type Info struct {
	Version  string // etiqueta de versión, o "dev"
	Commit   string // 7 primeros caracteres del commit, si se compiló desde git
	Time     string // fecha del commit (YYYY-MM-DD)
	Modified bool   // había cambios sin confirmar al compilar
	Go       string
	Platform string
}

// Get lee la versión fijada y los datos de git del binario.
func Get() Info {
	info := Info{Version: Version, Go: runtime.Version(), Platform: runtime.GOOS + "/" + runtime.GOARCH}
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				info.Commit = s.Value
				if len(info.Commit) > 7 {
					info.Commit = info.Commit[:7]
				}
			case "vcs.time":
				if len(s.Value) >= 10 {
					info.Time = s.Value[:10]
				}
			case "vcs.modified":
				info.Modified = s.Value == "true"
			}
		}
	}
	return info
}

// Short es la versión en una línea corta, para la TUI y el API: "dev 7a20421" o "1.2.0".
func (i Info) Short() string {
	if i.Version != "dev" || i.Commit == "" {
		return i.Version
	}
	s := i.Version + " " + i.Commit
	if i.Modified {
		s += "+"
	}
	return s
}

// String es el texto completo de `tuna version`.
func (i Info) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "tuna %s\n", i.Short())
	if i.Commit != "" {
		line := "commit " + i.Commit
		if i.Time != "" {
			line += " (" + i.Time + ")"
		}
		if i.Modified {
			line += ", con cambios sin confirmar"
		}
		b.WriteString(line + "\n")
	}
	fmt.Fprintf(&b, "%s, %s\n", i.Go, i.Platform)
	return b.String()
}
