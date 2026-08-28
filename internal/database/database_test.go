package database

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func ptr[T any](v T) *T { return &v }

func TestCRUD(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	// Create
	item, err := db.CreateItem("Milch", 2, nil)
	if err != nil {
		t.Fatal(err)
	}
	if item.Name != "Milch" || item.Quantity != 2 || item.Checked {
		t.Fatalf("unexpected item after create: %+v", item)
	}

	// Get
	items, err := db.GetItems()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != item.ID {
		t.Fatalf("expected 1 item, got %d", len(items))
	}

	// Update
	if err := db.UpdateItem(item.ID, "Butter", 1, true, nil); err != nil {
		t.Fatal(err)
	}
	items, _ = db.GetItems()
	if items[0].Name != "Butter" || items[0].Quantity != 1 || !items[0].Checked {
		t.Fatalf("update failed: %+v", items[0])
	}

	// Delete
	if err := db.DeleteItem(item.ID); err != nil {
		t.Fatal(err)
	}
	items, _ = db.GetItems()
	if len(items) != 0 {
		t.Fatalf("expected 0 items after delete, got %d", len(items))
	}
}

func TestReorder(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	a, _ := db.CreateItem("A", 1, nil)
	b, _ := db.CreateItem("B", 1, nil)
	c, _ := db.CreateItem("C", 1, nil)

	// Reorder: C, A, B
	if err := db.ReorderItems(nil, []int64{c.ID, a.ID, b.ID}); err != nil {
		t.Fatal(err)
	}

	items, err := db.GetItems()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
	if items[0].Name != "C" || items[1].Name != "A" || items[2].Name != "B" {
		t.Fatalf("reorder result wrong: got %s, %s, %s", items[0].Name, items[1].Name, items[2].Name)
	}
}

func TestGetItemsEmptyList(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	items, err := db.GetItems()
	if err != nil {
		t.Fatal(err)
	}
	if items != nil {
		t.Fatalf("expected nil slice for empty db, got %v", items)
	}
}

func TestCategoryCRUD(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	cat, err := db.CreateCategory("Obst & Gemüse")
	if err != nil {
		t.Fatal(err)
	}
	if cat.Name != "Obst & Gemüse" || cat.Position != 0 {
		t.Fatalf("unexpected category after create: %+v", cat)
	}

	if err := db.RenameCategory(cat.ID, "Obst"); err != nil {
		t.Fatal(err)
	}
	categories, err := db.GetCategories()
	if err != nil {
		t.Fatal(err)
	}
	if len(categories) != 1 || categories[0].Name != "Obst" {
		t.Fatalf("rename failed: %+v", categories)
	}

	if err := db.DeleteCategory(cat.ID); err != nil {
		t.Fatal(err)
	}
	categories, _ = db.GetCategories()
	if len(categories) != 0 {
		t.Fatalf("expected 0 categories after delete, got %d", len(categories))
	}
}

func TestCategoryReorder(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	a, _ := db.CreateCategory("A")
	b, _ := db.CreateCategory("B")
	c, _ := db.CreateCategory("C")

	if err := db.ReorderCategories([]int64{c.ID, a.ID, b.ID}); err != nil {
		t.Fatal(err)
	}

	categories, err := db.GetCategories()
	if err != nil {
		t.Fatal(err)
	}
	if len(categories) != 3 || categories[0].Name != "C" || categories[1].Name != "A" || categories[2].Name != "B" {
		t.Fatalf("category reorder result wrong: %+v", categories)
	}
}

func TestGetCategoriesEmptyList(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	categories, err := db.GetCategories()
	if err != nil {
		t.Fatal(err)
	}
	if categories != nil {
		t.Fatalf("expected nil slice for empty db, got %v", categories)
	}
}

func TestCreateItemScopedToCategory(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	cat, _ := db.CreateCategory("Milchprodukte")
	inCat, err := db.CreateItem("Milch", 1, ptr(cat.ID))
	if err != nil {
		t.Fatal(err)
	}
	uncategorized, err := db.CreateItem("Batterien", 1, nil)
	if err != nil {
		t.Fatal(err)
	}

	items, err := db.GetItems()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	// Categorized items sort before uncategorized ones.
	if items[0].ID != inCat.ID || items[0].CategoryID == nil || *items[0].CategoryID != cat.ID {
		t.Fatalf("expected categorized item first: %+v", items[0])
	}
	if items[1].ID != uncategorized.ID || items[1].CategoryID != nil {
		t.Fatalf("expected uncategorized item last: %+v", items[1])
	}
}

func TestCreateItemAtScopedToCategory(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	cat, _ := db.CreateCategory("Getränke")
	first, err := db.CreateItem("Wasser", 1, ptr(cat.ID))
	if err != nil {
		t.Fatal(err)
	}

	// afterID == 0: insert at the front of the category.
	front, err := db.CreateItemAt(ptr(cat.ID), 0, "Saft", 1)
	if err != nil {
		t.Fatal(err)
	}
	if front.CategoryID == nil || *front.CategoryID != cat.ID {
		t.Fatalf("expected front insert scoped to category: %+v", front)
	}

	// afterID != 0: scope derives from the referenced item's own category,
	// regardless of the categoryID argument passed in.
	after, err := db.CreateItemAt(nil, first.ID, "Cola", 1)
	if err != nil {
		t.Fatal(err)
	}
	if after.CategoryID == nil || *after.CategoryID != cat.ID {
		t.Fatalf("expected scope derived from afterID's category: %+v", after)
	}

	items, err := db.GetItems()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
	if items[0].Name != "Saft" || items[1].Name != "Wasser" || items[2].Name != "Cola" {
		t.Fatalf("unexpected order: %s, %s, %s", items[0].Name, items[1].Name, items[2].Name)
	}
}

func TestReorderItemsWithinCategory(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	cat, _ := db.CreateCategory("Brot & Backwaren")
	a, _ := db.CreateItem("A", 1, ptr(cat.ID))
	b, _ := db.CreateItem("B", 1, ptr(cat.ID))
	c, _ := db.CreateItem("C", 1, ptr(cat.ID))

	if err := db.ReorderItems(ptr(cat.ID), []int64{c.ID, a.ID, b.ID}); err != nil {
		t.Fatal(err)
	}

	items, err := db.GetItems()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 || items[0].Name != "C" || items[1].Name != "A" || items[2].Name != "B" {
		t.Fatalf("unexpected order: %+v", items)
	}
}

func TestMoveItemBetweenCategories(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	catA, _ := db.CreateCategory("A")
	catB, _ := db.CreateCategory("B")
	item, _ := db.CreateItem("Item", 1, ptr(catA.ID))
	bItem, _ := db.CreateItem("BItem", 1, ptr(catB.ID))

	// Move `item` from catA into catB, landing after bItem.
	if err := db.ReorderItems(ptr(catB.ID), []int64{bItem.ID, item.ID}); err != nil {
		t.Fatal(err)
	}

	items, err := db.GetItems()
	if err != nil {
		t.Fatal(err)
	}
	var moved *Item
	for i := range items {
		if items[i].ID == item.ID {
			moved = &items[i]
		}
	}
	if moved == nil || moved.CategoryID == nil || *moved.CategoryID != catB.ID {
		t.Fatalf("expected item moved to catB: %+v", moved)
	}

	// catA has no items left.
	catAItems := 0
	for _, it := range items {
		if it.CategoryID != nil && *it.CategoryID == catA.ID {
			catAItems++
		}
	}
	if catAItems != 0 {
		t.Fatalf("expected catA to have no items left, got %d", catAItems)
	}
}

func TestDeleteCategoryKeepsItemsUncategorized(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	cat, _ := db.CreateCategory("Tiefkühl")
	item, err := db.CreateItem("Pizza", 1, ptr(cat.ID))
	if err != nil {
		t.Fatal(err)
	}

	if err := db.DeleteCategory(cat.ID); err != nil {
		t.Fatal(err)
	}

	items, err := db.GetItems()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != item.ID {
		t.Fatalf("expected item to survive category deletion: %+v", items)
	}
	if items[0].CategoryID != nil {
		t.Fatalf("expected item to become uncategorized, got category_id=%v", *items[0].CategoryID)
	}
}

func TestReorderItemsRejectsUnknownCategory(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	item, _ := db.CreateItem("Item", 1, nil)
	bogusID := int64(999999)

	err = db.ReorderItems(&bogusID, []int64{item.ID})
	if !errors.Is(err, ErrCategoryNotFound) {
		t.Fatalf("expected ErrCategoryNotFound, got %v", err)
	}
}

func TestMigrationAddsCategoryColumnToExistingDB(t *testing.T) {
	path := filepath.Join(t.TempDir(), "existing.db")

	// Simulate a pre-categories database: create it with the old schema
	// and one row, using a raw connection (bypassing our Open/migrate).
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`CREATE TABLE items (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		name       TEXT    NOT NULL,
		quantity   INTEGER NOT NULL DEFAULT 1,
		checked    INTEGER NOT NULL DEFAULT 0,
		position   INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`INSERT INTO items (name, quantity, position) VALUES ('Alt-Artikel', 1, 0)`); err != nil {
		t.Fatal(err)
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}

	// Now open it through the real migration path.
	db, err := Open(path)
	if err != nil {
		t.Fatalf("migration of existing db failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	items, err := db.GetItems()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Name != "Alt-Artikel" || items[0].CategoryID != nil {
		t.Fatalf("expected pre-existing item unchanged and uncategorized: %+v", items)
	}

	cat, err := db.CreateCategory("Neu")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.CreateItem("Neuer Artikel", 1, ptr(cat.ID)); err != nil {
		t.Fatalf("expected category_id usable on migrated db: %v", err)
	}
}
