package smss

import (
	"fmt"
	"log"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/prop"
)

type paramValue struct {
	parameters []byte
	value      []byte
}

type algorithm interface {
	encrypt([]byte) (*paramValue, error)
	decrypt(*paramValue) ([]byte, error)
}

type algPlain struct {
}

func (a *algPlain) encrypt(data []byte) (*paramValue, error) {
	return &paramValue{
		parameters: []byte{},
		value:      data,
	}, nil
}

func (a *algPlain) decrypt(secret *paramValue) ([]byte, error) {
	return secret.value, nil
}

type algDhIetf1024Sha256Aes128CbcPkcs7 struct {
	key []byte
}

func (a *algDhIetf1024Sha256Aes128CbcPkcs7) encrypt(data []byte) (*paramValue, error) {
	iv, value, err := unauthenticatedAESCBCEncrypt(data, a.key)
	if err != nil {
		return nil, err
	}
	return &paramValue{
		parameters: iv,
		value:      value,
	}, nil
}

func (a *algDhIetf1024Sha256Aes128CbcPkcs7) decrypt(secret *paramValue) ([]byte, error) {
	return unauthenticatedAESCBCDecrypt(secret.parameters, secret.value, a.key)
}

type session struct {
	env       *Env
	id        uint32
	path      dbus.ObjectPath
	algorithm algorithm
}

func newSession(env *Env, id uint32, algorithm algorithm) *session {
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

func (s *session) encrypt(data []byte, contentType string) (*secret, error) {
	pv, err := s.algorithm.encrypt(data)
	if err != nil {
		return nil, err
	}

	return &secret{
		Session:     s.path,
		Parameters:  pv.parameters,
		Value:       pv.value,
		ContentType: contentType,
	}, nil
}

func (s *session) decrypt(secret *secret) ([]byte, error) {
	return s.algorithm.decrypt(&paramValue{parameters: secret.Parameters, value: secret.Value})
}

// org.freedesktop.Secret.Session

func (s *session) Close() *dbus.Error {
	s.env.removeSession(s.path)
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
	if iface != "org.freedesktop.Secret.Session" {
		return nil, prop.ErrIfaceNotFound
	}

	return map[string]dbus.Variant{}, nil
}
