package smss_test

import (
	"bufio"
	"database/sql"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/prop"
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

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()

	cmd := exec.CommandContext(cx, "dbus-daemon", "--session", "--nofork", "--print-address=3")
	cmd.ExtraFiles = []*os.File{w}

	if err := cmd.Start(); err != nil {
		panic(err)
	}
	t.Cleanup(func() {
		cmd.Process.Kill()
		cmd.Process.Wait()
	})

	sc := bufio.NewScanner(r)
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

	path := filepath.Join(t.TempDir(), "db.db")
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal()
	}
	t.Cleanup(func() { db.Close() })

	db.SetMaxOpenConns(1)

	if err := smss.Migrate(db); err != nil {
		t.Fatal(err)
	}

	env := smss.NewEnv(db, conn)
	env.SetClock(func() time.Time {
		return time.Unix(1234, 0)
	})
	if err := smss.Start(env); err != nil {
		t.Fatal(err)
	}

	client, err := dbus.Connect(addr)
	if err != nil {
		t.Fatal(err)
	}

	return client
}

const IFACE_SERVICE = "org.freedesktop.Secret.Service"
const METHOD_SERVICE_OPEN_SESSION = "org.freedesktop.Secret.Service.OpenSession"
const METHOD_SERVICE_CREATE_COLLECTION = "org.freedesktop.Secret.Service.CreateCollection"
const METHOD_SERVICE_SEARCH_ITEMS = "org.freedesktop.Secret.Service.SearchItems"
const METHOD_SERVICE_UNLOCK = "org.freedesktop.Secret.Service.Unlock"
const METHOD_SERVICE_LOCK = "org.freedesktop.Secret.Service.Lock"
const METHOD_SERVICE_GET_SECRETS = "org.freedesktop.Secret.Service.GetSecrets"
const METHOD_SERVICE_READ_ALIAS = "org.freedesktop.Secret.Service.ReadAlias"
const METHOD_SERVICE_SET_ALIAS = "org.freedesktop.Secret.Service.SetAlias"

const IFACE_COLLECTION = "org.freedesktop.Secret.Collection"
const METHOD_COLLECTION_DELETE = "org.freedesktop.Secret.Collection.Delete"
const METHOD_COLLECTION_SEARCH_ITEMS = "org.freedesktop.Secret.Collection.SearchItems"
const METHOD_COLLECTION_CREATE_ITEM = "org.freedesktop.Secret.Collection.CreateItem"

const IFACE_ITEM = "org.freedesktop.Secret.Item"
const METHOD_ITEM_DELETE = "org.freedesktop.Secret.Item.Delete"
const METHOD_ITEM_GET_SECRET = "org.freedesktop.Secret.Item.GetSecret"
const METHOD_ITEM_SET_SECRET = "org.freedesktop.Secret.Item.SetSecret"

const IFACE_SESSION = "org.freedesktop.Secret.Session"
const METHOD_SESSION_CLOSE = "org.freedesktop.Secret.Session.Close"

const IFACE_PROPERTIES = "org.freedesktop.DBus.Properties"
const METHOD_PROPERTIES_GET = "org.freedesktop.DBus.Properties.Get"
const METHOD_PROPERTIES_SET = "org.freedesktop.DBus.Properties.Set"
const METHOD_PROPERTIES_GET_ALL = "org.freedesktop.DBus.Properties.GetAll"

func call(t testing.TB, call *dbus.Call, retvalues ...any) {
	if err := call.Store(retvalues...); err != nil {
		t.Fatal(err)
	}
}

func callErr(t testing.TB, call *dbus.Call, expected *dbus.Error, retvalues ...any) {
	if err := call.Store(retvalues...); err != nil {
		if err, ok := err.(dbus.Error); ok {
			if err.Name != expected.Name {
				t.Fatal(err)
			}
			if slices.Equal(err.Body, expected.Body) {
				return
			}
		}

		t.Fatal(err)
	}

	t.Fatal("no error")
}

func TestSmss(t *testing.T) {
	conn := initDBusConnForTest(t)

	dest := "org.freedesktop.secrets"
	t.Run(IFACE_SERVICE, func(t *testing.T) {
		service := conn.Object(dest, "/org/freedesktop/secrets")

		t.Run(METHOD_SERVICE_OPEN_SESSION, func(t *testing.T) {
			t.Parallel()

			var sessionPath dbus.ObjectPath
			call(t, service.Call(METHOD_SERVICE_OPEN_SESSION, 0, "plain", dbus.MakeVariant("")),
				new(dbus.Variant), &sessionPath)

			t.Cleanup(func() {
				session := conn.Object(dest, sessionPath)
				call(t, session.Call(METHOD_SESSION_CLOSE, 0))
			})
		})

		t.Run(METHOD_SERVICE_CREATE_COLLECTION, func(t *testing.T) {
			t.Parallel()

			var collection dbus.ObjectPath
			props := map[string]dbus.Variant{
				"org.freedesktop.Secret.Collection.Label": dbus.MakeVariant("collection"),
			}
			call(t, service.Call(METHOD_SERVICE_CREATE_COLLECTION, 0, props, ""),
				&collection, new(dbus.ObjectPath))
			if collection == "/" {
				t.Fatal()
			}
		})

		t.Run(METHOD_COLLECTION_SEARCH_ITEMS, func(t *testing.T) {
			t.Parallel()

			var sessionPath dbus.ObjectPath
			call(t, service.Call(METHOD_SERVICE_OPEN_SESSION, 0, "plain", dbus.MakeVariant("")),
				new(dbus.Variant), &sessionPath)

			t.Cleanup(func() {
				session := conn.Object(dest, sessionPath)
				call(t, session.Call(METHOD_SESSION_CLOSE, 0))
			})

			var collectionPath dbus.ObjectPath
			props := map[string]dbus.Variant{
				"org.freedesktop.Secret.Collection.Label": dbus.MakeVariant("collection"),
			}
			call(t, service.Call(METHOD_SERVICE_CREATE_COLLECTION, 0, props, ""),
				&collectionPath, new(dbus.ObjectPath))

			collection := conn.Object(dest, collectionPath)

			var item1Path dbus.ObjectPath
			var item2Path dbus.ObjectPath
			var item3Path dbus.ObjectPath

			call(t, collection.Call(METHOD_COLLECTION_CREATE_ITEM, 0,
				map[string]dbus.Variant{
					"org.freedesktop.Secret.Item.Label": dbus.MakeVariant("item1"),
					"org.freedesktop.Secret.Item.Attributes": dbus.MakeVariant(map[string]string{
						"a": t.Name() + "1",
						"b": "2",
						"c": "3",
					}),
				},
				secret{
					Session:     sessionPath,
					Parameters:  []byte{},
					Value:       []byte("ITEM1"),
					ContentType: "text/plain",
				}, false),
				&item1Path, new(dbus.ObjectPath),
			)

			call(t, collection.Call(METHOD_COLLECTION_CREATE_ITEM, 0,
				map[string]dbus.Variant{
					"org.freedesktop.Secret.Item.Label": dbus.MakeVariant("item2"),
					"org.freedesktop.Secret.Item.Attributes": dbus.MakeVariant(map[string]string{
						"a": t.Name() + "1",
						"b": "2",
					}),
				},
				secret{
					Session:     sessionPath,
					Parameters:  []byte{},
					Value:       []byte("ITEM2"),
					ContentType: "text/plain",
				}, false),
				&item2Path, new(dbus.ObjectPath),
			)

			call(t, collection.Call(METHOD_COLLECTION_CREATE_ITEM, 0,
				map[string]dbus.Variant{
					"org.freedesktop.Secret.Item.Label": dbus.MakeVariant("item3"),
					"org.freedesktop.Secret.Item.Attributes": dbus.MakeVariant(map[string]string{
						"a": t.Name() + "2",
						"b": "2",
						"c": "3",
					}),
				},
				secret{
					Session:     sessionPath,
					Parameters:  []byte{},
					Value:       []byte("ITEM3"),
					ContentType: "text/plain",
				}, false),
				&item3Path, new(dbus.ObjectPath),
			)

			var results []dbus.ObjectPath
			attrs := map[string]string{
				"a": t.Name() + "1",
				"b": "2",
			}
			call(t, service.Call(METHOD_SERVICE_SEARCH_ITEMS, 0, attrs),
				&results, new([]dbus.ObjectPath))

			expected := []dbus.ObjectPath{
				item1Path,
				item2Path,
			}
			if !slices.Equal(results, expected) {
				t.Fatal()
			}
		})

		t.Run(METHOD_SERVICE_UNLOCK, func(t *testing.T) {
			t.Parallel()

			targets := []dbus.ObjectPath{
				"/org/freedesktop/secrets/collection/1",
				"/org/freedesktop/secrets/collection/1/1",
				"/",
			}
			var unlocked []dbus.ObjectPath
			call(t, service.Call(METHOD_SERVICE_UNLOCK, 0, targets), &unlocked, new(dbus.ObjectPath))
			if !slices.Equal(unlocked, targets) {
				t.Fatal()
			}
		})

		t.Run(METHOD_SERVICE_LOCK, func(t *testing.T) {
			t.Parallel()

			targets := []dbus.ObjectPath{
				"/org/freedesktop/secrets/collection/1",
				"/org/freedesktop/secrets/collection/1/1",
				"/",
			}
			var unlocked []dbus.ObjectPath
			call(t, service.Call(METHOD_SERVICE_LOCK, 0, targets), &unlocked, new(dbus.ObjectPath))
			if !slices.Equal(unlocked, []dbus.ObjectPath{}) {
				t.Fatal()
			}
		})

		t.Run(METHOD_SERVICE_GET_SECRETS, func(t *testing.T) {
			t.Parallel()

			var sessionPath dbus.ObjectPath
			call(t, service.Call(METHOD_SERVICE_OPEN_SESSION, 0, "plain", dbus.MakeVariant("")),
				new(dbus.Variant), &sessionPath)

			t.Cleanup(func() {
				session := conn.Object(dest, sessionPath)
				call(t, session.Call(METHOD_SESSION_CLOSE, 0))
			})

			var collectionPath dbus.ObjectPath
			props := map[string]dbus.Variant{
				"org.freedesktop.Secret.Collection.Label": dbus.MakeVariant("collection"),
			}
			call(t, service.Call(METHOD_SERVICE_CREATE_COLLECTION, 0, props, ""),
				&collectionPath, new(dbus.ObjectPath))

			collection := conn.Object(dest, collectionPath)

			var item1Path dbus.ObjectPath
			var item2Path dbus.ObjectPath
			var item3Path dbus.ObjectPath

			call(t, collection.Call(METHOD_COLLECTION_CREATE_ITEM, 0,
				map[string]dbus.Variant{
					"org.freedesktop.Secret.Item.Label": dbus.MakeVariant("item1"),
					"org.freedesktop.Secret.Item.Attributes": dbus.MakeVariant(map[string]string{
						"a": t.Name() + "1",
						"b": "2",
						"c": "3",
					}),
				},
				secret{
					Session:     sessionPath,
					Parameters:  []byte{},
					Value:       []byte("ITEM1"),
					ContentType: "text/plain",
				}, false),
				&item1Path, new(dbus.ObjectPath),
			)

			call(t, collection.Call(METHOD_COLLECTION_CREATE_ITEM, 0,
				map[string]dbus.Variant{
					"org.freedesktop.Secret.Item.Label": dbus.MakeVariant("item2"),
					"org.freedesktop.Secret.Item.Attributes": dbus.MakeVariant(map[string]string{
						"a": t.Name() + "1",
						"b": "2",
					}),
				},
				secret{
					Session:     sessionPath,
					Parameters:  []byte{},
					Value:       []byte("ITEM2"),
					ContentType: "text/plain",
				}, false),
				&item2Path, new(dbus.ObjectPath),
			)

			call(t, collection.Call(METHOD_COLLECTION_CREATE_ITEM, 0,
				map[string]dbus.Variant{
					"org.freedesktop.Secret.Item.Label": dbus.MakeVariant("item3"),
					"org.freedesktop.Secret.Item.Attributes": dbus.MakeVariant(map[string]string{
						"a": t.Name() + "2",
						"b": "2",
						"c": "3",
					}),
				},
				secret{
					Session:     sessionPath,
					Parameters:  []byte{},
					Value:       []byte("ITEM3"),
					ContentType: "text/plain",
				}, false),
				&item3Path, new(dbus.ObjectPath),
			)

			var secrets map[dbus.ObjectPath]secret
			call(t, service.Call(METHOD_SERVICE_GET_SECRETS, 0, []dbus.ObjectPath{item1Path, item2Path, item3Path}, sessionPath),
				&secrets)

			if string(secrets[item1Path].Value) != "ITEM1" {
				t.Fatal()
			}

			if string(secrets[item2Path].Value) != "ITEM2" {
				t.Fatal()
			}

			if string(secrets[item3Path].Value) != "ITEM3" {
				t.Fatal()
			}
		})

		t.Run(METHOD_SERVICE_READ_ALIAS, func(t *testing.T) {
			t.Parallel()

			var collection dbus.ObjectPath
			call(t, service.Call(METHOD_SERVICE_READ_ALIAS, 0, "default"), &collection)
			if collection != "/org/freedesktop/secrets/collection/1" {
				t.Fatal()
			}
		})

		t.Run(METHOD_SERVICE_SET_ALIAS, func(t *testing.T) {
			t.Parallel()

			call(t, service.Call(METHOD_SERVICE_SET_ALIAS, 0, "foo", "/"))
		})

		t.Run(METHOD_PROPERTIES_GET_ALL, func(t *testing.T) {
			t.Parallel()

			var m map[string]dbus.Variant
			call(t, service.Call(METHOD_PROPERTIES_GET_ALL, 0, IFACE_SERVICE), &m)

			collections := m["Collections"].Value().([]dbus.ObjectPath)

			if !slices.Contains(collections, dbus.ObjectPath("/org/freedesktop/secrets/collection/1")) {
				t.Fatal(collections)
			}
		})

		t.Run(METHOD_PROPERTIES_GET, func(t *testing.T) {
			t.Run("Collections", func(t *testing.T) {
				t.Parallel()

				var v dbus.Variant
				call(t, service.Call(METHOD_PROPERTIES_GET, 0, IFACE_SERVICE, "Collections"), &v)

				collections := v.Value().([]dbus.ObjectPath)

				if !slices.Contains(collections, dbus.ObjectPath("/org/freedesktop/secrets/collection/1")) {
					t.Fatal(collections)
				}
			})
		})

		t.Run(METHOD_PROPERTIES_SET, func(t *testing.T) {
			t.Run("Collections", func(t *testing.T) {
				t.Parallel()

				v := dbus.MakeVariant([]dbus.ObjectPath{})
				callErr(t, service.Call(METHOD_PROPERTIES_SET, 0, IFACE_SERVICE, "Collections", v), prop.ErrReadOnly)
			})
		})
	})

	t.Run(IFACE_COLLECTION, func(t *testing.T) {
		service := conn.Object(dest, "/org/freedesktop/secrets")

		var sessionPath dbus.ObjectPath
		call(t, service.Call(METHOD_SERVICE_OPEN_SESSION, 0, "plain", dbus.MakeVariant("")),
			new(dbus.Variant), &sessionPath)

		t.Cleanup(func() {
			session := conn.Object(dest, sessionPath)
			call(t, session.Call(METHOD_SESSION_CLOSE, 0))
		})

		fixture := func(t testing.TB) dbus.BusObject {
			var collectionPath dbus.ObjectPath
			props := map[string]dbus.Variant{
				"org.freedesktop.Secret.Collection.Label": dbus.MakeVariant("collection"),
			}
			call(t, service.Call(METHOD_SERVICE_CREATE_COLLECTION, 0, props, ""),
				&collectionPath, new(dbus.ObjectPath))

			return conn.Object(dest, collectionPath)
		}

		t.Run(METHOD_COLLECTION_DELETE, func(t *testing.T) {
			t.Parallel()

			collection := fixture(t)

			var prompt dbus.ObjectPath
			call(t, collection.Call(METHOD_COLLECTION_DELETE, 0), &prompt)
			if prompt != "/" {
				t.Fatal()
			}
		})

		t.Run(METHOD_COLLECTION_SEARCH_ITEMS, func(t *testing.T) {
			t.Parallel()

			collection := fixture(t)

			var item1Path dbus.ObjectPath
			var item2Path dbus.ObjectPath
			var item3Path dbus.ObjectPath

			call(t, conn.Object(dest, "/org/freedesktop/secrets/aliases/default").Call(METHOD_COLLECTION_CREATE_ITEM, 0,
				map[string]dbus.Variant{
					"org.freedesktop.Secret.Item.Label": dbus.MakeVariant("item1"),
					"org.freedesktop.Secret.Item.Attributes": dbus.MakeVariant(map[string]string{
						"a": t.Name() + "1",
						"b": "2",
						"c": "3",
					}),
				},
				secret{
					Session:     sessionPath,
					Parameters:  []byte{},
					Value:       []byte("ITEM1"),
					ContentType: "text/plain",
				}, false),
				&item1Path, new(dbus.ObjectPath),
			)

			call(t, collection.Call(METHOD_COLLECTION_CREATE_ITEM, 0,
				map[string]dbus.Variant{
					"org.freedesktop.Secret.Item.Label": dbus.MakeVariant("item2"),
					"org.freedesktop.Secret.Item.Attributes": dbus.MakeVariant(map[string]string{
						"a": t.Name() + "1",
						"b": "2",
					}),
				},
				secret{
					Session:     sessionPath,
					Parameters:  []byte{},
					Value:       []byte("ITEM2"),
					ContentType: "text/plain",
				}, false),
				&item2Path, new(dbus.ObjectPath),
			)

			call(t, collection.Call(METHOD_COLLECTION_CREATE_ITEM, 0,
				map[string]dbus.Variant{
					"org.freedesktop.Secret.Item.Label": dbus.MakeVariant("item3"),
					"org.freedesktop.Secret.Item.Attributes": dbus.MakeVariant(map[string]string{
						"a": t.Name() + "2",
						"b": "2",
						"c": "3",
					}),
				},
				secret{
					Session:     sessionPath,
					Parameters:  []byte{},
					Value:       []byte("ITEM3"),
					ContentType: "text/plain",
				}, false),
				&item3Path, new(dbus.ObjectPath),
			)

			var results []dbus.ObjectPath
			attrs := map[string]string{
				"a": t.Name() + "1",
				"b": "2",
			}
			call(t, collection.Call(METHOD_COLLECTION_SEARCH_ITEMS, 0, attrs), &results)

			if !slices.Equal(results, []dbus.ObjectPath{item2Path}) {
				t.Fatal()
			}
		})

		t.Run(METHOD_COLLECTION_CREATE_ITEM, func(t *testing.T) {
			t.Parallel()

			collection := fixture(t)

			call(t, collection.Call(METHOD_COLLECTION_CREATE_ITEM, 0,
				map[string]dbus.Variant{
					"org.freedesktop.Secret.Item.Label": dbus.MakeVariant("item1"),
					"org.freedesktop.Secret.Item.Attributes": dbus.MakeVariant(map[string]string{
						"a": t.Name() + "1",
						"b": "2",
						"c": "3",
					}),
				},
				secret{
					Session:     sessionPath,
					Parameters:  []byte{},
					Value:       []byte("ITEM1"),
					ContentType: "text/plain",
				}, false),
				new(dbus.ObjectPath), new(dbus.ObjectPath),
			)
		})

		t.Run(METHOD_PROPERTIES_GET_ALL, func(t *testing.T) {
			t.Parallel()

			collection := fixture(t)

			var m map[string]dbus.Variant
			call(t, collection.Call(METHOD_PROPERTIES_GET_ALL, 0, IFACE_COLLECTION), &m)

			if !slices.Equal(m["Items"].Value().([]dbus.ObjectPath), []dbus.ObjectPath{}) {
				t.Fatal(m["Items"])
			}
			if m["Label"].Value() != "collection" {
				t.Fatal(m["Label"])
			}
			if m["Locked"].Value() != false {
				t.Fatal(m["Locked"])
			}
			if m["Created"].Value() != uint64(1234) {
				t.Fatal(m["Created"])
			}
			if m["Modified"].Value() != uint64(1234) {
				t.Fatal(m["Modified"])
			}
		})

		t.Run(METHOD_PROPERTIES_GET, func(t *testing.T) {
			t.Run("Items", func(t *testing.T) {
				t.Parallel()

				collection := fixture(t)

				var v dbus.Variant
				call(t, collection.Call(METHOD_PROPERTIES_GET, 0, IFACE_COLLECTION, "Items"), &v)

				expected := []dbus.ObjectPath{ /* todo */ }

				if !slices.Equal(v.Value().([]dbus.ObjectPath), expected) {
					t.Fatal(v, expected)
				}
			})

			t.Run("Label", func(t *testing.T) {
				t.Parallel()

				collection := fixture(t)

				var v dbus.Variant
				call(t, collection.Call(METHOD_PROPERTIES_GET, 0, IFACE_COLLECTION, "Label"), &v)

				expected := "collection"

				if v.Value() != expected {
					t.Fatal(v, expected)
				}
			})

			t.Run("Locked", func(t *testing.T) {
				t.Parallel()

				collection := fixture(t)

				var v dbus.Variant
				call(t, collection.Call(METHOD_PROPERTIES_GET, 0, IFACE_COLLECTION, "Locked"), &v)

				expected := false

				if v.Value() != expected {
					t.Fatal(v, expected)
				}
			})

			t.Run("Created", func(t *testing.T) {
				t.Parallel()

				collection := fixture(t)

				var v dbus.Variant
				call(t, collection.Call(METHOD_PROPERTIES_GET, 0, IFACE_COLLECTION, "Created"), &v)

				expected := uint64(1234)

				if v.Value() != expected {
					t.Fatal(v, expected)
				}
			})

			t.Run("Modified", func(t *testing.T) {
				t.Parallel()

				collection := fixture(t)

				var v dbus.Variant
				call(t, collection.Call(METHOD_PROPERTIES_GET, 0, IFACE_COLLECTION, "Modified"), &v)

				expected := uint64(1234)

				if v.Value() != expected {
					t.Fatal(v, expected)
				}
			})
		})

		t.Run(METHOD_PROPERTIES_SET, func(t *testing.T) {
			t.Run("Items", func(t *testing.T) {
				t.Parallel()

				collection := fixture(t)

				v := dbus.MakeVariant([]dbus.ObjectPath{})
				callErr(t, collection.Call(METHOD_PROPERTIES_SET, 0, IFACE_COLLECTION, "Items", v), prop.ErrReadOnly)
			})

			t.Run("Label", func(t *testing.T) {
				t.Parallel()

				collection := fixture(t)

				v := dbus.MakeVariant("Changed")
				call(t, collection.Call(METHOD_PROPERTIES_SET, 0, IFACE_COLLECTION, "Label", v))

				var v2 dbus.Variant
				call(t, collection.Call(METHOD_PROPERTIES_GET, 0, IFACE_COLLECTION, "Label"), &v2)

				if v != v2 {
					t.Fatal(v2)
				}
			})

			t.Run("Locked", func(t *testing.T) {
				t.Parallel()

				collection := fixture(t)

				v := dbus.MakeVariant(true)
				callErr(t, collection.Call(METHOD_PROPERTIES_SET, 0, IFACE_COLLECTION, "Locked", v), prop.ErrReadOnly)
			})

			t.Run("Created", func(t *testing.T) {
				t.Parallel()

				collection := fixture(t)

				v := dbus.MakeVariant(uint64(0))
				callErr(t, collection.Call(METHOD_PROPERTIES_SET, 0, IFACE_COLLECTION, "Created", v), prop.ErrReadOnly)
			})

			t.Run("Modified", func(t *testing.T) {
				t.Parallel()

				collection := fixture(t)

				v := dbus.MakeVariant(uint64(0))
				callErr(t, collection.Call(METHOD_PROPERTIES_SET, 0, IFACE_COLLECTION, "Modified", v), prop.ErrReadOnly)
			})
		})
	})

	t.Run(IFACE_ITEM, func(t *testing.T) {
		service := conn.Object(dest, "/org/freedesktop/secrets")

		var collectionPath dbus.ObjectPath
		props := map[string]dbus.Variant{
			"org.freedesktop.Secret.Collection.Label": dbus.MakeVariant("collection"),
		}
		call(t, service.Call(METHOD_SERVICE_CREATE_COLLECTION, 0, props, ""),
			&collectionPath, new(dbus.ObjectPath))
		collection := conn.Object(dest, collectionPath)

		var sessionPath dbus.ObjectPath
		call(t, service.Call(METHOD_SERVICE_OPEN_SESSION, 0, "plain", dbus.MakeVariant("")),
			new(dbus.Variant), &sessionPath)

		t.Cleanup(func() {
			session := conn.Object(dest, sessionPath)
			call(t, session.Call(METHOD_SESSION_CLOSE, 0))
		})

		fixture := func(t testing.TB) dbus.BusObject {
			var itemPath dbus.ObjectPath
			call(t, collection.Call(METHOD_COLLECTION_CREATE_ITEM, 0,
				map[string]dbus.Variant{
					"org.freedesktop.Secret.Item.Label": dbus.MakeVariant("item1"),
					"org.freedesktop.Secret.Item.Attributes": dbus.MakeVariant(map[string]string{
						"a": t.Name() + "1",
						"b": "2",
						"c": "3",
					}),
				},
				secret{
					Session:     sessionPath,
					Parameters:  []byte{},
					Value:       []byte("ITEM1"),
					ContentType: "text/plain",
				}, false),
				&itemPath, new(dbus.ObjectPath),
			)

			return conn.Object(dest, itemPath)

		}

		t.Run(METHOD_ITEM_DELETE, func(t *testing.T) {
			t.Parallel()

			item := fixture(t)

			call(t, item.Call(METHOD_ITEM_DELETE, 0), new(dbus.ObjectPath))
			// TODO check
		})

		t.Run(METHOD_ITEM_GET_SECRET, func(t *testing.T) {
			t.Parallel()

			item := fixture(t)

			var s secret
			call(t, item.Call(METHOD_ITEM_GET_SECRET, 0, sessionPath), &s)

			if string(s.Value) != "ITEM1" {
				t.Fatal()
			}
		})

		t.Run(METHOD_ITEM_SET_SECRET, func(t *testing.T) {
			t.Parallel()

			item := fixture(t)

			call(t, item.Call(METHOD_ITEM_SET_SECRET, 0, secret{
				Session:     sessionPath,
				Value:       []byte("CHANGED"),
				Parameters:  []byte{},
				ContentType: "text/plain",
			}))

			var s secret
			call(t, item.Call(METHOD_ITEM_GET_SECRET, 0, sessionPath), &s)

			if string(s.Value) != "CHANGED" {
				t.Fatal()
			}
		})

		t.Run(METHOD_PROPERTIES_GET_ALL, func(t *testing.T) {
			t.Parallel()

			item := fixture(t)

			var m map[string]dbus.Variant
			call(t, item.Call(METHOD_PROPERTIES_GET_ALL, 0, IFACE_ITEM), &m)

			if m["Locked"].Value() != false {
				t.Fatal(m["Locked"])
			}
			if !maps.Equal(m["Attributes"].Value().(map[string]string), map[string]string{
				"a": t.Name() + "1",
				"b": "2",
				"c": "3",
			}) {
				t.Fatal(m["Attributes"])
			}
		})

		t.Run(METHOD_PROPERTIES_GET, func(t *testing.T) {
			t.Run("Locked", func(t *testing.T) {
				t.Parallel()

				item := fixture(t)

				var v dbus.Variant
				call(t, item.Call(METHOD_PROPERTIES_GET, 0, IFACE_ITEM, "Locked"), &v)

				expected := false

				if v.Value() != expected {
					t.Fatal(v, expected)
				}
			})

			t.Run("Attributes", func(t *testing.T) {
				t.Parallel()

				item := fixture(t)

				var v dbus.Variant
				call(t, item.Call(METHOD_PROPERTIES_GET, 0, IFACE_ITEM, "Attributes"), &v)

				expected := map[string]string{
					"a": t.Name() + "1",
					"b": "2",
					"c": "3",
				}

				if !maps.Equal(v.Value().(map[string]string), expected) {
					t.Fatal(v, expected)
				}
			})

			t.Run("Label", func(t *testing.T) {
				t.Parallel()

				item := fixture(t)

				var v dbus.Variant
				call(t, item.Call(METHOD_PROPERTIES_GET, 0, IFACE_ITEM, "Label"), &v)

				expected := "item1"

				if v.Value() != expected {
					t.Fatal(v, expected)
				}
			})

			t.Run("Created", func(t *testing.T) {
				t.Parallel()

				item := fixture(t)

				var v dbus.Variant
				call(t, item.Call(METHOD_PROPERTIES_GET, 0, IFACE_ITEM, "Created"), &v)

				expected := uint64(1234)

				if v.Value() != expected {
					t.Fatal(v, expected)
				}
			})

			t.Run("Modified", func(t *testing.T) {
				t.Parallel()

				item := fixture(t)

				var v dbus.Variant
				call(t, item.Call(METHOD_PROPERTIES_GET, 0, IFACE_ITEM, "Modified"), &v)

				expected := uint64(1234)

				if v.Value() != expected {
					t.Fatal(v, expected)
				}
			})
		})

		t.Run(METHOD_PROPERTIES_SET, func(t *testing.T) {
			t.Run("Locked", func(t *testing.T) {
				t.Parallel()

				item := fixture(t)

				v := dbus.MakeVariant(true)
				callErr(t, item.Call(METHOD_PROPERTIES_SET, 0, IFACE_ITEM, "Locked", v), prop.ErrReadOnly)
			})

			t.Run("Attributes", func(t *testing.T) {
				t.Parallel()

				item := fixture(t)

				v := dbus.MakeVariant(map[string]string{
					"a": t.Name() + "2",
					"b": "2",
					"c": "3",
				})
				call(t, item.Call(METHOD_PROPERTIES_SET, 0, IFACE_ITEM, "Attributes", v))

				var v2 dbus.Variant
				call(t, item.Call(METHOD_PROPERTIES_GET, 0, IFACE_ITEM, "Attributes"), &v2)

				if !maps.Equal(v.Value().(map[string]string), v2.Value().(map[string]string)) {
					t.Fatal(v, v2)
				}
			})

			t.Run("Label", func(t *testing.T) {
				t.Parallel()

				item := fixture(t)

				v := dbus.MakeVariant("changed")
				call(t, item.Call(METHOD_PROPERTIES_SET, 0, IFACE_ITEM, "Label", v))

				var v2 dbus.Variant
				call(t, item.Call(METHOD_PROPERTIES_GET, 0, IFACE_ITEM, "Label"), &v2)

				if v != v2 {
					t.Fatal(v, v2)
				}
			})

			t.Run("Created", func(t *testing.T) {
				t.Parallel()

				item := fixture(t)

				v := dbus.MakeVariant(true)
				callErr(t, item.Call(METHOD_PROPERTIES_SET, 0, IFACE_ITEM, "Created", v), prop.ErrReadOnly)
			})

			t.Run("Modified", func(t *testing.T) {
				t.Parallel()

				item := fixture(t)

				v := dbus.MakeVariant(true)
				callErr(t, item.Call(METHOD_PROPERTIES_SET, 0, IFACE_ITEM, "Modified", v), prop.ErrReadOnly)
			})
		})
	})

	t.Run(IFACE_SESSION, func(t *testing.T) {
		tests := map[string]func(t *testing.T, obj dbus.BusObject){
			METHOD_SESSION_CLOSE: func(t *testing.T, obj dbus.BusObject) {
				// call cleanup
			},

			METHOD_PROPERTIES_GET_ALL: func(t *testing.T, obj dbus.BusObject) {
				var r map[string]*dbus.Variant
				call(t, obj.Call(METHOD_PROPERTIES_GET_ALL, 0, IFACE_SESSION), &r)

				if !maps.Equal(r, map[string]*dbus.Variant{}) {
					t.Fatal()
				}
			},

			METHOD_PROPERTIES_GET: func(t *testing.T, obj dbus.BusObject) {
			},

			METHOD_PROPERTIES_SET: func(t *testing.T, obj dbus.BusObject) {
			},
		}

		for name, test := range tests {
			service := conn.Object(dest, "/org/freedesktop/secrets")

			t.Run(name, func(t *testing.T) {
				t.Parallel()

				var sessionPath dbus.ObjectPath
				call(t, service.Call(METHOD_SERVICE_OPEN_SESSION, 0, "plain", dbus.MakeVariant("")),
					new(dbus.Variant), &sessionPath)

				obj := conn.Object(dest, sessionPath)
				t.Cleanup(func() {
					call(t, obj.Call(METHOD_SESSION_CLOSE, 0))
				})

				test(t, obj)
			})
		}
	})
}
