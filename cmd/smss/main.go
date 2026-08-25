package main

import (
	"database/sql"

	"github.com/godbus/dbus/v5"
	_ "modernc.org/sqlite"

	"github.com/yskszk63/smss"
)

func main() {
	// TODO
	dsn := "file:./data.db"

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	if err := smss.Migrate(db); err != nil {
		panic(err)
	}

	conn, err := dbus.SessionBus()
	if err != nil {
		panic(err)
	}

	env := smss.NewEnv(db, conn)

	if err := smss.Start(env); err != nil {
		panic(err)
	}

	select {}
}

