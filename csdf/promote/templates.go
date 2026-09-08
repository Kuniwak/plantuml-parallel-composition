package promote

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"text/template"

	"github.com/Kuniwak/puml-parallel/csdf"
)

// DefaultTemplates is the symbolic form of the phrases expansion generates. It
// is deliberately not written in any natural language: a specification written
// in one replaces it with -template.
//
// The clauses are:
//
//	exists     the instance is in the map
//	absent     the instance is not in the map
//	at         the instance is in the local state it comes from
//	insert     the instance is created
//	update     the instance moves to the local state it goes to
//	delete     the instance is removed
//	keep       the instance was absent and stays absent
//	unchanged  one other state variable of the global state does not move
//
// The guard and the post of the local edge are inserted verbatim, and the
// clauses of one predicate are joined with " ∧ ".
const DefaultTemplates = `
{{define "exists"}}{{.ID}} ∈ dom {{.Map}}{{end}}
{{define "absent"}}{{.ID}} ∉ dom {{.Map}}{{end}}
{{define "at"}}{{.Map}}({{.ID}}) ∈ 〈{{.Src}}〉{{end}}
{{define "insert"}}{{.Map}}' = {{.Map}} ∪ {{printf "{%s ↦ 〈%s〉}" .ID .Dst}}{{end}}
{{define "update"}}{{.Map}}' = {{.Map}} ⊕ {{printf "{%s ↦ 〈%s〉}" .ID .Dst}}{{end}}
{{define "delete"}}{{.Map}}' = {{printf "{%s}" .ID}} ⩤ {{.Map}}{{end}}
{{define "keep"}}{{.Map}}' = {{.Map}}{{end}}
{{define "unchanged"}}{{.Other}}' = {{.Other}}{{end}}
`

// Conjunction joins the clauses of one generated predicate.
const Conjunction = " ∧ "

// clauseData is what every clause template is given. Src and Dst are the names
// of the local states the edge runs between, and Other is the state variable a
// frame clause is about.
type clauseData struct {
	Map       csdf.Var
	ID        string
	Src       string
	Dst       string
	Guard     csdf.Predicate
	Post      csdf.Predicate
	Other     csdf.Var
	OtherMaps []csdf.Var
}

// Templates renders the generated phrases of an expansion.
type Templates struct {
	t *template.Template
}

// ParseTemplates reads the clause templates of text over the default ones, so
// that a file may redefine only the clauses it cares about.
func ParseTemplates(text string) (*Templates, error) {
	t, err := template.New("promote").Parse(DefaultTemplates)
	if err != nil {
		return nil, fmt.Errorf("promote.ParseTemplates: the default templates are broken: %w", err)
	}
	if _, err := t.Parse(text); err != nil {
		return nil, fmt.Errorf("promote.ParseTemplates: %w", err)
	}
	return &Templates{t: t}, nil
}

// MustParseTemplates is ParseTemplates for templates known to be valid, such as
// a constant in the program itself.
func MustParseTemplates(text string) *Templates {
	templates, err := ParseTemplates(text)
	if err != nil {
		panic(err)
	}
	return templates
}

// LoadTemplates reads clause templates from the file at path, or returns the
// symbolic ones when path is empty.
func LoadTemplates(path string) (*Templates, error) {
	if path == "" {
		return DefaultClauseTemplates(), nil
	}
	bs, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("promote.LoadTemplates: %w", err)
	}
	return ParseTemplates(string(bs))
}

// DefaultClauseTemplates returns the symbolic phrases, parsed.
func DefaultClauseTemplates() *Templates { return defaultTemplates }

var defaultTemplates = MustParseTemplates("")

// renderer renders the clauses of one expansion, collecting the faults of a
// template that parses but cannot run. Such a fault would otherwise end up
// inside an opaque predicate, which nothing downstream can check.
type renderer struct {
	templates *Templates
	diags     []Diagnostic
}

func newRenderer(templates *Templates) *renderer {
	if templates == nil {
		templates = defaultTemplates
	}
	return &renderer{templates: templates}
}

// clause renders one clause. A clause is used by many edges, so the same fault
// is reported once.
func (r *renderer) clause(name string, data clauseData) string {
	var sb strings.Builder
	if err := r.templates.t.ExecuteTemplate(&sb, name, data); err != nil {
		message := fmt.Sprintf("template %q cannot be rendered: %v", name, err)
		if !slices.ContainsFunc(r.diags, func(diag Diagnostic) bool { return diag.Message == message }) {
			r.diags = append(r.diags, Diagnostic{Severity: SeverityError, Message: message})
		}
		return ""
	}
	return sb.String()
}
