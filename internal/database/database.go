package database

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	sqlite "modernc.org/sqlite"
)

// DB wraps an SQLite connection.
type DB struct {
	db *sql.DB
}

// ErrCategoryNotFound is returned when an operation references a category_id
// that does not exist (surfaced by the FK constraint on items.category_id).
var ErrCategoryNotFound = errors.New("category not found")

// Item represents a shopping list entry.
type Item struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Quantity   int    `json:"quantity"`
	Checked    bool   `json:"checked"`
	Position   int    `json:"position"`
	CategoryID *int64 `json:"category_id"`
}

// Category represents a shopping-list department/section that items can be
// grouped into.
type Category struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	ColorIdx int    `json:"color_idx"`
	Position int    `json:"position"`
}

// categoryColorCount is the number of distinct accent colors category_idx
// cycles through (see static/style.css's --cat-0..--cat-N tokens).
const categoryColorCount = 6

// Open opens (or creates) the SQLite database at the given path.
// Use ":memory:" for an in-memory database (e.g. in tests).
func Open(path string) (*DB, error) {
	dsn := path
	if strings.Contains(dsn, "?") {
		dsn += "&_pragma=foreign_keys(1)"
	} else {
		dsn += "?_pragma=foreign_keys(1)"
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	// SQLite supports only one writer at a time.
	db.SetMaxOpenConns(1)
	if err := migrate(db); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return &DB{db: db}, nil
}

// Close closes the underlying database connection.
func (d *DB) Close() error {
	return d.db.Close()
}

func migrate(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS items (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		name       TEXT    NOT NULL,
		quantity   INTEGER NOT NULL DEFAULT 1,
		checked    INTEGER NOT NULL DEFAULT 0,
		position   INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		return err
	}

	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS categories (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		name       TEXT    NOT NULL,
		color_idx  INTEGER NOT NULL DEFAULT 0,
		position   INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		return err
	}

	hasCategoryID, err := columnExists(db, "items", "category_id")
	if err != nil {
		return err
	}
	if !hasCategoryID {
		if _, err := db.Exec(
			`ALTER TABLE items ADD COLUMN category_id INTEGER REFERENCES categories(id) ON DELETE SET NULL`,
		); err != nil {
			return err
		}
	}
	return nil
}

// columnExists reports whether the given table has a column with the given
// name. Used to guard schema migrations, since SQLite has no
// "ADD COLUMN IF NOT EXISTS".
func columnExists(db *sql.DB, table, column string) (bool, error) {
	var exists bool
	row := db.QueryRow(
		`SELECT EXISTS (SELECT 1 FROM pragma_table_info(?) WHERE name = ?)`,
		table, column,
	)
	return exists, row.Scan(&exists)
}

// isFKViolation reports whether err was caused specifically by a foreign key
// constraint violation (e.g. an INSERT/UPDATE referencing a category_id that
// doesn't exist) — not just any constraint violation (NOT NULL/UNIQUE/CHECK
// share the same base SQLITE_CONSTRAINT code, so the FK-specific extended
// code must be checked, not just the low byte).
func isFKViolation(err error) bool {
	var sqliteErr *sqlite.Error
	if !errors.As(err, &sqliteErr) {
		return false
	}
	const sqliteConstraintForeignKey = 787 // SQLITE_CONSTRAINT_FOREIGNKEY, see sqlite.org/rescode.html
	return sqliteErr.Code() == sqliteConstraintForeignKey
}

// GetItems returns all items, categorized items grouped by their category's
// position first (each category's items ordered by their own position),
// followed by uncategorized items last (ordered by their own position).
func (d *DB) GetItems() ([]Item, error) {
	rows, err := d.db.Query(`
		SELECT i.id, i.name, i.quantity, i.checked, i.position, i.category_id
		FROM items i
		LEFT JOIN categories c ON c.id = i.category_id
		ORDER BY (i.category_id IS NULL) ASC, c.position ASC, i.position ASC, i.id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var items []Item
	for rows.Next() {
		var it Item
		var checked int
		var categoryID sql.NullInt64
		if err := rows.Scan(&it.ID, &it.Name, &it.Quantity, &checked, &it.Position, &categoryID); err != nil {
			return nil, err
		}
		it.Checked = checked == 1
		if categoryID.Valid {
			id := categoryID.Int64
			it.CategoryID = &id
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// CreateItem inserts a new item and appends it at the end of categoryID's
// list (categoryID nil = uncategorized).
func (d *DB) CreateItem(name string, quantity int, categoryID *int64) (Item, error) {
	var maxPos int
	if categoryID == nil {
		_ = d.db.QueryRow(`SELECT COALESCE(MAX(position), -1) FROM items WHERE category_id IS NULL`).Scan(&maxPos)
	} else {
		_ = d.db.QueryRow(`SELECT COALESCE(MAX(position), -1) FROM items WHERE category_id = ?`, *categoryID).Scan(&maxPos)
	}

	res, err := d.db.Exec(
		`INSERT INTO items (name, quantity, position, category_id) VALUES (?, ?, ?, ?)`,
		name, quantity, maxPos+1, categoryID,
	)
	if err != nil {
		if isFKViolation(err) {
			return Item{}, ErrCategoryNotFound
		}
		return Item{}, err
	}
	id, _ := res.LastInsertId()
	return Item{ID: id, Name: name, Quantity: quantity, Position: maxPos + 1, CategoryID: categoryID}, nil
}

// CreateItemAt inserts a new item right after the item with afterID, in that
// item's own category. Pass afterID = 0 to insert before the first item of
// categoryID instead (categoryID nil = uncategorized).
func (d *DB) CreateItemAt(categoryID *int64, afterID int64, name string, quantity int) (Item, error) {
	tx, err := d.db.Begin()
	if err != nil {
		return Item{}, err
	}
	defer tx.Rollback() //nolint:errcheck

	var insertPos int
	scope := categoryID
	if afterID == 0 {
		var minPos sql.NullInt64
		if categoryID == nil {
			err = tx.QueryRow(`SELECT MIN(position) FROM items WHERE category_id IS NULL`).Scan(&minPos)
		} else {
			err = tx.QueryRow(`SELECT MIN(position) FROM items WHERE category_id = ?`, *categoryID).Scan(&minPos)
		}
		if err != nil {
			return Item{}, err
		}
		if minPos.Valid {
			insertPos = int(minPos.Int64)
		}
	} else {
		var refCategoryID sql.NullInt64
		if err := tx.QueryRow(`SELECT position, category_id FROM items WHERE id=?`, afterID).
			Scan(&insertPos, &refCategoryID); err != nil {
			return Item{}, fmt.Errorf("item not found: %w", err)
		}
		insertPos++
		if refCategoryID.Valid {
			id := refCategoryID.Int64
			scope = &id
		} else {
			scope = nil
		}
	}

	// Shift all items at or beyond insertPos, within scope, down by one.
	if scope == nil {
		_, err = tx.Exec(`UPDATE items SET position=position+1 WHERE category_id IS NULL AND position>=?`, insertPos)
	} else {
		_, err = tx.Exec(`UPDATE items SET position=position+1 WHERE category_id=? AND position>=?`, *scope, insertPos)
	}
	if err != nil {
		return Item{}, err
	}

	res, err := tx.Exec(
		`INSERT INTO items (name, quantity, position, category_id) VALUES (?, ?, ?, ?)`,
		name, quantity, insertPos, scope,
	)
	if err != nil {
		if isFKViolation(err) {
			return Item{}, ErrCategoryNotFound
		}
		return Item{}, err
	}
	id, _ := res.LastInsertId()
	if err := tx.Commit(); err != nil {
		return Item{}, err
	}
	return Item{ID: id, Name: name, Quantity: quantity, Position: insertPos, CategoryID: scope}, nil
}

// UpdateItem updates name, quantity, checked state and category assignment
// of an item. It does not change position — a category change is expected
// to be followed by a ReorderItems call for the destination category.
func (d *DB) UpdateItem(id int64, name string, quantity int, checked bool, categoryID *int64) error {
	checkedInt := 0
	if checked {
		checkedInt = 1
	}
	_, err := d.db.Exec(
		`UPDATE items SET name=?, quantity=?, checked=?, category_id=? WHERE id=?`,
		name, quantity, checkedInt, categoryID, id,
	)
	if isFKViolation(err) {
		return ErrCategoryNotFound
	}
	return err
}

// DeleteItem removes an item by id.
func (d *DB) DeleteItem(id int64) error {
	_, err := d.db.Exec(`DELETE FROM items WHERE id=?`, id)
	return err
}

// ReorderItems assigns sequential positions (0..n-1) to ids and sets their
// category_id to categoryID (nil = uncategorized). ids must be the full,
// final ordered list of items belonging to categoryID after the operation;
// this both reorders items within one category and moves items into it from
// elsewhere in a single call — items removed from their previous category
// are left with a (harmless) position gap there.
func (d *DB) ReorderItems(categoryID *int64, ids []int64) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	for i, id := range ids {
		if _, err := tx.Exec(`UPDATE items SET position=?, category_id=? WHERE id=?`, i, categoryID, id); err != nil {
			if isFKViolation(err) {
				return ErrCategoryNotFound
			}
			return err
		}
	}
	return tx.Commit()
}

// GetCategories returns all categories ordered by position, then id.
func (d *DB) GetCategories() ([]Category, error) {
	rows, err := d.db.Query(
		`SELECT id, name, color_idx, position FROM categories ORDER BY position ASC, id ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var categories []Category
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Name, &c.ColorIdx, &c.Position); err != nil {
			return nil, err
		}
		categories = append(categories, c)
	}
	return categories, rows.Err()
}

// CreateCategory inserts a new category and appends it at the end of the
// category list. Its accent color cycles through a fixed palette keyed off
// its own (never-reused, AUTOINCREMENT) id, so it stays stable across later
// reordering and doesn't collide with an existing category's color just
// because an earlier one was deleted.
func (d *DB) CreateCategory(name string) (Category, error) {
	var maxPos int
	_ = d.db.QueryRow(`SELECT COALESCE(MAX(position), -1) FROM categories`).Scan(&maxPos)

	res, err := d.db.Exec(
		`INSERT INTO categories (name, color_idx, position) VALUES (?, 0, ?)`,
		name, maxPos+1,
	)
	if err != nil {
		return Category{}, err
	}
	id, _ := res.LastInsertId()
	colorIdx := int(id % categoryColorCount)
	if _, err := d.db.Exec(`UPDATE categories SET color_idx=? WHERE id=?`, colorIdx, id); err != nil {
		return Category{}, err
	}
	return Category{ID: id, Name: name, ColorIdx: colorIdx, Position: maxPos + 1}, nil
}

// RenameCategory updates a category's name.
func (d *DB) RenameCategory(id int64, name string) error {
	_, err := d.db.Exec(`UPDATE categories SET name=? WHERE id=?`, name, id)
	return err
}

// DeleteCategory removes a category by id. Its items are not deleted — the
// category_id foreign key's ON DELETE SET NULL moves them to "uncategorized".
func (d *DB) DeleteCategory(id int64) error {
	_, err := d.db.Exec(`DELETE FROM categories WHERE id=?`, id)
	return err
}

// ReorderCategories assigns new positions according to the provided id order.
func (d *DB) ReorderCategories(ids []int64) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	for i, id := range ids {
		if _, err := tx.Exec(`UPDATE categories SET position=? WHERE id=?`, i, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}
