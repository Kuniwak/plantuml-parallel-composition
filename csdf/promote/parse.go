package promote

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/pngsrc"
)

// The directives are all line-shaped, and so is PlantUML's own syntax for them,
// so they are lifted out line by line. Every line that is lifted is replaced by
// an empty one rather than dropped, which keeps the line numbers csdf.Parse
// reports pointing at the source the author wrote.
var (
	compositeOpenRe = regexp.MustCompile(`^state\s+"((?:[^"\\]|\\.)*)"\s+as\s+([\w-]+)\s*\{$`)
	promoteRe       = regexp.MustCompile(`^state\s+"((?:[^"\\]|\\.)*)"\s+as\s+([\w-]+)\s+<<promote>>\s*(\{?)$`)
	closeRe         = regexp.MustCompile(`^}$`)
	includeRe       = regexp.MustCompile(`^!include\s+(.+)$`)
	noteFloatingRe  = regexp.MustCompile(`^note\s+as\s+([\w-]+)$`)
	noteAnchoredRe  = regexp.MustCompile(`^note\s+(?:left|right|top|bottom)\s+of\s+([\w-]+)$`)
	noteEndRe       = regexp.MustCompile(`^end\s+note$`)
	promoteTitleRe  = regexp.MustCompile(`^(\S+)\s*:\s*(.+?)\s*(?:⇸|->>)\s*(\S+)$`)
	syncBodyRe      = regexp.MustCompile(`^sync\s+([^(;:]+?)\s*:\s*(.+)$`)
	constrainBodyRe = regexp.MustCompile(`^constrain\s+([^(;]+?)\s*\(([^()]*)\)\s*;\s*(.+)$`)
	mapRefRe        = regexp.MustCompile(`^([^\s()]+)\s*\(([^()]*)\)$`)
)

// ParseGlobal reads a global diagram written in the upper-compatible grammar.
func ParseGlobal(source string) (*GlobalDiagram, error) {
	s := &scanner{lines: strings.Split(source, "\n")}
	if err := s.run(); err != nil {
		return nil, fmt.Errorf("promote.ParseGlobal: %w", err)
	}

	core, err := csdf.Parse(strings.Join(s.out, "\n"))
	if err != nil {
		return nil, fmt.Errorf("promote.ParseGlobal: %w", err)
	}

	return &GlobalDiagram{
		Core:        core,
		Promotes:    s.promotes,
		Syncs:       s.syncs,
		Constrains:  s.constrains,
		Diagnostics: s.diags,
	}, nil
}

// scanner walks the source once, writing the plain-CSDF rewrite into out as it
// goes and collecting the directives it lifts.
type scanner struct {
	lines []string
	out   []string

	promotes   []Promote
	syncs      []Sync
	constrains []Constrain
	diags      []Diagnostic

	// parent is the composite state the scanner is inside, empty at the top
	// level. Only one level is allowed, so this is a name rather than a stack.
	parent csdf.StateID
}

func (s *scanner) run() error {
	s.out = make([]string, 0, len(s.lines))

	for i := 0; i < len(s.lines); {
		line := s.lines[i]
		text := strings.TrimSpace(line)

		switch {
		case promoteRe.MatchString(text):
			n, err := s.readPromote(i)
			if err != nil {
				return err
			}
			i = n

		case compositeOpenRe.MatchString(text):
			n, err := s.readComposite(i)
			if err != nil {
				return err
			}
			i = n

		case noteFloatingRe.MatchString(text), noteAnchoredRe.MatchString(text):
			n, err := s.readNote(i)
			if err != nil {
				return err
			}
			i = n

		case strings.HasPrefix(text, "!"):
			s.readPreprocessor(i)
			i++

		default:
			s.out = append(s.out, line)
			i++
		}
	}

	if s.parent != "" {
		return fmt.Errorf("unterminated composite state %q", s.parent)
	}
	return nil
}

// readComposite flattens "state ... {" into an ordinary state declaration and
// keeps scanning its body, which may hold state variables and <<promote>> blocks
// and nothing else.
func (s *scanner) readComposite(i int) (int, error) {
	m := compositeOpenRe.FindStringSubmatch(strings.TrimSpace(s.lines[i]))
	if s.parent != "" {
		return 0, fmt.Errorf("line %d: composite state %q is nested inside %q; only a <<promote>> block may be nested", i+1, m[2], s.parent)
	}
	s.parent = csdf.StateID(m[2])
	s.out = append(s.out, fmt.Sprintf("state %q as %s", m[1], m[2]))

	for i++; i < len(s.lines); {
		text := strings.TrimSpace(s.lines[i])
		if closeRe.MatchString(text) {
			s.parent = ""
			s.blank(1)
			return i + 1, nil
		}

		switch {
		case promoteRe.MatchString(text):
			n, err := s.readPromote(i)
			if err != nil {
				return 0, err
			}
			i = n
		case compositeOpenRe.MatchString(text):
			return 0, fmt.Errorf("line %d: composite state is nested inside %q; only a <<promote>> block may be nested", i+1, s.parent)
		case noteFloatingRe.MatchString(text), noteAnchoredRe.MatchString(text):
			// PlantUML lets a note sit inside the state it points at, which is
			// where the constrain of one family naturally goes.
			n, err := s.readNote(i)
			if err != nil {
				return 0, err
			}
			i = n
		case strings.HasPrefix(text, "!"):
			s.readPreprocessor(i)
			i++
		default:
			s.out = append(s.out, s.lines[i])
			i++
		}
	}

	return 0, fmt.Errorf("unterminated composite state %q", s.parent)
}

// readPromote lifts one <<promote>> block out of the source. The block's body,
// when it has one, is a single !include naming the local diagram.
func (s *scanner) readPromote(i int) (int, error) {
	line := i + 1
	m := promoteRe.FindStringSubmatch(strings.TrimSpace(s.lines[i]))

	title := promoteTitleRe.FindStringSubmatch(m[1])
	if title == nil {
		return 0, fmt.Errorf("line %d: expected a <<promote>> title of the form \"<map> : <ID> ⇸ <Type>\", got %q", line, m[1])
	}

	p := Promote{
		Map:     csdf.Var(title[1]),
		IDParam: title[2],
		Type:    title[3],
		Alias:   csdf.StateID(m[2]),
		In:      s.parent,
		Line:    line,
	}

	if m[3] == "" { // No body: the block only says that the family moves here.
		s.blank(1)
		s.promotes = append(s.promotes, p)
		return i + 1, nil
	}

	body := i + 1
	for body < len(s.lines) && strings.TrimSpace(s.lines[body]) == "" {
		body++
	}
	if body >= len(s.lines) {
		return 0, fmt.Errorf("line %d: unterminated <<promote>> block", line)
	}

	inc := includeRe.FindStringSubmatch(strings.TrimSpace(s.lines[body]))
	if inc == nil {
		return 0, fmt.Errorf("line %d: expected a single !include in the <<promote>> block opened at line %d, got %q", body+1, line, strings.TrimSpace(s.lines[body]))
	}
	p.Path = unquote(strings.TrimSpace(inc[1]))

	end := body + 1
	for end < len(s.lines) && strings.TrimSpace(s.lines[end]) == "" {
		end++
	}
	if end >= len(s.lines) || !closeRe.MatchString(strings.TrimSpace(s.lines[end])) {
		return 0, fmt.Errorf("line %d: expected a single !include in the <<promote>> block opened at line %d", line, line)
	}

	s.blank(end - i + 1)
	s.promotes = append(s.promotes, p)
	return end + 1, nil
}

// readPreprocessor drops a PlantUML preprocessor line. An !include here is not
// inside a <<promote>> block, so it names no local diagram; that is worth
// saying, because it is what a first attempt at a promotion looks like.
func (s *scanner) readPreprocessor(i int) {
	if includeRe.MatchString(strings.TrimSpace(s.lines[i])) {
		s.diags = append(s.diags, Diagnostic{
			Severity: SeverityInfo,
			Line:     i + 1,
			Message:  "this !include is not inside a <<promote>> block, so it names no local diagram and is dropped",
		})
	}
	s.blank(1)
}

// blank writes n empty lines, standing in for the source lines just consumed.
func (s *scanner) blank(n int) {
	for range n {
		s.out = append(s.out, "")
	}
}

func unquote(s string) string {
	if len(s) >= 2 && strings.HasPrefix(s, `"`) && strings.HasSuffix(s, `"`) {
		return s[1 : len(s)-1]
	}
	return s
}

// readNote lifts one note out of the source. A note whose first non-empty line
// starts with "sync" or "constrain" is a directive; any other note is there for
// the picture alone and is dropped without comment.
func (s *scanner) readNote(i int) (int, error) {
	open := i + 1
	text := strings.TrimSpace(s.lines[i])

	var anchor Anchor
	if m := noteFloatingRe.FindStringSubmatch(text); m != nil {
		anchor.NoteID = m[1]
	} else {
		anchor.State = csdf.StateID(noteAnchoredRe.FindStringSubmatch(text)[1])
	}

	end := i + 1
	for end < len(s.lines) && !noteEndRe.MatchString(strings.TrimSpace(s.lines[end])) {
		end++
	}
	if end >= len(s.lines) {
		return 0, fmt.Errorf("line %d: unterminated note", open)
	}

	read := false
	for body := i + 1; body < end; body++ {
		line := strings.TrimSpace(s.lines[body])
		if line == "" {
			continue
		}
		if !read {
			if err := s.readDirective(line, anchor, body+1); err != nil {
				return 0, err
			}
			read = len(s.syncs)+len(s.constrains) > 0
			if !read {
				// A note that is not a directive is there for the picture, and
				// the rest of it is prose.
				break
			}
			continue
		}
		if isDirective(line) {
			return 0, fmt.Errorf("line %d: this note holds a second directive; write one note per directive", body+1)
		}
	}

	s.blank(end - i + 1)
	return end + 1, nil
}

// readDirective interprets the first line of a note. Anything that is not a
// directive is a note the author wrote for the reader, so it is left alone.
func (s *scanner) readDirective(line string, anchor Anchor, at int) error {
	switch {
	case strings.HasPrefix(line, "sync "):
		m := syncBodyRe.FindStringSubmatch(line)
		if m == nil {
			return fmt.Errorf("line %d: expected \"sync <event> : <map>(<param>), ...\", got %q", at, line)
		}
		targets, err := parseMapRefs(m[2], at)
		if err != nil {
			return err
		}
		s.syncs = append(s.syncs, Sync{Anchor: anchor, Event: m[1], Targets: targets, Line: at})

	case strings.HasPrefix(line, "constrain "):
		m := constrainBodyRe.FindStringSubmatch(line)
		if m == nil {
			return fmt.Errorf("line %d: expected \"constrain <event>(<param>, ...) ; <guard>\", got %q", at, line)
		}
		s.constrains = append(s.constrains, Constrain{
			Anchor: anchor,
			Event:  m[1],
			Params: splitTrimmed(m[2]),
			Guard:  csdf.Predicate(strings.TrimSpace(m[3])),
			Line:   at,
		})

	default:
		// A note that is not a directive is there for the picture. One whose
		// first word is a directive's name in another case is a typo, though,
		// and silently drawing it is the worst of the two readings.
		if word := firstWord(line); word != "" && isDirective(strings.ToLower(word)+" ") {
			s.diags = append(s.diags, Diagnostic{
				Severity: SeverityWarning,
				Line:     at,
				Message:  fmt.Sprintf("this note starts with %q, which is not a directive; write %q to make it one", word, strings.ToLower(word)),
			})
		}
	}
	return nil
}

// isDirective reports whether a note line opens a directive.
func isDirective(line string) bool {
	return strings.HasPrefix(line, "sync ") || strings.HasPrefix(line, "constrain ")
}

func firstWord(line string) string {
	return strings.Fields(line)[0]
}

func parseMapRefs(s string, at int) ([]MapRef, error) {
	refs := make([]MapRef, 0, 2)
	for _, part := range splitTrimmed(s) {
		m := mapRefRe.FindStringSubmatch(part)
		if m == nil {
			return nil, fmt.Errorf("line %d: expected \"<map>(<param>)\", got %q", at, part)
		}
		refs = append(refs, MapRef{Map: csdf.Var(m[1]), Param: strings.TrimSpace(m[2])})
	}
	return refs, nil
}

// splitTrimmed splits a comma-separated list. Commas cannot be nested here: a
// map reference holds one parameter and an event pattern holds bare names.
func splitTrimmed(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// ParseGlobalBytes reads a global diagram from raw .puml text or .png bytes
// (the embedded PlantUML source is extracted from PNG inputs).
func ParseGlobalBytes(content []byte) (*GlobalDiagram, error) {
	source, err := pngsrc.Extract(content)
	if err != nil {
		return nil, fmt.Errorf("promote.ParseGlobalBytes: reading PlantUML source: %w", err)
	}
	return ParseGlobal(source)
}

// LoadGlobal reads and parses the global diagram stored at path.
func LoadGlobal(path string) (*GlobalDiagram, error) {
	bs, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("promote.LoadGlobal: cannot read file: %w: %q", err, path)
	}

	g, err := ParseGlobalBytes(bs)
	if err != nil {
		return nil, fmt.Errorf("promote.LoadGlobal: cannot parse file %q: %w", path, err)
	}
	return g, nil
}
