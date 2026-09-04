package smss

import (
	"database/sql"
	"fmt"
	"log"
	"math/big"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/prop"
)

type service struct {
	env *Env
}

func newService(env *Env) *service {
	return &service{
		env: env,
	}
}

func (s *service) export() error {
	mappingSecretService := map[string]string{
		"OpenSession":      "OpenSession",
		"CreateCollection": "CreateCollection",
		"SearchItems":      "SearchItems",
		"Unlock":           "Unlock",
		"Lock":             "Lock",
		"GetSecrets":       "GetSecrets",
		"ReadAlias":        "ReadAlias",
		"SetAlias":         "SetAlias",
	}

	path := dbus.ObjectPath("/org/freedesktop/secrets")
	if err := s.env.conn.ExportWithMap(s, mappingSecretService, path, "org.freedesktop.Secret.Service"); err != nil {
		return err
	}

	mappingProperties := map[string]string{
		"Get":    "Get",
		"Set":    "Set",
		"GetAll": "GetAll",
	}

	if err := s.env.conn.ExportWithMap(s, mappingProperties, path, "org.freedesktop.DBus.Properties"); err != nil {
		return err
	}

	return nil
}

func (s *service) getProperties(tx *sql.Tx, interests ...string) (map[string]*dbus.Variant, *dbus.Error) {
	result := make(map[string]*dbus.Variant, len(interests))

	for _, name := range interests {
		switch name {
		case "Collections":
			collections := make([]dbus.ObjectPath, 0)
			for c, err := range listCollections(tx) {
				if err != nil {
					log.Println(err)
					return nil, dbus.MakeFailedError(fmt.Errorf("failure"))
				}
				collections = append(collections, dbus.ObjectPath(*c))
			}
			result[name] = new(dbus.MakeVariant(collections))

		default:
			return nil, prop.ErrPropNotFound
		}
	}

	return result, nil
}

func (s *service) setProperty(tx *sql.Tx, name string, value dbus.Variant) *dbus.Error {
	switch name {
	case "Collections":
		return prop.ErrReadOnly

	default:
		return prop.ErrPropNotFound
	}
}

// org.freedesktop.Secret.Service

func (s *service) OpenSession(algorithmName string, input dbus.Variant) (*dbus.Variant, *dbus.ObjectPath, *dbus.Error) {
	var alg algorithm
	var output *dbus.Variant

	switch algorithmName {
	case "plain":
		alg = new(algPlain)
		output = new(dbus.MakeVariant(""))

	case "dh-ietf1024-sha256-aes128-cbc-pkcs7":
		if input.Signature().String() != "ay" {
			return nil, nil, &dbus.ErrMsgInvalidArg
		}

		b, ok := input.Value().([]byte)
		if !ok {
			return nil, nil, &dbus.ErrMsgInvalidArg
		}

		g := rfc2409SecondOakleyGroup()
		myPrivate, myPublic, err := g.NewKeypair()
		if err != nil {
			log.Println(err)
			return nil, nil, dbus.MakeFailedError(fmt.Errorf("failure"))
		}
		theirPublic := new(big.Int).SetBytes(b)
		key, err := g.keygenHKDFSHA256AES128(theirPublic, myPrivate)
		if err != nil {
			log.Println(err)
			return nil, nil, dbus.MakeFailedError(fmt.Errorf("failure"))
		}

		alg = &algDhIetf1024Sha256Aes128CbcPkcs7{
			key: key,
		}

		b2 := make([]byte, 128)
		myPublic.FillBytes(b2)
		output = new(dbus.MakeVariant(b2))

	default:
		return nil, nil, dbus.MakeFailedError(fmt.Errorf("Not implemented"))
	}

	session := s.env.putSession(func(e *Env, u uint32) *session { return newSession(e, u, alg) })
	if err := session.export(); err != nil {
		log.Println(err)
		return nil, nil, dbus.MakeFailedError(fmt.Errorf("failure"))
	}

	return output, &session.path, nil
}

func (s *service) CreateCollection(properties map[string]dbus.Variant, alias string) (*dbus.ObjectPath, *dbus.ObjectPath, *dbus.Error) {
	if alias != "" {
		return nil, nil, dbus.MakeFailedError(fmt.Errorf("Setting alias not supported."))
	}

	tx, err := s.env.db.Begin()
	if err != nil {
		log.Println(err)
		return nil, nil, dbus.MakeFailedError(fmt.Errorf("failure"))
	}
	defer tx.Rollback()

	labelv, ok := properties["org.freedesktop.Secret.Collection.Label"]
	if !ok || labelv.Signature().String() != "s" {
		return nil, nil, &dbus.ErrMsgInvalidArg
	}

	label := labelv.Value().(string)

	col := dbCollection{
		label:    new(label),
		created:  new(s.env.now()),
		modified: new(s.env.now()),
	}
	if err := insertCollection(tx, &col); err != nil {
		log.Println(err)
		return nil, nil, dbus.MakeFailedError(fmt.Errorf("failure"))
	}

	if err := tx.Commit(); err != nil {
		log.Println(err)
		return nil, nil, dbus.MakeFailedError(fmt.Errorf("failure"))
	}

	if err := newCollection(s.env, *col.path).export(); err != nil {
		log.Println(err)
		return nil, nil, dbus.MakeFailedError(fmt.Errorf("failure"))
	}

	return new(dbus.ObjectPath(*col.path)), new(dbus.ObjectPath("/")), nil
}

func (s *service) SearchItems(attributes map[string]string) ([]dbus.ObjectPath, []dbus.ObjectPath, *dbus.Error) {
	tx, err := s.env.db.Begin()
	if err != nil {
		log.Println(err)
		return nil, nil, dbus.MakeFailedError(fmt.Errorf("failure"))
	}
	defer tx.Rollback()

	paths := []dbus.ObjectPath{}
	for path, err := range searchItems(tx, attributes) {
		if err != nil {
			log.Println(err)
			return nil, nil, dbus.MakeFailedError(fmt.Errorf("failure"))
		}

		paths = append(paths, dbus.ObjectPath(*path))
	}

	return paths, []dbus.ObjectPath{}, nil
}

func (s *service) Unlock(objects []dbus.ObjectPath) ([]dbus.ObjectPath, *dbus.ObjectPath, *dbus.Error) {
	return objects, new(dbus.ObjectPath("/")), nil
}

func (s *service) Lock(objects []dbus.ObjectPath) ([]dbus.ObjectPath, *dbus.ObjectPath, *dbus.Error) {
	return []dbus.ObjectPath{}, new(dbus.ObjectPath("/")), nil
}

func (s *service) GetSecrets(items []dbus.ObjectPath, sessionPath dbus.ObjectPath) (map[dbus.ObjectPath]*secret, *dbus.Error) {
	sessionobj, unlock, found := s.env.lookupSession(sessionPath)
	if !found {
		return nil, errNoSession
	}
	defer unlock()

	tx, err := s.env.db.Begin()
	if err != nil {
		log.Println(err)
		return nil, dbus.MakeFailedError(fmt.Errorf("failure"))
	}
	defer tx.Rollback()

	results := make(map[dbus.ObjectPath]*secret, len(items))
	for _, item := range items {
		dbItem, err := findItem(tx, string(item))
		if err != nil {
			log.Println(err)
			return nil, dbus.MakeFailedError(fmt.Errorf("failure"))
		}

		s, err := sessionobj.encrypt(dbItem.secret, *dbItem.contentType)
		if err != nil {
			log.Println(err)
			return nil, dbus.MakeFailedError(fmt.Errorf("failure"))
		}
		results[item] = s
	}

	return results, nil
}

func (s *service) ReadAlias(name string) (*dbus.ObjectPath, *dbus.Error) {
	if name != "default" {
		return nil, dbus.MakeFailedError(fmt.Errorf("Not supported"))
	}

	return new(dbus.ObjectPath("/org/freedesktop/secrets/collection/1")), nil
}

func (s *service) SetAlias(name string, collection dbus.ObjectPath) *dbus.Error {
	if name == "default" {
		return dbus.MakeFailedError(fmt.Errorf("Could not remove default alias"))
	}

	if collection != "/" {
		return dbus.MakeFailedError(fmt.Errorf("Only remove supported"))
	}

	return nil
}

// org.freedesktop.DBus.Properties

func (s *service) Get(iface, name string) (*dbus.Variant, *dbus.Error) {
	switch iface {
	case "org.freedesktop.Secret.Service":
		switch name {
		case "Collections":
		default:
			return nil, prop.ErrPropNotFound
		}

	default:
		return nil, prop.ErrIfaceNotFound
	}

	tx, err := s.env.db.Begin()
	if err != nil {
		log.Println(err)
		return nil, dbus.MakeFailedError(fmt.Errorf("failure"))
	}
	defer tx.Rollback()

	props, dbusErr := s.getProperties(tx, name)
	if dbusErr != nil {
		return nil, dbusErr
	}
	return props[name], nil
}

func (s *service) Set(iface, name string, value dbus.Variant) *dbus.Error {
	switch iface {
	case "org.freedesktop.Secret.Service":
		switch name {
		case "Collections":
		default:
			return prop.ErrPropNotFound
		}

	default:
		return prop.ErrIfaceNotFound
	}

	tx, err := s.env.db.Begin()
	if err != nil {
		log.Println(err)
		return dbus.MakeFailedError(fmt.Errorf("failure"))
	}
	defer tx.Rollback()

	return s.setProperty(tx, name, value)
}

func (s *service) GetAll(iface string) (map[string]*dbus.Variant, *dbus.Error) {
	switch iface {
	case "org.freedesktop.Secret.Service":

	default:
		return nil, prop.ErrIfaceNotFound
	}

	tx, err := s.env.db.Begin()
	if err != nil {
		log.Println(err)
		return nil, dbus.MakeFailedError(fmt.Errorf("failure"))
	}
	defer tx.Rollback()

	return s.getProperties(tx, "Collections")
}
