package handlers

import (
	"embed"
	"encoding/json"
	"html/template"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"datagroup.ghe.com/DGOPS/cloud-ops.k8s.shopping-list/internal/auth"
	"datagroup.ghe.com/DGOPS/cloud-ops.k8s.shopping-list/internal/database"
)

// Handler holds all HTTP handler dependencies.
type Handler struct {
	db      *database.DB
	auth    *auth.Auth
	version string
	tmpls   map[string]*template.Template
}

// New creates a Handler, parsing templates from the provided embed.FS.
func New(db *database.DB, a *auth.Auth, tmplFS embed.FS, version string) (*Handler, error) {
	h := &Handler{
		db:      db,
		auth:    a,
		version: version,
		tmpls:   make(map[string]*template.Template),
	}
	var err error
	h.tmpls["login"], err = template.ParseFS(tmplFS, "templates/base.html", "templates/login.html")
	if err != nil {
		return nil, err
	}
	h.tmpls["index"], err = template.ParseFS(tmplFS, "templates/base.html", "templates/index.html")
	if err != nil {
		return nil, err
	}
	return h, nil
}

// RequireAuth wraps a handler so that unauthenticated requests are redirected
// to /login (or receive 401 for /api/ paths).
func (h *Handler) RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !h.auth.IsAuthenticated(r) {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				jsonError(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}

// Healthz returns a simple JSON health check response.
func (h *Handler) Healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok","version":"` + h.version + `"}`))
}

// Index redirects to /list when authenticated, otherwise to /login.
func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if h.auth.IsAuthenticated(r) {
		http.Redirect(w, r, "/list", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// LoginGET renders the login form.
func (h *Handler) LoginGET(w http.ResponseWriter, r *http.Request) {
	if h.auth.IsAuthenticated(r) {
		http.Redirect(w, r, "/list", http.StatusSeeOther)
		return
	}
	h.render(w, "login", map[string]any{
		"Error":    "",
		"Version":  h.version,
		"LoggedIn": false,
	})
}

// LoginPOST validates credentials and sets a session cookie.
func (h *Handler) LoginPOST(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	username := r.FormValue("username")
	password := r.FormValue("password")

	if !h.auth.Validate(username, password) {
		h.render(w, "login", map[string]any{
			"Error":    "Ungültige Zugangsdaten.",
			"Version":  h.version,
			"LoggedIn": false,
		})
		return
	}
	h.auth.SetSessionCookie(w, r)
	http.Redirect(w, r, "/list", http.StatusSeeOther)
}

// Logout clears the session cookie and redirects to /login.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	h.auth.ClearSessionCookie(w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// List renders the shopping list page.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	items, err := h.db.GetItems()
	if err != nil {
		slog.Error("get items", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	h.render(w, "index", map[string]any{
		"Items":    items,
		"Version":  h.version,
		"LoggedIn": true,
	})
}

// APIGetItems returns all items as JSON.
func (h *Handler) APIGetItems(w http.ResponseWriter, _ *http.Request) {
	items, err := h.db.GetItems()
	if err != nil {
		jsonError(w, "failed to fetch items", http.StatusInternalServerError)
		return
	}
	if items == nil {
		items = []database.Item{}
	}
	jsonOK(w, items)
}

// APICreateItem creates a new item.
func (h *Handler) APICreateItem(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string `json:"name"`
		Quantity int    `json:"quantity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Name) == "" {
		jsonError(w, "invalid request", http.StatusBadRequest)
		return
	}
	if req.Quantity < 1 {
		req.Quantity = 1
	}
	item, err := h.db.CreateItem(strings.TrimSpace(req.Name), req.Quantity)
	if err != nil {
		jsonError(w, "failed to create item", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(item)
}

// APIUpdateItem updates name, quantity and checked state of an item.
func (h *Handler) APIUpdateItem(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}
	var req struct {
		Name     string `json:"name"`
		Quantity int    `json:"quantity"`
		Checked  bool   `json:"checked"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Name) == "" {
		jsonError(w, "invalid request", http.StatusBadRequest)
		return
	}
	if req.Quantity < 1 {
		req.Quantity = 1
	}
	if err := h.db.UpdateItem(id, strings.TrimSpace(req.Name), req.Quantity, req.Checked); err != nil {
		jsonError(w, "failed to update item", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// APIDeleteItem removes an item.
func (h *Handler) APIDeleteItem(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := h.db.DeleteItem(id); err != nil {
		jsonError(w, "failed to delete item", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// APIReorderItems updates item positions based on the provided id order.
func (h *Handler) APIReorderItems(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs []int64 `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.IDs) == 0 {
		jsonError(w, "invalid request", http.StatusBadRequest)
		return
	}
	if err := h.db.ReorderItems(req.IDs); err != nil {
		jsonError(w, "failed to reorder items", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) render(w http.ResponseWriter, name string, data any) {
	tmpl, ok := h.tmpls[name]
	if !ok {
		http.Error(w, "template not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
		slog.Error("template render error", "template", name, "err", err)
	}
}

func jsonOK(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func jsonError(w http.ResponseWriter, msg string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
