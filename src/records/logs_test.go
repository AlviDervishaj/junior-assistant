package records

import (
	"github.com/AlviDervishaj/junior-assistant/src/model"
	"testing"
	"time"
)

func TestAppendLogPreservesRecordAndOrdersEqualTimestamps(t *testing.T) {
	s := model.Empty()
	r, e := Add(&s, model.Record{Title: "bug", Type: "bug", Notes: "original"})
	if e != nil {
		t.Fatal(e)
	}
	done := "done"
	Edit(&s, r.ID, Patch{Status: &done})
	now := time.Date(2026, 10, 4, 21, 0, 0, 0, time.FixedZone("local", 2*3600))
	first, e := Log(&s, r.ID, "Reproduced", now)
	if e != nil {
		t.Fatal(e)
	}
	second, e := Log(&s, r.ID, "Fixed", now)
	if e != nil {
		t.Fatal(e)
	}
	if first.ID != 1 || second.ID != 2 || first.CreatedAt != "2026-10-04T19:00:00Z" {
		t.Fatalf("unexpected history %+v %+v", first, second)
	}
	got, _ := Find(s, r.ID)
	if got.Notes != "original" || got.Status != "done" || got.Type != "bug" {
		t.Fatal("log changed record")
	}
	if _, e = Log(&s, r.ID, " \n ", now); e == nil {
		t.Fatal("blank message accepted")
	}
	if _, e = Log(&s, 999, "bad", now); e == nil {
		t.Fatal("unknown record accepted")
	}
	if logs, _ := Logs(s, r.ID); len(logs) != 2 {
		t.Fatal("invalid append changed history")
	}
	if e = Delete(&s, r.ID); e != nil {
		t.Fatal(e)
	}
	if _, e = Logs(s, r.ID); e == nil {
		t.Fatal("deleted record retained history")
	}
}
