package smss

import (
	"fmt"

	"github.com/godbus/dbus/v5"
)

func loadAndExport(env *Env) (*collection, error) {
	tx, err := env.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	for path, err := range searchItems(tx, map[string]string{}) {
		if err != nil {
			return nil, err
		}

		item := newItem(env, *path)

		if err := item.export(); err != nil {
			return nil, err
		}
	}

	var defaultCollection *collection
	for path, err := range listCollections(tx) {
		if err != nil {
			return nil, err
		}

		collection := newCollection(env, *path)

		if err := collection.export(); err != nil {
			return nil, err
		}

		if collection.path != "/org/freedesktop/secrets/collection/1" {
			continue
		}
		defaultCollection = collection
	}

	if defaultCollection == nil {
		panic("no default")
	}

	return defaultCollection, nil
}

func Start(env *Env) error {
	defaultCollection, err := loadAndExport(env)
	if err != nil {
		return err
	}

	service := newSecretService(env)

	if err = service.export(); err != nil {
		return err
	}

	if err := defaultCollection.exportTo("/org/freedesktop/secrets/aliases/default"); err != nil {
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
