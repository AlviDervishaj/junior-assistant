package storage

import (
	"context"
	"database/sql"
	"github.com/AlviDervishaj/junior-assistant/src/model"
	"github.com/AlviDervishaj/junior-assistant/src/records"
	"path/filepath"
	"sync"
	"testing"
)

func TestTransactionsConcurrentIDsAndReopen(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s, e := Open(ctx, dir)
	if e != nil {
		t.Fatal(e)
	}
	other, e := Open(ctx, dir)
	if e != nil {
		t.Fatal(e)
	}
	defer other.Close()
	var wg sync.WaitGroup
	errs := make(chan error, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			db := s
			if i%2 == 0 {
				db = other
			}
			errs <- db.Update(ctx, func(state *model.State) error { _, e := records.Add(state, model.Record{Title: "work"}); return e })
		}(i)
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	state, e := s.Read(ctx)
	if e != nil || len(state.Records) != 12 || state.NextRecord != 13 {
		t.Fatalf("%+v %v", state, e)
	}
	e = s.Update(ctx, func(state *model.State) error { state.Records[0].Status = "invalid"; return nil })
	if e == nil {
		t.Fatal("invalid update succeeded")
	}
	s.Close()
	s, e = Open(ctx, dir)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	state, e = s.Read(ctx)
	if e != nil || state.Records[0].Status != "open" {
		t.Fatalf("rollback failed %+v %v", state, e)
	}
}
func TestRejectFutureSchemaWithoutChanges(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s, e := Open(ctx, dir)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Update(ctx, func(state *model.State) error { _, e := records.Add(state, model.Record{Title: "retained"}); return e }); e != nil {
		t.Fatal(e)
	}
	s.db.Exec("PRAGMA user_version=2")
	s.Close()
	if s, e = Open(ctx, dir); e == nil {
		s.Close()
		t.Fatal("opened newer schema")
	}
	db, e := sql.Open("sqlite", filepath.Join(dir, "assistant.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	var title string
	if e = db.QueryRow("SELECT title FROM records").Scan(&title); e != nil || title != "retained" {
		t.Fatalf("data changed %s %v", title, e)
	}
}

func TestWriteFailureRollsBackAllTables(t *testing.T) {
	ctx := context.Background()
	s, e := Open(ctx, t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	e = s.Update(ctx, func(state *model.State) error { _, e := records.Add(state, model.Record{Title: "original"}); return e })
	if e != nil {
		t.Fatal(e)
	}
	_, e = s.db.Exec(`CREATE TRIGGER refuse_record BEFORE INSERT ON records WHEN NEW.title='refused' BEGIN SELECT RAISE(ABORT,'injected write failure'); END;`)
	if e != nil {
		t.Fatal(e)
	}
	e = s.Update(ctx, func(state *model.State) error {
		state.Records[0].Title = "refused"
		state.Preferences["changed"] = "yes"
		return nil
	})
	if e == nil {
		t.Fatal("write failure did not propagate")
	}
	state, e := s.Read(ctx)
	if e != nil || state.Records[0].Title != "original" || len(state.Preferences) != 0 {
		t.Fatalf("partial write persisted %+v %v", state, e)
	}
}
func TestConcurrentInitialization(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	var wg sync.WaitGroup
	errs := make(chan error, 6)
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, e := Open(ctx, dir)
			if e == nil {
				s.Close()
			}
			errs <- e
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
}
