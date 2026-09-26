package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/steled/shopping-list/internal/database"
)

// Friday, 25 September 2026.
var fixedNow = time.Date(2026, time.September, 25, 12, 0, 0, 0, time.Local)

func newPlanHandler(t *testing.T, recipes int) *Handler {
	t.Helper()
	h := newTestHandler(t)
	h.now = func() time.Time { return fixedNow }
	for i := range recipes {
		r, err := h.db.CreateRecipe(string(rune('A' + i)))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := h.db.UpdateRecipe(r.ID, r.Name, true, []database.Ingredient{{Name: "Zutat " + r.Name, Quantity: 1}}); err != nil {
			t.Fatal(err)
		}
	}
	return h
}

type planResp struct {
	Month         string         `json:"month"`
	Today         string         `json:"today"`
	ActiveRecipes int            `json:"active_recipes"`
	Slots         []planSlotView `json:"slots"`
}

func getPlan(t *testing.T, h *Handler, month string) (planResp, int) {
	t.Helper()
	url := "/api/plan"
	if month != "" {
		url += "?month=" + month
	}
	w := httptest.NewRecorder()
	h.APIGetPlan(w, httptest.NewRequest(http.MethodGet, url, nil))
	var p planResp
	if w.Code == http.StatusOK {
		if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
			t.Fatal(err)
		}
	}
	return p, w.Code
}

func TestAPIGetPlanFillsUpcomingDaysOnly(t *testing.T) {
	h := newPlanHandler(t, 8)
	p, code := getPlan(t, h, "")
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if p.Month != "2026-09" || len(p.Slots) != 8 {
		t.Fatalf("unexpected plan: %+v", p)
	}
	seen := map[int64]bool{}
	for _, s := range p.Slots {
		past := s.Date < "2026-09-25"
		if past != (s.RecipeID == nil) {
			t.Fatalf("past days must stay empty, upcoming days must be filled: %+v", s)
		}
		if s.RecipeID != nil {
			if seen[*s.RecipeID] {
				t.Fatalf("recipe repeated within month: %+v", p.Slots)
			}
			seen[*s.RecipeID] = true
		}
	}

	// A second load returns the same, stored plan.
	again, _ := getPlan(t, h, "")
	for i := range p.Slots {
		a, b := p.Slots[i].RecipeID, again.Slots[i].RecipeID
		if (a == nil) != (b == nil) || (a != nil && *a != *b) {
			t.Fatalf("plan changed between loads: %+v vs %+v", p.Slots, again.Slots)
		}
	}
}

func TestAPIGetPlanRejectsInvalidMonth(t *testing.T) {
	h := newPlanHandler(t, 1)
	for _, m := range []string{"2026-13", "abc", "2028-01"} {
		if _, code := getPlan(t, h, m); code != http.StatusBadRequest {
			t.Fatalf("month %q: expected 400, got %d", m, code)
		}
	}
}

func TestAPISetPlanRecipeValidatesDate(t *testing.T) {
	h := newPlanHandler(t, 2)
	cases := map[string]int{
		"2026-09-26": http.StatusNoContent,  // upcoming Saturday
		"2026-09-20": http.StatusBadRequest, // past Sunday
		"2026-09-30": http.StatusBadRequest, // Wednesday
		"garbage":    http.StatusBadRequest,
	}
	for date, want := range cases {
		r := httptest.NewRequest(http.MethodPut, "/api/plan/"+date, bytes.NewBufferString(`{"recipe_id":1}`))
		r.SetPathValue("date", date)
		w := httptest.NewRecorder()
		h.APISetPlanRecipe(w, r)
		if w.Code != want {
			t.Fatalf("date %s: expected %d, got %d: %s", date, want, w.Code, w.Body.String())
		}
	}

	r := httptest.NewRequest(http.MethodPut, "/api/plan/2026-09-26", bytes.NewBufferString(`{"recipe_id":999}`))
	r.SetPathValue("date", "2026-09-26")
	w := httptest.NewRecorder()
	h.APISetPlanRecipe(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("unknown recipe: expected 400, got %d", w.Code)
	}
}

func TestAPITransferAndUndo(t *testing.T) {
	h := newPlanHandler(t, 3)
	p, _ := getPlan(t, h, "")
	var date string
	var rid int64
	for _, s := range p.Slots {
		if s.RecipeID != nil {
			date, rid = s.Date, *s.RecipeID
			break
		}
	}
	recipe, err := h.db.GetRecipe(rid)
	if err != nil {
		t.Fatal(err)
	}

	body := `{"entries":[{"date":"` + date + `","ingredient_ids":[` + jsonInt(recipe.Ingredients[0].ID) + `]}]}`
	w := httptest.NewRecorder()
	h.APITransferIngredients(w, httptest.NewRequest(http.MethodPost, "/api/plan/transfer", strings.NewReader(body)))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"items":1`) {
		t.Fatalf("transfer: expected 200 with 1 item, got %d: %s", w.Code, w.Body.String())
	}
	if items, _ := h.db.GetItems(); len(items) != 1 {
		t.Fatalf("expected 1 item on list, got %d", len(items))
	}

	r := httptest.NewRequest(http.MethodDelete, "/api/plan/"+date+"/items", nil)
	r.SetPathValue("date", date)
	w = httptest.NewRecorder()
	h.APIRemovePlanItems(w, r)
	if w.Code != http.StatusNoContent {
		t.Fatalf("undo: expected 204, got %d", w.Code)
	}
	if items, _ := h.db.GetItems(); len(items) != 0 {
		t.Fatalf("expected empty list after undo, got %d", len(items))
	}
}

func TestAPITransferRejectsBadInput(t *testing.T) {
	h := newPlanHandler(t, 1)
	for _, body := range []string{
		`{}`,
		`{"entries":[]}`,
		`{"entries":[{"date":"2026-09-20","ingredient_ids":[]}]}`,   // past
		`{"entries":[{"date":"2026-09-26"},{"date":"2026-09-26"}]}`, // duplicate
		`{"entries":[{"date":"2026-10-03","ingredient_ids":[1]}]}`,  // not planned yet
	} {
		w := httptest.NewRecorder()
		h.APITransferIngredients(w, httptest.NewRequest(http.MethodPost, "/api/plan/transfer", strings.NewReader(body)))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("body %s: expected 400, got %d", body, w.Code)
		}
	}
}

func TestAPIUpdateRecipeValidation(t *testing.T) {
	h := newPlanHandler(t, 1)
	for _, body := range []string{
		`{"name":"  ","active":true,"ingredients":[]}`,
		`{"name":"X","active":true,"ingredients":[{"name":"  ","quantity":1}]}`,
		`{"name":"X","active":true,"ingredients":[{"name":"Y","quantity":1000}]}`,
		`{"name":"X","active":true,"ingredients":[{"name":"Y","quantity":1,"amount":"` + strings.Repeat("a", 51) + `"}]}`,
	} {
		r := httptest.NewRequest(http.MethodPut, "/api/recipes/1", strings.NewReader(body))
		r.SetPathValue("id", "1")
		w := httptest.NewRecorder()
		h.APIUpdateRecipe(w, r)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("body %s: expected 400, got %d", body, w.Code)
		}
	}

	r := httptest.NewRequest(http.MethodPut, "/api/recipes/1", strings.NewReader(`{"name":"Suppe","active":true,"ingredients":[{"name":"Kartoffeln","quantity":0}]}`))
	r.SetPathValue("id", "1")
	w := httptest.NewRecorder()
	h.APIUpdateRecipe(w, r)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"quantity":1`) {
		t.Fatalf("expected 200 with quantity defaulted to 1, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListShowsPendingWeekendBanner(t *testing.T) {
	h := newPlanHandler(t, 2)
	w := httptest.NewRecorder()
	h.List(w, httptest.NewRequest(http.MethodGet, "/list", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "/plan?check=2026-09-26") || !strings.Contains(body, "Zutaten prüfen") {
		t.Fatal("expected banner for the upcoming weekend")
	}
	if !strings.Contains(body, `aria-current="page"`) {
		t.Fatal("expected active tab in navigation")
	}
}

func TestUpcomingWeekend(t *testing.T) {
	day := func(d int) time.Time { return time.Date(2026, time.September, d, 0, 0, 0, 0, time.UTC) }
	cases := map[int][]int{25: {26, 27}, 26: {26, 27}, 27: {27}, 28: {3 + 30, 4 + 30}}
	for in, want := range cases {
		got := upcomingWeekend(day(in))
		if len(got) != len(want) {
			t.Fatalf("%d: got %v", in, got)
		}
		for i := range got {
			if !got[i].Equal(day(want[i])) {
				t.Fatalf("%d: got %v, want day offsets %v", in, got, want)
			}
		}
	}
}

func jsonInt(v int64) string {
	b, _ := json.Marshal(v)
	return string(b)
}
