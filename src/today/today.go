package today

import (
	"github.com/AlviDervishaj/junior-assistant/src/model"
	"github.com/AlviDervishaj/junior-assistant/src/records"
	"sort"
	"time"
)

type Group struct {
	Category string         `json:"category"`
	Records  []model.Record `json:"records"`
}

func View(s model.State, scope records.Scope, now time.Time) []Group {
	date := now.Format("2006-01-02")
	gs := []Group{{"overdue", []model.Record{}}, {"due-today", []model.Record{}}, {"missed-plan", []model.Record{}}, {"planned-today", []model.Record{}}, {"in-progress", []model.Record{}}}
	for _, r := range records.List(s, scope, false) {
		i := -1
		switch {
		case r.Due != "" && r.Due < date:
			i = 0
		case r.Due == date:
			i = 1
		case r.Planned != "" && r.Planned < date:
			i = 2
		case r.Planned == date:
			i = 3
		case r.Status == "in-progress":
			i = 4
		}
		if i >= 0 {
			gs[i].Records = append(gs[i].Records, r)
		}
	}
	for i := range gs {
		sort.Slice(gs[i].Records, func(a, b int) bool {
			x, y := gs[i].Records[a], gs[i].Records[b]
			var xd, yd string
			if i < 2 {
				xd, yd = x.Due, y.Due
			} else if i < 4 {
				xd, yd = x.Planned, y.Planned
			}
			if xd != yd {
				return xd < yd
			}
			return x.ID < y.ID
		})
	}
	return gs
}
