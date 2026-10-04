package search

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestRankingExclusionsOverlapAndLinks(t *testing.T) {
	dir := t.TempDir()
	paths := []string{"invoice", "invoice-a", "my-invoice", "invoice-parent/unrelated", ".hidden/invoice", "node_modules/invoice", "custom/invoice"}
	for _, p := range paths {
		p = filepath.Join(dir, p)
		if e := os.MkdirAll(filepath.Dir(p), 0700); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(p, []byte("invoice inside unrelated contents"), 0600); e != nil {
			t.Fatal(e)
		}
	}
	external := t.TempDir()
	os.WriteFile(filepath.Join(external, "invoice-external"), nil, 0600)
	if e := os.Symlink(external, filepath.Join(dir, "linked")); e != nil {
		t.Fatal(e)
	}
	roots := []Root{{dir, []string{"custom"}}, {filepath.Join(dir, "invoice-parent"), nil}}
	got, e := Find(context.Background(), roots, "INVOICE", Options{Limit: 2})
	if e != nil {
		t.Fatal(e)
	}
	if got.Total != 5 || !got.Truncated || len(got.Matches) != 2 || got.Matches[0].Path != filepath.Join(dir, "invoice") || got.Matches[1].Path != filepath.Join(dir, "invoice-a") {
		t.Fatalf("unexpected results %+v", got)
	}
	full, e := Find(context.Background(), roots, "invoice", Options{Hidden: true, IncludeExcluded: true})
	if e != nil {
		t.Fatal(e)
	}
	if full.Total != 8 {
		t.Fatalf("unexpected complete count %+v", full)
	}
	for _, m := range full.Matches {
		if m.Path == filepath.Join(dir, "linked", "invoice-external") {
			t.Fatal("followed symlink")
		}
	}
	// An explicitly hidden/excluded root is traversed even without override flags.
	explicit, e := Find(context.Background(), []Root{{filepath.Join(dir, ".hidden"), nil}, {filepath.Join(dir, "node_modules"), nil}}, "invoice", Options{})
	if e != nil || explicit.Total != 2 {
		t.Fatalf("explicit roots %+v %v", explicit, e)
	}
	none, _ := Find(context.Background(), roots, "invoice inside unrelated contents", Options{})
	if none.Total != 0 {
		t.Fatal("searched contents")
	}
	again, _ := Find(context.Background(), roots, "INVOICE", Options{Limit: 2})
	if !reflect.DeepEqual(got, again) {
		t.Fatal("unstable results")
	}
}
func TestPartialAndCancellation(t *testing.T) {
	d := t.TempDir()
	os.WriteFile(filepath.Join(d, "yes"), nil, 0600)
	r, e := Find(context.Background(), []Root{{filepath.Join(d, "missing"), nil}, {d, nil}}, "yes", Options{})
	if e != nil || r.Total != 1 || len(r.Warnings) != 1 {
		t.Fatalf("%+v %v", r, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = Find(ctx, []Root{{d, nil}}, "yes", Options{}); e != context.Canceled {
		t.Fatalf("expected cancellation: %v", e)
	}
}
