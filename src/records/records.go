package records

import (
	"fmt"
	"github.com/AlviDervishaj/junior-assistant/src/model"
)

type Scope struct {
	ProjectID int64
	Personal  bool
}

func (s Scope) Matches(r model.Record) bool {
	if s.Personal {
		return r.ProjectID == 0
	}
	return s.ProjectID == 0 || r.ProjectID == s.ProjectID
}
func List(s model.State, scope Scope, finished bool) []model.Record {
	xs := []model.Record{}
	for _, r := range s.Records {
		if scope.Matches(r) && (finished || r.Active()) {
			xs = append(xs, r)
		}
	}
	return xs
}
func Find(s model.State, id int64) (model.Record, error) {
	for _, r := range s.Records {
		if r.ID == id {
			return r, nil
		}
	}
	return model.Record{}, fmt.Errorf("record %d not found", id)
}
func Add(s *model.State, r model.Record) (model.Record, error) {
	if r.Type == "" {
		r.Type = "task"
	}
	r.Status = "open"
	if e := model.ValidRecord(r); e != nil {
		return r, e
	}
	r.ID = s.NextRecord
	s.NextRecord++
	s.Records = append(s.Records, r)
	return r, s.Validate()
}

type Patch struct {
	Title, Notes, Type, Planned, Due, Status *string
	ProjectID                                *int64
}

func Edit(s *model.State, id int64, p Patch) (model.Record, error) {
	r, e := Find(*s, id)
	if e != nil {
		return r, e
	}
	if p.Title != nil {
		r.Title = *p.Title
	}
	if p.Notes != nil {
		r.Notes = *p.Notes
	}
	if p.Type != nil {
		r.Type = *p.Type
	}
	if p.Planned != nil {
		r.Planned = *p.Planned
	}
	if p.Due != nil {
		r.Due = *p.Due
	}
	if p.Status != nil {
		r.Status = *p.Status
	}
	if p.ProjectID != nil {
		r.ProjectID = *p.ProjectID
	}
	if e = model.ValidRecord(r); e != nil {
		return r, e
	}
	for i, old := range s.Records {
		if old.ID == id {
			s.Records[i] = r
		}
	}
	return r, s.Validate()
}
func Delete(s *model.State, id int64) error {
	for i, r := range s.Records {
		if r.ID == id {
			s.Records = append(s.Records[:i], s.Records[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("record %d not found", id)
}
