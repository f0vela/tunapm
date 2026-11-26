package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	tea "charm.land/bubbletea/v2"

	"tunapm/internal/store"
	"tunapm/internal/ui"
	"tunapm/internal/version"
)

func main() {
	dbPath := flag.String("db", defaultDBPath(), "ruta de la base de datos SQLite (modo local y serve)")
	apiURL := flag.String("api", os.Getenv("TUNA_API"), "URL del API (ej. https://tuna.example.com); la TUI y 'web' la usan en vez de SQLite")
	token := flag.String("token", os.Getenv("TUNA_TOKEN"), "token Bearer del API")
	showVersion := flag.Bool("version", false, "muestra la versión y sale")
	flag.Usage = usage
	flag.Parse()

	if *showVersion {
		fmt.Print(version.Get())
		return
	}

	args := flag.Args()
	if len(args) > 0 {
		switch args[0] {
		case "serve":
			runServe(*dbPath, *token, args[1:])
			return
		case "web":
			runWeb(*apiURL, *token, args[1:])
			return
		case "version":
			fmt.Print(version.Get())
			return
		case "status":
			runStatus(*dbPath, *apiURL, *token)
			return
		case "sync":
			runSync(*dbPath, *apiURL, *token)
			return
		}
	}

	// Modo remoto: la TUI lee de un espejo local y sincroniza con el API en segundo plano.
	if *apiURL != "" && len(args) == 0 {
		repo, err := openCache(*dbPath, *apiURL, *token)
		if err != nil {
			fmt.Fprintln(os.Stderr, "no se pudo abrir la caché local:", err)
			os.Exit(1)
		}
		if repo.IsEmpty() {
			// Primer arranque: espera un momento a traer los datos para no mostrar todo vacío.
			repo.WaitFirstSync(3 * time.Second)
		}
		ui.ConfigDir = filepath.Dir(*dbPath)
		ui.Version = version.Get().Short()
		ui.Source, ui.Remote = hostOf(*apiURL), true
		err = runTUI(repo)
		repo.Close() // intenta enviar lo pendiente antes de salir
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		return
	}

	st, err := store.Open(*dbPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "no se pudo abrir la base de datos:", err)
		os.Exit(1)
	}
	defer st.Close()

	if len(args) > 0 {
		switch args[0] {
		case "seed":
			created, err := seed(st)
			if err != nil {
				fmt.Fprintln(os.Stderr, "seed:", err)
				os.Exit(1)
			}
			if created {
				fmt.Println("Datos de ejemplo creados en", *dbPath)
			} else {
				fmt.Println("La base ya tiene datos; no se agregó nada")
			}
		case "import":
			if len(args) < 2 {
				fmt.Fprintln(os.Stderr, "uso: tuna [-db destino] import <database.sqlite>")
				os.Exit(2)
			}
			if err := importOld(st, args[1]); err != nil {
				fmt.Fprintln(os.Stderr, "import:", err)
				os.Exit(1)
			}
			fmt.Println("Importación completa en", *dbPath)
		default:
			fmt.Fprintln(os.Stderr, "subcomando desconocido:", args[0])
			usage()
			os.Exit(2)
		}
		return
	}

	ui.ConfigDir = filepath.Dir(*dbPath)
	ui.Version = version.Get().Short()
	ui.Source, ui.Remote = "local", false
	if err := runTUI(st); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func runTUI(repo store.Repo) error {
	program := tea.NewProgram(ui.NewApp(repo))
	_, err := program.Run()
	return err
}

func usage() {
	fmt.Fprint(os.Stderr, `uso: tuna [opciones] [comando]

Sin comando abre la TUI (SQLite local, o el API si se da -api / TUNA_API).

Comandos:
  serve [-addr :8080] [-web]   API REST (y opcionalmente la web app) sobre la base SQLite
  web   [-addr :8081]          web app que consume el API remoto (-api, -token)
  status                       muestra si usas el API o la base local (y cambios sin enviar)
  sync                         envía los cambios pendientes y trae los del servidor
  version                      muestra la versión (también: tuna -version)
  seed                         datos de ejemplo
  import <database.sqlite>     importar base antigua

Opciones globales:
  -db <ruta>      base SQLite (default ~/.local/share/tunapm/tuna.db)
  -api <url>      URL del API         (env TUNA_API)
  -token <token>  token Bearer        (env TUNA_TOKEN)

Variables: TUNA_WEB_PASSWORD (contraseña de la web; default = token)
`)
}

func defaultDBPath() string {
	dir := os.Getenv("XDG_DATA_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "tuna.db"
		}
		dir = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dir, "tunapm", "tuna.db")
}
