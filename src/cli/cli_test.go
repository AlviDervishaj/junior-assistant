package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func invoke(t *testing.T, dir string, code int, args ...string) json.RawMessage {
	t.Helper()
	var out, err bytes.Buffer
	full := append([]string{"--data-dir", dir, "--json"}, args...)
	got := Run(context.Background(), full, bytes.NewBuffer(nil), &out, &err)
	if got != code {
		t.Fatalf("%v exit %d want %d\nstdout %s\nstderr %s", args, got, code, &out, &err)
	}
	if code != 0 && code != 3 {
		return nil
	}
	var envelope struct {
		Data     json.RawMessage `json:"data"`
		Warnings []string        `json:"warnings"`
	}
	if e := json.Unmarshal(out.Bytes(), &envelope); e != nil {
		t.Fatalf("non-JSON output: %s (%v)", &out, e)
	}
	return envelope.Data
}
func TestCLICompleteWorkflow(t *testing.T) {
	dir := t.TempDir()
	root := t.TempDir()
	nested := filepath.Join(root, "nested")
	os.Mkdir(nested, 0700)
	os.WriteFile(filepath.Join(root, "invoice.txt"), []byte("untouched"), 0600)
	invoke(t, dir, 0, "project", "add", "outer", root)
	invoke(t, dir, 0, "project", "add", "inner", nested)
	invoke(t, dir, 0, "root", "add", root, "--exclude", "cache")
	cwd, e := os.Getwd()
	if e != nil {
		t.Fatal(e)
	}
	if e = os.Chdir(nested); e != nil {
		t.Fatal(e)
	}
	defer os.Chdir(cwd)
	data := invoke(t, dir, 0, "task", "add", "fix login", "--type", "bug", "--planned", time.Now().Format("2006-01-02"))
	var record struct {
		ID      int64  `json:"id"`
		Project int64  `json:"project_id"`
		Status  string `json:"status"`
		Planned string `json:"planned_date"`
	}
	json.Unmarshal(data, &record)
	if record.Project != 2 || record.ID != 1 {
		t.Fatalf("bad inference %s", data)
	}
	invoke(t, dir, 0, "task", "add", "personal job", "--personal")
	invoke(t, dir, 2, "task", "add", "bad", "--personal", "--project", "outer")
	invoke(t, dir, 2, "task", "add", "bad date", "--due", "2026-02-30")
	invoke(t, dir, 2, "task", "edit", "1", "--planned", "2026-10-01", "--clear-planned")
	invoke(t, dir, 0, "task", "edit", "1", "--title", "session expiry", "--notes", "details", "--due", time.Now().Format("2006-01-02"))
	invoke(t, dir, 0, "task", "start", "1")
	invoke(t, dir, 0, "today")
	invoke(t, dir, 0, "task", "done", "1")
	var list []any
	json.Unmarshal(invoke(t, dir, 0, "task", "list"), &list)
	if len(list) != 0 {
		t.Fatal("done record remains active")
	}
	invoke(t, dir, 0, "task", "reopen", "1")
	invoke(t, dir, 0, "project", "remove", "inner")
	data = invoke(t, dir, 0, "task", "add", "outer work")
	json.Unmarshal(data, &record)
	if record.Project != 1 {
		t.Fatal("archived project was inferred")
	}
	data = invoke(t, dir, 0, "task", "list", "--project", "inner")
	json.Unmarshal(data, &list)
	if len(list) != 1 {
		t.Fatal("archived ownership lost")
	}
	invoke(t, dir, 0, "project", "restore", "inner")
	changed := t.TempDir()
	invoke(t, dir, 0, "project", "edit", "inner", "--path", changed)
	invoke(t, dir, 0, "task", "show", "1")
	invoke(t, dir, 0, "find", "INVOICE", "--all")
	invoke(t, dir, 2, "find", "invoice", "--all", "--limit", "2")
	invoke(t, dir, 0, "root", "add", filepath.Join(root, "missing"))
	invoke(t, dir, 3, "find", "invoice")
	invoke(t, dir, 0, "root", "remove", filepath.Join(root, "missing"))
	invoke(t, dir, 2, "task", "delete", "2")
	invoke(t, dir, 0, "task", "cancel", "2")
	invoke(t, dir, 0, "task", "delete", "2", "--yes")
	invoke(t, dir, 1, "task", "show", "2")
	backupPath := filepath.Join(t.TempDir(), "backup.json")
	invoke(t, dir, 0, "export", backupPath)
	invoke(t, dir, 1, "restore", backupPath)
	target := t.TempDir()
	invoke(t, target, 0, "restore", backupPath)
	invoke(t, target, 0, "task", "list", "--all", "--include-finished")
	if b, e := os.ReadFile(filepath.Join(root, "invoice.txt")); e != nil || string(b) != "untouched" {
		t.Fatal("search changed file")
	}
}
func TestUsageAndHelpDoNotCreateState(t *testing.T) {
	for _, args := range [][]string{{"find"}, {"unknown"}, {"task", "add", "x", "--type", "wrong"}, {"today", "--personal", "--all"}, {"root", "edit", "1"}, {"task", "edit", "1"}} {
		d := filepath.Join(t.TempDir(), "new")
		var out, err bytes.Buffer
		code := Run(context.Background(), append([]string{"--data-dir", d}, args...), bytes.NewBuffer(nil), &out, &err)
		if code != 2 {
			t.Fatalf("%v exit %d: %s", args, code, &err)
		}
		if _, e := os.Stat(d); !os.IsNotExist(e) {
			t.Fatalf("usage created state for %v", args)
		}
	}
	d := filepath.Join(t.TempDir(), "new")
	var out, err bytes.Buffer
	if code := Run(context.Background(), []string{"--data-dir", d, "--help"}, nil, &out, &err); code != 0 {
		t.Fatal(code)
	}
	if _, e := os.Stat(d); !os.IsNotExist(e) {
		t.Fatal("help created state")
	}
}
func TestOptionsAfterArgumentsAndLiteralDashTitle(t *testing.T) {
	d := t.TempDir()
	var out, err bytes.Buffer
	code := Run(context.Background(), []string{"task", "add", "--data-dir", d, "--json", "--", "--json"}, nil, &out, &err)
	if code != 0 {
		t.Fatalf("%d %s", code, &err)
	}
	if !bytes.Contains(out.Bytes(), []byte(`"title":"--json"`)) {
		t.Fatal(out.String())
	}
	invoke(t, d, 0, "task", "edit", "1", "--clear-notes", "--clear-due", "--clear-planned", "--personal")
}
func TestSafeTerminalOutput(t *testing.T) {
	if got := safe("hello\x1b[2J\n"); got != "hello\\u001b[2J\\u000a" {
		t.Fatal(got)
	}
}
