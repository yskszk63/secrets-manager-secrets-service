package smss

import (
	//"maps"
	"database/sql"
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

	if col.id == nil {
		t.Fatal()
	}

	c, err := findCollection(tx, *col.id)
	if err != nil {
		t.Fatal(err)
	}

	if *c.id != *col.id {
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

	if err := deleteCollection(tx, *col.id); err != nil {
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

	if *c1.id != 2 {
		t.Fatal()
	}
	if *c2.id != 3 {
		t.Fatal()
	}

	wants := map[int64]any{
		1: struct{}{},
		2: struct{}{},
		3: struct{}{},
	}
	for c, err := range listCollections(tx) {
		if err != nil {
			t.Fatal(err)
		}

		_, ok := wants[*c.id]
		if !ok {
			t.Fatal()
		}
		delete(wants, *c.id)
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

	attr := map[string]string{
		"a": "1",
	}
	item := dbItem{collectionId: new(int64(1)), secret: []byte("secret"), label: new("ok"), attributes: attr}
	if err := insertItem(tx, &item); err != nil {
		t.Fatal(err)
	}

	if item.id == nil {
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

	if err := deleteItem(tx, *item.id); err != nil {
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
		collectionId: new(int64(1)),
		secret:       []byte("secret1"),
		label:        new("item1"),
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
		collectionId: new(int64(1)),
		secret:       []byte("secret2"),
		label:        new("item2"),
		attributes: map[string]string{
			"a": "1",
			"b": "2",
		},
	}
	if err := insertItem(tx, &item2); err != nil {
		t.Fatal(err)
	}

	item3 := dbItem{
		collectionId: new(int64(1)),
		secret:       []byte("secret3"),
		label:        new("item3"),
		attributes: map[string]string{
			"a": "2",
		},
	}
	if err := insertItem(tx, &item3); err != nil {
		t.Fatal(err)
	}

	attr := map[string]string{
		"a": "1",
		"b": "2",
	}
	var i int
	for item, err := range searchItems(tx, attr) {
		if err != nil {
			t.Fatal(err)
		}

		switch i {
		case 0:
			if *item1.id != *item.id {
				t.Fatal()
			}
		case 1:
			if *item2.id != *item.id {
				t.Fatal()
			}
		default:
			t.Fatal()
		}
		i++
	}
	if i != 2 {
		t.Fatalf("%d", i)
	}

	i = 0
	for item, err := range searchItems(tx, map[string]string{}) {
		if err != nil {
			t.Fatal(err)
		}

		switch i {
		case 0:
			if *item1.id != *item.id {
				t.Fatal()
			}
		case 1:
			if *item2.id != *item.id {
				t.Fatal()
			}
		case 2:
			if *item3.id != *item.id {
				t.Fatal()
			}
		default:
			t.Fatal()
		}
		i++
	}
	if i != 3 {
		t.Fatalf("%d", i)
	}
}
