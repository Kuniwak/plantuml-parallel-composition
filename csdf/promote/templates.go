package promote

import (
	"fmt"
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

func mustParseTemplates(text string) *Templates {
	templates, err := ParseTemplates(text)
	if err != nil {
		panic(err)
	}
	return templates
}

var defaultTemplates = mustParseTemplates("")

// clause renders one clause. A template that fails to render leaves its own
// error in the predicate, where a reader cannot miss it: a predicate is opaque
// text, so there is nothing else to check it against.
func (t *Templates) clause(name string, data clauseData) string {
	var sb strings.Builder
	if err := t.t.ExecuteTemplate(&sb, name, data); err != nil {
		return fmt.Sprintf("<%s: %v>", name, err)
	}
	return sb.String()
}
