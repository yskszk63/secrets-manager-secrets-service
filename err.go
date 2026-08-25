package smss

import "github.com/godbus/dbus/v5"

var errIsLocked = dbus.NewError("org.freedesktop.Secret.Error.IsLocked ", []any{"Locked"})

var errNoSession = dbus.NewError("org.freedesktop.Secret.Error.NoSession", []any{"No Session"})

var errNoSuchObject = dbus.NewError("org.freedesktop.Secret.Error.NoSuchObject", []any{"No such object"})
