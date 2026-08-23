package main

import "github.com/godbus/dbus/v5"

type Secret struct {
	Session     dbus.ObjectPath
	Parameters  []byte
	Value       []byte
	ContentType string
}
