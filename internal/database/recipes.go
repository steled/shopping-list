package database

import (
	"database/sql"
	"errors"
	"strings"
)

// ErrNotFound is returned when the referenced row does not exist.
var ErrNotFound = errors.New("not found")

// Ingredient is one line of a recipe. Quantity is what lands on the shopping
// list; Amount is an optional free-text hint such as "500 g".
type Ingredient struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Quantity   int    `json:"quantity"`
	Amount     string `json:"amount"`
	CategoryID *int64 `json:"category_id"`
	Pantry     bool   `json:"pantry"`
}

// Recipe is a dish that can be planned for a weekend day.
type Recipe struct {
	ID          int64        `json:"id"`
	Name        string       `json:"name"`
	Active      bool         `json:"active"`
	Ingredients []Ingredient `json:"ingredients"`
}

// PlanSlot is the recipe planned for one weekend day. RecipeID is nil when
// nothing is planned (e.g. no active recipes, or the recipe was deleted).
type PlanSlot struct {
	Date        string `json:"date"`
	RecipeID    *int64 `json:"recipe_id"`
	Transferred bool   `json:"transferred"`
}

// ItemSource links a shopping-list item to the planned dish it came from.
type ItemSource struct {
	Date       string
	RecipeID   int64
	RecipeName string
	Amount     string
}

// TransferEntry lists which ingredients of the dish planned on Date should be
// added to the shopping list.
type TransferEntry struct {
	Date          string
	IngredientIDs []int64
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// GetRecipes returns all recipes with their ingredients, ordered by name.
func (d *DB) GetRecipes() ([]Recipe, error) {
	rows, err := d.db.Query(`SELECT id, name, active FROM recipes ORDER BY name COLLATE NOCASE, id`)
	if err != nil {
		return nil, err
	}
	recipes := []Recipe{}
	index := map[int64]int{}
	for rows.Next() {
		var r Recipe
		var active int
		if err := rows.Scan(&r.ID, &r.Name, &active); err != nil {
			_ = rows.Close()
			return nil, err
		}
		r.Active = active == 1
		r.Ingredients = []Ingredient{}
		index[r.ID] = len(recipes)
		recipes = append(recipes, r)
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	irows, err := d.db.Query(`
		SELECT id, recipe_id, name, quantity, amount, category_id, pantry
		FROM recipe_ingredients ORDER BY recipe_id, position, id`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = irows.Close() }()
	for irows.Next() {
		var ing Ingredient
		var recipeID int64
		var cat sql.NullInt64
		var pantry int
		if err := irows.Scan(&ing.ID, &recipeID, &ing.Name, &ing.Quantity, &ing.Amount, &cat, &pantry); err != nil {
			return nil, err
		}
		if cat.Valid {
			id := cat.Int64
			ing.CategoryID = &id
		}
		ing.Pantry = pantry == 1
		if i, ok := index[recipeID]; ok {
			recipes[i].Ingredients = append(recipes[i].Ingredients, ing)
		}
	}
	return recipes, irows.Err()
}

// GetRecipe returns a single recipe with its ingredients.
func (d *DB) GetRecipe(id int64) (Recipe, error) {
	recipes, err := d.GetRecipes()
	if err != nil {
		return Recipe{}, err
	}
	for _, r := range recipes {
		if r.ID == id {
			return r, nil
		}
	}
	return Recipe{}, ErrNotFound
}

// ActiveRecipeIDs returns the ids of all recipes that take part in the rotation.
func (d *DB) ActiveRecipeIDs() ([]int64, error) {
	rows, err := d.db.Query(`SELECT id FROM recipes WHERE active = 1 ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// CreateRecipe inserts a new, active recipe without ingredients.
func (d *DB) CreateRecipe(name string) (Recipe, error) {
	res, err := d.db.Exec(`INSERT INTO recipes (name) VALUES (?)`, name)
	if err != nil {
		return Recipe{}, err
	}
	id, _ := res.LastInsertId()
	return Recipe{ID: id, Name: name, Active: true, Ingredients: []Ingredient{}}, nil
}

// UpdateRecipe replaces a recipe's name, active flag and full ingredient list.
func (d *DB) UpdateRecipe(id int64, name string, active bool, ingredients []Ingredient) (Recipe, error) {
	tx, err := d.db.Begin()
	if err != nil {
		return Recipe{}, err
	}
	defer tx.Rollback() //nolint:errcheck

	res, err := tx.Exec(`UPDATE recipes SET name=?, active=? WHERE id=?`, name, boolInt(active), id)
	if err != nil {
		return Recipe{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Recipe{}, ErrNotFound
	}
	if _, err := tx.Exec(`DELETE FROM recipe_ingredients WHERE recipe_id=?`, id); err != nil {
		return Recipe{}, err
	}
	for i, ing := range ingredients {
		if _, err := tx.Exec(
			`INSERT INTO recipe_ingredients (recipe_id, name, quantity, amount, category_id, pantry, position)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			id, ing.Name, ing.Quantity, ing.Amount, ing.CategoryID, boolInt(ing.Pantry), i,
		); err != nil {
			if isFKViolation(err) {
				return Recipe{}, ErrCategoryNotFound
			}
			return Recipe{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Recipe{}, err
	}
	return d.GetRecipe(id)
}

// DeleteRecipe removes a recipe. Plan slots that used it become empty; items
// on the shopping list stay, only their link to the recipe is removed.
func (d *DB) DeleteRecipe(id int64) error {
	res, err := d.db.Exec(`DELETE FROM recipes WHERE id=?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// GetPlanRange returns stored plan slots with from <= date <= to, in date order.
func (d *DB) GetPlanRange(from, to string) ([]PlanSlot, error) {
	rows, err := d.db.Query(
		`SELECT date, recipe_id, transferred FROM meal_plan WHERE date >= ? AND date <= ? ORDER BY date`,
		from, to,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var slots []PlanSlot
	for rows.Next() {
		var s PlanSlot
		var rid sql.NullInt64
		var tr int
		if err := rows.Scan(&s.Date, &rid, &tr); err != nil {
			return nil, err
		}
		if rid.Valid {
			id := rid.Int64
			s.RecipeID = &id
		}
		s.Transferred = tr == 1
		slots = append(slots, s)
	}
	return slots, rows.Err()
}

// GetPlanSlot returns the stored slot for date.
func (d *DB) GetPlanSlot(date string) (PlanSlot, error) {
	slots, err := d.GetPlanRange(date, date)
	if err != nil {
		return PlanSlot{}, err
	}
	if len(slots) == 0 {
		return PlanSlot{}, ErrNotFound
	}
	return slots[0], nil
}

// SavePlanSlots inserts slots that don't exist yet and updates the recipe of
// existing ones. Changing the recipe resets the transferred flag.
func (d *DB) SavePlanSlots(slots []PlanSlot) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	for _, s := range slots {
		if _, err := tx.Exec(`
			INSERT INTO meal_plan (date, recipe_id, transferred) VALUES (?, ?, 0)
			ON CONFLICT(date) DO UPDATE SET
				transferred = CASE WHEN recipe_id IS excluded.recipe_id THEN transferred ELSE 0 END,
				recipe_id   = excluded.recipe_id`,
			s.Date, s.RecipeID,
		); err != nil {
			if isFKViolation(err) {
				return ErrNotFound
			}
			return err
		}
	}
	return tx.Commit()
}

// Item links of a date are selected either for the recipe currently planned
// there, or for every other recipe — links left over after the planned dish
// was changed ("stale").
const (
	sqlLinksRecipe = `plan_date = ? AND recipe_id = ?`
	sqlLinksOthers = `plan_date = ? AND recipe_id != ?`
)

func linkFilter(stale bool) string {
	if stale {
		return sqlLinksOthers
	}
	return sqlLinksRecipe
}

// CountPlanItems returns how many open (unchecked) shopping-list items are
// linked to date — for recipeID, or with stale set for any other recipe.
func (d *DB) CountPlanItems(date string, recipeID int64, stale bool) (int, error) {
	var n int
	err := d.db.QueryRow(`
		SELECT COUNT(*) FROM items WHERE checked = 0
		AND id IN (SELECT item_id FROM item_sources WHERE `+linkFilter(stale)+`)`,
		date, recipeID).Scan(&n)
	return n, err
}

// KeepStalePlanItems keeps the items left over after the dish on date was
// changed, but drops their link to the old dish.
func (d *DB) KeepStalePlanItems(date string, recipeID int64) error {
	_, err := d.db.Exec(`DELETE FROM item_sources WHERE `+sqlLinksOthers, date, recipeID)
	return err
}

// removePlanItems drops item links for date (of recipeID, or of every other
// recipe when others is set) and deletes items that were created by a
// transfer, are still unchecked and have no other link left.
func removePlanItems(tx *sql.Tx, date string, recipeID int64, others bool) (int, error) {
	where := linkFilter(others)
	rows, err := tx.Query(`SELECT DISTINCT item_id FROM item_sources WHERE `+where, date, recipeID)
	if err != nil {
		return 0, err
	}
	var itemIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return 0, err
		}
		itemIDs = append(itemIDs, id)
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	if _, err := tx.Exec(`DELETE FROM item_sources WHERE `+where, date, recipeID); err != nil {
		return 0, err
	}
	removed := 0
	for _, id := range itemIDs {
		res, err := tx.Exec(`
			DELETE FROM items WHERE id = ? AND from_recipe = 1 AND checked = 0
			AND NOT EXISTS (SELECT 1 FROM item_sources WHERE item_id = ?)`, id, id)
		if err != nil {
			return 0, err
		}
		n, _ := res.RowsAffected()
		removed += int(n)
	}
	if !others {
		if _, err := tx.Exec(`UPDATE meal_plan SET transferred = 0 WHERE date = ?`, date); err != nil {
			return 0, err
		}
	}
	return removed, nil
}

// RemovePlanItems undoes the ingredient transfer of the dish currently planned
// on date, or — with stale set — removes the items left over from a dish that
// was planned there before (see removePlanItems).
func (d *DB) RemovePlanItems(date string, stale bool) (int, error) {
	tx, err := d.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback() //nolint:errcheck
	var rid sql.NullInt64
	if err := tx.QueryRow(`SELECT recipe_id FROM meal_plan WHERE date = ?`, date).Scan(&rid); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrNotFound
		}
		return 0, err
	}
	// A deleted recipe leaves recipe_id NULL; 0 then matches no link (undo)
	// or every link (stale).
	n, err := removePlanItems(tx, date, rid.Int64, stale)
	if err != nil {
		return 0, err
	}
	return n, tx.Commit()
}

// TransferIngredients adds the chosen ingredients of each entry's planned
// dish to the shopping list. An open item with the same name (ignoring case)
// is reused instead of creating a duplicate: it keeps the larger quantity,
// because "potatoes" in two dishes still means "buy potatoes". Repeating a
// transfer for the same date only adds what is missing. Returns the number
// of list entries created or updated.
func (d *DB) TransferIngredients(entries []TransferEntry) (int, error) {
	tx, err := d.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback() //nolint:errcheck

	touched := map[int64]bool{}
	for _, e := range entries {
		var rid sql.NullInt64
		if err := tx.QueryRow(`SELECT recipe_id FROM meal_plan WHERE date = ?`, e.Date).Scan(&rid); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return 0, ErrNotFound
			}
			return 0, err
		}
		if !rid.Valid {
			return 0, ErrNotFound
		}

		for _, ingID := range e.IngredientIDs {
			var name, amount string
			var qty int
			var cat sql.NullInt64
			err := tx.QueryRow(
				`SELECT name, quantity, amount, category_id FROM recipe_ingredients WHERE id = ? AND recipe_id = ?`,
				ingID, rid.Int64,
			).Scan(&name, &qty, &amount, &cat)
			if errors.Is(err, sql.ErrNoRows) {
				return 0, ErrNotFound
			}
			if err != nil {
				return 0, err
			}

			var itemID int64
			var itemQty int
			err = tx.QueryRow(
				`SELECT id, quantity FROM items WHERE checked = 0 AND lower(trim(name)) = lower(trim(?)) ORDER BY id LIMIT 1`,
				name,
			).Scan(&itemID, &itemQty)
			switch {
			case errors.Is(err, sql.ErrNoRows):
				var catID *int64
				if cat.Valid {
					catID = &cat.Int64
				}
				itemID, err = insertItemTx(tx, name, qty, catID)
				if err != nil {
					return 0, err
				}
			case err != nil:
				return 0, err
			case qty > itemQty:
				if _, err := tx.Exec(`UPDATE items SET quantity = ? WHERE id = ?`, qty, itemID); err != nil {
					return 0, err
				}
			}

			if _, err := tx.Exec(
				`INSERT OR IGNORE INTO item_sources (item_id, plan_date, recipe_id, amount) VALUES (?, ?, ?, ?)`,
				itemID, e.Date, rid.Int64, amount,
			); err != nil {
				return 0, err
			}
			touched[itemID] = true
		}
		if _, err := tx.Exec(`UPDATE meal_plan SET transferred = 1 WHERE date = ?`, e.Date); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(touched), nil
}

func insertItemTx(tx *sql.Tx, name string, qty int, categoryID *int64) (int64, error) {
	var maxPos int
	var err error
	if categoryID == nil {
		err = tx.QueryRow(`SELECT COALESCE(MAX(position), -1) FROM items WHERE category_id IS NULL`).Scan(&maxPos)
	} else {
		err = tx.QueryRow(`SELECT COALESCE(MAX(position), -1) FROM items WHERE category_id = ?`, *categoryID).Scan(&maxPos)
	}
	if err != nil {
		return 0, err
	}
	res, err := tx.Exec(
		`INSERT INTO items (name, quantity, position, category_id, from_recipe) VALUES (?, ?, ?, ?, 1)`,
		strings.TrimSpace(name), qty, maxPos+1, categoryID,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// GetItemSources returns, per item id, the planned dishes the item was added for.
func (d *DB) GetItemSources() (map[int64][]ItemSource, error) {
	rows, err := d.db.Query(`
		SELECT s.item_id, s.plan_date, s.recipe_id, r.name, s.amount
		FROM item_sources s JOIN recipes r ON r.id = s.recipe_id
		ORDER BY s.plan_date, r.name`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := map[int64][]ItemSource{}
	for rows.Next() {
		var itemID int64
		var s ItemSource
		if err := rows.Scan(&itemID, &s.Date, &s.RecipeID, &s.RecipeName, &s.Amount); err != nil {
			return nil, err
		}
		out[itemID] = append(out[itemID], s)
	}
	return out, rows.Err()
}
