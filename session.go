package smss

import (
	"fmt"
	"log"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/prop"
)

type algorithm interface {
	encrypt(dbus.ObjectPath, []byte) (*secret, error)
	decrypt(*secret) ([]byte, error)
}

type algPlain struct {
}

func (a *algPlain) encrypt(session dbus.ObjectPath, data []byte) (*secret, error) {
	return &secret{
		Session:     session,
		Parameters:  []byte{},
		Value:       data,
		ContentType: "text/plain",
	}, nil
}

func (a *algPlain) decrypt(secret *secret) ([]byte, error) {
	return secret.Value, nil
}

type algDhIetf1024Sha256Aes128CbcPkcs7 struct {
	key []byte
}

func (a *algDhIetf1024Sha256Aes128CbcPkcs7) encrypt(session dbus.ObjectPath, data []byte) (*secret, error) {
	iv, value, err := unauthenticatedAESCBCEncrypt(data, a.key)
	if err != nil {
		return nil, err
	}
	return &secret{
		Session:     session,
		Parameters:  iv,
		Value:       value,
		ContentType: "text/plain",
	}, nil
}

func (a *algDhIetf1024Sha256Aes128CbcPkcs7) decrypt(secret *secret) ([]byte, error) {
	return unauthenticatedAESCBCDecrypt(secret.Parameters, secret.Value, a.key)
}

type session struct {
	env       *Env
	id        int
	path      dbus.ObjectPath
	algorithm algorithm
}

func newSession(env *Env, id int, algorithm algorithm) *session {
	path := dbus.ObjectPath(fmt.Sprintf(
		"/org/freedesktop/secrets/session/%d", id))
	return &session{
		env:       env,
		id:        id,
		path:      path,
		algorithm: algorithm,
	}
}

func (s *session) export() error {
	mappingSession := map[string]string{
		"Close": "Close",
	}

	if err := s.env.conn.ExportWithMap(s, mappingSession, s.path, "org.freedesktop.Secret.Session"); err != nil {
		return err
	}

	mappingProperties := map[string]string{
		"Get":    "Get",
		"Set":    "Set",
		"GetAll": "GetAll",
	}

	if err := s.env.conn.ExportWithMap(s, mappingProperties, s.path, "org.freedesktop.DBus.Properties"); err != nil {
		return err
	}

	return nil
}

func (s *session) encrypt(data []byte) (*secret, error) {
	return s.algorithm.encrypt(s.path, data)
}

func (s *session) decrypt(secret *secret) ([]byte, error) {
	return s.algorithm.decrypt(secret)
}

// org.freedesktop.Secret.Session

func (s *session) Close() *dbus.Error {
	delete(s.env.sessions, s.path)
	// TODO really??
	if err := s.env.conn.Export(nil, s.path, "org.freedesktop.Secret.Session"); err != nil {
		log.Println(err)
		return dbus.MakeFailedError(fmt.Errorf("failure"))
	}

	return nil
}

// org.freedesktop.DBus.Properties

func (s *session) Get(iface, name string) (*dbus.Variant, *dbus.Error) {
	return nil, prop.ErrIfaceNotFound
}

func (s *session) Set(iface, name string, value dbus.Variant) *dbus.Error {
	return prop.ErrIfaceNotFound
}

func (s *session) GetAll(iface string) (map[string]dbus.Variant, *dbus.Error) {
	return nil, prop.ErrIfaceNotFound
}
