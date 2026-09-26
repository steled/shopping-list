package handlers

import (
	"encoding/json"
	"errors"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/steled/shopping-list/internal/database"
	"github.com/steled/shopping-list/internal/mealplan"
)

const (
	maxRecipeNameLength = 100
	maxIngredientAmount = 50
	maxIngredients      = 100
	maxTransferEntries  = 10
	maxPlanMonthsAhead  = 12
	monthLayout         = "2006-01"
	prevTailLength      = 2
)

func newRand() *rand.Rand {
	return rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
}

// today returns the current local calendar date as a UTC midnight, the same
// representation mealplan.WeekendDates uses.
func (h *Handler) today() time.Time {
	n := h.now()
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
}

func monthStart(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}

func monthRange(m time.Time) (string, string) {
	return m.Format(mealplan.DateLayout), m.AddDate(0, 1, -1).Format(mealplan.DateLayout)
}

// upcomingWeekend returns the remaining days of the current weekend, or the
// next Saturday and Sunday on weekdays.
func upcomingWeekend(today time.Time) []time.Time {
	switch today.Weekday() {
	case time.Sunday:
		return []time.Time{today}
	case time.Saturday:
		return []time.Time{today, today.AddDate(0, 0, 1)}
	}
	sat := today.AddDate(0, 0, int(time.Saturday-today.Weekday()))
	return []time.Time{sat, sat.AddDate(0, 0, 1)}
}

// ensurePlan generates plan entries for every month from the current one up
// to and including target: each future weekend day without a dish gets one.
// Callers must hold h.planMu.
func (h *Handler) ensurePlan(target time.Time) error {
	today := h.today()
	for m := monthStart(today); !m.After(monthStart(target)); m = m.AddDate(0, 1, 0) {
		if err := h.fillMonth(m, today, false); err != nil {
			return err
		}
	}
	return nil
}

// monthSlots returns the stored plan of month m with one entry per weekend
// day (missing days have a nil recipe) plus the dates in order.
func (h *Handler) monthSlots(m time.Time) ([]database.PlanSlot, error) {
	from, to := monthRange(m)
	stored, err := h.db.GetPlanRange(from, to)
	if err != nil {
		return nil, err
	}
	byDate := make(map[string]database.PlanSlot, len(stored))
	for _, s := range stored {
		byDate[s.Date] = s
	}
	dates := mealplan.WeekendDates(m.Year(), m.Month())
	out := make([]database.PlanSlot, len(dates))
	for i, d := range dates {
		key := d.Format(mealplan.DateLayout)
		if s, ok := byDate[key]; ok {
			out[i] = s
		} else {
			out[i] = database.PlanSlot{Date: key}
		}
	}
	return out, nil
}

func recipeIDs(slots []database.PlanSlot) []int64 {
	out := make([]int64, len(slots))
	for i, s := range slots {
		if s.RecipeID != nil {
			out[i] = *s.RecipeID
		}
	}
	return out
}

func (h *Handler) prevTail(m time.Time) ([]int64, error) {
	prev, err := h.monthSlots(m.AddDate(0, -1, 0))
	if err != nil {
		return nil, err
	}
	var tail []int64
	for _, id := range recipeIDs(prev) {
		if id != 0 {
			tail = append(tail, id)
		}
	}
	if len(tail) > prevTailLength {
		tail = tail[len(tail)-prevTailLength:]
	}
	return tail, nil
}

// fillMonth assigns dishes to the future weekend days of month m that have
// none. With reroll set, future days whose ingredients are not on the list
// yet are re-assigned as well.
func (h *Handler) fillMonth(m, today time.Time, reroll bool) error {
	slots, err := h.monthSlots(m)
	if err != nil {
		return err
	}
	active, err := h.db.ActiveRecipeIDs()
	if err != nil {
		return err
	}
	todayKey := today.Format(mealplan.DateLayout)
	ids := recipeIDs(slots)
	fill := make([]bool, len(slots))
	needsFill := false
	for i, s := range slots {
		if s.Date < todayKey {
			continue
		}
		if ids[i] == 0 {
			fill[i] = true
		} else if reroll && !s.Transferred {
			stale, err := h.db.CountPlanItems(s.Date, ids[i], true)
			if err != nil {
				return err
			}
			fill[i] = stale == 0
		}
		needsFill = needsFill || fill[i]
	}
	if !needsFill || len(active) == 0 {
		return nil
	}
	tail, err := h.prevTail(m)
	if err != nil {
		return err
	}
	for i := range ids {
		if fill[i] {
			ids[i] = 0
		}
	}
	filled := mealplan.Fill(ids, fill, active, tail, newRand())
	var changed []database.PlanSlot
	for i, s := range slots {
		if fill[i] && filled[i] != 0 {
			id := filled[i]
			changed = append(changed, database.PlanSlot{Date: s.Date, RecipeID: &id})
		}
	}
	return h.db.SavePlanSlots(changed)
}

/* ── Pages ──────────────────────────────────────────────────────────────── */

// Plan renders the weekend plan page.
func (h *Handler) Plan(w http.ResponseWriter, _ *http.Request) {
	h.render(w, "plan", map[string]any{"Version": h.version, "LoggedIn": true, "Page": "plan"})
}

// Recipes renders the recipe management page.
func (h *Handler) Recipes(w http.ResponseWriter, _ *http.Request) {
	h.render(w, "recipes", map[string]any{"Version": h.version, "LoggedIn": true, "Page": "recipes"})
}

/* ── Recipes API ────────────────────────────────────────────────────────── */

// APIGetRecipes returns all recipes with their ingredients.
func (h *Handler) APIGetRecipes(w http.ResponseWriter, _ *http.Request) {
	recipes, err := h.db.GetRecipes()
	if err != nil {
		slog.Error("get recipes", "err", err)
		jsonError(w, "failed to fetch recipes", http.StatusInternalServerError)
		return
	}
	jsonOK(w, recipes)
}

func validRecipeName(raw string) (string, bool) {
	name := strings.TrimSpace(raw)
	return name, name != "" && len(name) <= maxRecipeNameLength
}

// APICreateRecipe creates a new recipe without ingredients.
func (h *Handler) APICreateRecipe(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request", http.StatusBadRequest)
		return
	}
	name, ok := validRecipeName(req.Name)
	if !ok {
		jsonError(w, "invalid name", http.StatusBadRequest)
		return
	}
	recipe, err := h.db.CreateRecipe(name)
	if err != nil {
		slog.Error("create recipe", "err", err)
		jsonError(w, "failed to create recipe", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(recipe)
}

// APIUpdateRecipe replaces name, active flag and ingredient list of a recipe.
func (h *Handler) APIUpdateRecipe(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}
	var req struct {
		Name        string `json:"name"`
		Active      bool   `json:"active"`
		Ingredients []struct {
			Name       string `json:"name"`
			Quantity   int    `json:"quantity"`
			Amount     string `json:"amount"`
			CategoryID *int64 `json:"category_id"`
			Pantry     bool   `json:"pantry"`
		} `json:"ingredients"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request", http.StatusBadRequest)
		return
	}
	name, ok := validRecipeName(req.Name)
	if !ok {
		jsonError(w, "invalid name", http.StatusBadRequest)
		return
	}
	if len(req.Ingredients) > maxIngredients {
		jsonError(w, "too many ingredients", http.StatusBadRequest)
		return
	}
	ings := make([]database.Ingredient, 0, len(req.Ingredients))
	for _, in := range req.Ingredients {
		ingName := strings.TrimSpace(in.Name)
		amount := strings.TrimSpace(in.Amount)
		if ingName == "" || len(ingName) > maxNameLength || len(amount) > maxIngredientAmount {
			jsonError(w, "invalid ingredient", http.StatusBadRequest)
			return
		}
		qty := in.Quantity
		if qty < 1 {
			qty = 1
		}
		if qty > maxQuantity {
			jsonError(w, "quantity out of range", http.StatusBadRequest)
			return
		}
		ings = append(ings, database.Ingredient{
			Name: ingName, Quantity: qty, Amount: amount, CategoryID: in.CategoryID, Pantry: in.Pantry,
		})
	}
	recipe, err := h.db.UpdateRecipe(id, name, req.Active, ings)
	switch {
	case errors.Is(err, database.ErrNotFound):
		jsonError(w, "recipe not found", http.StatusNotFound)
	case errors.Is(err, database.ErrCategoryNotFound):
		jsonError(w, "category not found", http.StatusBadRequest)
	case err != nil:
		slog.Error("update recipe", "err", err)
		jsonError(w, "failed to update recipe", http.StatusInternalServerError)
	default:
		jsonOK(w, recipe)
	}
}

// APIDeleteRecipe removes a recipe. Upcoming plan days that used it get a new
// dish the next time the plan is loaded.
func (h *Handler) APIDeleteRecipe(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}
	h.planMu.Lock()
	defer h.planMu.Unlock()
	if err := h.db.DeleteRecipe(id); err != nil {
		if errors.Is(err, database.ErrNotFound) {
			jsonError(w, "recipe not found", http.StatusNotFound)
			return
		}
		slog.Error("delete recipe", "err", err)
		jsonError(w, "failed to delete recipe", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

/* ── Plan API ───────────────────────────────────────────────────────────── */

type planSlotView struct {
	Date        string `json:"date"`
	RecipeID    *int64 `json:"recipe_id"`
	Transferred bool   `json:"transferred"`
	OpenItems   int    `json:"open_items"`
	StaleItems  int    `json:"stale_items"`
}

func parseMonth(raw string) (time.Time, bool) {
	m, err := time.Parse(monthLayout, raw)
	return m, err == nil
}

// parseFutureWeekendDate validates a plan date path value: a Saturday or
// Sunday that is not in the past.
func (h *Handler) parseFutureWeekendDate(raw string) (string, bool) {
	d, err := time.Parse(mealplan.DateLayout, raw)
	if err != nil || !mealplan.IsWeekend(d) || d.Before(h.today()) {
		return "", false
	}
	return d.Format(mealplan.DateLayout), true
}

func (h *Handler) withinPlanHorizon(m time.Time) bool {
	return !monthStart(m).After(monthStart(h.today()).AddDate(0, maxPlanMonthsAhead, 0))
}

// APIGetPlan returns the plan of one month (?month=YYYY-MM, default: current),
// generating dishes for upcoming weekend days that have none yet.
func (h *Handler) APIGetPlan(w http.ResponseWriter, r *http.Request) {
	m := monthStart(h.today())
	if raw := r.URL.Query().Get("month"); raw != "" {
		var ok bool
		if m, ok = parseMonth(raw); !ok || !h.withinPlanHorizon(m) {
			jsonError(w, "invalid month", http.StatusBadRequest)
			return
		}
	}

	h.planMu.Lock()
	defer h.planMu.Unlock()
	if err := h.ensurePlan(m); err != nil {
		slog.Error("ensure plan", "err", err)
		jsonError(w, "failed to build plan", http.StatusInternalServerError)
		return
	}
	slots, err := h.monthSlots(m)
	if err != nil {
		slog.Error("get plan", "err", err)
		jsonError(w, "failed to fetch plan", http.StatusInternalServerError)
		return
	}
	active, err := h.db.ActiveRecipeIDs()
	if err != nil {
		slog.Error("active recipes", "err", err)
		jsonError(w, "failed to fetch plan", http.StatusInternalServerError)
		return
	}
	views := make([]planSlotView, len(slots))
	for i, s := range slots {
		v := planSlotView{Date: s.Date, RecipeID: s.RecipeID, Transferred: s.Transferred}
		var rid int64
		if s.RecipeID != nil {
			rid = *s.RecipeID
		}
		if v.OpenItems, err = h.db.CountPlanItems(s.Date, rid, false); err == nil {
			v.StaleItems, err = h.db.CountPlanItems(s.Date, rid, true)
		}
		if err != nil {
			slog.Error("count plan items", "err", err)
			jsonError(w, "failed to fetch plan", http.StatusInternalServerError)
			return
		}
		views[i] = v
	}
	jsonOK(w, map[string]any{
		"month":          m.Format(monthLayout),
		"today":          h.today().Format(mealplan.DateLayout),
		"active_recipes": len(active),
		"slots":          views,
	})
}

// APIRerollPlan re-assigns every upcoming day of a month whose ingredients
// are not on the shopping list yet.
func (h *Handler) APIRerollPlan(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var req struct {
		Month string `json:"month"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request", http.StatusBadRequest)
		return
	}
	m, ok := parseMonth(req.Month)
	if !ok || !h.withinPlanHorizon(m) || m.Before(monthStart(h.today())) {
		jsonError(w, "invalid month", http.StatusBadRequest)
		return
	}
	h.planMu.Lock()
	defer h.planMu.Unlock()
	err := h.ensurePlan(m)
	if err == nil {
		err = h.fillMonth(m, h.today(), true)
	}
	if err != nil {
		slog.Error("reroll plan", "err", err)
		jsonError(w, "failed to reroll plan", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// APISetPlanRecipe sets the dish of one day manually.
func (h *Handler) APISetPlanRecipe(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	date, ok := h.parseFutureWeekendDate(r.PathValue("date"))
	if !ok {
		jsonError(w, "invalid date", http.StatusBadRequest)
		return
	}
	var req struct {
		RecipeID int64 `json:"recipe_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RecipeID <= 0 {
		jsonError(w, "invalid request", http.StatusBadRequest)
		return
	}
	h.planMu.Lock()
	defer h.planMu.Unlock()
	if err := h.db.SavePlanSlots([]database.PlanSlot{{Date: date, RecipeID: &req.RecipeID}}); err != nil {
		if errors.Is(err, database.ErrNotFound) {
			jsonError(w, "recipe not found", http.StatusBadRequest)
			return
		}
		slog.Error("set plan recipe", "err", err)
		jsonError(w, "failed to update plan", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// APIRerollPlanDay picks a different dish for one day.
func (h *Handler) APIRerollPlanDay(w http.ResponseWriter, r *http.Request) {
	date, ok := h.parseFutureWeekendDate(r.PathValue("date"))
	if !ok {
		jsonError(w, "invalid date", http.StatusBadRequest)
		return
	}
	d, _ := time.Parse(mealplan.DateLayout, date)
	m := monthStart(d)
	if !h.withinPlanHorizon(m) {
		jsonError(w, "invalid date", http.StatusBadRequest)
		return
	}

	h.planMu.Lock()
	defer h.planMu.Unlock()
	fail := func(err error) {
		slog.Error("reroll plan day", "err", err)
		jsonError(w, "failed to update plan", http.StatusInternalServerError)
	}
	if err := h.ensurePlan(m); err != nil {
		fail(err)
		return
	}
	slots, err := h.monthSlots(m)
	if err != nil {
		fail(err)
		return
	}
	active, err := h.db.ActiveRecipeIDs()
	if err != nil {
		fail(err)
		return
	}
	tail, err := h.prevTail(m)
	if err != nil {
		fail(err)
		return
	}
	ids := recipeIDs(slots)
	for i, s := range slots {
		if s.Date != date {
			continue
		}
		next := mealplan.Reroll(ids, i, active, tail, newRand())
		if next != 0 && next != ids[i] {
			if err := h.db.SavePlanSlots([]database.PlanSlot{{Date: date, RecipeID: &next}}); err != nil {
				fail(err)
				return
			}
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// APIResolveStalePlanItems handles items left over after the dish of a day
// was changed: {"remove": true} takes them off the list, otherwise they stay
// and just lose their link to the old dish.
func (h *Handler) APIResolveStalePlanItems(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	date, ok := h.parseFutureWeekendDate(r.PathValue("date"))
	if !ok {
		jsonError(w, "invalid date", http.StatusBadRequest)
		return
	}
	var req struct {
		Remove bool `json:"remove"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request", http.StatusBadRequest)
		return
	}
	h.planMu.Lock()
	defer h.planMu.Unlock()
	var err error
	if req.Remove {
		_, err = h.db.RemovePlanItems(date, true)
	} else {
		var slot database.PlanSlot
		if slot, err = h.db.GetPlanSlot(date); err == nil {
			var rid int64
			if slot.RecipeID != nil {
				rid = *slot.RecipeID
			}
			err = h.db.KeepStalePlanItems(date, rid)
		}
	}
	switch {
	case errors.Is(err, database.ErrNotFound):
		jsonError(w, "plan day not found", http.StatusNotFound)
	case err != nil:
		slog.Error("resolve stale plan items", "err", err)
		jsonError(w, "failed to update list", http.StatusInternalServerError)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

// APIRemovePlanItems undoes the ingredient transfer of one day.
func (h *Handler) APIRemovePlanItems(w http.ResponseWriter, r *http.Request) {
	date, ok := h.parseFutureWeekendDate(r.PathValue("date"))
	if !ok {
		jsonError(w, "invalid date", http.StatusBadRequest)
		return
	}
	h.planMu.Lock()
	defer h.planMu.Unlock()
	if _, err := h.db.RemovePlanItems(date, false); err != nil {
		if errors.Is(err, database.ErrNotFound) {
			jsonError(w, "plan day not found", http.StatusNotFound)
			return
		}
		slog.Error("remove plan items", "err", err)
		jsonError(w, "failed to update list", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// APITransferIngredients puts the chosen ingredients of one or more planned
// days on the shopping list.
func (h *Handler) APITransferIngredients(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var req struct {
		Entries []struct {
			Date          string  `json:"date"`
			IngredientIDs []int64 `json:"ingredient_ids"`
		} `json:"entries"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Entries) == 0 || len(req.Entries) > maxTransferEntries {
		jsonError(w, "invalid request", http.StatusBadRequest)
		return
	}
	entries := make([]database.TransferEntry, 0, len(req.Entries))
	seenDates := map[string]bool{}
	for _, e := range req.Entries {
		date, ok := h.parseFutureWeekendDate(e.Date)
		if !ok || seenDates[date] || len(e.IngredientIDs) > maxIngredients {
			jsonError(w, "invalid request", http.StatusBadRequest)
			return
		}
		seenDates[date] = true
		seenIngs := map[int64]bool{}
		ids := make([]int64, 0, len(e.IngredientIDs))
		for _, id := range e.IngredientIDs {
			if !seenIngs[id] {
				seenIngs[id] = true
				ids = append(ids, id)
			}
		}
		entries = append(entries, database.TransferEntry{Date: date, IngredientIDs: ids})
	}

	h.planMu.Lock()
	defer h.planMu.Unlock()
	n, err := h.db.TransferIngredients(entries)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			jsonError(w, "plan day or ingredient not found", http.StatusBadRequest)
			return
		}
		slog.Error("transfer ingredients", "err", err)
		jsonError(w, "failed to update list", http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]int{"items": n})
}
