package promote

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Kuniwak/puml-parallel/csdf"
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
		Core:       core,
		Promotes:   s.promotes,
		Syncs:      s.syncs,
		Constrains: s.constrains,
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

		case strings.HasPrefix(text, "!"):
			s.blank(1)
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
		case strings.HasPrefix(text, "!"):
			s.blank(1)
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
