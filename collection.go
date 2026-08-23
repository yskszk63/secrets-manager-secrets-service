package main

import (
	"fmt"
	"log"

	"github.com/godbus/dbus/v5"
)

type Collection struct {
	env  *env
	id   int64
	path dbus.ObjectPath
}

func newCollection(env *env, id int64) *Collection {
	path := dbus.ObjectPath(fmt.Sprintf("/org/freedesktop/secrets/collection/%d", id))
	return &Collection{
		env:  env,
		id:   id,
		path: path,
	}
}

func (c *Collection) export() error {
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

func (s *Collection) Delete() (*dbus.ObjectPath, *dbus.Error) {
	return nil, dbus.MakeFailedError(fmt.Errorf("Not Implemented"))
}

func (s *Collection) SearchItems(attributes map[string]string) ([]dbus.ObjectPath, *dbus.Error) {
	return nil, dbus.MakeFailedError(fmt.Errorf("Not Implemented"))
}

func (s *Collection) CreateItem(properties map[string]dbus.Variant, secret Secret, replace bool) (*dbus.ObjectPath, *dbus.ObjectPath, *dbus.Error) {
	session, found := s.env.sessions[secret.Session]
	if !found {
		return nil, nil, errNoSession
	}

	sec, err := unauthenticatedAESCBCDecrypt(secret.Parameters, secret.Value, session.key)
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

	dbItem := dbItem{
		collectionId: &s.id,
		// TODO encrypt
		secret:     sec,
		label:      &label,
		attributes: attr,
	}

	tx, err := s.env.db.Begin()
	if err != nil {
		log.Println(err)
		return nil, nil, dbus.MakeFailedError(fmt.Errorf("failure"))
	}
	defer tx.Rollback()

	if err := insertItem(tx, &dbItem); err != nil {
		log.Println(err)
		return nil, nil, dbus.MakeFailedError(fmt.Errorf("failure"))
	}

	item := newItem(s.env, *dbItem.collectionId, *dbItem.id)

	if err := item.export(); err != nil {
		log.Println(err)
		return nil, nil, dbus.MakeFailedError(fmt.Errorf("failure"))
	}

	if err := tx.Commit(); err != nil {
		log.Println(err)
		return nil, nil, dbus.MakeFailedError(fmt.Errorf("failure"))
	}

	noPrompt := dbus.ObjectPath("/")
	return &item.path, &noPrompt, nil
}

// org.freedesktop.DBus.Properties

func (c *Collection) Get(iface, name string) (*dbus.Variant, *dbus.Error) {
	return nil, dbus.MakeFailedError(fmt.Errorf("Not Implemented"))
}

func (c *Collection) Set(iface, name string, value dbus.Variant) *dbus.Error {
	return dbus.MakeFailedError(fmt.Errorf("Not Implemented"))
}

func (c *Collection) GetAll(iface string) (map[string]dbus.Variant, *dbus.Error) {
	return nil, dbus.MakeFailedError(fmt.Errorf("Not Implemented"))
}
