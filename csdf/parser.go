package csdf

import (
	"fmt"
	"strings"
)

const (
	ignoreBeginMarker = "CSDF-IGNORE-BEGIN"
	ignoreEndMarker   = "CSDF-IGNORE-END"
)

type Parser struct {
	input string
	pos   int
	line  int
	col   int
}

func NewParser(input string) *Parser {
	return &Parser{
		input: input,
		pos:   0,
		line:  1,
		col:   1,
	}
}

func (p *Parser) Parse() (*Diagram, error) {
	diagram := &Diagram{
		States:     make(map[StateID]State),
		Edges:      []Edge{},
		Promotes:   []Promote{},
		Syncs:      []Sync{},
		Constrains: []Constrain{},
	}

	if !p.expectString("@startuml") {
		return nil, fmt.Errorf("csdf.Parser.Parse: expected @startuml at line %d, col %d", p.line, p.col)
	}

	if err := p.skipInlineTrivia(); err != nil {
		return nil, fmt.Errorf("csdf.Parser.Parse: %w", err)
	}
	// The diagram name carries no meaning, but the rest of the line is kept
	// verbatim so that printing the diagram round-trips it.
	nameStart := p.pos
	for !p.isAtEnd() && p.peek() != '\n' {
		p.advance()
	}
	diagram.Name = strings.TrimSpace(p.input[nameStart:p.pos])
	if !p.expectNewlines() {
		return nil, fmt.Errorf("csdf.Parser.Parse: expected newline after @startuml at line %d, col %d", p.line, p.col)
	}
	if err := p.skipTrivia(); err != nil {
		return nil, fmt.Errorf("csdf.Parser.Parse: %w", err)
	}

	// Parse all content until @enduml
	for !p.isAtEnd() && !p.peekString("@enduml") {
		if err := p.skipTrivia(); err != nil {
			return nil, fmt.Errorf("csdf.Parser.Parse: %w", err)
		}
		if p.isAtEnd() || p.peekString("@enduml") {
			break
		}
		if diagram.EndEdge != nil {
			return nil, fmt.Errorf("csdf.Parser.Parse: expected @enduml after end edge at line %d, col %d", p.line, p.col)
		}

		if p.peekString("state") {
			state, err := p.parseStateWithID()
			if err != nil {
				return nil, fmt.Errorf("csdf.Parser.Parse: %w", err)
			}
			diagram.States[state.ID] = state.State
		} else if p.peekString("[*]") {
			startEdge, err := p.parseStartEdge()
			if err != nil {
				return nil, fmt.Errorf("csdf.Parser.Parse: %w", err)
			}
			diagram.StartEdge = startEdge
		} else {
			isEdge, err := p.isEdge()
			if err != nil {
				return nil, fmt.Errorf("csdf.Parser.Parse: %w", err)
			}
			if !isEdge {
				// A promotion directive is never an edge, so the edge test
				// comes first: it leaves "sync" and "promote" usable as state
				// IDs.
				parsed, err := p.parseDirective(diagram)
				if err != nil {
					return nil, fmt.Errorf("csdf.Parser.Parse: %w", err)
				}
				if !parsed {
					return nil, fmt.Errorf("csdf.Parser.Parse: unexpected syntax at line %d, col %d", p.line, p.col)
				}
				continue
			}

			isEndEdge, err := p.isEndEdge()
			if err != nil {
				return nil, fmt.Errorf("csdf.Parser.Parse: %w", err)
			}
			if isEndEdge {
				endEdge, err := p.parseEndEdge()
				if err != nil {
					return nil, fmt.Errorf("csdf.Parser.Parse: %w", err)
				}
				diagram.EndEdge = &endEdge
			} else {
				edge, err := p.parseEdge()
				if err != nil {
					return nil, fmt.Errorf("csdf.Parser.Parse: %w", err)
				}
				diagram.Edges = append(diagram.Edges, edge)
			}
		}
	}

	if !p.expectString("@enduml") {
		return nil, fmt.Errorf("csdf.Parser.Parse: expected @enduml at line %d, col %d", p.line, p.col)
	}

	return diagram, nil
}

func (p *Parser) parseStateWithID() (StateWithID, error) {
	if !p.expectString("state") {
		return StateWithID{}, fmt.Errorf("csdf.Parser.parseState: expected 'state' at line %d, col %d", p.line, p.col)
	}
	if err := p.skipInlineTrivia(); err != nil {
		return StateWithID{}, fmt.Errorf("csdf.Parser.parseState: %w", err)
	}

	name, err := p.parseStateName()
	if err != nil {
		return StateWithID{}, fmt.Errorf("csdf.Parser.parseState: %w", err)
	}
	if err := p.skipInlineTrivia(); err != nil {
		return StateWithID{}, fmt.Errorf("csdf.Parser.parseState: %w", err)
	}

	if !p.expectString("as") {
		return StateWithID{}, fmt.Errorf("csdf.Parser.parseState: expected 'as' at line %d, col %d", p.line, p.col)
	}
	if err := p.skipInlineTrivia(); err != nil {
		return StateWithID{}, fmt.Errorf("csdf.Parser.parseState: %w", err)
	}

	id, err := p.parseID()
	if err != nil {
		return StateWithID{}, fmt.Errorf("csdf.Parser.parseState: %w", err)
	}

	state := StateWithID{
		ID: StateID(id),
		State: State{
			Name: name,
			Vars: []StateVar{},
			Line: p.line,
		},
	}

	if err := p.skipInlineTrivia(); err != nil {
		return StateWithID{}, fmt.Errorf("csdf.Parser.parseState: %w", err)
	}
	if !p.expectNewlines() {
		return StateWithID{}, fmt.Errorf("csdf.Parser.parseState: expected newline after state declaration at line %d, col %d", p.line, p.col)
	}
	if err := p.skipTrivia(); err != nil {
		return StateWithID{}, fmt.Errorf("csdf.Parser.parseState: %w", err)
	}

	for !p.isAtEnd() {
		isStateVar, err := p.isStateVar(state.ID)
		if err != nil {
			return StateWithID{}, fmt.Errorf("csdf.Parser.parseState: %w", err)
		}
		if !isStateVar {
			break
		}

		if _, err := p.parseID(); err != nil {
			return StateWithID{}, fmt.Errorf("csdf.Parser.parseState: %w", err)
		}
		if err := p.skipInlineTrivia(); err != nil {
			return StateWithID{}, fmt.Errorf("csdf.Parser.parseState: %w", err)
		}
		if !p.expectChar(':') {
			return StateWithID{}, fmt.Errorf("csdf.Parser.parseState: expected ':' after state ID in variable declaration at line %d, col %d", p.line, p.col)
		}
		if err := p.skipInlineTrivia(); err != nil {
			return StateWithID{}, fmt.Errorf("csdf.Parser.parseState: %w", err)
		}
		varName, err := p.parseID()
		if err != nil {
			return StateWithID{}, fmt.Errorf("csdf.Parser.parseState: %w", err)
		}
		if err := p.skipInlineTrivia(); err != nil {
			return StateWithID{}, fmt.Errorf("csdf.Parser.parseState: %w", err)
		}

		var varType string
		if p.expectChar(';') {
			if err := p.skipInlineTrivia(); err != nil {
				return StateWithID{}, fmt.Errorf("csdf.Parser.parseState: %w", err)
			}
			varType, err = p.parseUntilSemicolon()
			if err != nil {
				return StateWithID{}, fmt.Errorf("csdf.Parser.parseState: %w", err)
			}
			if p.peek() == ';' {
				return StateWithID{}, fmt.Errorf("csdf.Parser.parseState: unexpected ';' in variable type at line %d, col %d", p.line, p.col)
			}
		}

		state.Vars = append(state.Vars, StateVar{
			Name: Var(varName),
			Type: varType,
		})
		if !p.expectNewlines() {
			return StateWithID{}, fmt.Errorf("csdf.Parser.parseState: expected newline after variable declaration at line %d, col %d", p.line, p.col)
		}
		if err := p.skipTrivia(); err != nil {
			return StateWithID{}, fmt.Errorf("csdf.Parser.parseState: %w", err)
		}
	}

	return state, nil
}

func (p *Parser) parseStateName() (string, error) {
	if !p.expectChar('"') {
		return "", fmt.Errorf("csdf.Parser.parseStateName: expected '\"' at line %d, col %d", p.line, p.col)
	}

	var result strings.Builder
	for !p.isAtEnd() && p.peek() != '"' {
		if p.peek() == '\\' {
			p.advance()
			if p.isAtEnd() {
				return "", fmt.Errorf("csdf.Parser.parseStateName: unexpected end of input in string at line %d, col %d", p.line, p.col)
			}
			switch p.peek() {
			case '\\':
				result.WriteByte('\\')
			case '"':
				result.WriteByte('"')
			default:
				result.WriteByte('\\')
				result.WriteByte(p.peek())
			}
		} else {
			result.WriteByte(p.peek())
		}
		p.advance()
	}

	if !p.expectChar('"') {
		return "", fmt.Errorf("csdf.Parser.parseStateName: expected closing '\"' at line %d, col %d", p.line, p.col)
	}

	return result.String(), nil
}

func (p *Parser) parseStartEdge() (StartEdge, error) {
	line := p.line
	if !p.expectString("[*]") {
		return StartEdge{}, fmt.Errorf("csdf.Parser.parseStartEdge: expected '[*]' at line %d, col %d", p.line, p.col)
	}
	if err := p.skipInlineTrivia(); err != nil {
		return StartEdge{}, fmt.Errorf("csdf.Parser.parseStartEdge: %w", err)
	}

	if !p.expectString("-->") {
		return StartEdge{}, fmt.Errorf("csdf.Parser.parseStartEdge: expected '-->' at line %d, col %d", p.line, p.col)
	}
	if err := p.skipInlineTrivia(); err != nil {
		return StartEdge{}, fmt.Errorf("csdf.Parser.parseStartEdge: %w", err)
	}

	dst, err := p.parseID()
	if err != nil {
		return StartEdge{}, fmt.Errorf("csdf.Parser.parseStartEdge: expected destination state ID after '-->' in start edge at line %d, col %d", p.line, p.col)
	}
	if err := p.skipInlineTrivia(); err != nil {
		return StartEdge{}, fmt.Errorf("csdf.Parser.parseStartEdge: %w", err)
	}

	post := "true" // Default value when post is omitted
	if p.peek() == ':' {
		p.advance() // consume ':'
		if err := p.skipInlineTrivia(); err != nil {
			return StartEdge{}, fmt.Errorf("csdf.Parser.parseStartEdge: %w", err)
		}
		post, err = p.parseUntilNewline()
		if err != nil {
			return StartEdge{}, fmt.Errorf("csdf.Parser.parseStartEdge: %w", err)
		}
	}

	if !p.expectNewlines() {
		return StartEdge{}, fmt.Errorf("csdf.Parser.parseStartEdge: expected newline after start edge declaration at line %d, col %d", p.line, p.col)
	}

	return StartEdge{
		Dst:  StateID(dst),
		Post: Predicate(post),
		Line: line,
	}, nil
}

func (p *Parser) parseEdge() (Edge, error) {
	line := p.line
	src, err := p.parseID()
	if err != nil {
		return Edge{}, fmt.Errorf("csdf.Parser.parseEdge: %w", err)
	}
	if err := p.skipInlineTrivia(); err != nil {
		return Edge{}, fmt.Errorf("csdf.Parser.parseEdge: %w", err)
	}

	if !p.expectString("-->") {
		return Edge{}, fmt.Errorf("csdf.Parser.parseEdge: expected '-->' at line %d, col %d", p.line, p.col)
	}
	if err := p.skipInlineTrivia(); err != nil {
		return Edge{}, fmt.Errorf("csdf.Parser.parseEdge: %w", err)
	}

	dst, err := p.parseID()
	if err != nil {
		return Edge{}, fmt.Errorf("csdf.Parser.parseEdge: %w", err)
	}
	if err := p.skipInlineTrivia(); err != nil {
		return Edge{}, fmt.Errorf("csdf.Parser.parseEdge: %w", err)
	}

	if !p.expectChar(':') {
		return Edge{}, fmt.Errorf("csdf.Parser.parseEdge: expected ':' at line %d, col %d", p.line, p.col)
	}
	if err := p.skipInlineTrivia(); err != nil {
		return Edge{}, fmt.Errorf("csdf.Parser.parseEdge: %w", err)
	}

	event, err := p.parseEvent()
	if err != nil {
		return Edge{}, fmt.Errorf("csdf.Parser.parseEdge: %w", err)
	}
	if err := p.skipInlineTrivia(); err != nil {
		return Edge{}, fmt.Errorf("csdf.Parser.parseEdge: %w", err)
	}

	guard := "true" // Default value when guard is omitted
	post := "true"  // Default value when post is omitted

	if p.peek() == ';' {
		p.advance() // consume first ';'
		if err := p.skipInlineTrivia(); err != nil {
			return Edge{}, fmt.Errorf("csdf.Parser.parseEdge: %w", err)
		}
		guard, err = p.parseUntilSemicolon()
		if err != nil {
			return Edge{}, fmt.Errorf("csdf.Parser.parseEdge: %w", err)
		}
		if err := p.skipInlineTrivia(); err != nil {
			return Edge{}, fmt.Errorf("csdf.Parser.parseEdge: %w", err)
		}

		if p.peek() == ';' {
			p.advance() // consume second ';'
			if err := p.skipInlineTrivia(); err != nil {
				return Edge{}, fmt.Errorf("csdf.Parser.parseEdge: %w", err)
			}
			post, err = p.parseUntilNewline()
			if err != nil {
				return Edge{}, fmt.Errorf("csdf.Parser.parseEdge: %w", err)
			}
		}
	}

	if !p.expectNewlines() {
		return Edge{}, fmt.Errorf("csdf.Parser.parseEdge: expected newline after edge declaration at line %d, col %d", p.line, p.col)
	}

	return Edge{
		Src:   StateID(src),
		Dst:   StateID(dst),
		Event: event,
		Guard: Predicate(guard),
		Post:  Predicate(post),
		Line:  line,
	}, nil
}

// parseDirective parses one promotion directive into the diagram, reporting
// whether the input at the current position was a directive at all.
func (p *Parser) parseDirective(diagram *Diagram) (bool, error) {
	switch {
	case p.peekKeyword("promote"):
		promote, err := p.parsePromote()
		if err != nil {
			return false, err
		}
		diagram.Promotes = append(diagram.Promotes, promote)
	case p.peekKeyword("sync"):
		sync, err := p.parseSync()
		if err != nil {
			return false, err
		}
		diagram.Syncs = append(diagram.Syncs, sync)
	case p.peekKeyword("constrain"):
		constrain, err := p.parseConstrain()
		if err != nil {
			return false, err
		}
		diagram.Constrains = append(diagram.Constrains, constrain)
	default:
		return false, nil
	}
	return true, nil
}

// parsePromote parses
//
//	promote <path> as <Type> via <map>(<idParam>) [in <stateID>, ...]
//
// The "in" clause is the set of global states the local diagram is expanded
// into; omitting it means the destination of the start edge alone.
func (p *Parser) parsePromote() (Promote, error) {
	line := p.line
	if !p.expectString("promote") {
		return Promote{}, fmt.Errorf("csdf.Parser.parsePromote: expected 'promote' at line %d, col %d", p.line, p.col)
	}
	if err := p.skipInlineTrivia(); err != nil {
		return Promote{}, fmt.Errorf("csdf.Parser.parsePromote: %w", err)
	}

	path, err := p.parsePath()
	if err != nil {
		return Promote{}, fmt.Errorf("csdf.Parser.parsePromote: %w", err)
	}
	if err := p.skipInlineTrivia(); err != nil {
		return Promote{}, fmt.Errorf("csdf.Parser.parsePromote: %w", err)
	}

	if !p.expectString("as") {
		return Promote{}, fmt.Errorf("csdf.Parser.parsePromote: expected 'as' after the local diagram path at line %d, col %d", p.line, p.col)
	}
	if err := p.skipInlineTrivia(); err != nil {
		return Promote{}, fmt.Errorf("csdf.Parser.parsePromote: %w", err)
	}

	typeName, err := p.parseID()
	if err != nil {
		return Promote{}, fmt.Errorf("csdf.Parser.parsePromote: expected a type name after 'as' at line %d, col %d", p.line, p.col)
	}
	if err := p.skipInlineTrivia(); err != nil {
		return Promote{}, fmt.Errorf("csdf.Parser.parsePromote: %w", err)
	}

	if !p.expectString("via") {
		return Promote{}, fmt.Errorf("csdf.Parser.parsePromote: expected 'via' after the type name at line %d, col %d", p.line, p.col)
	}
	if err := p.skipInlineTrivia(); err != nil {
		return Promote{}, fmt.Errorf("csdf.Parser.parsePromote: %w", err)
	}

	mapRef, err := p.parseMapRef()
	if err != nil {
		return Promote{}, fmt.Errorf("csdf.Parser.parsePromote: %w", err)
	}
	if err := p.skipInlineTrivia(); err != nil {
		return Promote{}, fmt.Errorf("csdf.Parser.parsePromote: %w", err)
	}

	var in []StateID
	if p.peekKeyword("in") {
		in, err = p.parseInClause()
		if err != nil {
			return Promote{}, fmt.Errorf("csdf.Parser.parsePromote: %w", err)
		}
	}

	if !p.expectNewlines() {
		return Promote{}, fmt.Errorf("csdf.Parser.parsePromote: expected newline after promote directive at line %d, col %d", p.line, p.col)
	}

	return Promote{
		Path:    path,
		Type:    typeName,
		Map:     mapRef.Map,
		IDParam: mapRef.Param,
		In:      in,
		Line:    line,
	}, nil
}

// parseSync parses
//
//	sync <event> : <map1>(<param1>), <map2>(<param2>), ...
//
// The event is the local event name, so it is written without its arguments.
func (p *Parser) parseSync() (Sync, error) {
	line := p.line
	if !p.expectString("sync") {
		return Sync{}, fmt.Errorf("csdf.Parser.parseSync: expected 'sync' at line %d, col %d", p.line, p.col)
	}
	if err := p.skipInlineTrivia(); err != nil {
		return Sync{}, fmt.Errorf("csdf.Parser.parseSync: %w", err)
	}

	event, err := p.parseUntil(':', '(', ';', '\n')
	if err != nil {
		return Sync{}, fmt.Errorf("csdf.Parser.parseSync: %w", err)
	}
	if event == "" {
		return Sync{}, fmt.Errorf("csdf.Parser.parseSync: expected an event name after 'sync' at line %d, col %d", p.line, p.col)
	}
	if !p.expectChar(':') {
		return Sync{}, fmt.Errorf("csdf.Parser.parseSync: expected ':' after the event name at line %d, col %d", p.line, p.col)
	}
	if err := p.skipInlineTrivia(); err != nil {
		return Sync{}, fmt.Errorf("csdf.Parser.parseSync: %w", err)
	}

	var targets []MapRef
	for {
		target, err := p.parseMapRef()
		if err != nil {
			return Sync{}, fmt.Errorf("csdf.Parser.parseSync: %w", err)
		}
		targets = append(targets, target)
		if err := p.skipInlineTrivia(); err != nil {
			return Sync{}, fmt.Errorf("csdf.Parser.parseSync: %w", err)
		}
		if !p.expectChar(',') {
			break
		}
		if err := p.skipInlineTrivia(); err != nil {
			return Sync{}, fmt.Errorf("csdf.Parser.parseSync: %w", err)
		}
	}

	if !p.expectNewlines() {
		return Sync{}, fmt.Errorf("csdf.Parser.parseSync: expected newline after sync directive at line %d, col %d", p.line, p.col)
	}

	return Sync{Event: event, Targets: targets, Line: line}, nil
}

// parseConstrain parses
//
//	constrain <event>(<param>, ...) ; <guard>
//
// The event is written in its promoted form, so its first parameter is the
// instance id. The guard is opaque, as every predicate is.
func (p *Parser) parseConstrain() (Constrain, error) {
	line := p.line
	if !p.expectString("constrain") {
		return Constrain{}, fmt.Errorf("csdf.Parser.parseConstrain: expected 'constrain' at line %d, col %d", p.line, p.col)
	}
	if err := p.skipInlineTrivia(); err != nil {
		return Constrain{}, fmt.Errorf("csdf.Parser.parseConstrain: %w", err)
	}

	event, err := p.parseUntil('(', ';', '\n')
	if err != nil {
		return Constrain{}, fmt.Errorf("csdf.Parser.parseConstrain: %w", err)
	}
	if event == "" {
		return Constrain{}, fmt.Errorf("csdf.Parser.parseConstrain: expected an event name after 'constrain' at line %d, col %d", p.line, p.col)
	}
	if !p.expectChar('(') {
		return Constrain{}, fmt.Errorf("csdf.Parser.parseConstrain: expected '(' after the event name at line %d, col %d", p.line, p.col)
	}

	var params []string
	for {
		if err := p.skipInlineTrivia(); err != nil {
			return Constrain{}, fmt.Errorf("csdf.Parser.parseConstrain: %w", err)
		}
		param, err := p.parseUntil(',', ')', ';', '\n')
		if err != nil {
			return Constrain{}, fmt.Errorf("csdf.Parser.parseConstrain: %w", err)
		}
		if param == "" {
			return Constrain{}, fmt.Errorf("csdf.Parser.parseConstrain: expected a parameter name in '%s(...)' at line %d, col %d", event, p.line, p.col)
		}
		params = append(params, param)
		if !p.expectChar(',') {
			break
		}
	}
	if !p.expectChar(')') {
		return Constrain{}, fmt.Errorf("csdf.Parser.parseConstrain: expected ')' after the parameter list at line %d, col %d", p.line, p.col)
	}
	if err := p.skipInlineTrivia(); err != nil {
		return Constrain{}, fmt.Errorf("csdf.Parser.parseConstrain: %w", err)
	}

	if !p.expectChar(';') {
		return Constrain{}, fmt.Errorf("csdf.Parser.parseConstrain: expected ';' before the guard at line %d, col %d", p.line, p.col)
	}
	if err := p.skipInlineTrivia(); err != nil {
		return Constrain{}, fmt.Errorf("csdf.Parser.parseConstrain: %w", err)
	}
	guard, err := p.parseUntilNewline()
	if err != nil {
		return Constrain{}, fmt.Errorf("csdf.Parser.parseConstrain: %w", err)
	}
	if guard == "" {
		return Constrain{}, fmt.Errorf("csdf.Parser.parseConstrain: expected a guard after ';' at line %d, col %d", p.line, p.col)
	}

	if !p.expectNewlines() {
		return Constrain{}, fmt.Errorf("csdf.Parser.parseConstrain: expected newline after constrain directive at line %d, col %d", p.line, p.col)
	}

	return Constrain{Event: event, Params: params, Guard: Predicate(guard), Line: line}, nil
}

func (p *Parser) parseInClause() ([]StateID, error) {
	if !p.expectString("in") {
		return nil, fmt.Errorf("csdf.Parser.parseInClause: expected 'in' at line %d, col %d", p.line, p.col)
	}
	if err := p.skipInlineTrivia(); err != nil {
		return nil, fmt.Errorf("csdf.Parser.parseInClause: %w", err)
	}

	var in []StateID
	for {
		id, err := p.parseID()
		if err != nil {
			return nil, fmt.Errorf("csdf.Parser.parseInClause: expected a state ID after 'in' at line %d, col %d", p.line, p.col)
		}
		in = append(in, StateID(id))
		if err := p.skipInlineTrivia(); err != nil {
			return nil, fmt.Errorf("csdf.Parser.parseInClause: %w", err)
		}
		if !p.expectChar(',') {
			return in, nil
		}
		if err := p.skipInlineTrivia(); err != nil {
			return nil, fmt.Errorf("csdf.Parser.parseInClause: %w", err)
		}
	}
}

// parseMapRef parses "<map>(<param>)". The parameter is free text, since
// instance ids are named in the same natural language as the predicates.
func (p *Parser) parseMapRef() (MapRef, error) {
	name, err := p.parseID()
	if err != nil {
		return MapRef{}, fmt.Errorf("csdf.Parser.parseMapRef: expected a state variable name at line %d, col %d", p.line, p.col)
	}
	if err := p.skipInlineTrivia(); err != nil {
		return MapRef{}, fmt.Errorf("csdf.Parser.parseMapRef: %w", err)
	}
	if !p.expectChar('(') {
		return MapRef{}, fmt.Errorf("csdf.Parser.parseMapRef: expected '(' after the state variable name at line %d, col %d", p.line, p.col)
	}
	if err := p.skipInlineTrivia(); err != nil {
		return MapRef{}, fmt.Errorf("csdf.Parser.parseMapRef: %w", err)
	}
	param, err := p.parseUntil(',', ')', ';', '\n')
	if err != nil {
		return MapRef{}, fmt.Errorf("csdf.Parser.parseMapRef: %w", err)
	}
	if param == "" {
		return MapRef{}, fmt.Errorf("csdf.Parser.parseMapRef: expected a parameter name in '%s(...)' at line %d, col %d", name, p.line, p.col)
	}
	if !p.expectChar(')') {
		return MapRef{}, fmt.Errorf("csdf.Parser.parseMapRef: expected ')' after the parameter name at line %d, col %d", p.line, p.col)
	}
	return MapRef{Map: Var(name), Param: param}, nil
}

// parsePath parses the path of a local diagram: bare up to the next space, or
// double-quoted when it contains one.
func (p *Parser) parsePath() (string, error) {
	if p.peek() == '"' {
		return p.parseStateName()
	}

	var result strings.Builder
	for !p.isAtEnd() && p.peek() != ' ' && p.peek() != '\t' && p.peek() != '\r' && p.peek() != '\n' {
		result.WriteByte(p.peek())
		p.advance()
	}
	if result.Len() == 0 {
		return "", fmt.Errorf("csdf.Parser.parsePath: expected a local diagram path at line %d, col %d", p.line, p.col)
	}
	return result.String(), nil
}

func (p *Parser) parseEvent() (Event, error) {
	event, err := p.parseUntilSemicolon()
	if err != nil {
		return "", fmt.Errorf("csdf.Parser.parseEvent: %w", err)
	}
	if event == "" {
		return "", fmt.Errorf("csdf.Parser.parseEvent: expected event after ':' in edge at line %d, col %d", p.line, p.col)
	}
	return Event(event), nil
}

func (p *Parser) parseID() (string, error) {
	var result strings.Builder

	if p.isAtEnd() || !p.isIDChar(p.peek()) {
		return "", fmt.Errorf("csdf.Parser.parseID: expected identifier at line %d, col %d", p.line, p.col)
	}

	for !p.isAtEnd() && p.isIDChar(p.peek()) {
		result.WriteByte(p.peek())
		p.advance()
	}

	return result.String(), nil
}

func (p *Parser) parseUntilSemicolon() (string, error) {
	return p.parseUntil(';', '\n')
}

func (p *Parser) parseUntilNewline() (string, error) {
	return p.parseUntil('\n')
}

func (p *Parser) parseUntil(stops ...byte) (string, error) {
	var result strings.Builder
	inString := false
	escaped := false
	var lastWritten byte

	for !p.isAtEnd() && !containsByte(stops, p.peek()) {
		if !inString && p.peekString("/'") {
			needsSeparator := result.Len() > 0 && lastWritten != ' ' && lastWritten != '\t' && lastWritten != '\r' && lastWritten != '\n'
			if err := p.skipBlockComment(); err != nil {
				return "", fmt.Errorf("csdf.Parser.parseUntil: %w", err)
			}
			if needsSeparator && !p.isAtEnd() && !containsByte(stops, p.peek()) &&
				p.peek() != ' ' && p.peek() != '\t' && p.peek() != '\r' && p.peek() != '\n' {
				result.WriteByte(' ')
				lastWritten = ' '
			}
			continue
		}

		c := p.peek()
		result.WriteByte(p.peek())
		p.advance()
		lastWritten = c

		if escaped {
			escaped = false
			continue
		}
		if inString && c == '\\' {
			escaped = true
			continue
		}
		if c == '"' {
			inString = !inString
		}
	}
	return strings.TrimSpace(result.String()), nil
}

func (p *Parser) isIDChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-'
}

func (p *Parser) isEdge() (bool, error) {
	probe := *p
	_, err := probe.parseID()
	if err != nil {
		return false, nil
	}
	if err := probe.skipInlineTrivia(); err != nil {
		return false, fmt.Errorf("csdf.Parser.isEdge: %w", err)
	}
	return probe.peekString("-->"), nil
}

func (p *Parser) isEndEdge() (bool, error) {
	probe := *p
	_, err := probe.parseID()
	if err != nil {
		return false, nil
	}
	if err := probe.skipInlineTrivia(); err != nil {
		return false, fmt.Errorf("csdf.Parser.isEndEdge: %w", err)
	}
	if !probe.expectString("-->") {
		return false, nil
	}
	if err := probe.skipInlineTrivia(); err != nil {
		return false, fmt.Errorf("csdf.Parser.isEndEdge: %w", err)
	}
	return probe.peekString("[*]"), nil
}

func (p *Parser) isStateVar(stateID StateID) (bool, error) {
	probe := *p
	id, err := probe.parseID()
	if err != nil || StateID(id) != stateID {
		return false, nil
	}
	if err := probe.skipInlineTrivia(); err != nil {
		return false, fmt.Errorf("csdf.Parser.isStateVar: %w", err)
	}
	return probe.peek() == ':', nil
}

func (p *Parser) peek() byte {
	if p.isAtEnd() {
		return 0
	}
	return p.input[p.pos]
}

func (p *Parser) advance() byte {
	if p.isAtEnd() {
		return 0
	}
	c := p.input[p.pos]
	p.pos++
	if c == '\n' {
		p.line++
		p.col = 1
	} else {
		p.col++
	}
	return c
}

func (p *Parser) isAtEnd() bool {
	return p.pos >= len(p.input)
}

func (p *Parser) expectChar(expected byte) bool {
	if p.isAtEnd() || p.peek() != expected {
		return false
	}
	p.advance()
	return true
}

func (p *Parser) expectString(expected string) bool {
	if p.pos+len(expected) > len(p.input) {
		return false
	}
	if p.input[p.pos:p.pos+len(expected)] != expected {
		return false
	}
	for i := 0; i < len(expected); i++ {
		p.advance()
	}
	return true
}

// peekKeyword reports whether the input at the current position is the given
// directive keyword followed by a space. The trailing space is what keeps a
// state named "promoted" or "sync" from being read as a directive.
func (p *Parser) peekKeyword(keyword string) bool {
	if !p.peekString(keyword) {
		return false
	}
	if p.pos+len(keyword) >= len(p.input) {
		return false
	}
	next := p.input[p.pos+len(keyword)]
	return next == ' ' || next == '\t'
}

func (p *Parser) peekString(expected string) bool {
	if p.pos+len(expected) > len(p.input) {
		return false
	}
	return p.input[p.pos:p.pos+len(expected)] == expected
}

func (p *Parser) skipTrivia() error {
	for {
		for !p.isAtEnd() && (p.peek() == ' ' || p.peek() == '\t' || p.peek() == '\n' || p.peek() == '\r') {
			p.advance()
		}
		if p.peekString("/'") {
			if err := p.skipBlockComment(); err != nil {
				return fmt.Errorf("csdf.Parser.skipTrivia: %w", err)
			}
			continue
		}
		if p.peek() == '\'' {
			if p.lineCommentBody() == ignoreBeginMarker {
				if err := p.skipIgnoreRegion(); err != nil {
					return fmt.Errorf("csdf.Parser.skipTrivia: %w", err)
				}
				continue
			}
			p.skipLine()
			continue
		}
		return nil
	}
}

// lineCommentBody returns the trimmed text of the line comment at the current
// position (the leading "'" excluded). The Parser is not advanced.
func (p *Parser) lineCommentBody() string {
	end := p.pos + 1
	for end < len(p.input) && p.input[end] != '\n' {
		end++
	}
	return strings.TrimSpace(p.input[p.pos+1 : end])
}

// skipIgnoreRegion consumes lines from the "' CSDF-IGNORE-BEGIN" marker (current
// position) through the matching "' CSDF-IGNORE-END" marker, inclusive.
func (p *Parser) skipIgnoreRegion() error {
	startLine := p.line
	startCol := p.col
	p.skipLine() // consume the begin-marker line
	for !p.isAtEnd() {
		for !p.isAtEnd() && (p.peek() == ' ' || p.peek() == '\t') {
			p.advance()
		}
		if p.peek() == '\'' && p.lineCommentBody() == ignoreEndMarker {
			p.skipLine() // consume the end-marker line
			return nil
		}
		p.skipLine()
	}
	return fmt.Errorf("csdf.Parser.skipIgnoreRegion: unterminated CSDF-IGNORE region at line %d, col %d", startLine, startCol)
}

func (p *Parser) skipInlineTrivia() error {
	for {
		for !p.isAtEnd() && (p.peek() == ' ' || p.peek() == '\t') {
			p.advance()
		}
		if !p.peekString("/'") {
			return nil
		}
		if err := p.skipBlockComment(); err != nil {
			return fmt.Errorf("csdf.Parser.skipInlineTrivia: %w", err)
		}
	}
}

func (p *Parser) skipBlockComment() error {
	startLine := p.line
	startCol := p.col
	p.expectString("/'")
	for !p.isAtEnd() && !p.peekString("'/") {
		p.advance()
	}
	if !p.expectString("'/") {
		return fmt.Errorf("csdf.Parser.skipBlockComment: unterminated block comment at line %d, col %d", startLine, startCol)
	}
	return nil
}

func (p *Parser) expectNewlines() bool {
	count := 0
	for !p.isAtEnd() && (p.peek() == '\n' || p.peek() == '\r') {
		if p.peek() == '\n' {
			count++
		}
		p.advance()
	}
	return count > 0
}

func (p *Parser) skipLine() {
	for !p.isAtEnd() && p.peek() != '\n' {
		p.advance()
	}
	if !p.isAtEnd() {
		p.advance()
	}
}

func (p *Parser) parseEndEdge() (EndEdge, error) {
	line := p.line
	src, err := p.parseID()
	if err != nil {
		return EndEdge{}, fmt.Errorf("csdf.Parser.parseEndEdge: %w", err)
	}
	if err := p.skipInlineTrivia(); err != nil {
		return EndEdge{}, fmt.Errorf("csdf.Parser.parseEndEdge: %w", err)
	}

	if !p.expectString("-->") {
		return EndEdge{}, fmt.Errorf("csdf.Parser.parseEndEdge: expected '-->' at line %d, col %d", p.line, p.col)
	}
	if err := p.skipInlineTrivia(); err != nil {
		return EndEdge{}, fmt.Errorf("csdf.Parser.parseEndEdge: %w", err)
	}

	if !p.expectString("[*]") {
		return EndEdge{}, fmt.Errorf("csdf.Parser.parseEndEdge: expected '[*]' at line %d, col %d", p.line, p.col)
	}
	if err := p.skipInlineTrivia(); err != nil {
		return EndEdge{}, fmt.Errorf("csdf.Parser.parseEndEdge: %w", err)
	}

	var guard string
	if p.expectChar(':') {
		if err := p.skipInlineTrivia(); err != nil {
			return EndEdge{}, fmt.Errorf("csdf.Parser.parseEndEdge: %w", err)
		}
		guard, err = p.parseUntilSemicolon()
		if err != nil {
			return EndEdge{}, fmt.Errorf("csdf.Parser.parseEndEdge: %w", err)
		}
		if p.peek() == ';' {
			return EndEdge{}, fmt.Errorf("csdf.Parser.parseEndEdge: unexpected ';' in end edge guard at line %d, col %d", p.line, p.col)
		}
	}

	if !p.expectNewlines() {
		return EndEdge{}, fmt.Errorf("csdf.Parser.parseEndEdge: expected newline after end edge declaration at line %d, col %d", p.line, p.col)
	}

	return EndEdge{
		Src:   StateID(src),
		Guard: Predicate(guard),
		Line:  line,
	}, nil
}

func containsByte(values []byte, target byte) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
