#!/usr/bin/env python3
"""Run the built CLI end-to-end in disposable state with network denied on macOS."""
import datetime
import json
import pathlib
import shutil
import subprocess
import sys
import tempfile

binary = pathlib.Path(sys.argv[1] if len(sys.argv) > 1 else "bin/assistant").resolve()
sandbox = shutil.which("sandbox-exec") if sys.platform == "darwin" else None
prefix = [sandbox, "-p", "(version 1) (allow default) (deny network*)"] if sandbox else []
if not sandbox:
    raise SystemExit("This acceptance check requires macOS sandbox-exec to deny runtime network access.")

with tempfile.TemporaryDirectory(prefix="assistant-smoke-") as tmp:
    base = pathlib.Path(tmp).resolve()
    state, restored = base / "state", base / "restored"
    project = base / "project"
    project.mkdir()
    docs = base / "documents"
    docs.mkdir()
    fixture = project / "Invoice.txt"
    fixture.write_text("This file must remain unchanged.\n")
    hidden = project / ".hidden"
    hidden.mkdir()
    (hidden / "invoice-secret").write_text("private\n")
    (project / "node_modules").mkdir()
    (project / "node_modules" / "invoice-dependency").write_text("dependency\n")
    original = {str(p): p.read_bytes() for p in project.rglob("*") if p.is_file()}
    calls = 0

    def run(*args, destination=state, expected=0):
        global calls
        cmd = prefix + [str(binary), "--data-dir", str(destination), "--json", *args]
        result = subprocess.run(cmd, capture_output=True, text=True, cwd=project, timeout=20)
        assert result.returncode == expected, (args, result.returncode, result.stderr)
        calls += 1
        return json.loads(result.stdout)["data"] if expected in (0, 3) else None

    run("project", "add", "app", str(project))
    run("root", "add", str(docs), "--exclude", "cache")
    run("root", "add", str(project))  # overlap must not duplicate results
    result = run("find", "invoice")
    assert result["total"] == 1 and result["matches"][0]["path"] == str(fixture)
    assert run("find", "invoice", "--hidden", "--include-excluded", "--all")["total"] == 3
    day = datetime.date.today().isoformat()
    task = run("task", "add", "Fix session expiry", "--type", "bug", "--planned", day)
    assert task["id"] == 1 and task["project_id"] == 1
    run("task", "edit", "1", "--notes", "Reproduce after sleep", "--due", day)
    run("task", "start", "1")
    groups = run("today")
    assert sum(len(g["records"]) for g in groups) == 1
    assert groups[1]["records"][0]["id"] == 1
    run("task", "done", "1")
    assert run("task", "list") == []
    run("task", "reopen", "1")
    run("task", "add", "Renew passport", "--personal")
    run("project", "remove", "app")
    assert run("task", "list", "--project", "app")[0]["project_id"] == 1
    assert run("find", "invoice")["total"] == 1  # independent root survives archive
    run("project", "restore", "app")
    run("task", "delete", "2", expected=2)
    run("task", "cancel", "2")
    run("task", "delete", "2", "--yes")
    run("task", "add", "Next work", "--personal")
    missing = docs / "missing"
    run("root", "add", str(missing))
    assert run("find", "invoice", expected=3)["total"] == 1
    run("root", "remove", str(missing))
    backup = base / "backup.json"
    run("export", str(backup))
    run("export", str(backup), expected=1)
    run("restore", str(backup), expected=1)
    run("restore", str(backup), destination=restored)
    records = run("task", "list", "--all", destination=restored)
    assert [r["id"] for r in records] == [1, 3]
    assert run("task", "add", "After restore", "--personal", destination=restored)["id"] == 4
    assert original == {str(p): p.read_bytes() for p in project.rglob("*") if p.is_file()}
    print(f"PASS: {calls} CLI calls with runtime network denied; search, lifecycle, archive, today, and backup/restore verified; fixture files unchanged.")
