package main

import (
	"reflect"

	"github.com/godbus/dbus/v5"
)

func loadAndExport(env *env) ([]*Collection, error) {
	tx, err := env.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	items := map[int64][]dbus.ObjectPath{}
	for item, err := range searchItems(tx, map[string]string{}) {
		if err != nil {
			return nil, err
		}

		item := newItem(env, *item.collectionId, *item.id)

		l, ok := items[item.collectionId]
		if !ok {
			l = []dbus.ObjectPath{}
		}
		items[item.collectionId] = append(l, item.path)

		if err := item.export(); err != nil {
			return nil, err
		}
	}

	collections := []*Collection{}
	for collection, err := range listCollections(tx) {
		if err != nil {
			return nil, err
		}

		collection := newCollection(env, *collection.id)

		if err := collection.export(); err != nil {
			return nil, err
		}
		collections = append(collections, collection)
	}

	return collections, nil
}

func main() {
	val := reflect.ValueOf(&Item{})
	typ := val.Type()
	for i := 0; i < typ.NumMethod(); i++ {
		methtype := typ.Method(i)
		method := val.Method(i)
		t := method.Type()

		// only track valid methods must return *Error as last arg
		// and must be exported
		if t.NumOut() == 0 ||
			t.Out(t.NumOut()-1) != reflect.TypeOf(&dbus.ErrMsgInvalidArg) ||
			methtype.PkgPath != "" {
			continue
		}
	}

	// TODO
	dbpath := "file:./data.db"

	db, err := openDb(dbpath)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	sessions := make(map[dbus.ObjectPath]*Session)

	conn, err := dbus.SessionBus()
	if err != nil {
		panic(err)
	}

	env := &env{
		sessions: sessions,
		db:       db,
		conn:     conn,
	}

	collections, err := loadAndExport(env)
	if err != nil {
		panic(err)
	}

	service := newSecretService(env, collections)

	if err = service.export(); err != nil {
		panic(err)
	}

	var defaultCollection *Collection
	for _, c := range collections {
		if c.id != 1 {
			continue
		}

		defaultCollection = c
		break
	}

	if defaultCollection == nil {
		panic("no default")
	}

	err = conn.Export(defaultCollection, "/org/freedesktop/secrets/aliases/default", "org.freedesktop.Secret.Collection")
	if err != nil {
		panic(err)
	}

	reply, err := conn.RequestName("org.freedesktop.secrets", dbus.NameFlagDoNotQueue)
	if err != nil {
		panic(err)
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		panic("name already owned")
	}

	select {}
}
