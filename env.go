package main

import (
	"database/sql"

	"github.com/godbus/dbus/v5"
)

type env struct {
	sessions map[dbus.ObjectPath]*Session
	db       *sql.DB
	conn     *dbus.Conn
}
