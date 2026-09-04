package smss

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/prop"
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

func (i *item) getProperties(tx *sql.Tx, interests ...string) (map[string]*dbus.Variant, *dbus.Error) {
	var data *dbItem
	result := make(map[string]*dbus.Variant, len(interests))

	for _, name := range interests {
		switch name {
		case "Locked":
			result[name] = new(dbus.MakeVariant(false))

		case "Attributes":
			if data == nil {
				var err error
				data, err = findItem(tx, string(i.path))
				if err != nil {
					log.Println(err)
					return nil, dbus.MakeFailedError(fmt.Errorf("failure"))
				}
			}

			result[name] = new(dbus.MakeVariant(data.attributes))

		case "Label":
			if data == nil {
				var err error
				data, err = findItem(tx, string(i.path))
				if err != nil {
					log.Println(err)
					return nil, dbus.MakeFailedError(fmt.Errorf("failure"))
				}
			}

			result[name] = new(dbus.MakeVariant(data.label))

		case "Created":
			if data == nil {
				var err error
				data, err = findItem(tx, string(i.path))
				if err != nil {
					log.Println(err)
					return nil, dbus.MakeFailedError(fmt.Errorf("failure"))
				}
			}

			result[name] = new(dbus.MakeVariant(data.created))

		case "Modified":
			if data == nil {
				var err error
				data, err = findItem(tx, string(i.path))
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

func (i *item) setProperty(tx *sql.Tx, name string, value dbus.Variant) *dbus.Error {
	switch name {
	case "Locked":
		return prop.ErrReadOnly

	case "Attributes":
		v, ok := value.Value().(map[string]string)
		if !ok {
			return &dbus.ErrMsgInvalidArg
		}

		if err := deleteItemAttr(tx, string(i.path)); err != nil {
			log.Println(err)
			return dbus.MakeFailedError(fmt.Errorf("failure"))
		}

		if err := insertItemAttr(tx, string(i.path), v); err != nil {
			log.Println(err)
			return dbus.MakeFailedError(fmt.Errorf("failure"))
		}

	case "Label":
		v, ok := value.Value().(string)
		if !ok {
			return &dbus.ErrMsgInvalidArg
		}

		if err := updateItemLabel(tx, string(i.path), v); err != nil {
			log.Println(err)
			return dbus.MakeFailedError(fmt.Errorf("failure"))
		}

	case "Created":
		return prop.ErrReadOnly

	case "Modified":
		return prop.ErrReadOnly

	default:
		return prop.ErrPropNotFound
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
	s, unlock, ok := i.env.lookupSession(session)
	if !ok {
		return nil, errNoSession
	}
	defer unlock()

	tx, err := i.env.db.Begin()
	if err != nil {
		log.Println(err)
		return nil, dbus.MakeFailedError(fmt.Errorf("Failure"))
	}
	defer tx.Rollback()

	dbItem, err := findItem(tx, string(i.path))
	if err != nil {
		log.Println(err)
		return nil, dbus.MakeFailedError(fmt.Errorf("Failure"))
	}

	secret, err := s.encrypt(dbItem.secret, *dbItem.contentType)
	if err != nil {
		log.Println(err)
		return nil, dbus.MakeFailedError(fmt.Errorf("Failure"))
	}

	return secret, nil
}

func (i *item) SetSecret(secret *secret) *dbus.Error {
	s, unlock, ok := i.env.lookupSession(secret.Session)
	if !ok {
		return errNoSession
	}
	defer unlock()

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

	if err := updateItemSecret(tx, string(i.path), data, secret.ContentType, i.env.now()); err != nil {
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
	switch iface {
	case "org.freedesktop.Secret.Item":
		switch name {
		case "Locked":
		case "Attributes":
		case "Label":
		case "Created":
		case "Modified":

		default:
			return nil, prop.ErrPropNotFound
		}

	default:
		return nil, prop.ErrIfaceNotFound
	}

	tx, err := i.env.db.Begin()
	if err != nil {
		log.Println(err)
		return nil, dbus.MakeFailedError(fmt.Errorf("failure"))
	}
	defer tx.Rollback()

	props, dbusErr := i.getProperties(tx, name)
	if dbusErr != nil {
		return nil, dbusErr
	}
	return props[name], nil
}

func (i *item) Set(iface, name string, value dbus.Variant) *dbus.Error {
	switch iface {
	case "org.freedesktop.Secret.Item":
		switch name {
		case "Locked":
			return prop.ErrReadOnly
		case "Attributes":
		case "Label":
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

	tx, err := i.env.db.Begin()
	if err != nil {
		log.Println(err)
		return dbus.MakeFailedError(fmt.Errorf("failure"))
	}
	defer tx.Rollback()

	if err := i.setProperty(tx, name, value); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		log.Println(err)
		return dbus.MakeFailedError(fmt.Errorf("failure"))
	}

	return nil
}

func (i *item) GetAll(iface string) (map[string]*dbus.Variant, *dbus.Error) {
	switch iface {
	case "org.freedesktop.Secret.Item":

	default:
		return nil, prop.ErrIfaceNotFound
	}

	tx, err := i.env.db.Begin()
	if err != nil {
		log.Println(err)
		return nil, dbus.MakeFailedError(fmt.Errorf("Failure"))
	}
	defer tx.Rollback()

	return i.getProperties(tx, "Locked", "Attributes", "Label", "Created", "Modified")
}
