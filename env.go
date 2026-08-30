package smss

import (
	"database/sql"
	"sync"
	"sync/atomic"

	"github.com/godbus/dbus/v5"
)

type Env struct {
	mu       *sync.Mutex
	seq      *atomic.Uint32
	sessions map[dbus.ObjectPath]*session
	db       *sql.DB
	conn     *dbus.Conn
}

func NewEnv(db *sql.DB, conn *dbus.Conn) *Env {
	sessions := make(map[dbus.ObjectPath]*session)

	return &Env{
		mu:       new(sync.Mutex),
		seq:      new(atomic.Uint32),
		sessions: sessions,
		db:       db,
		conn:     conn,
	}
}

func (e *Env) lookupSession(p dbus.ObjectPath) (*session, func(), bool) {
	e.mu.Lock()
	s, found := e.sessions[p]
	if !found {
		return nil, nil, false
	}
	return s, e.mu.Unlock, true
}

func (e *Env) putSession(factory func(*Env, uint32) *session) *session {
	e.mu.Lock()
	defer e.mu.Unlock()

	s := factory(e, e.seq.Add(1))

	_, found := e.sessions[s.path]
	if found {
		panic("Session already exists")
	}
	e.sessions[s.path] = s

	return s
}

func (e *Env) removeSession(p dbus.ObjectPath) {
	e.mu.Lock()
	defer e.mu.Unlock()

	delete(e.sessions, p)
}
