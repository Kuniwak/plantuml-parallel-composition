package csdf

import (
	"errors"
	"fmt"
	"os"

	"github.com/Kuniwak/puml-parallel/pngsrc"
)

// ParseBytes parses a Composable State Diagram from raw .puml text or .png
// bytes (the embedded PlantUML source is extracted from PNG inputs). It rejects
// a diagram that still carries promotion directives, because the edges of such
// a diagram are not the whole of its behaviour.
func ParseBytes(content []byte) (*Diagram, error) {
	diagram, err := ParseBytesAllowingDirectives(content)
	if err != nil {
		return nil, err
	}
	if diagram.HasDirectives() {
		return nil, fmt.Errorf("csdf.ParseBytes: %w", ErrDirectives)
	}
	return diagram, nil
}

// ErrDirectives is what every tool but csdfpromote reports for a diagram that
// still carries promotion directives.
var ErrDirectives = errors.New("diagram contains promotion directives (promote/sync/constrain); run csdfpromote first")

// ParseBytesAllowingDirectives is ParseBytes without that rejection. Only
// csdfpromote, which consumes the directives, may use it.
func ParseBytesAllowingDirectives(content []byte) (*Diagram, error) {
	source, err := pngsrc.Extract(content)
	if err != nil {
		return nil, fmt.Errorf("csdf.ParseBytes: reading PlantUML source: %w", err)
	}
	diagram, err := NewParser(source).Parse()
	if err != nil {
		return nil, fmt.Errorf("csdf.ParseBytes: parse: %w", err)
	}
	return diagram, nil
}

func Parse(content string) (*Diagram, error) {
	diagram, err := NewParser(content).Parse()
	if err != nil {
		return nil, fmt.Errorf("csdf.Parse: parse: %w", err)
	}
	return diagram, nil
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
