package csdf

import (
	"fmt"
	"os"
	"regexp"

	"github.com/Kuniwak/puml-parallel/pngsrc"
)

// promotionTraceRe matches what a global diagram carries and a plain one does
// not: a <<promote>> block, the !include that names its local diagram, or the
// note a sync or constrain is written in. Such a diagram's edges are not the
// whole of its behaviour, so parsing it here would be worse than failing - and
// it does fail, on syntax the core grammar has no rule for. The hint says what
// the author is missing rather than leaving them to read a column number.
var promotionTraceRe = regexp.MustCompile(`(?m)^\s*(?:<<promote>>|!include\s|note\s+(?:as|left|right|top|bottom)\s)|<<promote>>`)

// promotionHint is appended to a parse error when the source looks like a global
// diagram that has not been expanded yet.
const promotionHint = "the source holds promotion directives; run csdfpromote on it first"

// ParseBytes parses a Composable State Diagram from raw .puml text or .png
// bytes (the embedded PlantUML source is extracted from PNG inputs).
func ParseBytes(content []byte) (*Diagram, error) {
	source, err := pngsrc.Extract(content)
	if err != nil {
		return nil, fmt.Errorf("csdf.ParseBytes: reading PlantUML source: %w", err)
	}
	diagram, err := NewParser(source).Parse()
	if err != nil {
		return nil, fmt.Errorf("csdf.ParseBytes: parse: %w%s", err, hint(source))
	}
	return diagram, nil
}

func Parse(content string) (*Diagram, error) {
	diagram, err := NewParser(content).Parse()
	if err != nil {
		return nil, fmt.Errorf("csdf.Parse: parse: %w%s", err, hint(content))
	}
	return diagram, nil
}

// hint returns the promotion hint, prefixed for appending to an error, when the
// source looks like a global diagram; otherwise the empty string.
func hint(source string) string {
	if promotionTraceRe.MatchString(source) {
		return ": " + promotionHint
	}
	return ""
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
