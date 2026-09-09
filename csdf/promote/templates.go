package promote

import (
	"fmt"
	"os"
	"strings"
	"text/template"
)

// Clause is what a clause template is rendered with. Src and Dst are the names
// of the local states the edge runs between, and Var is the state variable a
// frame clause is about; each template uses the fields its own clause needs.
type Clause struct {
	Map string
	ID  string
	Src string
	Dst string
	Var string
}

// The clauses the expansion generates, each replaceable on its own. "and" is
// what the clauses of one predicate are joined with.
const (
	clauseInDom    = "inDom"
	clauseNotInDom = "notInDom"
	clauseAtState  = "atState"
	clauseUpdate   = "update"
	clauseCreate   = "create"
	clauseDelete   = "delete"
	clauseFrame    = "frame"
	clauseAnd      = "and"
)

// defaultTemplates is the symbolic wording, which is the default because it does
// not belong to any one natural language. A -template file replaces the clauses
// it names and leaves the rest of these in place.
const defaultTemplates = `
{{define "inDom"}}{{.ID}} ∈ dom {{.Map}}{{end}}
{{define "notInDom"}}{{.ID}} ∉ dom {{.Map}}{{end}}
{{define "atState"}}{{.Map}}({{.ID}}) ∈ 〈{{.Src}}〉{{end}}
{{define "update"}}{{.Map}}' = {{.Map}} ⊕ { {{- .ID}} ↦ 〈{{.Dst}}〉}{{end}}
{{define "create"}}{{.Map}}' = {{.Map}} ∪ { {{- .ID}} ↦ 〈{{.Dst}}〉}{{end}}
{{define "delete"}}{{.Map}}' = { {{- .ID -}} } ⩤ {{.Map}}{{end}}
{{define "frame"}}{{.Var}}' = {{.Var}}{{end}}
{{define "and"}} ∧ {{end}}
`

// Templates is the wording of the generated clauses.
type Templates struct{ t *template.Template }

// DefaultTemplates returns the symbolic wording.
func DefaultTemplates() *Templates {
	return &Templates{t: template.Must(template.New("promote").Parse(defaultTemplates))}
}

// LoadTemplates reads a template file over the defaults. A clause the file does
// not define keeps its symbolic wording.
//
// Every clause is rendered once here, so a template that cannot render is a
// refusal rather than an opaque predicate in the output.
func LoadTemplates(path string) (*Templates, error) {
	bs, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("promote.LoadTemplates: cannot read file: %w: %q", err, path)
	}

	t, err := DefaultTemplates().t.Clone()
	if err != nil {
		return nil, fmt.Errorf("promote.LoadTemplates: %w", err)
	}
	if _, err := t.Parse(string(bs)); err != nil {
		return nil, fmt.Errorf("promote.LoadTemplates: %q: %w", path, err)
	}

	ts := &Templates{t: t}
	probe := Clause{Map: "m", ID: "id", Src: "S", Dst: "T", Var: "v"}
	for _, name := range []string{clauseInDom, clauseNotInDom, clauseAtState, clauseUpdate, clauseCreate, clauseDelete, clauseFrame, clauseAnd} {
		if _, err := ts.render(name, probe); err != nil {
			return nil, fmt.Errorf("promote.LoadTemplates: %q: %w", path, err)
		}
	}
	return ts, nil
}

// clause renders one clause. A template that fails here has already been
// rendered once by LoadTemplates, so the error is reported rather than hidden
// inside a predicate.
func (ts *Templates) clause(name string, c Clause) string {
	out, err := ts.render(name, c)
	if err != nil {
		panic(fmt.Errorf("promote: rendering the %q clause: %w", name, err))
	}
	return out
}

func (ts *Templates) render(name string, c Clause) (string, error) {
	var sb strings.Builder
	if err := ts.t.ExecuteTemplate(&sb, name, c); err != nil {
		return "", err
	}
	return sb.String(), nil
}

// join puts the clauses of one predicate together.
func (ts *Templates) join(clauses []string) string {
	return strings.Join(clauses, ts.clause(clauseAnd, Clause{}))
}
