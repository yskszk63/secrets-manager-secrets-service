package main

import (
	"fmt"
	"log"

	"github.com/godbus/dbus/v5"
)

type Item struct {
	env          *env
	collectionId int64
	id           int64
	path         dbus.ObjectPath
}

func newItem(env *env, collectionId, id int64) *Item {
	path := dbus.ObjectPath(fmt.Sprintf("/org/freedesktop/secrets/collection/%d/%d", collectionId, id))
	return &Item{
		env:          env,
		collectionId: collectionId,
		id:           id,
		path:         path,
	}
}

func (i *Item) export() error {
	mappingItem := map[string]string{
		"Delete":    "Delete",
		"GetSecret": "GetSecret",
		"SetSecret": "SetSecret",
	}

	if err := i.env.conn.ExportWithMap(i, mappingItem, i.path, "org.freedesktop.Secret.Item"); err != nil {
		return err
	}

	mappingProperties := map[string]string{
		"Get":    "Get",
		"Set":    "Set",
		"GetAll": "GetAll",
	}

	if err := i.env.conn.ExportWithMap(i, mappingProperties, i.path, "org.freedesktop.DBus.Properties"); err != nil {
		return err
	}

	return nil
}

// org.freedesktop.Secret.Item

func (i *Item) Delete() (*dbus.ObjectPath, *dbus.Error) {
	return nil, dbus.MakeFailedError(fmt.Errorf("Not Implemented"))
}

func (i *Item) GetSecret(session dbus.ObjectPath) (*Secret, *dbus.Error) {
	s, ok := i.env.sessions[session]
	if !ok {
		return nil, errNoSession
	}

	tx, err := i.env.db.Begin()
	if err != nil {
		log.Println(err)
		return nil, dbus.MakeFailedError(fmt.Errorf("Failure"))
	}
	defer tx.Rollback()

	dbItem, err := getItem(tx, i.id)
	if err != nil {
		log.Println(err)
		return nil, dbus.MakeFailedError(fmt.Errorf("Failure"))
	}

	iv, cipherText, err := unauthenticatedAESCBCEncrypt(dbItem.secret, s.key)
	if err != nil {
		log.Println(err)
		return nil, dbus.MakeFailedError(fmt.Errorf("Failure"))
	}

	secret := Secret{
		Session:     session,
		Parameters:  iv,
		Value:       cipherText,
		ContentType: "text/plain", // TODO
	}

	return &secret, nil
}

func (i *Item) SetSecret(secret *Secret) *dbus.Error {
	return dbus.MakeFailedError(fmt.Errorf("Not Implemented"))
}

// org.freedesktop.DBus.Properties

func (i *Item) Get(iface, name string) (*dbus.Variant, *dbus.Error) {
	return nil, dbus.MakeFailedError(fmt.Errorf("Not Implemented"))
}

func (i *Item) Set(iface, name string, value dbus.Variant) *dbus.Error {
	return dbus.MakeFailedError(fmt.Errorf("Not Implemented"))
}

func (i *Item) GetAll(iface string) (map[string]dbus.Variant, *dbus.Error) {
	if iface != "org.freedesktop.Secret.Item" {
		return nil, &dbus.ErrMsgUnknownInterface
	}

	tx, err := i.env.db.Begin()
	if err != nil {
		log.Println(err)
		return nil, dbus.MakeFailedError(fmt.Errorf("Failure"))
	}
	defer tx.Rollback()

	dbItem, err := getItem(tx, i.id)
	if err != nil {
		log.Println(err)
		return nil, dbus.MakeFailedError(fmt.Errorf("Failure"))
	}

	result := map[string]dbus.Variant{
		"Locked":     dbus.MakeVariant(false),
		"Attributes": dbus.MakeVariant(dbItem.attributes),
		"Label":      dbus.MakeVariant(dbItem.label),
		"Created":    dbus.MakeVariant(uint64(0)), // TODO time
		"Modified":   dbus.MakeVariant(uint64(0)), // TODO time
	}

	return result, nil
}
