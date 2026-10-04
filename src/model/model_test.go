package model

import "testing"

func TestBackupProjectNamesUseSameCaseFoldingAsCommands(t *testing.T) {
	s := Empty()
	s.Projects = []Project{{ID: 1, Name: "Σ", Path: "/one"}, {ID: 2, Name: "ς", Path: "/two"}}
	s.NextProject = 3
	s.Roots = []Root{{ID: 1, Path: "/one", ProjectID: 1}, {ID: 2, Path: "/two", ProjectID: 2}}
	s.NextRoot = 3
	if e := s.Validate(); e == nil {
		t.Fatal("case-equivalent backup names accepted")
	}
}
