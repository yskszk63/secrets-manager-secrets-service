package smss_test

import (
	"bufio"
	"database/sql"
	"os/exec"
	"testing"

	"github.com/godbus/dbus/v5"
	"github.com/yskszk63/smss"
	_ "modernc.org/sqlite"
)

func runBroker(t *testing.T) string {
	cx := t.Context()

	// TODO print-address=<extra>
	cmd := exec.CommandContext(cx, "dbus-daemon", "--session", "--nofork", "--print-address=1")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		panic(err)
	}
	defer stdout.Close()

	if err := cmd.Start(); err != nil {
		panic(err)
	}
	t.Cleanup(func() {
		cmd.Process.Kill()
		cmd.Process.Wait()
	})

	sc := bufio.NewScanner(stdout)
	if !sc.Scan() {
		panic("empty")
	}
	return sc.Text()
}

func TestSmss(t *testing.T) {
	addr := runBroker(t)

	conn, err := dbus.Connect(addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal()
	}
	defer db.Close()

	if err := smss.Migrate(db); err != nil {
		t.Fatal(err)
	}

	env := smss.NewEnv(db, conn)
	if err := smss.Start(env); err != nil {
		t.Fatal(err)
	}

	client, err := dbus.Connect(addr)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	// TODO
}
