package today

import (
	"github.com/AlviDervishaj/junior-assistant/src/model"
	"github.com/AlviDervishaj/junior-assistant/src/records"
	"reflect"
	"testing"
	"time"
)

func TestCategoryPrecedenceDateOrderAndScope(t *testing.T) {
	s := model.Empty()
	s.Records = []model.Record{
		{ID: 2, Status: "in-progress", Due: "2026-10-03", Planned: "2026-10-01", ProjectID: 1},
		{ID: 1, Status: "open", Due: "2026-10-02"},
		{ID: 3, Status: "open", Due: "2026-10-04", Planned: "2026-10-01"},
		{ID: 4, Status: "open", Due: "2026-10-05", Planned: "2026-10-03"},
		{ID: 5, Status: "in-progress", Planned: "2026-10-04"},
		{ID: 6, Status: "in-progress", Due: "2026-10-05"},
		{ID: 7, Status: "open"}, {ID: 8, Status: "done", Due: "2026-10-01"}, {ID: 9, Status: "cancelled", Planned: "2026-10-04"},
	}
	// It is still Oct 3 UTC, but Oct 4 in the user's timezone.
	zone := time.FixedZone("local", 2*60*60)
	now := time.Date(2026, 10, 3, 23, 0, 0, 0, time.UTC).In(zone)
	gs := View(s, records.Scope{}, now)
	ids := [][]int64{}
	for _, g := range gs {
		x := []int64{}
		for _, r := range g.Records {
			x = append(x, r.ID)
		}
		ids = append(ids, x)
	}
	want := [][]int64{{1, 2}, {3}, {4}, {5}, {6}}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("%v want %v", ids, want)
	}
	gs = View(s, records.Scope{ProjectID: 1}, now)
	if len(gs[0].Records) != 1 || gs[0].Records[0].ID != 2 {
		t.Fatal("project filter failed")
	}
	gs = View(s, records.Scope{Personal: true}, now)
	if len(gs[0].Records) != 1 || gs[0].Records[0].ID != 1 {
		t.Fatal("personal filter failed")
	}
}
