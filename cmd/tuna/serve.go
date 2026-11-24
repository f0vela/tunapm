package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"tunapm/internal/api"
	"tunapm/internal/client"
	"tunapm/internal/store"
	"tunapm/internal/web"
)

// runServe expone la base SQLite como API REST (+ web app opcional en "/").
func runServe(dbPath, token string, args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	addr := fs.String("addr", envOr("TUNA_ADDR", "127.0.0.1:8080"), "dirección de escucha")
	withWeb := fs.Bool("web", false, "servir también la web app en /")
	secure := fs.Bool("secure-cookie", false, "marcar la cookie de la web como Secure (detrás de HTTPS)")
	_ = fs.Parse(args)

	if token == "" && !isLoopback(*addr) {
		fatal("serve: escuchar fuera de localhost requiere token (-token o TUNA_TOKEN)")
	}
	st, err := store.Open(dbPath)
	if err != nil {
		fatal("no se pudo abrir la base de datos: " + err.Error())
	}
	defer st.Close()

	mux := http.NewServeMux()
	mux.Handle("/api/", api.New(st, token))
	if *withWeb {
		h, err := web.New(st, web.Options{Password: envOr("TUNA_WEB_PASSWORD", token), SecureCookie: *secure})
		if err != nil {
			fatal("web: " + err.Error())
		}
		mux.Handle("/", h)
	}
	serve(*addr, mux, fmt.Sprintf("API en http://%s/api (db: %s, web: %v)", *addr, dbPath, *withWeb))
}

// runWeb levanta solo la web app, hablando con un API remoto.
func runWeb(apiURL, token string, args []string) {
	fs := flag.NewFlagSet("web", flag.ExitOnError)
	addr := fs.String("addr", envOr("TUNA_ADDR", "127.0.0.1:8081"), "dirección de escucha")
	secure := fs.Bool("secure-cookie", false, "marcar la cookie como Secure (detrás de HTTPS)")
	_ = fs.Parse(args)

	if apiURL == "" {
		fatal("web: falta -api (o TUNA_API)")
	}
	password := envOr("TUNA_WEB_PASSWORD", token)
	if password == "" && !isLoopback(*addr) {
		fatal("web: escuchar fuera de localhost requiere TUNA_WEB_PASSWORD")
	}
	c := client.New(apiURL, token)
	if err := c.Ping(); err != nil {
		log.Println("aviso: el API no responde todavía:", err)
	}
	h, err := web.New(c, web.Options{Password: password, SecureCookie: *secure})
	if err != nil {
		fatal("web: " + err.Error())
	}
	serve(*addr, h, fmt.Sprintf("Web en http://%s (API: %s)", *addr, apiURL))
}

func serve(addr string, h http.Handler, banner string) {
	srv := &http.Server{
		Addr: addr, Handler: h,
		ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second,
		WriteTimeout: 30 * time.Second, IdleTimeout: 120 * time.Second,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()
	log.Println(banner)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fatal(err.Error())
	}
}

func isLoopback(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	return host == "localhost" || strings.HasPrefix(host, "127.") || host == "::1"
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}
