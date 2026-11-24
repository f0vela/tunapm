package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"tunapm/internal/cache"
	"tunapm/internal/client"
	"tunapm/internal/store"
)

// cachePath es el espejo local del API (uno por servidor, junto a la base local).
func cachePath(dbPath, apiURL string) string {
	sum := sha256.Sum256([]byte(apiURL))
	return filepath.Join(filepath.Dir(dbPath), "cache-"+hex.EncodeToString(sum[:4])+".db")
}

func openCache(dbPath, apiURL, token string) (*cache.Repo, error) {
	return cache.Open(cachePath(dbPath, apiURL), client.New(apiURL, token))
}

// runSync fuerza un ciclo de sincronización y muestra el resultado.
func runSync(dbPath, apiURL, token string) {
	if apiURL == "" {
		fatal("sync: define TUNA_API (o -api)")
	}
	repo, err := openCache(dbPath, apiURL, token)
	if err != nil {
		fatal("sync: " + err.Error())
	}
	err = repo.SyncNow()
	_, pending, _ := repo.SyncStatus()
	for _, n := range repo.TakeNotices() {
		fmt.Println("⚠", n)
	}
	if cerr := repo.Close(); cerr != nil && err == nil {
		err = cerr
	}
	if err != nil {
		fmt.Printf("✗ no se pudo sincronizar (%d cambios pendientes): %v\n", pending, err)
		os.Exit(1)
	}
	fmt.Printf("✓ sincronizado (%d cambios pendientes)\n", pending)
}

func hostOf(raw string) string {
	if u, err := url.Parse(raw); err == nil && u.Host != "" {
		return u.Host
	}
	return raw
}

// runStatus dice a qué datos se conectaría la TUI y si están accesibles.
func runStatus(dbPath, apiURL, token string) {
	var repo store.Repo
	if apiURL != "" {
		fmt.Println("Modo:   API remota")
		fmt.Println("API:    ", apiURL)
		if token == "" {
			fmt.Println("Token:   (vacío) — define TUNA_TOKEN")
		} else {
			fmt.Println("Token:   definido")
		}
		c := client.New(apiURL, token)
		if err := c.Ping(); err != nil {
			fmt.Println("Estado:  ✗ sin conexión —", err)
			os.Exit(1)
		}
		fmt.Println("Estado:  ✓ conectado")
		if n, err := cache.PendingCount(cachePath(dbPath, apiURL)); err == nil {
			fmt.Printf("Caché:   %s (%d cambios sin enviar)\n", cachePath(dbPath, apiURL), n)
		}
		repo = c
	} else {
		fmt.Println("Modo:   LOCAL (SQLite)")
		fmt.Println("Base:   ", dbPath)
		st, err := store.Open(dbPath)
		if err != nil {
			fmt.Println("Estado:  ✗", err)
			os.Exit(1)
		}
		defer st.Close()
		fmt.Println("Estado:  ✓ abierta")
		fmt.Println()
		fmt.Println("⚠ Estos datos NO llegan a la web. Para usar el servidor define TUNA_API y TUNA_TOKEN")
		fmt.Println("  (o usa -api / -token) en este shell; revisa ~/.zshrc y abre una terminal nueva.")
		repo = st
	}

	clients, _ := repo.ListClients()
	projects, _ := repo.ListProjects()
	tasks, _ := repo.ListTasks(store.TaskFilter{Mode: store.FilterAll})
	payments, _ := repo.ListPayments()
	fmt.Printf("\nDatos:   %d clientes · %d proyectos · %d tareas · %d cuotas\n",
		len(clients), len(projects), len(tasks), len(payments))
}
