package web

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const defaultUserID = "local-default"

func openSQLite(dbPath string) (*sql.DB, error) {
	if dbPath == "" {
		return nil, errors.New("db path is required")
	}

	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	if _, err := db.Exec(`PRAGMA foreign_keys = ON;`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	if err := initSchema(db); err != nil {
		_ = db.Close()
		return nil, err
	}

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	return db, nil
}

func initSchema(db *sql.DB) error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS profiles (
	user_id TEXT PRIMARY KEY,
	hourly_wage TEXT NOT NULL,
	currency TEXT NOT NULL DEFAULT '€',
	default_wait_preset TEXT NOT NULL DEFAULT '24h',
	default_wait_custom_hours TEXT NOT NULL DEFAULT '',
	ntfy_endpoint TEXT NOT NULL DEFAULT '',
	ntfy_topic TEXT NOT NULL DEFAULT '',
	updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS items (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id TEXT NOT NULL,
	title TEXT NOT NULL,
	price TEXT NOT NULL DEFAULT '',
	price_value REAL,
	has_price_value INTEGER NOT NULL DEFAULT 0,
	link TEXT NOT NULL DEFAULT '',
	note TEXT NOT NULL DEFAULT '',
	tags TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL,
	wait_preset TEXT NOT NULL,
	wait_custom_hours TEXT NOT NULL DEFAULT '',
	purchase_allowed_at TEXT NOT NULL,
	created_at TEXT NOT NULL,
	ntfy_attempted INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_items_user_id ON items(user_id);
CREATE INDEX IF NOT EXISTS idx_items_status_allowed ON items(status, purchase_allowed_at);
`)
	if err != nil {
		return fmt.Errorf("init schema: %w", err)
	}

	if _, err := db.Exec(`ALTER TABLE profiles ADD COLUMN default_wait_preset TEXT NOT NULL DEFAULT '24h'`); err != nil && !strings.Contains(err.Error(), "duplicate column name") {
		return fmt.Errorf("migrate profiles.default_wait_preset: %w", err)
	}
	if _, err := db.Exec(`ALTER TABLE profiles ADD COLUMN currency TEXT NOT NULL DEFAULT '€'`); err != nil && !strings.Contains(err.Error(), "duplicate column name") {
		return fmt.Errorf("migrate profiles.currency: %w", err)
	}
	if _, err := db.Exec(`ALTER TABLE profiles ADD COLUMN default_wait_custom_hours TEXT NOT NULL DEFAULT ''`); err != nil && !strings.Contains(err.Error(), "duplicate column name") {
		return fmt.Errorf("migrate profiles.default_wait_custom_hours: %w", err)
	}
	return nil
}

func (a *App) loadStateFromDB(userID string) error {
	if a.db == nil {
		return nil
	}

	a.items = nil
	a.nextID = 1

	row := a.db.QueryRow(`SELECT hourly_wage, currency, default_wait_preset, default_wait_custom_hours, ntfy_endpoint, ntfy_topic FROM profiles WHERE user_id = ?`, userID)
	var hourlyWage, currency, defaultPreset, defaultCustomHours, ntfyEndpoint, ntfyTopic string
	switch err := row.Scan(&hourlyWage, &currency, &defaultPreset, &defaultCustomHours, &ntfyEndpoint, &ntfyTopic); {
	case errors.Is(err, sql.ErrNoRows):
	case err != nil:
		return fmt.Errorf("load profile: %w", err)
	default:
		a.hourlyWage = hourlyWage
		a.currency = normalizeCurrency(currency)
		a.defaultWaitPreset = defaultWaitPreset(defaultPreset)
		if a.defaultWaitPreset == "custom" {
			a.defaultWaitCustomHours = defaultCustomHours
		}
		a.ntfyURL = ntfyEndpoint
		a.ntfyTopic = ntfyTopic
	}

	rows, err := a.db.Query(`
SELECT id, title, price, COALESCE(price_value, 0), has_price_value, link, note, tags, status, wait_preset, wait_custom_hours, purchase_allowed_at, created_at, ntfy_attempted
FROM items
WHERE user_id = ?
ORDER BY id DESC
`, userID)
	if err != nil {
		return fmt.Errorf("load items: %w", err)
	}
	defer rows.Close()

	maxID := 0
	for rows.Next() {
		var item Item
		var purchaseAllowedAtRaw, createdAtRaw string
		var hasPriceValueInt, ntfyAttemptedInt int
		if err := rows.Scan(
			&item.ID,
			&item.Title,
			&item.Price,
			&item.PriceValue,
			&hasPriceValueInt,
			&item.Link,
			&item.Note,
			&item.Tags,
			&item.Status,
			&item.WaitPreset,
			&item.WaitCustomHours,
			&purchaseAllowedAtRaw,
			&createdAtRaw,
			&ntfyAttemptedInt,
		); err != nil {
			return fmt.Errorf("scan item: %w", err)
		}

		purchaseAllowedAt, err := time.Parse(time.RFC3339Nano, purchaseAllowedAtRaw)
		if err != nil {
			return fmt.Errorf("parse purchase_allowed_at: %w", err)
		}
		createdAt, err := time.Parse(time.RFC3339Nano, createdAtRaw)
		if err != nil {
			return fmt.Errorf("parse created_at: %w", err)
		}

		item.HasPriceValue = hasPriceValueInt == 1
		item.NtfyAttempted = ntfyAttemptedInt == 1
		item.PurchaseAllowedAt = purchaseAllowedAt
		item.CreatedAt = createdAt

		a.items = append(a.items, item)
		if item.ID > maxID {
			maxID = item.ID
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate items: %w", err)
	}

	a.nextID = maxID + 1
	return nil
}

func (a *App) persistProfileLocked() error {
	userID := a.currentUserIDLocked()
	if a.db == nil {
		return nil
	}
	_, err := a.db.Exec(`
INSERT INTO profiles(user_id, hourly_wage, currency, default_wait_preset, default_wait_custom_hours, ntfy_endpoint, ntfy_topic, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(user_id) DO UPDATE SET
	hourly_wage = excluded.hourly_wage,
	currency = excluded.currency,
	default_wait_preset = excluded.default_wait_preset,
	default_wait_custom_hours = excluded.default_wait_custom_hours,
	ntfy_endpoint = excluded.ntfy_endpoint,
	ntfy_topic = excluded.ntfy_topic,
	updated_at = excluded.updated_at
`, userID, a.hourlyWage, normalizeCurrency(a.currency), defaultWaitPreset(a.defaultWaitPreset), a.defaultWaitCustomHours, a.ntfyURL, a.ntfyTopic, time.Now().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("persist profile: %w", err)
	}
	return nil
}

func (a *App) insertItemLocked(item *Item) error {
	userID := a.currentUserIDLocked()
	if a.db == nil {
		item.ID = a.nextID
		a.nextID++
		return nil
	}

	res, err := a.db.Exec(`
INSERT INTO items(user_id, title, price, price_value, has_price_value, link, note, tags, status, wait_preset, wait_custom_hours, purchase_allowed_at, created_at, ntfy_attempted)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`,
		userID,
		item.Title,
		item.Price,
		item.PriceValue,
		boolToInt(item.HasPriceValue),
		item.Link,
		item.Note,
		item.Tags,
		item.Status,
		item.WaitPreset,
		item.WaitCustomHours,
		item.PurchaseAllowedAt.Format(time.RFC3339Nano),
		item.CreatedAt.Format(time.RFC3339Nano),
		boolToInt(item.NtfyAttempted),
	)
	if err != nil {
		return fmt.Errorf("insert item: %w", err)
	}

	insertedID, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("read inserted id: %w", err)
	}
	item.ID = int(insertedID)
	if item.ID >= a.nextID {
		a.nextID = item.ID + 1
	}
	return nil
}

func (a *App) updateItemLocked(item Item) error {
	userID := a.currentUserIDLocked()
	if a.db == nil {
		return nil
	}

	_, err := a.db.Exec(`
UPDATE items
SET title = ?, price = ?, price_value = ?, has_price_value = ?, link = ?, note = ?, tags = ?, status = ?, wait_preset = ?, wait_custom_hours = ?, purchase_allowed_at = ?, ntfy_attempted = ?
WHERE id = ? AND user_id = ?
`,
		item.Title,
		item.Price,
		item.PriceValue,
		boolToInt(item.HasPriceValue),
		item.Link,
		item.Note,
		item.Tags,
		item.Status,
		item.WaitPreset,
		item.WaitCustomHours,
		item.PurchaseAllowedAt.Format(time.RFC3339Nano),
		boolToInt(item.NtfyAttempted),
		item.ID,
		userID,
	)
	if err != nil {
		return fmt.Errorf("update item: %w", err)
	}
	return nil
}

func (a *App) deleteItemLocked(itemID int) error {
	userID := a.currentUserIDLocked()
	if a.db == nil {
		return nil
	}

	_, err := a.db.Exec(`DELETE FROM items WHERE id = ? AND user_id = ?`, itemID, userID)
	if err != nil {
		return fmt.Errorf("delete item: %w", err)
	}
	return nil
}

func (a *App) updateItemStatusLocked(itemID int, status string) error {
	userID := a.currentUserIDLocked()
	if a.db == nil {
		return nil
	}

	_, err := a.db.Exec(`UPDATE items SET status = ? WHERE id = ? AND user_id = ?`, status, itemID, userID)
	if err != nil {
		return fmt.Errorf("update item status: %w", err)
	}
	return nil
}

func (a *App) markNtfyAttemptedLocked(itemID int) error {
	userID := a.currentUserIDLocked()
	if a.db == nil {
		return nil
	}

	_, err := a.db.Exec(`UPDATE items SET ntfy_attempted = 1 WHERE id = ? AND user_id = ?`, itemID, userID)
	if err != nil {
		return fmt.Errorf("mark ntfy attempted: %w", err)
	}
	return nil
}

func (a *App) updatePromotedItemLocked(item Item) error {
	userID := a.currentUserIDLocked()
	if a.db == nil {
		return nil
	}

	_, err := a.db.Exec(`UPDATE items SET status = ?, ntfy_attempted = ? WHERE id = ? AND user_id = ?`, item.Status, boolToInt(item.NtfyAttempted), item.ID, userID)
	if err != nil {
		return fmt.Errorf("update promoted item: %w", err)
	}
	return nil
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
