package backup

import (
	"context"
	"encoding/json"
	"github.com/AlviDervishaj/junior-assistant/src/model"
	"github.com/AlviDervishaj/junior-assistant/src/projects"
	"github.com/AlviDervishaj/junior-assistant/src/records"
	"github.com/AlviDervishaj/junior-assistant/src/storage"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestRoundTripRemappingAndEmptyGuard(t *testing.T) {
	ctx := context.Background()
	s, e := storage.Open(ctx, t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	old := t.TempDir()
	e = s.Update(ctx, func(st *model.State) error {
		p, e := projects.Add(st, "app", old)
		if e != nil {
			return e
		}
		if _, e = projects.AddRoot(st, filepath.Join(old, "personal"), []string{"cache"}); e != nil {
			return e
		}
		r, e := records.Add(st, model.Record{Title: "fix", Type: "bug", ProjectID: p.ID, Due: "2026-10-04"})
		if e != nil {
			return e
		}
		if e = records.Delete(st, r.ID); e != nil {
			return e
		}
		if _, e = records.Add(st, model.Record{Title: "retained", ProjectID: p.ID}); e != nil {
			return e
		}
		yes := true
		_, e = projects.Edit(st, "app", "", &yes)
		st.Preferences["display"] = "plain"
		return e
	})
	if e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "backup.json")
	if e = Export(ctx, s, path); e != nil {
		t.Fatal(e)
	}
	if e = Export(ctx, s, path); e == nil {
		t.Fatal("overwrote backup")
	}
	target, e := storage.Open(ctx, t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer target.Close()
	newer := t.TempDir()
	warnings, e := Restore(ctx, target, path, map[string]string{old: newer})
	if e != nil {
		t.Fatal(e)
	}
	if len(warnings) == 0 {
		t.Fatal("missing independent path did not warn")
	}
	original, _ := s.Read(ctx)
	want := original
	if e = Remap(&want, map[string]string{old: newer}); e != nil {
		t.Fatal(e)
	}
	got, _ := target.Read(ctx)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%+v != %+v", got, want)
	}
	if _, e = Restore(ctx, target, path, nil); e == nil {
		t.Fatal("restored into nonempty state")
	}
	after, _ := target.Read(ctx)
	if !reflect.DeepEqual(got, after) {
		t.Fatal("failed restore mutated data")
	}
	e = target.Update(ctx, func(st *model.State) error {
		r, e := records.Add(st, model.Record{Title: "next"})
		if r.ID != 3 {
			t.Errorf("reused ID %d", r.ID)
		}
		return e
	})
	if e != nil {
		t.Fatal(e)
	}
}
func TestInvalidBackupLeavesDatabaseUnchanged(t *testing.T) {
	ctx := context.Background()
	db, e := storage.Open(ctx, t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	before, _ := db.Read(ctx)
	invalid := []string{`{"version":2,"state":{}}`, `{"version":1,"state":{}}`, `{"version":1,"state":{},"unknown":1}`, `{} {}`}
	st := model.Empty()
	st.Records = append(st.Records, model.Record{ID: 1, Title: "bad owner", Type: "task", Status: "open", ProjectID: 999})
	st.NextRecord = 2
	b, _ := json.Marshal(Document{1, st})
	invalid = append(invalid, string(b))
	for _, body := range invalid {
		p := filepath.Join(t.TempDir(), "bad.json")
		os.WriteFile(p, []byte(body), 0600)
		if _, e = Restore(ctx, db, p, nil); e == nil {
			t.Fatal("invalid backup accepted")
		}
		after, _ := db.Read(ctx)
		if !reflect.DeepEqual(before, after) {
			t.Fatal("invalid backup mutated state")
		}
	}
	if _, e = Decode(strings.NewReader(`{"version":1,"state":{}}`)); e == nil {
		t.Fatal("missing counters accepted")
	}
}
