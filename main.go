package main

import (
	"context"
	"embed"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/steled/shopping-list/internal/auth"
	"github.com/steled/shopping-list/internal/database"
	"github.com/steled/shopping-list/internal/handlers"
)

//go:embed templates static
var embeddedFS embed.FS

var version = "dev"

func main() {
	username := getenv("APP_USERNAME", "admin")
	password := mustenv("APP_PASSWORD")
	sessionSecret := mustenv("APP_SESSION_SECRET")
	if len(sessionSecret) < 32 {
		slog.Error("APP_SESSION_SECRET must be at least 32 characters (use: openssl rand -hex 32)")
		os.Exit(1)
	}
	secureCookies := os.Getenv("APP_SECURE_COOKIES") == "true"
	dbPath := getenv("DATABASE_PATH", "/data/shopping.db")
	addr := getenv("APP_ADDR", ":8080")

	db, err := database.Open(dbPath)
	if err != nil {
		slog.Error("failed to open database", "err", err)
		os.Exit(1)
	}
	defer func() { _ = db.Close() }()

	a := auth.New(username, password, sessionSecret, secureCookies)

	staticSubFS, err := fs.Sub(embeddedFS, "static")
	if err != nil {
		slog.Error("failed to create static sub-fs", "err", err)
		os.Exit(1)
	}

	h, err := handlers.New(db, a, embeddedFS, version)
	if err != nil {
		slog.Error("failed to create handlers", "err", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.Healthz)
	mux.HandleFunc("GET /", h.Index)
	mux.HandleFunc("GET /login", h.LoginGET)
	mux.HandleFunc("POST /login", h.LoginPOST)
	mux.HandleFunc("POST /logout", h.Logout)
	mux.HandleFunc("GET /list", h.RequireAuth(h.List))
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(staticSubFS)))
	mux.HandleFunc("GET /api/items", h.RequireAuth(h.APIGetItems))
	mux.HandleFunc("POST /api/items", h.RequireAuth(h.APICreateItem))
	mux.HandleFunc("PATCH /api/items/reorder", h.RequireAuth(h.APIReorderItems))
	mux.HandleFunc("PUT /api/items/{id}", h.RequireAuth(h.APIUpdateItem))
	mux.HandleFunc("DELETE /api/items/{id}", h.RequireAuth(h.APIDeleteItem))

	srv := &http.Server{
		Addr:         addr,
		Handler:      securityHeaders(mux),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	slog.Info("starting server", "addr", addr, "version", version)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self'; "+
				"style-src 'self'; "+
				"img-src 'self' data:; "+
				"font-src 'self';")
		next.ServeHTTP(w, r)
	})
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func mustenv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		slog.Error("required environment variable not set", "var", key)
		os.Exit(1)
	}
	return v
}
