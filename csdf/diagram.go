package csdf

import (
	"fmt"
	"os"
	"regexp"

	"github.com/Kuniwak/puml-parallel/pngsrc"
)

// What a global diagram carries and a plain one does not. Such a diagram's edges
// are not the whole of its behaviour, so parsing it here would be worse than
// failing - and it does fail, on syntax the core grammar has no rule for. The
// hint says what the author is missing rather than leaving them to read a column
// number.
//
// A note on its own is not a trace: notes are PlantUML's, and the advice for one
// that was not wrapped is CSDF-IGNORE. It counts only next to a directive body.
var (
	promotionBlockRe     = regexp.MustCompile(`(?m)^\s*state\s.*<<promote>>|^\s*!include\s`)
	promotionNoteRe      = regexp.MustCompile(`(?m)^\s*note\s+(?:as|left|right|top|bottom)\s`)
	promotionDirectiveRe = regexp.MustCompile(`(?m)^\s*(?:sync|constrain)\s`)
)

// PromotionHintError is a parse error on a source that still holds promotion
// directives. It wraps the parse error, so a caller can still reach it.
type PromotionHintError struct{ err error }

func (e *PromotionHintError) Error() string {
	return e.err.Error() + ": the source holds promotion directives; run csdfpromote on it first"
}

func (e *PromotionHintError) Unwrap() error { return e.err }

// UserFacing marks this error's own message as the one to show, so that the
// hint is not unwrapped away on its road to the terminal.
func (e *PromotionHintError) UserFacing() {}

// ParseBytes parses a Composable State Diagram from raw .puml text or .png
// bytes (the embedded PlantUML source is extracted from PNG inputs).
func ParseBytes(content []byte) (*Diagram, error) {
	source, err := pngsrc.Extract(content)
	if err != nil {
		return nil, fmt.Errorf("csdf.ParseBytes: reading PlantUML source: %w", err)
	}
	diagram, err := NewParser(source).Parse()
	if err != nil {
		return nil, fmt.Errorf("csdf.ParseBytes: parse: %w", withHint(err, source))
	}
	return diagram, nil
}

func Parse(content string) (*Diagram, error) {
	diagram, err := NewParser(content).Parse()
	if err != nil {
		return nil, fmt.Errorf("csdf.Parse: parse: %w", withHint(err, content))
	}
	return diagram, nil
}

// withHint adds the promotion hint to a parse error when the source looks like a
// global diagram.
func withHint(err error, source string) error {
	if !holdsPromotionDirectives(source) {
		return err
	}
	return &PromotionHintError{err: err}
}

func holdsPromotionDirectives(source string) bool {
	if promotionBlockRe.MatchString(source) {
		return true
	}
	return promotionNoteRe.MatchString(source) && promotionDirectiveRe.MatchString(source)
}

func MustParse(content string) *Diagram {
	d, err := Parse(content)
	if err != nil {
		panic(fmt.Errorf("csdf.MustParse: %w", err))
	}
	return d
}

// LoadDiagram reads and parses the diagram stored at path, which may be either
// .puml text or a .png image written by PlantUML.
func LoadDiagram(path string) (*Diagram, error) {
	bs, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read file: %w: %q", err, path)
	}

	diagram, err := ParseBytes(bs)
	if err != nil {
		return nil, fmt.Errorf("cannot parse file: %w: %q", err, path)
	}
	return diagram, nil
}

func LoadDiagrams(files []string) ([]*Diagram, error) {
	diagrams := make([]*Diagram, 0, len(files))
	for _, file := range files {
		diagram, err := LoadDiagram(file)
		if err != nil {
			return nil, fmt.Errorf("csdf.LoadDiagrams: %w", err)
		}
		diagrams = append(diagrams, diagram)
	}
	return diagrams, nil
}

func MustLoadDiagrams(paths ...string) []*Diagram {
	diagrams, err := LoadDiagrams(paths)
	if err != nil {
		panic(err.Error())
	}
	return diagrams
}
