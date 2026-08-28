package smss

import (
	//"maps"
	"database/sql"
	"slices"
	"testing"

	_ "modernc.org/sqlite"
)

func openDb() *sql.DB {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		panic(err)
	}

	if err := Migrate(db); err != nil {
		panic(err)
	}

	return db
}

func TestInsertFindCollection(t *testing.T) {
	db := openDb()
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()

	col := &dbCollection{label: new("test")}
	if err := insertCollection(tx, col); err != nil {
		t.Fatal(err)
	}

	if col.path == nil {
		t.Fatal()
	}

	c, err := findCollection(tx, *col.path)
	if err != nil {
		t.Fatal(err)
	}

	if *c.path != *col.path {
		t.Fatal()
	}
	if *c.label != "test" {
		t.Fatal()
	}
	if c.created == nil {
		t.Fatal()
	}
	if c.modified == nil {
		t.Fatal()
	}

	if err := deleteCollection(tx, *col.path); err != nil {
		t.Fatal(err)
	}
}

func TestListCollections(t *testing.T) {
	db := openDb()
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()

	c1 := &dbCollection{label: new("a")}
	if err := insertCollection(tx, c1); err != nil {
		t.Fatal(err)
	}
	c2 := &dbCollection{label: new("b")}
	if err := insertCollection(tx, c2); err != nil {
		t.Fatal(err)
	}

	if *c1.path != "/org/freedesktop/secrets/collection/2" {
		t.Fatal()
	}
	if *c2.path != "/org/freedesktop/secrets/collection/3" {
		t.Fatal()
	}

	wants := map[string]any{
		"/org/freedesktop/secrets/collection/1": struct{}{},
		"/org/freedesktop/secrets/collection/2": struct{}{},
		"/org/freedesktop/secrets/collection/3": struct{}{},
	}
	for p, err := range listCollections(tx) {
		if err != nil {
			t.Fatal(err)
		}

		_, ok := wants[*p]
		if !ok {
			t.Fatal()
		}
		delete(wants, *p)
	}

	if len(wants) != 0 {
		t.Fatal()
	}
}

func TestItemInsertUpdate(t *testing.T) {
	db := openDb()
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()

	item := dbItem{
		collectionPath: new("/org/freedesktop/secrets/collection/1"),
		secret:         []byte("secret"),
		label:          new("ok"),
		attributes: map[string]string{
			"a": "1",
		},
	}
	if err := insertItem(tx, &item); err != nil {
		t.Fatal(err)
	}

	if item.path == nil {
		t.Fatal()
	}
	if item.created == nil {
		t.Fatal()
	}
	if item.modified == nil {
		t.Fatal()
	}

	if err := updateItem(tx, &item); err != nil {
		t.Fatal(err)
	}

	if err := deleteItem(tx, *item.path); err != nil {
		t.Fatal(err)
	}
}

func TestSearchItems(t *testing.T) {
	db := openDb()
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()

	item1 := dbItem{
		collectionPath: new("/org/freedesktop/secrets/collection/1"),
		secret:         []byte("secret1"),
		label:          new("item1"),
		attributes: map[string]string{
			"a": "1",
			"b": "2",
			"c": "3",
		},
	}
	if err := insertItem(tx, &item1); err != nil {
		t.Fatal(err)
	}

	item2 := dbItem{
		collectionPath: new("/org/freedesktop/secrets/collection/1"),
		secret:         []byte("secret2"),
		label:          new("item2"),
		attributes: map[string]string{
			"a": "1",
			"b": "2",
		},
	}
	if err := insertItem(tx, &item2); err != nil {
		t.Fatal(err)
	}

	item3 := dbItem{
		collectionPath: new("/org/freedesktop/secrets/collection/1"),
		secret:         []byte("secret3"),
		label:          new("item3"),
		attributes: map[string]string{
			"a": "2",
		},
	}
	if err := insertItem(tx, &item3); err != nil {
		t.Fatal(err)
	}

	result := []string{}

	attr := map[string]string{
		"a": "1",
		"b": "2",
	}
	for path, err := range searchItems(tx, attr) {
		if err != nil {
			t.Fatal(err)
		}
		result = append(result, *path)
	}

	if !slices.Equal(result, []string{*item1.path, *item2.path}) {
		t.Fatal()
	}

	result = []string{}
	for path, err := range searchItems(tx, map[string]string{}) {
		if err != nil {
			t.Fatal(err)
		}
		result = append(result, *path)
	}

	if !slices.Equal(result, []string{*item1.path, *item2.path, *item3.path}) {
		t.Fatal()
	}

}
