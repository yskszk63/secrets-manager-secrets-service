package smss

import (
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"iter"
	"path"
	"sort"
	"strconv"

	_ "modernc.org/sqlite"
)

//go:embed db-migrations/*.sql
var migrations embed.FS

func runMigrate(db *sql.DB, name string, version int, current *int) error {
	if version < 0 {
		return fmt.Errorf("invalid version: %d", version)
	}

	if *current >= version {
		return nil
	}

	sql, err := fs.ReadFile(migrations, path.Join("db-migrations", name))
	if err != nil {
		return err
	}

	if _, err := db.Exec(string(sql)); err != nil {
		return err
	}

	if _, err := db.Exec("PRAGMA user_version = " + strconv.Itoa(version)); err != nil {
		return err
	}

	return nil
}

func Migrate(db *sql.DB) error {
	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return err
	}

	if err := runMigrate(db, "001-initial.sql", 1, &version); err != nil {
		return err
	}

	return nil
}

type dbCollection struct {
	path     *string
	label    *string
	created  *string
	modified *string
}

type dbItem struct {
	collectionPath *string
	path           *string
	secret         []byte
	contentType    *string
	label          *string
	created        *string
	modified       *string
	attributes     map[string]string
}

func insertCollection(tx *sql.Tx, collection *dbCollection) error {
	q := "INSERT INTO collection (label) VALUES (?) RETURNING path"

	if err := tx.QueryRow(q, collection.label).Scan(&collection.path); err != nil {
		return err
	}

	return nil
}

func deleteCollection(tx *sql.Tx, path string) error {
	q2 := "DELETE FROM item_attr WHERE item_id IN (SELECT id FROM item where collection_id IN (SELECT id FROM collection WHERE path = ?))"
	if _, err := tx.Exec(q2, path); err != nil {
		return err
	}

	q3 := "DELETE FROM item WHERE collection_id IN (SELECT id FROM collection WHERE path = ?)"
	if _, err := tx.Exec(q3, path); err != nil {
		return err
	}

	q := "DELETE FROM collection WHERE id = path"
	if _, err := tx.Exec(q, path); err != nil {
		return err
	}

	return nil
}

func findCollection(tx *sql.Tx, path string) (*dbCollection, error) {
	q := "SELECT path, label, created, modified FROM collection WHERE path = ?"

	var r dbCollection
	if err := tx.QueryRow(q, path).Scan(&r.path, &r.label, &r.created, &r.modified); err != nil {
		return nil, err
	}

	return &r, nil
}

func listCollections(tx *sql.Tx) iter.Seq2[*string, error] {
	return func(yield func(*string, error) bool) {
		q := "SELECT path FROM collection ORDER BY id ASC"

		rows, err := tx.Query(q)
		if err != nil {
			yield(nil, err)
			return
		}
		defer rows.Close()

		for rows.Next() {
			var r string
			if err := rows.Scan(&r); err != nil {
				yield(nil, err)
				return
			}

			if !yield(&r, nil) {
				return
			}
		}

		if err := rows.Err(); err != nil {
			yield(nil, err)
		}
	}
}

func insertItem(tx *sql.Tx, item *dbItem) error {
	if len(item.attributes) < 1 {
		return fmt.Errorf("must len(attributes) > 0")
	}

	q := "INSERT INTO item(collection_id, secret, content_type, label) VALUES ((SELECT id FROM collection WHERE path = ?), ?, ?, ?) RETURNING path, created, modified"

	if err := tx.QueryRow(q, item.collectionPath, item.secret, item.contentType, item.label).Scan(&item.path, &item.created, &item.modified); err != nil {
		return err
	}

	if err := insertItemAttr(tx, *item.path, item.attributes); err != nil {
		return err
	}

	return nil
}

func updateItemSecret(tx *sql.Tx, path string, secret []byte, contentType string) error {
	q := "UPDATE item SET secret = ?, content_type = ? WHERE path = ?"

	if _, err := tx.Exec(q, secret, contentType, path); err != nil {
		return err
	}

	return nil
}

func updateItem(tx *sql.Tx, item *dbItem) error {
	if len(item.attributes) < 1 {
		return fmt.Errorf("must len(attributes) > 0")
	}

	q := "UPDATE item SET secret = ?, content_type = ?, label = ? WHERE path = ? RETURNING modified"

	if err := tx.QueryRow(q, item.secret, item.contentType, item.label, item.path).Scan(&item.modified); err != nil {
		return err
	}

	q2 := "DELETE FROM item_attr WHERE item_id IN (SELECT id FROM item WHERE path = ?)"

	if _, err := tx.Exec(q2, item.path); err != nil {
		return err
	}

	if err := insertItemAttr(tx, *item.path, item.attributes); err != nil {
		return err
	}

	return nil
}

func deleteItem(tx *sql.Tx, path string) error {
	q2 := "DELETE FROM item_attr WHERE item_id IN (SELECT id FROM item WHERE path = ?)"
	if _, err := tx.Exec(q2, path); err != nil {
		return err
	}

	q := "DELETE FROM item WHERE path = ?"
	if _, err := tx.Exec(q, path); err != nil {
		return err
	}

	return nil
}

func insertItemAttr(tx *sql.Tx, path string, attributes map[string]string) error {
	if len(attributes) < 1 {
		return fmt.Errorf("must len(attributes) > 0")
	}

	keys := make([]string, 0, len(attributes))
	for k := range attributes {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	q := "INSERT INTO item_attr(item_id, key, value) VALUES ((SELECT id FROM item WHERE path = ?), ?, ?)"
	stmt, err := tx.Prepare(q)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, key := range keys {
		val := attributes[key]
		if _, err := stmt.Exec(path, key, val); err != nil {
			return err
		}
	}

	return nil
}

func findItem(tx *sql.Tx, path string) (*dbItem, error) {
	q := "SELECT path, collection_path, secret, content_type, label, created, modified, (SELECT json_group_object(a.key, a.value) FROM item_attr a WHERE a.item_id = i.id) FROM item i WHERE i.path = ?"

	var item dbItem
	var b []byte

	if err := tx.QueryRow(q, path).Scan(&item.path, &item.collectionPath, &item.secret, &item.contentType, &item.label, &item.created, &item.modified, &b); err != nil {
		return nil, err
	}

	if err := json.Unmarshal(b, &item.attributes); err != nil {
		return nil, err
	}

	return &item, nil
}

func searchItems(tx *sql.Tx, attributes map[string]string) iter.Seq2[*string, error] {
	return func(yield func(*string, error) bool) {
		q := "SELECT path FROM item i"

		keys := make([]string, 0, len(attributes))
		for k := range attributes {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		params := make([]any, 0, len(keys)*2)
		for _, key := range keys {
			val := attributes[key]

			if len(params) > 0 {
				q += " AND "
			} else {
				q += " WHERE "
			}
			q += "id IN (SELECT item_id FROM item_attr a WHERE i.id = a.item_id AND a.key = ? AND a.value = ?)"

			params = append(params, key, val)
		}
		params = append(params, " ORDER BY id ASC")

		rows, err := tx.Query(q, params...)
		if err != nil {
			yield(nil, err)
			return
		}
		defer rows.Close()

		for rows.Next() {
			var path string

			if err := rows.Scan(&path); err != nil {
				yield(nil, err)
				return
			}

			if !yield(&path, nil) {
				return
			}
		}
		if err := rows.Err(); err != nil {
			yield(nil, err)
		}
	}
}
