// Package search reads the filesystem without any dependency on application storage.
package search

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Root struct {
	Path       string
	Exclusions []string
}
type Options struct {
	Hidden, IncludeExcluded bool
	Limit                   int
}
type Match struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
	Rank int    `json:"rank"`
}
type Result struct {
	Matches   []Match  `json:"matches"`
	Total     int      `json:"total"`
	Truncated bool     `json:"truncated"`
	Warnings  []string `json:"warnings"`
}

var defaults = []string{".git", "node_modules", "vendor", "dist", "build"}

func Find(ctx context.Context, roots []Root, query string, o Options) (Result, error) {
	result := Result{Matches: []Match{}, Warnings: []string{}}
	if strings.TrimSpace(query) == "" {
		return result, fmt.Errorf("search text must not be empty")
	}
	q := strings.ToLower(query)
	seen := map[string]bool{}
	warned := map[string]bool{}
	warn := func(path string, e error) {
		if !warned[path] {
			result.Warnings = append(result.Warnings, fmt.Sprintf("%s: %v", path, e))
			warned[path] = true
		}
	}
	for _, root := range roots {
		if e := ctx.Err(); e != nil {
			return result, e
		}
		info, e := os.Stat(root.Path)
		if e != nil {
			warn(root.Path, e)
			continue
		}
		if !info.IsDir() {
			warn(root.Path, fmt.Errorf("not a directory"))
			continue
		}
		excluded := map[string]bool{}
		for _, name := range append(append([]string{}, defaults...), root.Exclusions...) {
			excluded[name] = true
		}
		e = filepath.WalkDir(root.Path, func(path string, d fs.DirEntry, err error) error {
			if e := ctx.Err(); e != nil {
				return e
			}
			if err != nil {
				warn(path, err)
				return nil
			}
			if path != root.Path {
				if (!o.Hidden && strings.HasPrefix(d.Name(), ".")) || (!o.IncludeExcluded && d.IsDir() && excluded[d.Name()]) {
					if d.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
			}
			if seen[path] {
				return nil
			}
			seen[path] = true
			name := strings.ToLower(d.Name())
			lower := strings.ToLower(path)
			rank := -1
			switch {
			case name == q:
				rank = 0
			case strings.HasPrefix(name, q):
				rank = 1
			case strings.Contains(name, q):
				rank = 2
			case strings.Contains(lower, q):
				rank = 3
			}
			if rank >= 0 {
				kind := "file"
				if d.IsDir() {
					kind = "directory"
				} else if d.Type()&os.ModeSymlink != 0 {
					kind = "symlink"
				}
				result.Matches = append(result.Matches, Match{path, kind, rank})
			}
			return nil
		})
		if e != nil {
			return result, e
		}
	}
	sort.Slice(result.Matches, func(i, j int) bool {
		x, y := result.Matches[i], result.Matches[j]
		if x.Rank != y.Rank {
			return x.Rank < y.Rank
		}
		return x.Path < y.Path
	})
	sort.Strings(result.Warnings)
	result.Total = len(result.Matches)
	if o.Limit > 0 && result.Total > o.Limit {
		result.Matches = result.Matches[:o.Limit]
		result.Truncated = true
	}
	return result, nil
}
