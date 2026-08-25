package smss

import (
	"fmt"
	"log"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/prop"
)

type Session struct {
	env  *Env
	id   int
	path dbus.ObjectPath
	key  []byte
}

func newSession(env *Env, id int, key []byte) *Session {
	path := dbus.ObjectPath(fmt.Sprintf(
		"/org/freedesktop/secrets/session/%d", id))
	return &Session{
		env:  env,
		id:   id,
		path: path,
		key:  key,
	}
}

func (s *Session) export() error {
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

// org.freedesktop.Secret.Session

func (s *Session) Close() error {
	delete(s.env.sessions, s.path)
	// TODO really??
	if err := s.env.conn.Export(nil, s.path, "org.freedesktop.Secret.Session"); err != nil {
		log.Println(err)
		return dbus.MakeFailedError(fmt.Errorf("failure"))
	}

	return nil
}

// org.freedesktop.DBus.Properties

func (s *Session) Get(iface, name string) (*dbus.Variant, *dbus.Error) {
	return nil, prop.ErrIfaceNotFound
}

func (s *Session) Set(iface, name string, value dbus.Variant) *dbus.Error {
	return prop.ErrIfaceNotFound
}

func (s *Session) GetAll(iface string) (map[string]dbus.Variant, *dbus.Error) {
	return nil, prop.ErrIfaceNotFound
}
