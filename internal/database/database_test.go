package database

import (
	"testing"

	_ "modernc.org/sqlite"
)

func TestCRUD(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	// Create
	item, err := db.CreateItem("Milch", 2)
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
	if err := db.UpdateItem(item.ID, "Butter", 1, true); err != nil {
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

	a, _ := db.CreateItem("A", 1)
	b, _ := db.CreateItem("B", 1)
	c, _ := db.CreateItem("C", 1)

	// Reorder: C, A, B
	if err := db.ReorderItems([]int64{c.ID, a.ID, b.ID}); err != nil {
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
