package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/AlviDervishaj/junior-assistant/src/config"
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

func TestTerminalDiagnosticsEscapePathControlsButJSONPreservesThem(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(t.TempDir(), "missing\x1b]0;injected\x07")
	missing, canonicalErr := config.Canonical(missing)
	if canonicalErr != nil {
		t.Fatal(canonicalErr)
	}
	var out, err bytes.Buffer
	code := Run(context.Background(), []string{"--data-dir", dir, "root", "add", missing}, nil, &out, &err)
	if code != 0 {
		t.Fatalf("registration failed %d %s", code, &err)
	}
	if bytes.Contains(err.Bytes(), []byte{0x1b}) || bytes.Contains(err.Bytes(), []byte{0x07}) {
		t.Fatal("warning emitted raw terminal controls")
	}
	if !bytes.Contains(err.Bytes(), []byte(`\u001b`)) {
		t.Fatalf("warning did not escape path %s", &err)
	}
	out.Reset()
	err.Reset()
	code = Run(context.Background(), []string{"--data-dir", dir, "export", filepath.Join(missing, "backup.json")}, nil, &out, &err)
	if code != 1 || bytes.Contains(err.Bytes(), []byte{0x1b}) {
		t.Fatalf("error emitted raw controls: %d %s", code, &err)
	}
	backupPath := filepath.Join(t.TempDir(), "backup\x1b.json")
	out.Reset()
	err.Reset()
	code = Run(context.Background(), []string{"--data-dir", dir, "export", backupPath}, nil, &out, &err)
	if code != 0 || bytes.Contains(out.Bytes(), []byte{0x1b}) || !bytes.Contains(out.Bytes(), []byte(`\u001b`)) {
		t.Fatalf("export output emitted raw controls: %d %s", code, &out)
	}
	data := invoke(t, dir, 0, "root", "list")
	var roots []struct {
		Path string `json:"path"`
	}
	if e := json.Unmarshal(data, &roots); e != nil {
		t.Fatal(e)
	}
	if len(roots) != 1 || roots[0].Path != missing {
		t.Fatal("JSON changed the original path")
	}
}

func TestTaskProgressHistoryAndBackup(t *testing.T) {
	dir := t.TempDir()
	invoke(t, dir, 0, "task", "add", "Investigate", "--type", "bug", "--notes", "keep notes", "--personal")
	data := invoke(t, dir, 0, "task", "logs", "1")
	if string(data) != "[]" {
		t.Fatalf("empty logs %s", data)
	}
	invoke(t, dir, 0, "task", "log", "1", "Reproduced after sleep")
	invoke(t, dir, 0, "task", "done", "1")
	invoke(t, dir, 0, "task", "log", "1", "Validated fix\x1b[2J")
	invoke(t, dir, 2, "task", "log", "1", "  ")
	invoke(t, dir, 1, "task", "log", "999", "unknown")
	var entries []struct {
		ID        int64  `json:"id"`
		Message   string `json:"message"`
		CreatedAt string `json:"created_at"`
	}
	data = invoke(t, dir, 0, "task", "logs", "1")
	if e := json.Unmarshal(data, &entries); e != nil {
		t.Fatal(e)
	}
	if len(entries) != 2 || entries[1].ID != 2 || entries[1].Message != "Validated fix\x1b[2J" {
		t.Fatalf("history %s", data)
	}
	if _, e := time.Parse(time.RFC3339Nano, entries[0].CreatedAt); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "backup.json")
	invoke(t, dir, 0, "export", path)
	target := t.TempDir()
	invoke(t, target, 0, "restore", path)
	if got := invoke(t, target, 0, "task", "logs", "1"); !bytes.Equal(got, data) {
		t.Fatalf("restore lost logs %s", got)
	}
	var record struct{ Notes, Status string }
	json.Unmarshal(invoke(t, target, 0, "task", "show", "1"), &record)
	if record.Notes != "keep notes" || record.Status != "done" {
		t.Fatal("history changed record")
	}
	var out, err bytes.Buffer
	code := Run(context.Background(), []string{"--data-dir", target, "task", "show", "1"}, nil, &out, &err)
	if code != 0 || !bytes.Contains(out.Bytes(), []byte("Reproduced after sleep")) || bytes.Contains(out.Bytes(), []byte{0x1b}) {
		t.Fatalf("show history %d %s", code, &out)
	}
	invoke(t, target, 0, "task", "delete", "1", "--yes")
	invoke(t, target, 1, "task", "logs", "1")
}
