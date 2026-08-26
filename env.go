package smss

import (
	"database/sql"

	"github.com/godbus/dbus/v5"
)

type Env struct {
	sessions map[dbus.ObjectPath]*session
	db       *sql.DB
	conn     *dbus.Conn
}

func NewEnv(db *sql.DB, conn *dbus.Conn) *Env {
	sessions := make(map[dbus.ObjectPath]*session)

	return &Env{
		sessions: sessions,
		db:       db,
		conn:     conn,
	}
}
