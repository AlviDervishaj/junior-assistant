package model

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

type Project struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Path     string `json:"path"`
	Archived bool   `json:"archived"`
}
type Root struct {
	ID         int64    `json:"id"`
	Path       string   `json:"path"`
	ProjectID  int64    `json:"project_id,omitempty"`
	Exclusions []string `json:"exclusions"`
}
type ProgressLog struct {
	ID        int64  `json:"id"`
	CreatedAt string `json:"created_at"`
	Message   string `json:"message"`
}

type Record struct {
	ID        int64         `json:"id"`
	Type      string        `json:"type"`
	ProjectID int64         `json:"project_id,omitempty"`
	Title     string        `json:"title"`
	Notes     string        `json:"notes"`
	Status    string        `json:"status"`
	Planned   string        `json:"planned_date,omitempty"`
	Due       string        `json:"due_date,omitempty"`
	Logs      []ProgressLog `json:"progress_logs,omitempty"`
}

func (r Record) Active() bool { return r.Status == "open" || r.Status == "in-progress" }

// Counters are retained even after permanent deletion so IDs are never reused.
type State struct {
	Projects    []Project         `json:"projects"`
	Roots       []Root            `json:"roots"`
	Records     []Record          `json:"records"`
	Preferences map[string]string `json:"preferences"`
	NextProject int64             `json:"next_project_id"`
	NextRoot    int64             `json:"next_root_id"`
	NextRecord  int64             `json:"next_record_id"`
}

func Empty() State {
	return State{Projects: []Project{}, Roots: []Root{}, Records: []Record{}, Preferences: map[string]string{}, NextProject: 1, NextRoot: 1, NextRecord: 1}
}
func Date(s string) error {
	if s == "" {
		return nil
	}
	t, e := time.Parse("2006-01-02", s)
	if e != nil || t.Format("2006-01-02") != s {
		return fmt.Errorf("invalid date %q: use YYYY-MM-DD", s)
	}
	return nil
}
func ValidRecord(r Record) error {
	if strings.TrimSpace(r.Title) == "" {
		return errors.New("title must not be empty")
	}
	if r.Type != "task" && r.Type != "bug" {
		return errors.New("type must be task or bug")
	}
	switch r.Status {
	case "open", "in-progress", "done", "cancelled":
	default:
		return errors.New("invalid record status")
	}
	if err := Date(r.Planned); err != nil {
		return err
	}
	if e := Date(r.Due); e != nil {
		return e
	}
	for i, entry := range r.Logs {
		if entry.ID != int64(i+1) || strings.TrimSpace(entry.Message) == "" {
			return errors.New("invalid progress log identity or message")
		}
		if _, e := time.Parse(time.RFC3339Nano, entry.CreatedAt); e != nil {
			return errors.New("invalid progress log timestamp")
		}
	}
	return nil
}
func Name(s string) bool { return strings.TrimSpace(s) != "" && !strings.ContainsAny(s, "/\\\x00\n\r") }
func ValidExclusions(xs []string) error {
	for _, s := range xs {
		if !Name(s) || s == "." || s == ".." {
			return fmt.Errorf("invalid exclusion %q: use a directory basename", s)
		}
	}
	return nil
}
func (s State) Validate() error {
	ps := map[int64]Project{}
	names := []string{}
	paths := map[string]bool{}
	for _, p := range s.Projects {
		if p.ID <= 0 || p.ID >= s.NextProject || !Name(p.Name) || !absolute(p.Path) {
			return errors.New("invalid project identity, path, or counter")
		}
		duplicateName := false
		for _, name := range names {
			if strings.EqualFold(name, p.Name) {
				duplicateName = true
				break
			}
		}
		if _, ok := ps[p.ID]; ok || duplicateName {
			return errors.New("duplicate project identity or name")
		}
		if !p.Archived && paths[p.Path] {
			return errors.New("duplicate active project path")
		}
		if !p.Archived {
			paths[p.Path] = true
		}
		ps[p.ID] = p
		names = append(names, p.Name)
	}
	roots := map[int64]bool{}
	owned := map[int64]bool{}
	independent := map[string]bool{}
	for _, r := range s.Roots {
		if r.ID <= 0 || r.ID >= s.NextRoot || roots[r.ID] || !absolute(r.Path) {
			return errors.New("invalid or duplicate search root")
		}
		roots[r.ID] = true
		if err := ValidExclusions(r.Exclusions); err != nil {
			return err
		}
		if r.ProjectID != 0 {
			p, ok := ps[r.ProjectID]
			if !ok || owned[p.ID] || r.Path != p.Path {
				return errors.New("invalid project search registration")
			}
			owned[p.ID] = true
		} else {
			if independent[r.Path] {
				return errors.New("duplicate independent root")
			}
			independent[r.Path] = true
		}
	}
	for _, p := range s.Projects {
		if !owned[p.ID] {
			return errors.New("project is missing its search registration")
		}
	}
	ids := map[int64]bool{}
	for _, r := range s.Records {
		if r.ID <= 0 || r.ID >= s.NextRecord || ids[r.ID] {
			return errors.New("invalid or duplicate record identity")
		}
		ids[r.ID] = true
		if r.ProjectID != 0 {
			if _, ok := ps[r.ProjectID]; !ok {
				return errors.New("record references unknown project")
			}
		}
		if err := ValidRecord(r); err != nil {
			return err
		}
	}
	if s.NextProject < 1 || s.NextRoot < 1 || s.NextRecord < 1 {
		return errors.New("invalid ID counters")
	}
	if s.Preferences == nil {
		return errors.New("preferences must be an object")
	}
	return nil
}
func absolute(s string) bool {
	return filepath.IsAbs(s) && filepath.Clean(s) == s && !strings.ContainsRune(s, 0)
}
