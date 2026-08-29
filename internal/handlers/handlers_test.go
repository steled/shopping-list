package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/steled/shopping-list/internal/auth"
	"github.com/steled/shopping-list/internal/database"
)

const testSecret = "test-hmac-secret-for-unit-tests-padding"

func newTestHandler(t *testing.T) *Handler {
	t.Helper()
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	a := auth.New("admin", "pw", testSecret, false)
	h, err := New(db, a, os.DirFS("../.."), "test")
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestAPICreateCategory(t *testing.T) {
	h := newTestHandler(t)

	body := bytes.NewBufferString(`{"name":"Obst & Gemüse"}`)
	r := httptest.NewRequest(http.MethodPost, "/api/categories", body)
	w := httptest.NewRecorder()

	h.APICreateCategory(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var got database.Category
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Name != "Obst & Gemüse" || got.ID == 0 {
		t.Fatalf("unexpected category in response: %+v", got)
	}
}

func TestAPICreateCategoryRejectsEmptyName(t *testing.T) {
	h := newTestHandler(t)

	body := bytes.NewBufferString(`{"name":"   "}`)
	r := httptest.NewRequest(http.MethodPost, "/api/categories", body)
	w := httptest.NewRecorder()

	h.APICreateCategory(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAPIReorderItemsUnknownCategoryReturns400(t *testing.T) {
	h := newTestHandler(t)

	item, err := h.db.CreateItem("Item", 1, nil)
	if err != nil {
		t.Fatal(err)
	}

	body := bytes.NewBufferString(fmt.Sprintf(`{"category_id": 999999, "ids": [%d]}`, item.ID))
	r := httptest.NewRequest(http.MethodPatch, "/api/items/reorder", body)
	w := httptest.NewRecorder()

	h.APIReorderItems(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRequireAuthRejectsUnauthenticatedAPIRequest(t *testing.T) {
	h := newTestHandler(t)

	r := httptest.NewRequest(http.MethodGet, "/api/categories", nil)
	w := httptest.NewRecorder()

	h.RequireAuth(h.APIGetCategories)(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
	var got map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["error"] == "" {
		t.Fatalf("expected JSON error body, got %s", w.Body.String())
	}
}
