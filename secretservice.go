package smss

import (
	"fmt"
	"log"
	"math/big"

	"github.com/godbus/dbus/v5"
)

type SecretService struct {
	env         *Env
	seq         int
	collections []*collection
}

func newSecretService(env *Env, collections []*collection) *SecretService {
	return &SecretService{
		env:         env,
		collections: collections,
	}
}

func (s *SecretService) export() error {
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

// org.freedesktop.Secret.Service

func (s *SecretService) OpenSession(algorithm string, input dbus.Variant) (*dbus.Variant, *dbus.ObjectPath, *dbus.Error) {
	if algorithm != "dh-ietf1024-sha256-aes128-cbc-pkcs7" {
		return nil, nil, dbus.MakeFailedError(fmt.Errorf("Not implemented"))
	}
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

	b2 := make([]byte, 128)
	myPublic.FillBytes(b2)
	o := dbus.MakeVariant(b2)

	s.seq += 1

	session := newSession(s.env, s.seq, key)
	if err := session.export(); err != nil {
		log.Println(err)
		return nil, nil, dbus.MakeFailedError(fmt.Errorf("failure"))
	}
	s.env.sessions[session.path] = session

	return &o, &session.path, nil
}

func (s *SecretService) CreateCollection(properties map[string]dbus.Variant, alias string) (*dbus.ObjectPath, *dbus.ObjectPath, *dbus.Error) {
	return nil, nil, dbus.MakeFailedError(fmt.Errorf("Not Implemented"))
}

func (s *SecretService) SearchItems(attributes map[string]string) ([]dbus.ObjectPath, []dbus.ObjectPath, *dbus.Error) {
	tx, err := s.env.db.Begin()
	if err != nil {
		log.Println(err)
		return nil, nil, dbus.MakeFailedError(fmt.Errorf("failure"))
	}
	defer tx.Rollback()

	paths := []dbus.ObjectPath{}
	for item, err := range searchItems(tx, attributes) {
		if err != nil {
			log.Println(err)
			return nil, nil, dbus.MakeFailedError(fmt.Errorf("failure"))
		}

		path := dbus.ObjectPath(fmt.Sprintf("/org/freedesktop/secrets/collection/%d/%d", *item.collectionId, *item.id))
		paths = append(paths, path)
	}

	return paths, []dbus.ObjectPath{}, nil
}

func (s *SecretService) Unlock(objects []dbus.ObjectPath) ([]dbus.ObjectPath, *dbus.ObjectPath, *dbus.Error) {
	return nil, nil, dbus.MakeFailedError(fmt.Errorf("Not Implemented"))
}

func (s *SecretService) Lock(objects []dbus.ObjectPath) ([]dbus.ObjectPath, *dbus.ObjectPath, *dbus.Error) {
	return nil, nil, dbus.MakeFailedError(fmt.Errorf("Not Implemented"))
}

func (s *SecretService) GetSecrets(items []dbus.ObjectPath, session dbus.ObjectPath) (map[dbus.ObjectPath]secret, *dbus.Error) {
	return nil, dbus.MakeFailedError(fmt.Errorf("Not Implemented"))
}

func (s *SecretService) ReadAlias(name string) (*dbus.ObjectPath, *dbus.Error) {
	return nil, dbus.MakeFailedError(fmt.Errorf("Not Implemented"))
}

func (s *SecretService) SetAlias(name string, collection dbus.ObjectPath) *dbus.Error {
	return dbus.MakeFailedError(fmt.Errorf("Not Implemented"))
}

// org.freedesktop.DBus.Properties

func (s *SecretService) Get(iface, name string) (*dbus.Variant, *dbus.Error) {
	return nil, dbus.MakeFailedError(fmt.Errorf("Not Implemented"))
}

func (s *SecretService) Set(iface, name string, value dbus.Variant) *dbus.Error {
	return dbus.MakeFailedError(fmt.Errorf("Not Implemented"))
}

func (s *SecretService) GetAll(iface string) (map[string]dbus.Variant, *dbus.Error) {
	return nil, dbus.MakeFailedError(fmt.Errorf("Not Implemented"))
}
