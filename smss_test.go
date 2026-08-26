package smss_test

import (
	"bufio"
	"database/sql"
	"fmt"
	"os/exec"
	"slices"
	"testing"

	"github.com/godbus/dbus/v5"
	"github.com/yskszk63/smss"
	_ "modernc.org/sqlite"
)

type secret struct {
	Session     dbus.ObjectPath
	Parameters  []byte
	Value       []byte
	ContentType string
}

func runBroker(t testing.TB) string {
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

func initDBusConnForTest(t testing.TB) *dbus.Conn {
	addr := runBroker(t)

	conn, err := dbus.Connect(addr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal()
	}
	t.Cleanup(func() { db.Close() })

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

	return client
}

func openSession(t testing.TB, conn *dbus.Conn) dbus.ObjectPath {
	var output dbus.Variant
	var session dbus.ObjectPath

	secretService := conn.Object("org.freedesktop.secrets", "/org/freedesktop/secrets")
	call := secretService.Call("org.freedesktop.Secret.Service.OpenSession", 0, "plain", dbus.MakeVariant(""))

	err := call.Store(&output, &session)
	if err != nil {
		t.Fatal(err)
	}

	return session
}

func searchItems(t testing.TB, conn *dbus.Conn, attrs map[string]string) []dbus.ObjectPath {
	var unlocked []dbus.ObjectPath
	var locked []dbus.ObjectPath

	secretService := conn.Object("org.freedesktop.secrets", "/org/freedesktop/secrets")
	call := secretService.Call("org.freedesktop.Secret.Service.SearchItems", 0, attrs)

	err := call.Store(&unlocked, &locked)
	if err != nil {
		t.Fatal(err)
	}

	return unlocked
}

func createItem(t testing.TB, conn *dbus.Conn, session dbus.ObjectPath, props map[string]dbus.Variant, data []byte, replace bool) dbus.ObjectPath {
	var item dbus.ObjectPath
	var prompt dbus.ObjectPath

	s := secret{
		Session:     session,
		Parameters:  []byte(""),
		Value:       data,
		ContentType: "",
	}

	secretService := conn.Object("org.freedesktop.secrets", "/org/freedesktop/secrets/aliases/default")
	call := secretService.Call("org.freedesktop.Secret.Collection.CreateItem", 0, props, s, replace)

	err := call.Store(&item, &prompt)
	if err != nil {
		t.Fatal(err)
	}

	return item
}

func deleteItem(t testing.TB, conn *dbus.Conn, item dbus.ObjectPath) {
	var prompt dbus.ObjectPath

	secretService := conn.Object("org.freedesktop.secrets", item)
	call := secretService.Call("org.freedesktop.Secret.Item.Delete", 0)

	err := call.Store(&prompt)
	if err != nil {
		t.Fatal(err)
	}
}

func getSecret(t testing.TB, conn *dbus.Conn, item dbus.ObjectPath, session dbus.ObjectPath) []byte {
	var secret *secret

	secretService := conn.Object("org.freedesktop.secrets", item)
	call := secretService.Call("org.freedesktop.Secret.Item.GetSecret", 0, session)

	err := call.Store(&secret)
	if err != nil {
		t.Fatal(err)
	}

	return secret.Value
}

func setSecret(t testing.TB, conn *dbus.Conn, item dbus.ObjectPath, session dbus.ObjectPath, data []byte) {
	secret := secret{
		Session:     session,
		Parameters:  []byte(""),
		Value:       data,
		ContentType: "",
	}

	secretService := conn.Object("org.freedesktop.secrets", item)
	call := secretService.Call("org.freedesktop.Secret.Item.SetSecret", 0, secret)

	err := call.Store()
	if err != nil {
		t.Fatal(err)
	}
}

func TestSmss(t *testing.T) {
	conn := initDBusConnForTest(t)

	session := openSession(t, conn)

	props1 := map[string]dbus.Variant{
		"org.freedesktop.Secret.Item.Attributes": dbus.MakeVariant(map[string]string{
			"a": "1",
			"b": "2",
			"c": "3",
		}),
		"org.freedesktop.Secret.Item.Label": dbus.MakeVariant("test"),
	}
	createItem(t, conn, session, props1, []byte("OK"), true)
	item1 := createItem(t, conn, session, props1, []byte("OK!"), true)

	props2 := map[string]dbus.Variant{
		"org.freedesktop.Secret.Item.Attributes": dbus.MakeVariant(map[string]string{
			"a": "1",
			"b": "2",
		}),
		"org.freedesktop.Secret.Item.Label": dbus.MakeVariant("test2"),
	}
	item2 := createItem(t, conn, session, props2, []byte("OK2"), false)

	props3 := map[string]dbus.Variant{
		"org.freedesktop.Secret.Item.Attributes": dbus.MakeVariant(map[string]string{
			"a": "4",
			"b": "2",
		}),
		"org.freedesktop.Secret.Item.Label": dbus.MakeVariant("test3"),
	}
	item3 := createItem(t, conn, session, props3, []byte("OK3"), true)
	_ = item3

	attrs := map[string]string{
		"a": "1",
		"b": "2",
	}
	found := searchItems(t, conn, attrs)

	if !slices.Equal(found, []dbus.ObjectPath{item1, item2}) {
		t.Fatal()
	}

	if !slices.Equal(getSecret(t, conn, item1, session), []byte("OK!")) {
		t.Fatal()
	}

	setSecret(t, conn, item1, session, []byte("OK"))

	if !slices.Equal(getSecret(t, conn, item1, session), []byte("OK")) {
		t.Fatal()
	}

	deleteItem(t, conn, item1)

	found2 := searchItems(t, conn, attrs)
	if !slices.Equal(found2, []dbus.ObjectPath{item2}) {
		fmt.Printf("%#v\n", found)
		t.Fatal()
	}
}
