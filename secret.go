package smss

import "github.com/godbus/dbus/v5"

type secret struct {
	Session     dbus.ObjectPath
	Parameters  []byte
	Value       []byte
	ContentType string
}
