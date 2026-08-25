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

type dbCollectionKey struct {
	id *int64
}

type dbCollection struct {
	dbCollectionKey
	label    *string
	created  *string
	modified *string
}

type dbItemKey struct {
	id           *int64
	collectionId *int64
}

type dbItem struct {
	dbItemKey
	secret     []byte
	label      *string
	created    *string
	modified   *string
	attributes map[string]string
}

func insertCollection(tx *sql.Tx, collection *dbCollection) error {
	q := "INSERT INTO collection (label) VALUES (?) RETURNING id"

	if err := tx.QueryRow(q, collection.label).Scan(&collection.id); err != nil {
		return err
	}

	return nil
}

func deleteCollection(tx *sql.Tx, id int64) error {
	q := "DELETE FROM collection WHERE id = ?"

	if _, err := tx.Exec(q, id); err != nil {
		return err
	}

	q2 := "DELETE FROM item_attr WHERE item_id IN (SELECT id FROM item where collection_id = ?)"

	if _, err := tx.Exec(q2, id); err != nil {
		return err
	}

	q3 := "DELETE FROM item WHERE collection_id = ?"

	if _, err := tx.Exec(q3, id); err != nil {
		return err
	}

	return nil
}

func findCollection(tx *sql.Tx, id int64) (*dbCollection, error) {
	q := "SELECT id, label, created, modified FROM collection WHERE id = ?"

	var r dbCollection
	if err := tx.QueryRow(q, id).Scan(&r.id, &r.label, &r.created, &r.modified); err != nil {
		return nil, err
	}

	return &r, nil
}

func listCollections(tx *sql.Tx) iter.Seq2[*dbCollectionKey, error] {
	return func(yield func(*dbCollectionKey, error) bool) {
		q := "SELECT id FROM collection ORDER BY id ASC"

		rows, err := tx.Query(q)
		if err != nil {
			yield(nil, err)
			return
		}
		defer rows.Close()

		for rows.Next() {
			var r dbCollectionKey
			if err := rows.Scan(&r.id); err != nil {
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

	q := "INSERT INTO item(collection_id, secret, label) VALUES (?, ?, ?) RETURNING id, created, modified"

	if err := tx.QueryRow(q, item.collectionId, item.secret, item.label).Scan(&item.id, &item.created, &item.modified); err != nil {
		return err
	}

	if err := insertItemAttr(tx, *item.id, item.attributes); err != nil {
		return err
	}

	return nil
}

func updateItem(tx *sql.Tx, item *dbItem) error {
	if len(item.attributes) < 1 {
		return fmt.Errorf("must len(attributes) > 0")
	}

	q := "UPDATE item SET secret = ?, label = ? WHERE id = ? RETURNING modified"

	if err := tx.QueryRow(q, item.secret, item.label, item.id).Scan(&item.modified); err != nil {
		return err
	}

	q2 := "DELETE FROM item_attr WHERE item_id = ?"

	if _, err := tx.Exec(q2, *item.id); err != nil {
		return err
	}

	if err := insertItemAttr(tx, *item.id, item.attributes); err != nil {
		return err
	}

	return nil
}

func deleteItem(tx *sql.Tx, id int64) error {
	q := "DELETE FROM item WHERE id = ?"

	if _, err := tx.Exec(q, id); err != nil {
		return err
	}

	q2 := "DELETE FROM item_attr WHERE item_id = ?"

	if _, err := tx.Exec(q2, id); err != nil {
		return err
	}

	return nil
}

func insertItemAttr(tx *sql.Tx, itemId int64, attributes map[string]string) error {
	if len(attributes) < 1 {
		return fmt.Errorf("must len(attributes) > 0")
	}

	keys := make([]string, 0, len(attributes))
	for k := range attributes {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	q := "INSERT INTO item_attr(item_id, key, value) VALUES (?, ?, ?)"
	stmt, err := tx.Prepare(q)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, key := range keys {
		val := attributes[key]
		if _, err := stmt.Exec(itemId, key, val); err != nil {
			return err
		}
	}

	return nil
}

func getItem(tx *sql.Tx, id int64) (*dbItem, error) {
	q := "SELECT id, collection_id, secret, label, created, modified, (SELECT json_group_object(a.key, a.value) FROM item_attr a WHERE a.item_id = i.id) FROM item i WHERE i.id = ?"

	var item dbItem
	var b []byte

	if err := tx.QueryRow(q, id).Scan(&item.id, &item.collectionId, &item.secret, &item.label, &item.created, &item.modified, &b); err != nil {
		return nil, err
	}

	if err := json.Unmarshal(b, &item.attributes); err != nil {
		return nil, err
	}

	return &item, nil
}

func searchItems(tx *sql.Tx, attributes map[string]string) iter.Seq2[*dbItemKey, error] {
	return func(yield func(*dbItemKey, error) bool) {
		q := "SELECT id, collection_id FROM item i"

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
			var r dbItemKey

			if err := rows.Scan(&r.id, &r.collectionId); err != nil {
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
