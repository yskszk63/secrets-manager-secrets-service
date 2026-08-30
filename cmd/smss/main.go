package main

import (
	"database/sql"
	"fmt"

	"github.com/adrg/xdg"
	"github.com/godbus/dbus/v5"
	_ "modernc.org/sqlite"

	"github.com/yskszk63/smss"
)

func main() {
	file, err := xdg.DataFile("smss/db.dat")
	if err != nil {
		panic(err)
	}

	dsn := fmt.Sprintf("file:%s", file)

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	db.SetMaxOpenConns(1)

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

