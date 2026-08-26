package smss

import (
	"fmt"
	"log"

	"github.com/godbus/dbus/v5"
)

type collection struct {
	env  *Env
	id   int64
	path dbus.ObjectPath
}

func newCollection(env *Env, id int64) *collection {
	path := dbus.ObjectPath(fmt.Sprintf("/org/freedesktop/secrets/collection/%d", id))
	return &collection{
		env:  env,
		id:   id,
		path: path,
	}
}

func (c *collection) export() error {
	mappingCollection := map[string]string{
		"Delete":      "Delete",
		"SearchItems": "SearchItems",
		"CreateItem":  "CreateItem",
	}

	if err := c.env.conn.ExportWithMap(c, mappingCollection, c.path, "org.freedesktop.Secret.Collection"); err != nil {
		return err
	}

	mappingProperties := map[string]string{
		"Get":    "Get",
		"Set":    "Set",
		"GetAll": "GetAll",
	}

	if err := c.env.conn.ExportWithMap(c, mappingProperties, c.path, "org.freedesktop.DBus.Properties"); err != nil {
		return err
	}

	return nil
}

// org.freedesktop.Secret.Collection

func (s *collection) Delete() (*dbus.ObjectPath, *dbus.Error) {
	return nil, dbus.MakeFailedError(fmt.Errorf("Not Implemented"))
}

func (s *collection) SearchItems(attributes map[string]string) ([]dbus.ObjectPath, *dbus.Error) {
	return nil, dbus.MakeFailedError(fmt.Errorf("Not Implemented"))
}

func (s *collection) CreateItem(properties map[string]dbus.Variant, secret secret, replace bool) (*dbus.ObjectPath, *dbus.ObjectPath, *dbus.Error) {
	session, found := s.env.sessions[secret.Session]
	if !found {
		return nil, nil, errNoSession
	}

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
		collectionId: &s.id,
		// TODO encrypt
		secret:     sec,
		label:      &label,
		attributes: attr,
	}

	if replace {
		for item, err := range searchItems(tx, attr) {
			if err != nil {
				log.Println(err)
				return nil, nil, dbus.MakeFailedError(fmt.Errorf("failure"))
			}

			if *dbItem.collectionId != *item.collectionId {
				continue
			}

			dbItem.id = item.id
			break
		}
	}

	var path dbus.ObjectPath
	if dbItem.id == nil {
		if err := insertItem(tx, &dbItem); err != nil {
			log.Println(err)
			return nil, nil, dbus.MakeFailedError(fmt.Errorf("failure"))
		}

		item := newItem(s.env, *dbItem.collectionId, *dbItem.id)

		if err := item.export(); err != nil {
			log.Println(err)
			return nil, nil, dbus.MakeFailedError(fmt.Errorf("failure"))
		}

		path = item.path

	} else {
		if err := updateItem(tx, &dbItem); err != nil {
			log.Println(err)
			return nil, nil, dbus.MakeFailedError(fmt.Errorf("failure"))
		}

		path = newItem(s.env, *dbItem.collectionId, *dbItem.id).path
	}

	if err := tx.Commit(); err != nil {
		log.Println(err)
		return nil, nil, dbus.MakeFailedError(fmt.Errorf("failure"))
	}

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
