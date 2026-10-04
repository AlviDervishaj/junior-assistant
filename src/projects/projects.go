package projects

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AlviDervishaj/junior-assistant/src/config"
	"github.com/AlviDervishaj/junior-assistant/src/model"
)

func Find(s model.State, name string) (model.Project, error) {
	for _, p := range s.Projects {
		if strings.EqualFold(p.Name, name) {
			return p, nil
		}
	}
	return model.Project{}, fmt.Errorf("project %q not found", name)
}
func Infer(s model.State, cwd string) int64 {
	best := ""
	var id int64
	for _, p := range s.Projects {
		if !p.Archived && Within(p.Path, cwd) && len(p.Path) > len(best) {
			best = p.Path
			id = p.ID
		}
	}
	return id
}
func Within(root, path string) bool {
	r, e := filepath.Rel(root, path)
	return e == nil && r != ".." && !strings.HasPrefix(r, ".."+string(filepath.Separator))
}
func Add(s *model.State, name, path string) (model.Project, error) {
	if !model.Name(name) {
		return model.Project{}, fmt.Errorf("invalid project name")
	}
	for _, p := range s.Projects {
		if strings.EqualFold(p.Name, name) {
			return model.Project{}, fmt.Errorf("project name already exists (including archived projects)")
		}
	}
	path, e := config.Canonical(path)
	if e != nil {
		return model.Project{}, e
	}
	if info, e := os.Stat(path); e == nil && !info.IsDir() {
		return model.Project{}, fmt.Errorf("project path must be a directory")
	} else if e != nil && !os.IsNotExist(e) {
		return model.Project{}, e
	}
	for _, p := range s.Projects {
		if !p.Archived && p.Path == path {
			return model.Project{}, fmt.Errorf("active project path already registered")
		}
	}
	p := model.Project{ID: s.NextProject, Name: name, Path: path}
	s.NextProject++
	s.Projects = append(s.Projects, p)
	s.Roots = append(s.Roots, model.Root{ID: s.NextRoot, Path: path, ProjectID: p.ID, Exclusions: []string{}})
	s.NextRoot++
	return p, nil
}
func Edit(s *model.State, name, path string, archive *bool) (model.Project, error) {
	p, e := Find(*s, name)
	if e != nil {
		return p, e
	}
	if path != "" {
		p.Path, e = config.Canonical(path)
		if e != nil {
			return p, e
		}
		if i, e := os.Stat(p.Path); e == nil && !i.IsDir() {
			return p, fmt.Errorf("project path must be a directory")
		} else if e != nil && !os.IsNotExist(e) {
			return p, e
		}
	}
	if archive != nil {
		p.Archived = *archive
	}
	for i, old := range s.Projects {
		if old.ID == p.ID {
			s.Projects[i] = p
		}
	}
	for i, r := range s.Roots {
		if r.ProjectID == p.ID {
			s.Roots[i].Path = p.Path
		}
	}
	return p, s.Validate()
}
func AddRoot(s *model.State, path string, exclusions []string) (model.Root, error) {
	if e := model.ValidExclusions(exclusions); e != nil {
		return model.Root{}, e
	}
	path, e := config.Canonical(path)
	if e != nil {
		return model.Root{}, e
	}
	if i, e := os.Stat(path); e == nil && !i.IsDir() {
		return model.Root{}, fmt.Errorf("root path must be a directory")
	} else if e != nil && !os.IsNotExist(e) {
		return model.Root{}, e
	}
	for _, r := range s.Roots {
		if r.ProjectID == 0 && r.Path == path {
			return model.Root{}, fmt.Errorf("independent root already exists")
		}
	}
	if exclusions == nil {
		exclusions = []string{}
	}
	r := model.Root{ID: s.NextRoot, Path: path, Exclusions: exclusions}
	s.NextRoot++
	s.Roots = append(s.Roots, r)
	return r, nil
}
func RemoveRoot(s *model.State, path string) error {
	path, e := config.Canonical(path)
	if e != nil {
		return e
	}
	for i, r := range s.Roots {
		if r.ProjectID == 0 && r.Path == path {
			s.Roots = append(s.Roots[:i], s.Roots[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("independent root not found; project roots are managed through project commands")
}
func Exclude(s *model.State, id int64, xs []string) error {
	if e := model.ValidExclusions(xs); e != nil {
		return e
	}
	if xs == nil {
		xs = []string{}
	}
	for i, r := range s.Roots {
		if r.ID == id {
			s.Roots[i].Exclusions = xs
			return nil
		}
	}
	return fmt.Errorf("root %d not found", id)
}
func ActiveRoots(s model.State) []model.Root {
	archived := map[int64]bool{}
	for _, p := range s.Projects {
		archived[p.ID] = p.Archived
	}
	roots := []model.Root{}
	for _, r := range s.Roots {
		if !archived[r.ProjectID] {
			roots = append(roots, r)
		}
	}
	return roots
}
func Warnings(s model.State) []string {
	xs := []string{}
	seen := map[string]bool{}
	for _, r := range ActiveRoots(s) {
		if seen[r.Path] {
			continue
		}
		seen[r.Path] = true
		if i, e := os.Stat(r.Path); e != nil {
			xs = append(xs, fmt.Sprintf("root %s: %v", r.Path, e))
		} else if !i.IsDir() {
			xs = append(xs, fmt.Sprintf("root %s is no longer a directory", r.Path))
		}
	}
	return xs
}
