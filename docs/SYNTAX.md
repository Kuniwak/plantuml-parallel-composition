Composable State Diagram Format
===============================
A string representation of composable state transition models. It is a subset of PlantUML's state diagram grammar rules.


Grammar Rules
-------------
```abnf
diagram = "@startuml" inlineTrivia 0*1(diagramName) LF trivia 1*(stateDecl trivia) startEdgeDecl trivia *(edgeDecl trivia) 0*1(endEdgeDecl trivia) "@enduml" LF
diagramName = 1*(HTAB / unicode_char)
stateDecl = "state" inlineSeparator stateName inlineSeparator "as" inlineSeparator stateID inlineTrivia LF trivia *(stateVarDecl trivia)
stateVarDecl = stateID inlineTrivia ":" inlineTrivia var inlineTrivia 0*1(";" inlineTrivia varType) LF
startEdgeDecl = "[*]" inlineSeparator "-->" inlineSeparator stateID 0*1(inlineTrivia ":" inlineSeparator post) inlineTrivia LF
edgeDecl = stateID inlineSeparator "-->" inlineSeparator stateID inlineTrivia ":" inlineTrivia event 0*1(inlineTrivia ";" inlineTrivia guard 0*1(inlineTrivia ";" inlineTrivia post)) inlineTrivia LF
endEdgeDecl = stateID inlineSeparator "-->" inlineSeparator "[*]" 0*1(inlineTrivia ":" inlineSeparator guard) inlineTrivia LF
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
	Name      string
	States    map[StateID]State
	StartEdge StartEdge
	Edges     []Edge
	EndEdge   *EndEdge
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


Global Diagram Format
---------------------
The grammar above is what every tool reads. `csdfpromote` reads an upper-compatible
one on top of it, in which a diagram may also declare a promotion. The directives
are spelled in PlantUML's own syntax, so that a hand-written global diagram still
renders; `csdfpromote` lifts them out and prints a diagram of the grammar above.
Refer to PROMOTION.md for what they mean.

```abnf
globalDiagram   = "@startuml" inlineTrivia 0*1(diagramName) LF trivia
                  *((stateDecl / compositeState / preprocessorLine / noteBlock) trivia)
                  startEdgeDecl trivia
                  *((edgeDecl / noteBlock / preprocessorLine) trivia)
                  0*1(endEdgeDecl trivia)
                  "@enduml" LF

compositeState  = "state" inlineSeparator stateName inlineSeparator "as" inlineSeparator stateID
                  inlineTrivia "{" LF trivia
                  *((stateVarDecl / promoteBlock / preprocessorLine) trivia)
                  "}" LF

promoteBlock    = "state" inlineSeparator promoteTitle inlineSeparator "as" inlineSeparator stateID
                  inlineSeparator "<<promote>>"
                  0*1(inlineTrivia "{" LF trivia includeLine trivia "}") inlineTrivia LF
promoteTitle    = DQUOTE var inlineTrivia ":" inlineTrivia param inlineTrivia ("⇸" / "->>") inlineTrivia typeName DQUOTE
includeLine     = "!include" inlineSeparator path inlineTrivia LF

noteBlock       = "note" inlineSeparator ("as" inlineSeparator noteID / noteAnchor) inlineTrivia LF
                  *(noteLine LF)
                  inlineTrivia "end" inlineSeparator "note" inlineTrivia LF
noteAnchor      = ("left" / "right" / "top" / "bottom") inlineSeparator "of" inlineSeparator stateID
noteLine        = *unicode_char_except_LF
noteID          = id

syncBody        = "sync" inlineSeparator eventName inlineTrivia ":" inlineTrivia mapRef *(inlineTrivia "," inlineTrivia mapRef)
constrainBody   = "constrain" inlineSeparator eventName inlineTrivia "(" inlineTrivia param *(inlineTrivia "," inlineTrivia param) inlineTrivia ")" inlineTrivia ";" inlineTrivia guard
mapRef          = var inlineTrivia "(" inlineTrivia param inlineTrivia ")"

preprocessorLine = "!" *unicode_char_except_LF LF
path            = 1*(unicode_char_except_space) / DQUOTE 1*(unicode_char_except_dquote) DQUOTE
typeName        = 1*(unicode_char_except_space)
param           = 1*(unicode_char_except_comma_paren)
eventName       = 1*(unicode_char_except_paren_semicolon_colon)
```

| Syntax Element     | Corresponding Type | Meaning                                                                                                                                                                                 |
|:-------------------|:-------------------|:------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `globalDiagram`    | `GlobalDiagram`    | A diagram together with the directives lifted out of it. What is left is a `Diagram` of the grammar above, with the composite states flattened into ordinary ones.                        |
| `compositeState`   | `State`            | An ordinary state whose braces hold its state variables and the `<<promote>>` blocks of the families that move in it. Nesting anything else is an error.                                   |
| `promoteBlock`     | `Promote`          | One family of instances. The title names the map, the parameter the instance ID is written as, and the type of one instance; the alias is a PlantUML identifier and carries no meaning. A block with no body only says that the family moves in that state too. |
| `includeLine`      | `string`           | The local diagram of the family. Exactly one block per map carries it.                                                                                                                    |
| `noteBlock`        | `Sync`/`Constrain` | A directive when its first non-empty line starts with `sync` or `constrain`; a note for the reader otherwise, and then discarded.                                                          |
| `noteAnchor`       | `Anchor`           | The block the note points at, which is drawn as a line. A floating `note as <id>` points at nothing: PlantUML's state diagrams have no `<id> .. <state>` link.                              |
| `preprocessorLine` | N/A                | A PlantUML preprocessor line such as `!define` or `!ifndef`. It is discarded, and it is how a local diagram hides its PlantUML-only lines when it is included.                              |
