package projects

import (
	"github.com/AlviDervishaj/junior-assistant/src/model"
	"github.com/AlviDervishaj/junior-assistant/src/records"
	"path/filepath"
	"testing"
)

func TestRegistrationOwnershipAndReconnection(t *testing.T) {
	s := model.Empty()
	dir := t.TempDir()
	outer, e := Add(&s, "outer", dir)
	if e != nil {
		t.Fatal(e)
	}
	inner, e := Add(&s, "inner", filepath.Join(outer.Path, "child"))
	if e != nil {
		t.Fatal(e)
	}
	_, e = AddRoot(&s, inner.Path, []string{"generated"})
	if e != nil {
		t.Fatal(e)
	}
	r, e := records.Add(&s, model.Record{Title: "owned", ProjectID: inner.ID})
	if e != nil {
		t.Fatal(e)
	}
	if got := Infer(s, filepath.Join(inner.Path, "deep")); got != inner.ID {
		t.Fatalf("nearest ancestor %d", got)
	}
	if got := Infer(s, outer.Path+"-sibling"); got != 0 {
		t.Fatal("matched a path prefix instead of ancestor")
	}
	archived := true
	if _, e = Edit(&s, "inner", "", &archived); e != nil {
		t.Fatal(e)
	}
	if got := Infer(s, inner.Path); got != outer.ID {
		t.Fatal("inferred archived project")
	}
	if len(ActiveRoots(s)) != 2 {
		t.Fatal("archive disabled independent root")
	}
	moved := filepath.Join(outer.Path, "moved")
	if _, e = Edit(&s, "inner", moved, nil); e != nil {
		t.Fatal(e)
	}
	if stored, _ := records.Find(s, r.ID); stored.ProjectID != inner.ID {
		t.Fatal("path change lost ownership")
	}
	archived = false
	if _, e = Edit(&s, "inner", "", &archived); e != nil {
		t.Fatal(e)
	}
	if Infer(s, moved) != inner.ID || len(ActiveRoots(s)) != 3 {
		t.Fatal("restore did not reactivate project")
	}
	if _, e = Add(&s, "INNER", filepath.Join(dir, "other")); e == nil {
		t.Fatal("duplicate project name accepted")
	}
}
