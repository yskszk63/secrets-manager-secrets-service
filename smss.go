package smss

import (
	"fmt"

	"github.com/godbus/dbus/v5"
)

func loadAndExport(env *Env) ([]*collection, error) {
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

	collections := []*collection{}
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

func Start(env *Env) error {
	collections, err := loadAndExport(env)
	if err != nil {
		return err
	}

	service := newSecretService(env, collections)

	if err = service.export(); err != nil {
		return err
	}

	var defaultCollection *collection
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

	err = env.conn.Export(defaultCollection, "/org/freedesktop/secrets/aliases/default", "org.freedesktop.Secret.Collection")
	if err != nil {
		return err
	}

	reply, err := env.conn.RequestName("org.freedesktop.secrets", dbus.NameFlagDoNotQueue)
	if err != nil {
		return err
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		return fmt.Errorf("name already owned")
	}

	return nil
}
