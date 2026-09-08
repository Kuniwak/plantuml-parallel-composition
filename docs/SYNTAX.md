Composable State Diagram Format
===============================
A string representation of composable state transition models. It is a subset of PlantUML's state diagram grammar rules.


Grammar Rules
-------------
```abnf
diagram = "@startuml" inlineTrivia 0*1(diagramName) LF trivia 1*(stateDecl trivia) startEdgeDecl trivia *((edgeDecl / directiveDecl) trivia) 0*1(endEdgeDecl trivia) "@enduml" LF
diagramName = 1*(HTAB / unicode_char)
stateDecl = "state" inlineSeparator stateName inlineSeparator "as" inlineSeparator stateID inlineTrivia LF trivia *(stateVarDecl trivia)
stateVarDecl = stateID inlineTrivia ":" inlineTrivia var inlineTrivia 0*1(";" inlineTrivia varType) LF
startEdgeDecl = "[*]" inlineSeparator "-->" inlineSeparator stateID 0*1(inlineTrivia ":" inlineSeparator post) inlineTrivia LF
edgeDecl = stateID inlineSeparator "-->" inlineSeparator stateID inlineTrivia ":" inlineTrivia event 0*1(inlineTrivia ";" inlineTrivia guard 0*1(inlineTrivia ";" inlineTrivia post)) inlineTrivia LF
endEdgeDecl = stateID inlineSeparator "-->" inlineSeparator "[*]" 0*1(inlineTrivia ":" inlineSeparator guard) inlineTrivia LF
directiveDecl = promoteDecl / syncDecl / constrainDecl
promoteDecl = "promote" inlineSeparator path inlineSeparator "as" inlineSeparator typeName inlineSeparator "via" inlineSeparator mapRef 0*1(inlineSeparator inClause) inlineTrivia LF
inClause = "in" inlineSeparator stateID *(inlineTrivia "," inlineTrivia stateID)
syncDecl = "sync" inlineSeparator syncEventName inlineTrivia ":" inlineTrivia mapRef *(inlineTrivia "," inlineTrivia mapRef) inlineTrivia LF
constrainDecl = "constrain" inlineSeparator constrainEventName inlineTrivia "(" inlineTrivia param *("," inlineTrivia param) ")" inlineTrivia ";" inlineTrivia guard inlineTrivia LF
mapRef = var inlineTrivia "(" inlineTrivia param ")"
path = stateName / 1*unicode_char_except_space
typeName = id
syncEventName = 1*unicode_char_except_colon_paren_semicolon
constrainEventName = 1*unicode_char_except_paren_semicolon
param = 1*unicode_char_except_comma_paren_semicolon
stateName = DQUOTE 1*(unicode_char_except_dquote_and_backslash / escape_backslash / escape_dquote) DQUOTE
escape_backslash = "\\"
escape_dquote = "\" DQUOTE
stateID = id
var = id
varType = *textElement
event = 1*textElement
guard = *textElement
post = *textElement
textElement = unicode_char_except_semicolon / block_comment
id = 1*(ALPHA / DIGIT / "_" / "-")
trivia = *(LF / HTAB / SP / block_comment / line_comment / ignore_region)
inlineTrivia = *(HTAB / SP / block_comment)
inlineSeparator = 1*(HTAB / SP / block_comment)
line_comment = "'" *unicode_char LF
ignore_region = ignore_begin *ignore_line ignore_end
ignore_begin = *(HTAB / SP) "'" *(HTAB / SP) "CSDF-IGNORE-BEGIN" *(HTAB / SP) LF
ignore_end = *(HTAB / SP) "'" *(HTAB / SP) "CSDF-IGNORE-END" *(HTAB / SP) LF
ignore_line = *unicode_char LF
block_comment = "/'" *(LF / unicode_char_except_squote / (%x27 unicode_char_except_slash)) "'/"
unicode_char = %x20-7F / %x80-10FFFF
unicode_char_except_dquote_and_backslash = %x20-21 / %x23-5B / %x5D-7F / %x80-10FFFF
unicode_char_except_squote = %x20-26 / %x28-7F / %x80-10FFFF
unicode_char_except_slash = %x20-2E / %x30-7F / %x80-10FFFF
unicode_char_except_semicolon = %x20-3A / %x3C-7F / %x80-10FFFF
unicode_char_except_space = %x21-7F / %x80-10FFFF
unicode_char_except_paren_semicolon = %x20-27 / %x2A-3A / %x3C-7F / %x80-10FFFF
unicode_char_except_colon_paren_semicolon = %x20-27 / %x2A-39 / %x3C-7F / %x80-10FFFF
unicode_char_except_comma_paren_semicolon = %x20-27 / %x2A-2B / %x2D-3A / %x3C-7F / %x80-10FFFF
```

Line comments are accepted between declarations and state-variable lines.
Block comments are accepted wherever horizontal whitespace is accepted, including
inside `varType`, `event`, `guard`, and `post`. Comments are discarded while parsing.
A line comment whose trimmed text is exactly `CSDF-IGNORE-BEGIN` opens an ignore region
(taking precedence over a plain `line_comment`) that extends through the next line comment
whose trimmed text is exactly `CSDF-IGNORE-END`. `ignore_line` matches any single line that
does not match `ignore_end`, so the region closes at the *first* `CSDF-IGNORE-END` marker
rather than the last (the rule is not greedy). Every line in between, along with both marker
lines, is discarded. Because the markers are themselves PlantUML line comments, the
wrapped content (for example Graphviz directives such as `left to right direction`) is still
rendered by PlantUML while CSDF ignores it. An unterminated ignore region is a parse error.
Comment delimiters inside double-quoted strings are treated as ordinary text.
An event must remain non-empty after comments and surrounding whitespace are removed.

A line is read as a `directiveDecl` only when it is not an `edgeDecl`, so `promote`, `sync`
and `constrain` remain usable as state IDs. Leading and trailing whitespace is removed from
a `path`, a `param` and a `guard`. Promotion directives are the input of `csdfpromote` and
of nothing else: every other tool refuses a diagram that still carries them, because such a
diagram's edges are not the whole of its behaviour. `csdfparse` is the one exception, since
reporting the diagram is all it does. See PROMOTION.md for what the directives mean.

The following symbols are ABNF core rules:

* `ALPHA`: ASCII uppercase and lowercase letters
* `DIGIT`: Decimal digits
* `DQUOTE`: Double quote
* `SP`: Space
* `LF`: Line feed


Types
-----

```go
package example

type ID string
type StateID ID
type Event string
type Var ID

type StateVar struct {
	Name Var
	Type string
}

type Diagram struct {
	Name       string
	States     map[StateID]State
	StartEdge  StartEdge
	Edges      []Edge
	EndEdge    *EndEdge
	Promotes   []Promote
	Syncs      []Sync
	Constrains []Constrain
}

type Promote struct {
	Path    string
	Type    string
	Map     Var
	IDParam string
	In      []StateID
}

type Sync struct {
	Event   string
	Targets []MapRef
}

type MapRef struct {
	Map   Var
	Param string
}

type Constrain struct {
	Event  string
	Params []string
	Guard  string
}

type State struct {
	ID   StateID
	Name string
	Vars []StateVar
}

type StartEdge struct {
	Dst  StateID
	Post string
}

type Edge struct {
	Src   StateID
	Dst   StateID
	Event Event
	Guard string
	Post  string
}

type EndEdge struct {
	Src   StateID
	Guard string
}
```


Semantics
---------
| Syntax Element                             | Corresponding Type | Meaning                                                                                                                                                                  |
|:-------------------------------------------|:-------------------|:-------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `diagram`                                  | `Diagram`          | Represents a declaration of a state transition model.                                                                                                                    |
| `diagramName`                              | `string`           | Optional PlantUML diagram name written on the `@startuml` line. Any text up to the end of the line is accepted, quoted or not, and comment delimiters within it are plain text. Leading and trailing whitespace is removed. It carries no meaning, but it is retained so that printing a parsed diagram reproduces it. |
| `stateDecl`                                | `State`            | Represents a state declaration.                                                                                                                                          |
| `stateVarDecl`                             | `StateVar`         | Represents a state variable name and its optional type.                                                                                                                   |
| `startEdgeDecl`                            | `StartEdge`        | Represents a declaration of transition to the initial state.                                                                                                             |
| `edgeDecl`                                 | `Edge`             | Represents a declaration of a directed edge.                                                                                                                             |
| `endEdgeDecl`                              | `EndEdge`          | Represents a declaration of transition to the end state.                                                                                                                 |
| `promoteDecl`                              | `Promote`          | Promotes the local diagram at `path` through the state variable named by `mapRef`, which holds a partial map from instance IDs to local states. `typeName` names the local state type, and the `param` of `mapRef` names the instance ID. See PROMOTION.md. |
| `inClause`                                 | `[]StateID`        | The global states the local diagram is expanded into. Omitting it means the destination of `startEdgeDecl` alone.                                                        |
| `syncDecl`                                 | `Sync`             | Merges the edges the named local event contributes to each of the referenced maps into a single global edge, so that the instances take the event together.              |
| `constrainDecl`                            | `Constrain`        | Conjoins `guard` onto the guard of every expanded edge whose event matches `constrainEventName` in name and in number of parameters. The event is written in its promoted form, so its first parameter is the instance ID. |
| `mapRef`                                   | `MapRef`           | A promoted map together with the parameter that stands for its instance ID.                                                                                              |
| `path`                                     | `string`           | Path of a local diagram, resolved against the directory of the file the directive is in. Double-quote it when it contains a space.                                       |
| `param`                                    | `string`           | Free-form parameter name, in the same natural language as the predicates.                                                                                                |
| `stateName`                                | `string`           | State name. Represents a string with leading and trailing double quotes removed and escapes resolved.                                                                    |
| `escape_backslash`                         | `rune`             | Represents `\`.                                                                                                                                                          |
| `escape_dquote`                            | `rune`             | Represents `"`.                                                                                                                                                          |
| `stateID`                                  | `StateID`          | Represents an ID string.                                                                                                                                                 |
| `var`                                      | `Var`              | Represents a variable name.                                                                                                                                              |
| `varType`                                  | `string`           | Represents an optional state-variable type. Leading and trailing whitespace is removed.                                                                                   |
| `event`                                    | `Event`            | Represents an event as a free-form string. Leading and trailing whitespace is removed. The entire string is used for synchronization. When it is exactly `tau`, it is an internal transition. |
| `guard`                                    | `string`           | Represents a natural language expression of guard conditions.                                                                                                            |
| `post`                                     | `string`           | Represents a natural language expression of post-conditions.                                                                                                             |
| `id`                                       | `string`           | Represents an ID string.                                                                                                                                                 |
| `trivia`                                   | N/A                | Whitespace and comments accepted between declarations.                                                                                                                   |
| `inlineTrivia`                             | N/A                | Horizontal whitespace and block comments accepted inside declarations.                                                                                                  |
| `line_comment`                             | N/A                | PlantUML line comment beginning with `'`. It is not retained in the AST.                                                                                                 |
| `ignore_region`                            | N/A                | Non-interpreted region delimited by `CSDF-IGNORE-BEGIN`/`CSDF-IGNORE-END` line comments, holding PlantUML-only directives. It is discarded while parsing.                  |
| `block_comment`                            | N/A                | PlantUML block comment delimited by `/'` and `'/`. It is not retained in the AST.                                                                                         |
| `unicode_char_except_dquote_and_backslash` | `rune`             | Represents Unicode characters except double quotes and backslashes.                                                                                                      |
| `unicode_char_except_semicolon`            | `rune`             | Represents Unicode characters except semicolons.                                                                                                                         |
