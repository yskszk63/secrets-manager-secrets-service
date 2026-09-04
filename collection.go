package smss

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/prop"
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

func (c *collection) getProperties(tx *sql.Tx, interests ...string) (map[string]*dbus.Variant, *dbus.Error) {
	var data *dbCollection
	result := make(map[string]*dbus.Variant, len(interests))

	for _, name := range interests {
		switch name {
		case "Items":
			// TODO remove me!
			prefix := c.path + "/"

			items := make([]dbus.ObjectPath, 0)
			for i, err := range searchItems(tx, map[string]string{}) {
				if err != nil {
					log.Println(err)
					return nil, dbus.MakeFailedError(fmt.Errorf("failure"))
				}

				if !strings.HasPrefix(*i, string(prefix)) {
					continue
				}

				items = append(items, dbus.ObjectPath(*i))
			}
			result[name] = new(dbus.MakeVariant(items))

		case "Label":
			if data == nil {
				var err error
				data, err = findCollection(tx, string(c.path))
				if err != nil {
					log.Println(err)
					return nil, dbus.MakeFailedError(fmt.Errorf("failure"))
				}
			}

			result[name] = new(dbus.MakeVariant(data.label))

		case "Locked":
			result[name] = new(dbus.MakeVariant(false))

		case "Created":
			if data == nil {
				var err error
				data, err = findCollection(tx, string(c.path))
				if err != nil {
					log.Println(err)
					return nil, dbus.MakeFailedError(fmt.Errorf("failure"))
				}
			}

			result[name] = new(dbus.MakeVariant(data.created))

		case "Modified":
			if data == nil {
				var err error
				data, err = findCollection(tx, string(c.path))
				if err != nil {
					log.Println(err)
					return nil, dbus.MakeFailedError(fmt.Errorf("failure"))
				}
			}

			result[name] = new(dbus.MakeVariant(data.modified))

		default:
			return nil, prop.ErrPropNotFound
		}
	}

	return result, nil
}

func (c *collection) setProperty(tx *sql.Tx, name string, value dbus.Variant) *dbus.Error {
	switch name {
	case "Items":
		return prop.ErrReadOnly

	case "Label":
		v, ok := value.Value().(string)
		if !ok {
			return &dbus.ErrMsgInvalidArg
		}
		if err := updateCollectionLabel(tx, string(c.path), v); err != nil {
			log.Println(err)
			return dbus.MakeFailedError(fmt.Errorf("failure"))
		}
		return nil

	case "Locked":
		return prop.ErrReadOnly

	case "Created":
		return prop.ErrReadOnly

	case "Modified":
		return prop.ErrReadOnly

	default:
		return prop.ErrPropNotFound
	}
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

	// TODO remove me!
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
		secret:      sec,
		contentType: &secret.ContentType,
		label:       &label,
		attributes:  attr,
		created:     new(s.env.now()),
		modified:    new(s.env.now()),
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
	switch iface {
	case "org.freedesktop.Secret.Collection":
		switch name {
		case "Items":
		case "Label":
		case "Locked":
		case "Created":
		case "Modified":

		default:
			return nil, prop.ErrPropNotFound
		}

	default:
		return nil, prop.ErrIfaceNotFound
	}

	tx, err := c.env.db.Begin()
	if err != nil {
		log.Println(err)
		return nil, dbus.MakeFailedError(fmt.Errorf("failure"))
	}
	defer tx.Rollback()

	props, dbusErr := c.getProperties(tx, name)
	if dbusErr != nil {
		return nil, dbusErr
	}
	return props[name], nil
}

func (c *collection) Set(iface, name string, value dbus.Variant) *dbus.Error {
	switch iface {
	case "org.freedesktop.Secret.Collection":
		switch name {
		case "Items":
			return prop.ErrReadOnly
		case "Label":
		case "Locked":
			return prop.ErrReadOnly
		case "Created":
			return prop.ErrReadOnly
		case "Modified":
			return prop.ErrReadOnly

		default:
			return prop.ErrPropNotFound
		}

	default:
		return prop.ErrIfaceNotFound
	}

	tx, err := c.env.db.Begin()
	if err != nil {
		log.Println(err)
		return dbus.MakeFailedError(fmt.Errorf("failure"))
	}
	defer tx.Rollback()

	if err := c.setProperty(tx, name, value); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		log.Println(err)
		return dbus.MakeFailedError(fmt.Errorf("failure"))
	}

	return nil
}

func (c *collection) GetAll(iface string) (map[string]*dbus.Variant, *dbus.Error) {
	switch iface {
	case "org.freedesktop.Secret.Collection":

	default:
		return nil, prop.ErrIfaceNotFound
	}

	tx, err := c.env.db.Begin()
	if err != nil {
		log.Println(err)
		return nil, dbus.MakeFailedError(fmt.Errorf("failure"))
	}
	defer tx.Rollback()

	return c.getProperties(tx, "Items", "Label", "Locked", "Created", "Modified")
}
