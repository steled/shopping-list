package database

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// DB wraps an SQLite connection.
type DB struct {
	db *sql.DB
}

// Item represents a shopping list entry.
type Item struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Quantity int    `json:"quantity"`
	Checked  bool   `json:"checked"`
	Position int    `json:"position"`
}

// Open opens (or creates) the SQLite database at the given path.
// Use ":memory:" for an in-memory database (e.g. in tests).
func Open(path string) (*DB, error) {
	db, err := sql.Open("sqlite", path)
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
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS items (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		name       TEXT    NOT NULL,
		quantity   INTEGER NOT NULL DEFAULT 1,
		checked    INTEGER NOT NULL DEFAULT 0,
		position   INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)
	return err
}

// GetItems returns all items ordered by position, then id.
func (d *DB) GetItems() ([]Item, error) {
	rows, err := d.db.Query(
		`SELECT id, name, quantity, checked, position FROM items ORDER BY position ASC, id ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []Item
	for rows.Next() {
		var it Item
		var checked int
		if err := rows.Scan(&it.ID, &it.Name, &it.Quantity, &checked, &it.Position); err != nil {
			return nil, err
		}
		it.Checked = checked == 1
		items = append(items, it)
	}
	return items, rows.Err()
}

// CreateItem inserts a new item and appends it at the end of the list.
func (d *DB) CreateItem(name string, quantity int) (Item, error) {
	var maxPos int
	_ = d.db.QueryRow(`SELECT COALESCE(MAX(position), -1) FROM items`).Scan(&maxPos)

	res, err := d.db.Exec(
		`INSERT INTO items (name, quantity, position) VALUES (?, ?, ?)`,
		name, quantity, maxPos+1,
	)
	if err != nil {
		return Item{}, err
	}
	id, _ := res.LastInsertId()
	return Item{ID: id, Name: name, Quantity: quantity, Position: maxPos + 1}, nil
}

// UpdateItem updates name, quantity and checked state of an item.
func (d *DB) UpdateItem(id int64, name string, quantity int, checked bool) error {
	checkedInt := 0
	if checked {
		checkedInt = 1
	}
	_, err := d.db.Exec(
		`UPDATE items SET name=?, quantity=?, checked=? WHERE id=?`,
		name, quantity, checkedInt, id,
	)
	return err
}

// DeleteItem removes an item by id.
func (d *DB) DeleteItem(id int64) error {
	_, err := d.db.Exec(`DELETE FROM items WHERE id=?`, id)
	return err
}

// ReorderItems assigns new positions according to the provided id order.
func (d *DB) ReorderItems(ids []int64) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	for i, id := range ids {
		if _, err := tx.Exec(`UPDATE items SET position=? WHERE id=?`, i, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}
