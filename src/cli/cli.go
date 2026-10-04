package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/AlviDervishaj/junior-assistant/src/backup"
	"github.com/AlviDervishaj/junior-assistant/src/config"
	"github.com/AlviDervishaj/junior-assistant/src/model"
	"github.com/AlviDervishaj/junior-assistant/src/projects"
	"github.com/AlviDervishaj/junior-assistant/src/records"
	"github.com/AlviDervishaj/junior-assistant/src/search"
	"github.com/AlviDervishaj/junior-assistant/src/storage"
	"github.com/AlviDervishaj/junior-assistant/src/today"
	"github.com/mattn/go-isatty"
)

const help = `assistant — local files, projects, tasks, and today's work

Usage: assistant [--data-dir DIR] [--json] COMMAND [OPTIONS]

  project add NAME PATH | list | edit NAME --path PATH | remove NAME | restore NAME
  root add PATH [--exclude NAME ...] | list | remove PATH
  root edit ID [--exclude NAME ... | --clear-exclusions]
  find TEXT [--hidden] [--include-excluded] [--limit N | --all]
  task add TITLE [--type task|bug] [--notes TEXT] [--planned DATE] [--due DATE]
  task list [--include-finished] | show ID
  task edit ID [--title TEXT] [--notes TEXT] [--type task|bug]
               [--planned DATE | --clear-planned] [--due DATE | --clear-due]
               [--clear-notes] [--project NAME | --personal]
  task start ID | done ID | cancel ID | reopen ID | delete ID [--yes]
  today
  export PATH
  restore PATH [--remap /old/path=/new/path ...]

Scopes: task add/edit/list and today accept --project NAME or --personal.
        task list and today also accept --all. Today defaults to all;
        task add/list infer the current project, otherwise personal/all.
Dates: YYYY-MM-DD; today uses the local timezone.
Global options also work after commands. Use -- before literal dash-prefixed arguments.
Exit codes: 0 success (including no matches), 1 operation failure,
            2 invalid usage, 3 partial search, 130 interrupted.
`

type usageError struct{ error }

func usage(s string) error { return usageError{errors.New(s)} }

type app struct {
	ctx      context.Context
	in       io.Reader
	out, err io.Writer
	dir      string
	json     bool
	store    *storage.Store
}

func Run(ctx context.Context, args []string, in io.Reader, out, errOut io.Writer) int {
	a := &app{ctx: ctx, in: in, out: out, err: errOut}
	defer func() {
		if a.store != nil {
			a.store.Close()
		}
	}()
	g := flag.NewFlagSet("assistant", flag.ContinueOnError)
	g.SetOutput(io.Discard)
	g.StringVar(&a.dir, "data-dir", "", "state directory")
	g.BoolVar(&a.json, "json", false, "JSON output")
	var h bool
	g.BoolVar(&h, "help", false, "help")
	g.BoolVar(&h, "h", false, "help")
	e := g.Parse(args)
	if e != nil {
		fmt.Fprintln(errOut, e)
		return 2
	}
	args = g.Args()
	if h || len(args) == 0 || args[0] == "help" {
		fmt.Fprint(out, help)
		return 0
	}
	code, e := a.dispatch(args)
	if e == nil {
		return code
	}
	fmt.Fprintln(errOut, "error:", e)
	if errors.Is(e, context.Canceled) {
		return 130
	}
	var u usageError
	if errors.As(e, &u) {
		return 2
	}
	return 1
}
func (a *app) open() error {
	if a.store != nil {
		return nil
	}
	if a.dir == "" {
		var e error
		a.dir, e = config.DataDir()
		if e != nil {
			return e
		}
	}
	p, e := config.Canonical(a.dir)
	if e != nil {
		return e
	}
	a.dir = p
	a.store, e = storage.Open(a.ctx, p)
	return e
}
func (a *app) state() (model.State, error) {
	if e := a.open(); e != nil {
		return model.State{}, e
	}
	return a.store.Read(a.ctx)
}
func (a *app) update(fn func(*model.State) error) error {
	if e := a.open(); e != nil {
		return e
	}
	return a.store.Update(a.ctx, fn)
}

type flags struct {
	set            *flag.FlagSet
	help           bool
	project        string
	personal, all  bool
	exclude, remap stringsFlag
}
type stringsFlag []string

func (s *stringsFlag) String() string     { return strings.Join(*s, ",") }
func (s *stringsFlag) Set(v string) error { *s = append(*s, v); return nil }
func (a *app) flags(name string) *flags {
	f := &flags{set: flag.NewFlagSet(name, flag.ContinueOnError)}
	f.set.SetOutput(io.Discard)
	f.set.StringVar(&a.dir, "data-dir", a.dir, "state directory")
	f.set.BoolVar(&a.json, "json", a.json, "JSON output")
	f.set.BoolVar(&f.help, "help", false, "help")
	f.set.BoolVar(&f.help, "h", false, "help")
	return f
}
func (f *flags) scopes(all bool) {
	f.set.StringVar(&f.project, "project", "", "project name")
	f.set.BoolVar(&f.personal, "personal", false, "personal work")
	if all {
		f.set.BoolVar(&f.all, "all", false, "all work")
	}
}

// The standard flag parser stops at the first argument. Reorder only recognized
// options, keeping their values intact, so ordinary CLI options can follow IDs.
func (f *flags) parse(args []string, n int) ([]string, error) {
	opts, pos := []string{}, []string{}
	for i := 0; i < len(args); i++ {
		token := args[i]
		if token == "--" {
			pos = append(pos, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(token, "-") || token == "-" {
			pos = append(pos, token)
			continue
		}
		key := strings.TrimLeft(token, "-")
		name, _, hasValue := strings.Cut(key, "=")
		opt := f.set.Lookup(name)
		if opt == nil {
			return nil, usage("unknown option: " + token)
		}
		opts = append(opts, token)
		bf, boolean := opt.Value.(interface{ IsBoolFlag() bool })
		boolean = boolean && bf.IsBoolFlag()
		if !hasValue && !boolean {
			i++
			if i >= len(args) {
				return nil, usage("missing value for " + token)
			}
			opts = append(opts, args[i])
		}
	}
	if e := f.set.Parse(opts); e != nil {
		return nil, usage(e.Error())
	}
	if f.help {
		return nil, flag.ErrHelp
	}
	if len(pos) != n {
		return nil, usage(fmt.Sprintf("%s expects %d argument(s); see assistant --help", f.set.Name(), n))
	}
	return pos, nil
}
func (f *flags) visited(name string) bool {
	found := false
	f.set.Visit(func(v *flag.Flag) {
		if v.Name == name {
			found = true
		}
	})
	return found
}
func (f *flags) validateScope() error {
	n := 0
	if f.project != "" {
		n++
	}
	if f.personal {
		n++
	}
	if f.all {
		n++
	}
	if n > 1 {
		return usage("--project, --personal, and --all cannot be combined")
	}
	if f.visited("project") && f.project == "" {
		return usage("project name must not be empty")
	}
	return nil
}
func (f *flags) scope(s model.State, infer bool) (records.Scope, error) {
	if e := f.validateScope(); e != nil {
		return records.Scope{}, e
	}
	if f.project != "" {
		p, e := projects.Find(s, f.project)
		return records.Scope{ProjectID: p.ID}, e
	}
	if f.personal {
		return records.Scope{Personal: true}, nil
	}
	if infer && !f.all {
		cwd, e := os.Getwd()
		if e != nil {
			return records.Scope{}, e
		}
		cwd, e = config.Canonical(cwd)
		if e != nil {
			return records.Scope{}, e
		}
		return records.Scope{ProjectID: projects.Infer(s, cwd)}, nil
	}
	return records.Scope{}, nil
}
func (a *app) dispatch(args []string) (int, error) {
	var code int
	var e error
	switch args[0] {
	case "project":
		e = a.project(args[1:])
	case "root":
		e = a.root(args[1:])
	case "task":
		e = a.task(args[1:])
	case "find":
		code, e = a.find(args[1:])
	case "today":
		e = a.today(args[1:])
	case "export", "restore":
		e = a.backup(args[0], args[1:])
	default:
		e = usage("unknown command: " + args[0])
	}
	if errors.Is(e, flag.ErrHelp) {
		fmt.Fprint(a.out, help)
		return 0, nil
	}
	return code, e
}
func id(raw string) (int64, error) {
	i, e := strconv.ParseInt(raw, 10, 64)
	if e != nil || i < 1 {
		return 0, usage("ID must be a positive integer")
	}
	return i, nil
}
func (a *app) print(v any, warnings []string) error {
	if warnings == nil {
		warnings = []string{}
	}
	for _, w := range warnings {
		fmt.Fprintln(a.err, "warning:", w)
	}
	if a.json {
		return json.NewEncoder(a.out).Encode(struct {
			Data     any      `json:"data"`
			Warnings []string `json:"warnings"`
		}{v, warnings})
	}
	switch x := v.(type) {
	case string:
		_, e := fmt.Fprintln(a.out, x)
		return e
	case model.Project:
		a.projectLine(x)
	case []model.Project:
		for _, p := range x {
			a.projectLine(p)
		}
		if len(x) == 0 {
			fmt.Fprintln(a.out, "No projects.")
		}
	case model.Root:
		a.rootLine(x)
	case []model.Root:
		for _, r := range x {
			a.rootLine(r)
		}
		if len(x) == 0 {
			fmt.Fprintln(a.out, "No search roots.")
		}
	case model.Record:
		a.recordLine(x)
		if x.Notes != "" {
			fmt.Fprintln(a.out, "  Notes:", safe(x.Notes))
		}
	case []model.Record:
		for _, r := range x {
			a.recordLine(r)
		}
		if len(x) == 0 {
			fmt.Fprintln(a.out, "No records.")
		}
	case []today.Group:
		n := 0
		for _, g := range x {
			if len(g.Records) == 0 {
				continue
			}
			fmt.Fprintln(a.out, g.Category+":")
			for _, r := range g.Records {
				a.recordLine(r)
				n++
			}
		}
		if n == 0 {
			fmt.Fprintln(a.out, "Nothing needs attention today.")
		}
	case search.Result:
		for _, m := range x.Matches {
			fmt.Fprintf(a.out, "%s  %s\n", m.Kind, safe(m.Path))
		}
		if x.Total == 0 {
			fmt.Fprintln(a.out, "No matches.")
		}
		if x.Truncated {
			fmt.Fprintf(a.out, "Showing %d of %d matches; use --all to see every match.\n", len(x.Matches), x.Total)
		}
	default:
		return fmt.Errorf("unsupported output type")
	}
	return nil
}

// Quote controls to prevent filenames or titles from injecting terminal commands.
func safe(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r < 32 || r == 127 {
			fmt.Fprintf(&b, "\\u%04x", r)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
func (a *app) projectLine(p model.Project) {
	status := "active"
	if p.Archived {
		status = "archived"
	}
	fmt.Fprintf(a.out, "%d  %s  %s  %s\n", p.ID, safe(p.Name), status, safe(p.Path))
}
func (a *app) rootLine(r model.Root) {
	fmt.Fprintf(a.out, "%d  %s  project=%d  exclusions=%s\n", r.ID, safe(r.Path), r.ProjectID, safe(strings.Join(r.Exclusions, ",")))
}
func (a *app) recordLine(r model.Record) {
	fmt.Fprintf(a.out, "%d  [%s/%s] %s  project=%d", r.ID, r.Type, r.Status, safe(r.Title), r.ProjectID)
	if r.Planned != "" {
		fmt.Fprint(a.out, "  planned="+r.Planned)
	}
	if r.Due != "" {
		fmt.Fprint(a.out, "  due="+r.Due)
	}
	fmt.Fprintln(a.out)
}
func (a *app) project(args []string) error {
	if len(args) == 0 {
		return usage("project requires a subcommand")
	}
	cmd := args[0]
	f := a.flags("project " + cmd)
	path := ""
	n := 1
	switch cmd {
	case "add":
		n = 2
	case "list":
		n = 0
	case "edit":
		f.set.StringVar(&path, "path", "", "new path")
	case "remove", "restore":
	default:
		if cmd == "--help" || cmd == "-h" {
			return flag.ErrHelp
		}
		return usage("unknown project command")
	}
	xs, e := f.parse(args[1:], n)
	if e != nil {
		return e
	}
	if cmd == "list" {
		s, e := a.state()
		if e != nil {
			return e
		}
		return a.print(s.Projects, projects.Warnings(s))
	}
	if cmd == "edit" && path == "" {
		return usage("project edit requires --path")
	}
	var p model.Project
	var warnings []string
	e = a.update(func(s *model.State) error {
		var e error
		switch cmd {
		case "add":
			p, e = projects.Add(s, xs[0], xs[1])
		case "edit":
			p, e = projects.Edit(s, xs[0], path, nil)
		case "remove", "restore":
			archived := cmd == "remove"
			p, e = projects.Edit(s, xs[0], "", &archived)
		}
		if e == nil {
			warnings = projects.Warnings(*s)
		}
		return e
	})
	if e != nil {
		return e
	}
	return a.print(p, warnings)
}
func (a *app) root(args []string) error {
	if len(args) == 0 {
		return usage("root requires a subcommand")
	}
	cmd := args[0]
	f := a.flags("root " + cmd)
	n := 1
	clear := false
	switch cmd {
	case "list":
		n = 0
	case "add", "edit":
		f.set.Var(&f.exclude, "exclude", "excluded directory basename")
		if cmd == "edit" {
			f.set.BoolVar(&clear, "clear-exclusions", false, "clear custom exclusions")
		}
	case "remove":
	default:
		if cmd == "--help" || cmd == "-h" {
			return flag.ErrHelp
		}
		return usage("unknown root command")
	}
	xs, e := f.parse(args[1:], n)
	if e != nil {
		return e
	}
	if cmd == "list" {
		s, e := a.state()
		if e != nil {
			return e
		}
		return a.print(s.Roots, projects.Warnings(s))
	}
	if cmd == "edit" && ((!clear && len(f.exclude) == 0) || (clear && len(f.exclude) > 0)) {
		return usage("root edit requires --exclude or --clear-exclusions")
	}
	var rid int64
	if cmd == "edit" {
		rid, e = id(xs[0])
		if e != nil {
			return e
		}
	}
	var r model.Root
	var warnings []string
	e = a.update(func(s *model.State) error {
		var e error
		switch cmd {
		case "add":
			r, e = projects.AddRoot(s, xs[0], f.exclude)
		case "remove":
			e = projects.RemoveRoot(s, xs[0])
		case "edit":
			e = projects.Exclude(s, rid, f.exclude)
			for _, x := range s.Roots {
				if x.ID == rid {
					r = x
				}
			}
		}
		if e == nil {
			warnings = projects.Warnings(*s)
		}
		return e
	})
	if e != nil {
		return e
	}
	if cmd == "remove" {
		return a.print("Search root removed.", warnings)
	}
	return a.print(r, warnings)
}
func (a *app) find(args []string) (int, error) {
	f := a.flags("find")
	var hidden, excluded, all bool
	limit := 50
	f.set.BoolVar(&hidden, "hidden", false, "include hidden entries")
	f.set.BoolVar(&excluded, "include-excluded", false, "include excluded directories")
	f.set.BoolVar(&all, "all", false, "all matches")
	f.set.IntVar(&limit, "limit", 50, "maximum results")
	xs, e := f.parse(args, 1)
	if e != nil {
		return 0, e
	}
	if limit < 1 {
		return 0, usage("--limit must be positive")
	}
	if all && f.visited("limit") {
		return 0, usage("--all and --limit cannot be combined")
	}
	if strings.TrimSpace(xs[0]) == "" {
		return 0, usage("search text must not be empty")
	}
	s, e := a.state()
	if e != nil {
		return 0, e
	}
	roots := []search.Root{}
	for _, r := range projects.ActiveRoots(s) {
		roots = append(roots, search.Root{Path: r.Path, Exclusions: r.Exclusions})
	}
	if all {
		limit = 0
	}
	result, e := search.Find(a.ctx, roots, xs[0], search.Options{Hidden: hidden, IncludeExcluded: excluded, Limit: limit})
	if e != nil {
		return 0, e
	}
	e = a.print(result, result.Warnings)
	code := 0
	if len(result.Warnings) > 0 {
		code = 3
	}
	return code, e
}
func (a *app) today(args []string) error {
	f := a.flags("today")
	f.scopes(true)
	if _, e := f.parse(args, 0); e != nil {
		return e
	}
	if e := f.validateScope(); e != nil {
		return e
	}
	s, e := a.state()
	if e != nil {
		return e
	}
	scope, e := f.scope(s, false)
	if e != nil {
		return e
	}
	return a.print(today.View(s, scope, time.Now()), nil)
}
func (a *app) task(args []string) error {
	if len(args) == 0 {
		return usage("task requires a subcommand")
	}
	cmd := args[0]
	f := a.flags("task " + cmd)
	n := 1
	var title, notes, kind, planned, due string
	var clearNotes, clearPlanned, clearDue, finished, yes bool
	switch cmd {
	case "add", "edit":
		f.scopes(false)
		f.set.StringVar(&notes, "notes", "", "notes")
		f.set.StringVar(&kind, "type", "", "task or bug")
		f.set.StringVar(&planned, "planned", "", "planned date")
		f.set.StringVar(&due, "due", "", "due date")
		if cmd == "edit" {
			f.set.StringVar(&title, "title", "", "title")
			f.set.BoolVar(&clearNotes, "clear-notes", false, "clear notes")
			f.set.BoolVar(&clearPlanned, "clear-planned", false, "clear planned date")
			f.set.BoolVar(&clearDue, "clear-due", false, "clear due date")
		}
	case "list":
		n = 0
		f.scopes(true)
		f.set.BoolVar(&finished, "include-finished", false, "include done/cancelled")
	case "delete":
		f.set.BoolVar(&yes, "yes", false, "confirm permanent deletion")
	case "show", "start", "done", "cancel", "reopen":
	default:
		if cmd == "--help" || cmd == "-h" {
			return flag.ErrHelp
		}
		return usage("unknown task command")
	}
	xs, e := f.parse(args[1:], n)
	if e != nil {
		return e
	}
	if e = f.validateScope(); e != nil {
		return e
	}
	if (clearNotes && f.visited("notes")) || (clearPlanned && f.visited("planned")) || (clearDue && f.visited("due")) {
		return usage("a field value cannot be combined with its clear option")
	}
	if e = model.Date(planned); e != nil {
		return usage(e.Error())
	}
	if e = model.Date(due); e != nil {
		return usage(e.Error())
	}
	if f.visited("type") && kind != "task" && kind != "bug" {
		return usage("type must be task or bug")
	}
	if cmd == "add" && strings.TrimSpace(xs[0]) == "" || cmd == "edit" && f.visited("title") && strings.TrimSpace(title) == "" {
		return usage("title must not be empty")
	}
	var rid int64
	if cmd != "add" && cmd != "list" {
		rid, e = id(xs[0])
		if e != nil {
			return e
		}
	}
	if cmd == "list" || cmd == "show" {
		s, e := a.state()
		if e != nil {
			return e
		}
		if cmd == "show" {
			r, e := records.Find(s, rid)
			if e != nil {
				return e
			}
			return a.print(r, nil)
		}
		scope, e := f.scope(s, true)
		if e != nil {
			return e
		}
		return a.print(records.List(s, scope, finished), nil)
	}
	if cmd == "delete" && !yes {
		if a.json {
			return usage("JSON deletion requires --yes")
		}
		// Require a real interactive terminal, never consume piped input as approval.
		input, ok := a.in.(*os.File)
		if !ok {
			return usage("noninteractive deletion requires --yes")
		}
		if !isatty.IsTerminal(input.Fd()) {
			return usage("noninteractive deletion requires --yes")
		}
		s, e := a.state()
		if e != nil {
			return e
		}
		r, e := records.Find(s, rid)
		if e != nil {
			return e
		}
		fmt.Fprintf(a.err, "Permanently delete record %d (%s)? Type yes: ", rid, safe(r.Title))
		line, e := bufio.NewReader(a.in).ReadString('\n')
		if e != nil {
			return e
		}
		if strings.TrimSpace(line) != "yes" {
			return a.print("Deletion cancelled.", nil)
		}
	}
	if cmd == "edit" && !f.visited("title") && !f.visited("notes") && !f.visited("type") && !f.visited("planned") && !f.visited("due") && !clearNotes && !clearPlanned && !clearDue && !f.visited("project") && !f.personal {
		return usage("task edit requires at least one change")
	}
	var result model.Record
	e = a.update(func(s *model.State) error {
		var e error
		if cmd == "add" {
			scope, e := f.scope(*s, true)
			if e != nil {
				return e
			}
			result, e = records.Add(s, model.Record{Title: xs[0], Notes: notes, Type: kind, Planned: planned, Due: due, ProjectID: scope.ProjectID})
			return e
		}
		if cmd == "delete" {
			return records.Delete(s, rid)
		}
		p := records.Patch{}
		if cmd == "edit" {
			if f.visited("title") {
				p.Title = &title
			}
			if f.visited("notes") || clearNotes {
				p.Notes = &notes
			}
			if f.visited("type") {
				p.Type = &kind
			}
			if f.visited("planned") || clearPlanned {
				p.Planned = &planned
			}
			if f.visited("due") || clearDue {
				p.Due = &due
			}
			if f.project != "" || f.personal {
				scope, e := f.scope(*s, false)
				if e != nil {
					return e
				}
				p.ProjectID = &scope.ProjectID
			}
		} else {
			statuses := map[string]string{"start": "in-progress", "done": "done", "cancel": "cancelled", "reopen": "open"}
			status := statuses[cmd]
			p.Status = &status
		}
		result, e = records.Edit(s, rid, p)
		return e
	})
	if e != nil {
		return e
	}
	if cmd == "delete" {
		return a.print(fmt.Sprintf("Record %d deleted.", rid), nil)
	}
	return a.print(result, nil)
}
func (a *app) backup(cmd string, args []string) error {
	f := a.flags(cmd)
	if cmd == "restore" {
		f.set.Var(&f.remap, "remap", "old=new path prefix")
	}
	xs, e := f.parse(args, 1)
	if e != nil {
		return e
	}
	mappings := map[string]string{}
	for _, raw := range f.remap {
		from, to, e := backup.ParseMapping(raw)
		if e != nil {
			return usage(e.Error())
		}
		if _, ok := mappings[from]; ok {
			return usage("duplicate remap source")
		}
		mappings[from] = to
	}
	if e = a.open(); e != nil {
		return e
	}
	if cmd == "export" {
		if e = backup.Export(a.ctx, a.store, xs[0]); e != nil {
			return e
		}
		return a.print("Backup exported to "+xs[0], nil)
	}
	warnings, e := backup.Restore(a.ctx, a.store, xs[0], mappings)
	if e != nil {
		return e
	}
	return a.print("Backup restored.", warnings)
}
