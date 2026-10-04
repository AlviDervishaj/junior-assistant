package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/AlviDervishaj/junior-assistant/src/model"
	_ "modernc.org/sqlite"
)

type Store struct{ db *sql.DB }

func Open(ctx context.Context, dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "assistant.db")
	// Precreate with private permissions rather than letting SQLite use umask.
	f, e := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return nil, e
	}
	if e = f.Close(); e != nil {
		return nil, e
	}
	u := url.URL{Scheme: "file", Path: path}
	query := url.Values{}
	query.Add("_pragma", "foreign_keys(1)")
	query.Add("_pragma", "busy_timeout(5000)")
	u.RawQuery = query.Encode()
	db, e := sql.Open("sqlite", u.String())
	if e != nil {
		return nil, e
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	conn, e := db.Conn(ctx)
	if e != nil {
		db.Close()
		return nil, e
	}
	if _, e = conn.ExecContext(ctx, "BEGIN IMMEDIATE"); e != nil {
		conn.Close()
		db.Close()
		return nil, e
	}
	defer func() { conn.ExecContext(context.Background(), "ROLLBACK"); conn.Close() }()
	var version int
	e = conn.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version)
	if e == nil && version > 1 {
		e = fmt.Errorf("database schema %d is newer than supported schema 1", version)
	}
	if e == nil && version == 0 {
		_, e = conn.ExecContext(ctx, `CREATE TABLE counters (singleton INTEGER PRIMARY KEY CHECK(singleton=1), project INTEGER NOT NULL, root INTEGER NOT NULL, record INTEGER NOT NULL);
INSERT INTO counters VALUES(1,1,1,1);
CREATE TABLE projects (id INTEGER PRIMARY KEY, name TEXT NOT NULL UNIQUE COLLATE NOCASE, path TEXT NOT NULL, archived INTEGER NOT NULL);
CREATE UNIQUE INDEX active_project_path ON projects(path) WHERE archived=0;
CREATE TABLE roots (id INTEGER PRIMARY KEY, path TEXT NOT NULL, project_id INTEGER REFERENCES projects(id), exclusions TEXT NOT NULL);
CREATE UNIQUE INDEX project_root ON roots(project_id) WHERE project_id IS NOT NULL;
CREATE UNIQUE INDEX independent_root ON roots(path) WHERE project_id IS NULL;
CREATE TABLE records (id INTEGER PRIMARY KEY, type TEXT NOT NULL, project_id INTEGER REFERENCES projects(id), title TEXT NOT NULL, notes TEXT NOT NULL, status TEXT NOT NULL, planned TEXT NOT NULL, due TEXT NOT NULL);
CREATE TABLE preferences (key TEXT PRIMARY KEY, value TEXT NOT NULL);
PRAGMA user_version=1;`)
	}
	if e == nil {
		_, e = conn.ExecContext(ctx, "COMMIT")
	}
	if e != nil {
		conn.ExecContext(context.Background(), "ROLLBACK")
		conn.Close()
		db.Close()
		return nil, e
	}
	return s, nil
}
func (s *Store) Close() error { return s.db.Close() }

type queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func load(ctx context.Context, tx queryer) (model.State, error) {
	state := model.Empty()
	err := tx.QueryRowContext(ctx, "SELECT project,root,record FROM counters WHERE singleton=1").Scan(&state.NextProject, &state.NextRoot, &state.NextRecord)
	if err != nil {
		return state, err
	}
	rows, err := tx.QueryContext(ctx, "SELECT id,name,path,archived FROM projects ORDER BY id")
	if err != nil {
		return state, err
	}
	for rows.Next() {
		var p model.Project
		if err = rows.Scan(&p.ID, &p.Name, &p.Path, &p.Archived); err != nil {
			rows.Close()
			return state, err
		}
		state.Projects = append(state.Projects, p)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return state, err
	}
	rows, err = tx.QueryContext(ctx, "SELECT id,path,COALESCE(project_id,0),exclusions FROM roots ORDER BY id")
	if err != nil {
		return state, err
	}
	for rows.Next() {
		var r model.Root
		var x string
		if err = rows.Scan(&r.ID, &r.Path, &r.ProjectID, &x); err == nil {
			err = json.Unmarshal([]byte(x), &r.Exclusions)
		}
		if err != nil {
			rows.Close()
			return state, err
		}
		state.Roots = append(state.Roots, r)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return state, err
	}
	rows, err = tx.QueryContext(ctx, "SELECT id,type,COALESCE(project_id,0),title,notes,status,planned,due FROM records ORDER BY id")
	if err != nil {
		return state, err
	}
	for rows.Next() {
		var r model.Record
		if err = rows.Scan(&r.ID, &r.Type, &r.ProjectID, &r.Title, &r.Notes, &r.Status, &r.Planned, &r.Due); err != nil {
			rows.Close()
			return state, err
		}
		state.Records = append(state.Records, r)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return state, err
	}
	rows, err = tx.QueryContext(ctx, "SELECT key,value FROM preferences")
	if err != nil {
		return state, err
	}
	for rows.Next() {
		var k, v string
		if err = rows.Scan(&k, &v); err != nil {
			rows.Close()
			return state, err
		}
		state.Preferences[k] = v
	}
	err = rows.Err()
	rows.Close()
	return state, err
}
func (s *Store) Read(ctx context.Context) (model.State, error) {
	tx, e := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if e != nil {
		return model.State{}, e
	}
	defer tx.Rollback()
	return load(ctx, tx)
}

// Update uses one write transaction and a domain snapshot. V1's small personal
// dataset favors a simple atomic replacement over a separate CRUD abstraction.
func (s *Store) Update(ctx context.Context, fn func(*model.State) error) error {
	conn, e := s.db.Conn(ctx)
	if e != nil {
		return e
	}
	defer conn.Close()
	// Immediate transaction ensures competing CLIs cannot read the same counters.
	if _, e = conn.ExecContext(ctx, "BEGIN IMMEDIATE"); e != nil {
		return e
	}
	defer conn.ExecContext(context.Background(), "ROLLBACK")
	// All reads and writes must use the reserved connection. Share loading via a
	// small query interface instead of opening a second transaction.
	state, e := load(ctx, conn)
	if e != nil {
		return e
	}
	if e = fn(&state); e != nil {
		return e
	}
	if e = state.Validate(); e != nil {
		return e
	}
	if e = save(ctx, conn, state); e != nil {
		return e
	}
	_, e = conn.ExecContext(ctx, "COMMIT")
	return e
}
func nullable(id int64) any {
	if id == 0 {
		return nil
	}
	return id
}
func save(ctx context.Context, c *sql.Conn, s model.State) error {
	if _, e := c.ExecContext(ctx, "DELETE FROM records; DELETE FROM roots; DELETE FROM projects; DELETE FROM preferences;"); e != nil {
		return e
	}
	for _, p := range s.Projects {
		if _, e := c.ExecContext(ctx, "INSERT INTO projects VALUES(?,?,?,?)", p.ID, p.Name, p.Path, p.Archived); e != nil {
			return e
		}
	}
	for _, r := range s.Roots {
		xs, e := json.Marshal(r.Exclusions)
		if e != nil {
			return e
		}
		if _, e = c.ExecContext(ctx, "INSERT INTO roots VALUES(?,?,?,?)", r.ID, r.Path, nullable(r.ProjectID), string(xs)); e != nil {
			return e
		}
	}
	for _, r := range s.Records {
		if _, e := c.ExecContext(ctx, "INSERT INTO records VALUES(?,?,?,?,?,?,?,?)", r.ID, r.Type, nullable(r.ProjectID), r.Title, r.Notes, r.Status, r.Planned, r.Due); e != nil {
			return e
		}
	}
	for k, v := range s.Preferences {
		if _, e := c.ExecContext(ctx, "INSERT INTO preferences VALUES(?,?)", k, v); e != nil {
			return e
		}
	}
	_, e := c.ExecContext(ctx, "UPDATE counters SET project=?,root=?,record=? WHERE singleton=1", s.NextProject, s.NextRoot, s.NextRecord)
	return e
}
