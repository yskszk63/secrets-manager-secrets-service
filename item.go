package smss

import (
	"fmt"
	"log"
	"strings"

	"github.com/godbus/dbus/v5"
)

type item struct {
	env  *Env
	path dbus.ObjectPath
}

func newItem(env *Env, path string) *item {
	if !strings.HasPrefix(path, "/org/freedesktop/secrets/collection/") {
		panic("Invalid path")
	}

	return &item{
		env:  env,
		path: dbus.ObjectPath(path),
	}
}

func (i *item) export() error {
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

func (i *item) Delete() (*dbus.ObjectPath, *dbus.Error) {
	tx, err := i.env.db.Begin()
	if err != nil {
		log.Println(err)
		return nil, dbus.MakeFailedError(fmt.Errorf("Failure"))
	}
	defer tx.Rollback()

	if err := deleteItem(tx, string(i.path)); err != nil {
		log.Println(err)
		return nil, dbus.MakeFailedError(fmt.Errorf("Failure"))
	}

	if err := tx.Commit(); err != nil {
		log.Println(err)
		return nil, dbus.MakeFailedError(fmt.Errorf("Failure"))
	}

	prompt := dbus.ObjectPath("/")
	return &prompt, nil
}

func (i *item) GetSecret(session dbus.ObjectPath) (*secret, *dbus.Error) {
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

	dbItem, err := findItemByPath(tx, string(i.path))
	if err != nil {
		log.Println(err)
		return nil, dbus.MakeFailedError(fmt.Errorf("Failure"))
	}

	secret, err := s.encrypt(dbItem.secret)
	if err != nil {
		log.Println(err)
		return nil, dbus.MakeFailedError(fmt.Errorf("Failure"))
	}

	return secret, nil
}

func (i *item) SetSecret(secret *secret) *dbus.Error {
	s, ok := i.env.sessions[secret.Session]
	if !ok {
		return errNoSession
	}

	tx, err := i.env.db.Begin()
	if err != nil {
		log.Println(err)
		return dbus.MakeFailedError(fmt.Errorf("Failure"))
	}
	defer tx.Rollback()

	data, err := s.decrypt(secret)
	if err != nil {
		log.Println(err)
		return dbus.MakeFailedError(fmt.Errorf("Failure"))
	}

	if err := updateItemSecret(tx, string(i.path), data); err != nil {
		log.Println(err)
		return dbus.MakeFailedError(fmt.Errorf("Failure"))
	}

	if err := tx.Commit(); err != nil {
		log.Println(err)
		return dbus.MakeFailedError(fmt.Errorf("Failure"))
	}

	return nil
}

// org.freedesktop.DBus.Properties

func (i *item) Get(iface, name string) (*dbus.Variant, *dbus.Error) {
	return nil, dbus.MakeFailedError(fmt.Errorf("Not Implemented"))
}

func (i *item) Set(iface, name string, value dbus.Variant) *dbus.Error {
	return dbus.MakeFailedError(fmt.Errorf("Not Implemented"))
}

func (i *item) GetAll(iface string) (map[string]dbus.Variant, *dbus.Error) {
	if iface != "org.freedesktop.Secret.Item" {
		return nil, &dbus.ErrMsgUnknownInterface
	}

	tx, err := i.env.db.Begin()
	if err != nil {
		log.Println(err)
		return nil, dbus.MakeFailedError(fmt.Errorf("Failure"))
	}
	defer tx.Rollback()

	dbItem, err := findItemByPath(tx, string(i.path))
	if err != nil {
		log.Println(err)
		return nil, dbus.MakeFailedError(fmt.Errorf("Failure"))
	}

	result := map[string]dbus.Variant{
		"Locked":     dbus.MakeVariant(false),
		"Attributes": dbus.MakeVariant(dbItem.attributes),
		"Label":      dbus.MakeVariant(dbItem.label),
		"Created":    dbus.MakeVariant(uint64(0)), // TODO time ... unix time (s)
		"Modified":   dbus.MakeVariant(uint64(0)), // TODO time ... unix time (s)
	}

	return result, nil
}
