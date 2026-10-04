package config

import (
	"os"
	"path/filepath"
)

func DataDir() (string, error) {
	h, e := os.UserHomeDir()
	if e != nil {
		return "", e
	}
	return filepath.Join(h, "Library", "Application Support", "Junior Assistant"), nil
}

// Canonical resolves a registration's symlinks, including existing ancestors of
// a missing folder, so later path comparisons use a consistent identity.
func Canonical(path string) (string, error) {
	p, e := filepath.Abs(path)
	if e != nil {
		return "", e
	}
	p = filepath.Clean(p)
	tail := []string{}
	base := p
	for {
		resolved, err := filepath.EvalSymlinks(base)
		if err == nil {
			for i := len(tail) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, tail[i])
			}
			return resolved, nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(base)
		if parent == base {
			return p, nil
		}
		tail = append(tail, filepath.Base(base))
		base = parent
	}
}
