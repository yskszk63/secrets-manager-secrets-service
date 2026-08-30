package smss

import (
	"fmt"
	"log"
	"strings"

	"github.com/godbus/dbus/v5"
)

type collection struct {
	env  *Env
	path dbus.ObjectPath
}

func newCollection(env *Env, path string) *collection {
	if !strings.HasPrefix(path, "/org/freedesktop/secrets/collection/") {
		panic("Invalid path")
	}
	return &collection{
		env:  env,
		path: dbus.ObjectPath(path),
	}
}

func (c *collection) exportTo(path dbus.ObjectPath) error {
	mappingCollection := map[string]string{
		"Delete":      "Delete",
		"SearchItems": "SearchItems",
		"CreateItem":  "CreateItem",
	}

	if err := c.env.conn.ExportWithMap(c, mappingCollection, path, "org.freedesktop.Secret.Collection"); err != nil {
		return err
	}

	mappingProperties := map[string]string{
		"Get":    "Get",
		"Set":    "Set",
		"GetAll": "GetAll",
	}

	if err := c.env.conn.ExportWithMap(c, mappingProperties, path, "org.freedesktop.DBus.Properties"); err != nil {
		return err
	}

	return nil
}

func (c *collection) export() error {
	return c.exportTo(c.path)
}

// org.freedesktop.Secret.Collection

func (s *collection) Delete() (*dbus.ObjectPath, *dbus.Error) {
	if s.path == "/org/freedesktop/secrets/collection/1" {
		return nil, dbus.MakeFailedError(fmt.Errorf("Could not delete #1"))
	}

	if err := s.env.conn.Export(nil, s.path, "org.freedesktop.Secret.Collection"); err != nil {
		log.Println(err)
		return nil, dbus.MakeFailedError(fmt.Errorf("failure"))
	}

	if err := s.env.conn.Export(nil, s.path, "org.freedesktop.DBus.Properties"); err != nil {
		log.Println(err)
		return nil, dbus.MakeFailedError(fmt.Errorf("failure"))
	}

	tx, err := s.env.db.Begin()
	if err != nil {
		log.Println(err)
		return nil, dbus.MakeFailedError(fmt.Errorf("failure"))
	}
	defer tx.Rollback()

	if err := deleteCollection(tx, string(s.path)); err != nil {
		log.Println(err)
		return nil, dbus.MakeFailedError(fmt.Errorf("failure"))
	}

	if err := tx.Commit(); err != nil {
		log.Println(err)
		return nil, dbus.MakeFailedError(fmt.Errorf("failure"))
	}

	return new(dbus.ObjectPath("/")), nil
}

func (s *collection) SearchItems(attributes map[string]string) ([]dbus.ObjectPath, *dbus.Error) {
	tx, err := s.env.db.Begin()
	if err != nil {
		log.Println(err)
		return nil, dbus.MakeFailedError(fmt.Errorf("failure"))
	}
	defer tx.Rollback()

	prefix := s.path + "/"

	paths := []dbus.ObjectPath{}
	for path, err := range searchItems(tx, attributes) {
		if err != nil {
			log.Println(err)
			return nil, dbus.MakeFailedError(fmt.Errorf("failure"))
		}

		if !strings.HasPrefix(*path, string(prefix)) {
			continue
		}

		paths = append(paths, dbus.ObjectPath(*path))
	}

	return paths, nil
}

func (s *collection) CreateItem(properties map[string]dbus.Variant, secret secret, replace bool) (*dbus.ObjectPath, *dbus.ObjectPath, *dbus.Error) {
	session, unlock, found := s.env.lookupSession(secret.Session)
	if !found {
		return nil, nil, errNoSession
	}
	defer unlock()

	sec, err := session.decrypt(&secret)
	if err != nil {
		return nil, nil, dbus.MakeFailedError(err)
	}

	attrv, ok := properties["org.freedesktop.Secret.Item.Attributes"]
	if !ok || attrv.Signature().String() != "a{ss}" {
		return nil, nil, &dbus.ErrMsgInvalidArg
	}
	attr := attrv.Value().(map[string]string)

	labelv, ok := properties["org.freedesktop.Secret.Item.Label"]
	if !ok || labelv.Signature().String() != "s" {
		return nil, nil, &dbus.ErrMsgInvalidArg
	}
	label := labelv.Value().(string)

	tx, err := s.env.db.Begin()
	if err != nil {
		log.Println(err)
		return nil, nil, dbus.MakeFailedError(fmt.Errorf("failure"))
	}
	defer tx.Rollback()

	dbItem := dbItem{
		collectionPath: (*string)(&s.path),
		// TODO encrypt
		secret:     sec,
		label:      &label,
		attributes: attr,
	}

	if replace {
		for p, err := range searchItems(tx, attr) { // TODO filter collection
			if err != nil {
				log.Println(err)
				return nil, nil, dbus.MakeFailedError(fmt.Errorf("failure"))
			}

			dbItem.path = p
			break
		}
	}

	if dbItem.path == nil {
		if err := insertItem(tx, &dbItem); err != nil {
			log.Println(err)
			return nil, nil, dbus.MakeFailedError(fmt.Errorf("failure"))
		}

		item := newItem(s.env, *dbItem.path)

		if err := item.export(); err != nil {
			log.Println(err)
			return nil, nil, dbus.MakeFailedError(fmt.Errorf("failure"))
		}

	} else {
		if err := updateItem(tx, &dbItem); err != nil {
			log.Println(err)
			return nil, nil, dbus.MakeFailedError(fmt.Errorf("failure"))
		}
	}

	if err := tx.Commit(); err != nil {
		log.Println(err)
		return nil, nil, dbus.MakeFailedError(fmt.Errorf("failure"))
	}

	path := dbus.ObjectPath(*dbItem.path)
	noPrompt := dbus.ObjectPath("/")
	return &path, &noPrompt, nil
}

// org.freedesktop.DBus.Properties

func (c *collection) Get(iface, name string) (*dbus.Variant, *dbus.Error) {
	return nil, dbus.MakeFailedError(fmt.Errorf("Not Implemented"))
}

func (c *collection) Set(iface, name string, value dbus.Variant) *dbus.Error {
	return dbus.MakeFailedError(fmt.Errorf("Not Implemented"))
}

func (c *collection) GetAll(iface string) (map[string]dbus.Variant, *dbus.Error) {
	return nil, dbus.MakeFailedError(fmt.Errorf("Not Implemented"))
}
