package backup

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/AlviDervishaj/junior-assistant/src/config"
	"github.com/AlviDervishaj/junior-assistant/src/model"
	"github.com/AlviDervishaj/junior-assistant/src/projects"
	"github.com/AlviDervishaj/junior-assistant/src/storage"
)

type Document struct {
	Version int         `json:"version"`
	State   model.State `json:"state"`
}

func Export(ctx context.Context, s *storage.Store, path string) error {
	state, e := s.Read(ctx)
	if e != nil {
		return e
	}
	if e = state.Validate(); e != nil {
		return e
	}
	bytes, e := json.MarshalIndent(Document{1, state}, "", "  ")
	if e != nil {
		return e
	}
	bytes = append(bytes, '\n')
	// Write privately to a temporary file and link without replacing an existing
	// destination. A crash cannot leave a seemingly complete partial backup.
	dir := filepath.Dir(path)
	f, e := os.CreateTemp(dir, ".assistant-backup-*")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	if _, e = f.Write(bytes); e != nil {
		f.Close()
		return e
	}
	if e = f.Sync(); e != nil {
		f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	return os.Link(f.Name(), path)
}
func Decode(r io.Reader) (model.State, error) {
	var doc Document
	d := json.NewDecoder(r)
	d.DisallowUnknownFields()
	if e := d.Decode(&doc); e != nil {
		return model.State{}, e
	}
	var extra any
	if e := d.Decode(&extra); e != io.EOF {
		return model.State{}, errors.New("backup must contain exactly one JSON document")
	}
	if doc.Version != 1 {
		return model.State{}, fmt.Errorf("unsupported backup version %d", doc.Version)
	}
	if e := doc.State.Validate(); e != nil {
		return model.State{}, e
	}
	return doc.State, nil
}

// Remap replaces longest matching path prefixes, once per original path.
func Remap(state *model.State, mappings map[string]string) error {
	keys := []string{}
	values := map[string]string{}
	for from, to := range mappings {
		if !filepath.IsAbs(from) || filepath.Clean(from) != from || to == "" {
			return fmt.Errorf("remap source must be an absolute clean path and destination must not be empty")
		}
		p, e := config.Canonical(to)
		if e != nil {
			return e
		}
		keys = append(keys, from)
		values[from] = p
	}
	sort.Slice(keys, func(i, j int) bool {
		if len(keys[i]) != len(keys[j]) {
			return len(keys[i]) > len(keys[j])
		}
		return keys[i] < keys[j]
	})
	apply := func(p string) string {
		for _, from := range keys {
			if projects.Within(from, p) {
				rel, _ := filepath.Rel(from, p)
				return filepath.Join(values[from], rel)
			}
		}
		return p
	}
	for i := range state.Projects {
		state.Projects[i].Path = apply(state.Projects[i].Path)
	}
	for i := range state.Roots {
		state.Roots[i].Path = apply(state.Roots[i].Path)
	}
	return state.Validate()
}
func Restore(ctx context.Context, s *storage.Store, path string, mappings map[string]string) ([]string, error) {
	f, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	state, e := Decode(f)
	if e != nil {
		return nil, e
	}
	if e = Remap(&state, mappings); e != nil {
		return nil, e
	}
	e = s.Update(ctx, func(current *model.State) error {
		if len(current.Projects)+len(current.Roots)+len(current.Records)+len(current.Preferences) != 0 || current.NextProject != 1 || current.NextRoot != 1 || current.NextRecord != 1 {
			return errors.New("restore requires an empty application database; use a new --data-dir")
		}
		*current = state
		return nil
	})
	if e != nil {
		return nil, e
	}
	warnings := projects.Warnings(state)
	// Archived folders are also relevant when transferring a complete backup.
	for _, p := range state.Projects {
		if p.Archived {
			if _, e := os.Stat(p.Path); e != nil {
				warnings = append(warnings, fmt.Sprintf("archived project %s: %v", p.Name, e))
			}
		}
	}
	return warnings, nil
}
func ParseMapping(raw string) (string, string, error) {
	from, to, ok := strings.Cut(raw, "=")
	if !ok || from == "" || to == "" {
		return "", "", errors.New("use --remap /old/path=/new/path")
	}
	return from, to, nil
}
