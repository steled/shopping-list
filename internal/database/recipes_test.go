package database

import (
	"errors"
	"testing"
)

func openTest(t *testing.T) *DB {
	t.Helper()
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func mustRecipe(t *testing.T, db *DB, name string, ings ...Ingredient) Recipe {
	t.Helper()
	r, err := db.CreateRecipe(name)
	if err != nil {
		t.Fatal(err)
	}
	r, err = db.UpdateRecipe(r.ID, name, true, ings)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func plan(t *testing.T, db *DB, date string, recipeID int64) {
	t.Helper()
	if err := db.SavePlanSlots([]PlanSlot{{Date: date, RecipeID: &recipeID}}); err != nil {
		t.Fatal(err)
	}
}

func itemByName(t *testing.T, db *DB, name string) *Item {
	t.Helper()
	items, err := db.GetItems()
	if err != nil {
		t.Fatal(err)
	}
	for i := range items {
		if items[i].Name == name {
			return &items[i]
		}
	}
	return nil
}

func TestRecipeCRUD(t *testing.T) {
	db := openTest(t)
	cat, _ := db.CreateCategory("Obst & Gemüse")

	r := mustRecipe(t, db, "Kartoffelsuppe",
		Ingredient{Name: "Kartoffeln", Quantity: 1, CategoryID: &cat.ID},
		Ingredient{Name: "Brühe", Quantity: 1, Pantry: true},
	)
	if len(r.Ingredients) != 2 || r.Ingredients[0].Name != "Kartoffeln" || !r.Ingredients[1].Pantry {
		t.Fatalf("unexpected ingredients: %+v", r.Ingredients)
	}
	if r.Ingredients[0].CategoryID == nil || *r.Ingredients[0].CategoryID != cat.ID {
		t.Fatalf("category not stored: %+v", r.Ingredients[0])
	}

	r, err := db.UpdateRecipe(r.ID, "Omas Kartoffelsuppe", false, []Ingredient{{Name: "Lauch", Quantity: 2, Amount: "1 Stange"}})
	if err != nil {
		t.Fatal(err)
	}
	if r.Name != "Omas Kartoffelsuppe" || r.Active || len(r.Ingredients) != 1 || r.Ingredients[0].Amount != "1 Stange" {
		t.Fatalf("unexpected recipe after update: %+v", r)
	}
	if ids, _ := db.ActiveRecipeIDs(); len(ids) != 0 {
		t.Fatalf("inactive recipe listed as active: %v", ids)
	}

	if _, err := db.UpdateRecipe(9999, "x", true, nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	if _, err := db.UpdateRecipe(r.ID, "x", true, []Ingredient{{Name: "y", Quantity: 1, CategoryID: ptr(int64(9999))}}); !errors.Is(err, ErrCategoryNotFound) {
		t.Fatalf("expected ErrCategoryNotFound, got %v", err)
	}

	if err := db.DeleteRecipe(r.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.DeleteRecipe(r.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound on second delete, got %v", err)
	}
}

func TestSavePlanSlotsResetsTransferredOnChange(t *testing.T) {
	db := openTest(t)
	a := mustRecipe(t, db, "A", Ingredient{Name: "Mehl", Quantity: 1})
	b := mustRecipe(t, db, "B")
	plan(t, db, "2026-10-03", a.ID)

	if _, err := db.TransferIngredients([]TransferEntry{{Date: "2026-10-03", IngredientIDs: []int64{a.Ingredients[0].ID}}}); err != nil {
		t.Fatal(err)
	}
	plan(t, db, "2026-10-03", a.ID) // same recipe: flag stays
	if s, _ := db.GetPlanSlot("2026-10-03"); !s.Transferred {
		t.Fatal("transferred flag reset although recipe did not change")
	}
	plan(t, db, "2026-10-03", b.ID)
	if s, _ := db.GetPlanSlot("2026-10-03"); s.Transferred || *s.RecipeID != b.ID {
		t.Fatalf("expected new recipe and reset flag, got %+v", s)
	}
	if err := db.SavePlanSlots([]PlanSlot{{Date: "2026-10-04", RecipeID: ptr(int64(9999))}}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for unknown recipe, got %v", err)
	}
}

func TestTransferMergesByNameWithMaxQuantity(t *testing.T) {
	db := openTest(t)
	suppe := mustRecipe(t, db, "Suppe", Ingredient{Name: "Kartoffeln", Quantity: 1}, Ingredient{Name: "Möhren", Quantity: 3})
	schnitzel := mustRecipe(t, db, "Schnitzel", Ingredient{Name: "kartoffeln ", Quantity: 2})
	plan(t, db, "2026-10-03", suppe.ID)
	plan(t, db, "2026-10-04", schnitzel.ID)

	n, err := db.TransferIngredients([]TransferEntry{
		{Date: "2026-10-03", IngredientIDs: []int64{suppe.Ingredients[0].ID, suppe.Ingredients[1].ID}},
		{Date: "2026-10-04", IngredientIDs: []int64{schnitzel.Ingredients[0].ID}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("expected 2 list entries, got %d", n)
	}
	k := itemByName(t, db, "Kartoffeln")
	if k == nil || k.Quantity != 2 {
		t.Fatalf("expected one Kartoffeln item with quantity 2, got %+v", k)
	}
	if m := itemByName(t, db, "Möhren"); m == nil || m.Quantity != 3 {
		t.Fatalf("expected Möhren with quantity 3, got %+v", m)
	}
	sources, _ := db.GetItemSources()
	if len(sources[k.ID]) != 2 {
		t.Fatalf("expected two dish links on Kartoffeln, got %+v", sources[k.ID])
	}

	// Repeating a transfer adds nothing twice.
	if _, err := db.TransferIngredients([]TransferEntry{{Date: "2026-10-03", IngredientIDs: []int64{suppe.Ingredients[0].ID}}}); err != nil {
		t.Fatal(err)
	}
	items, _ := db.GetItems()
	if len(items) != 2 {
		t.Fatalf("repeated transfer created duplicates: %+v", items)
	}
}

func TestTransferRejectsForeignIngredient(t *testing.T) {
	db := openTest(t)
	a := mustRecipe(t, db, "A")
	b := mustRecipe(t, db, "B", Ingredient{Name: "Reis", Quantity: 1})
	plan(t, db, "2026-10-03", a.ID)

	_, err := db.TransferIngredients([]TransferEntry{{Date: "2026-10-03", IngredientIDs: []int64{b.Ingredients[0].ID}}})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for ingredient of another recipe, got %v", err)
	}
	if _, err := db.TransferIngredients([]TransferEntry{{Date: "2026-10-10"}}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for unplanned day, got %v", err)
	}
	if items, _ := db.GetItems(); len(items) != 0 {
		t.Fatalf("failed transfer left items behind: %+v", items)
	}
}

func TestRemovePlanItemsKeepsManualAndCheckedItems(t *testing.T) {
	db := openTest(t)
	r := mustRecipe(t, db, "Lasagne",
		Ingredient{Name: "Zwiebeln", Quantity: 2},
		Ingredient{Name: "Hackfleisch", Quantity: 1},
		Ingredient{Name: "Nudeln", Quantity: 1},
	)
	plan(t, db, "2026-10-03", r.ID)
	manual, _ := db.CreateItem("Zwiebeln", 1, nil)

	if _, err := db.TransferIngredients([]TransferEntry{{Date: "2026-10-03", IngredientIDs: []int64{
		r.Ingredients[0].ID, r.Ingredients[1].ID, r.Ingredients[2].ID,
	}}}); err != nil {
		t.Fatal(err)
	}
	nudeln := itemByName(t, db, "Nudeln")
	if err := db.UpdateItem(nudeln.ID, "Nudeln", 1, true, nil); err != nil { // already bought
		t.Fatal(err)
	}

	n, err := db.RemovePlanItems("2026-10-03", false)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("expected only Hackfleisch to be deleted, got %d", n)
	}
	if itemByName(t, db, "Hackfleisch") != nil {
		t.Fatal("Hackfleisch should be gone")
	}
	if z := itemByName(t, db, "Zwiebeln"); z == nil || z.ID != manual.ID {
		t.Fatal("manually added Zwiebeln must stay")
	}
	if itemByName(t, db, "Nudeln") == nil {
		t.Fatal("checked Nudeln must stay")
	}
	if s, _ := db.GetPlanSlot("2026-10-03"); s.Transferred {
		t.Fatal("transferred flag should be reset")
	}
}

func TestStalePlanItemsAfterDishChange(t *testing.T) {
	db := openTest(t)
	a := mustRecipe(t, db, "A", Ingredient{Name: "Reis", Quantity: 1})
	b := mustRecipe(t, db, "B", Ingredient{Name: "Nudeln", Quantity: 1})
	plan(t, db, "2026-10-03", a.ID)
	if _, err := db.TransferIngredients([]TransferEntry{{Date: "2026-10-03", IngredientIDs: []int64{a.Ingredients[0].ID}}}); err != nil {
		t.Fatal(err)
	}
	plan(t, db, "2026-10-03", b.ID)

	if n, _ := db.CountPlanItems("2026-10-03", b.ID, true); n != 1 {
		t.Fatalf("expected 1 stale item, got %d", n)
	}
	// Transferring the new dish must not touch the old dish's items.
	if _, err := db.TransferIngredients([]TransferEntry{{Date: "2026-10-03", IngredientIDs: []int64{b.Ingredients[0].ID}}}); err != nil {
		t.Fatal(err)
	}
	if itemByName(t, db, "Reis") == nil {
		t.Fatal("stale item removed by transfer of the new dish")
	}

	if err := db.KeepStalePlanItems("2026-10-03", b.ID); err != nil {
		t.Fatal(err)
	}
	if n, _ := db.CountPlanItems("2026-10-03", b.ID, true); n != 0 {
		t.Fatalf("expected no stale items after keeping, got %d", n)
	}
	if itemByName(t, db, "Reis") == nil || itemByName(t, db, "Nudeln") == nil {
		t.Fatal("keeping stale items must not delete anything")
	}
}

func TestDeleteRecipeKeepsItemsAndClearsPlan(t *testing.T) {
	db := openTest(t)
	r := mustRecipe(t, db, "A", Ingredient{Name: "Reis", Quantity: 1})
	plan(t, db, "2026-10-03", r.ID)
	if _, err := db.TransferIngredients([]TransferEntry{{Date: "2026-10-03", IngredientIDs: []int64{r.Ingredients[0].ID}}}); err != nil {
		t.Fatal(err)
	}
	if err := db.DeleteRecipe(r.ID); err != nil {
		t.Fatal(err)
	}
	if itemByName(t, db, "Reis") == nil {
		t.Fatal("item must survive recipe deletion")
	}
	if s, _ := db.GetPlanSlot("2026-10-03"); s.RecipeID != nil {
		t.Fatalf("plan slot still references deleted recipe: %+v", s)
	}
	if src, _ := db.GetItemSources(); len(src) != 0 {
		t.Fatalf("dish links should be gone: %+v", src)
	}
}
