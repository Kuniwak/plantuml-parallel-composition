Promotion
=========

A system built from many instances of one thing — an account per customer, a
trade per order, a report per business day — has no readable single Composable
State Diagram. Written as one, the control state of every instance is absorbed
into a state variable holding a partial map from instance IDs to local states,
and the diagram collapses to one global state with a bundle of self-loops. That
form is what `csdflivelockfree`, `csdfrefinement` and `csdfrepl` want. It is not
what a reader wants.

The readable form is the diagram of *one* instance. The two cannot be the same
artefact, so the author writes the local diagrams plus a few directives, and
`csdfpromote` generates the global one.

The generation has to run that way round. A local diagram is already a
structured Composable State Diagram, so expanding it into the global form needs
no structure that is not already there. Projecting a global diagram back onto
one instance would instead need conventions about map names and state names
*inside* the predicates — and a predicate in this format is opaque natural
language, which is exactly what such a convention would have to break.

Coupling has to be writable too. A guard that reaches across maps, and an event
that two kinds of instance take together, are the substance of a specification;
without somewhere to write them, promotion produces the direct product of
independent maps, which says almost nothing.


Vocabulary
----------

The vocabulary is Z's.

| Directive   | Z                                                                | CSP                                                                               |
|:------------|:-----------------------------------------------------------------|:-----------------------------------------------------------------------------------|
| `promote`   | free promotion: `θLocal = m(id)`, `m' = m ⊕ {id ↦ θLocal'}`      | the symbolic form of `\|\|\| id : dom m @ Local(m(id))[[e ← e(id)]]`               |
| `sync`      | the conjunction of two framing schemas                            | alphabetised parallel, `[\| {e} \|]`                                               |
| `constrain` | constrained promotion                                             | nothing: a constraint process reading every instance is the global diagram itself  |


How the directives are written
------------------------------

They are written in PlantUML's own syntax, so that the diagram an author writes
is also the picture a reader looks at. A hand-written global diagram renders.

```plantuml
@startuml TRADES
!define PROMOTED

state "稼働中" as running {
  running : buys ; 約定ID ⇸ Buy
  running : cycles ; 基準日 ⇸ Cycle

  state "buys : 約定ID ⇸ Buy" as runningBuys <<promote>> {
    !include local/BUY.puml
  }
  state "cycles : 基準日 ⇸ Cycle" as runningCycles <<promote>> {
    !include local/CYCLE.puml
  }
}

[*] --> running : buys と cycles は空

note as sync1
  sync BOOK : buys(約定ID), cycles(基準日)
end note

note bottom of runningBuys
  constrain BUY(約定ID, 数量) ; 数量 は最小取引単位の倍数である
end note
@enduml
```

**`promote`** is a composite state with the `<<promote>>` stereotype, written
inside the state its family moves in. Its title is `<map> : <ID> ⇸ <Type>`: the
map the instances live in, the parameter the instance ID is written as, and the
type of one instance. Its body is one `!include` naming the local diagram. `⇸`
is U+21F8; `->>` is accepted in its place.

The map is named by the **title**, not by the PlantUML alias, because an alias
has to be unique across the diagram and one map may have a block in several
states.

**`sync`** and **`constrain`** are notes whose first non-empty line starts with
the directive's name. A note that starts with neither is there for the picture
alone and is ignored. A directive that points at one block is written as
`note <left|right|top|bottom> of <alias>`, which draws the line; one that spans
several maps is written as a floating `note as <id>`. PlantUML's state diagrams
have no `<id> .. <state>` link, so there is no way to draw a line from a
floating note.

Everything else in the file is ordinary Composable State Diagram syntax:
hand-written states, the start edge, and hand-written edges such as the ones
that switch operating modes.


Where a family moves
--------------------

The state a `<<promote>>` block is written in is where that family moves. There
is no shorthand: even a one-state diagram writes the state as a composite state
around the block.

To drive one map in several states, write a block in each. Only one of them
carries the `!include`; the others are bodyless:

```plantuml
state "縮退中" as degraded {
  degraded : accounts ; 口座ID ⇸ Account

  state "accounts : 口座ID ⇸ Account" as degradedAccounts <<promote>>
}
```

A second `!include` of the same file would collide with the first on every state
ID, and PlantUML would merge the two boxes into one.

A state that holds the map but has no block for it has that family **frozen**
there, which is reported as `info`. This is deliberate: expanding into every
state that holds the map would quietly write a specification in which trades go
on being booked during maintenance. The edges that switch modes, and the frames
that say every map is unchanged across them, are hand-written; promotion says
nothing about them.


The local-diagram contract
--------------------------

The start state `S₀` means *this instance does not exist*. So:

- `S₀` holds no state variables (error).
- An edge out of `S₀` creates an instance; an edge into it deletes one.
- `S₀` has no self-loop (error): it would be an event of an instance that is not
  there.
- The diagram has no end edge (error). Write completion as a state with no
  transition out of it.
- The post of the local start edge is ignored, and the post of a deletion edge is
  discarded, both with a warning. Neither can say anything: there is no instance
  for them to be about.
- The post of a creation edge alone decides the initial values of the state it
  leads to. Two creation edges with different destinations each decide their own;
  write a shared initial value on both.

Because `!include` drops every local diagram into one namespace, state IDs have
to be unique across the local diagrams of one global diagram (error). Prefix them
per family (`acc`, `buy`, …).

PlantUML-only lines — `title`, `skinparam`, standalone notes — belong inside
`!ifndef PROMOTED … !endif`, with `!define PROMOTED` at the top of the global
diagram, so they show when the local diagram is rendered on its own and not when
it is included. Wrap them in a `CSDF-IGNORE-BEGIN`/`CSDF-IGNORE-END` region as
well, so the core parser skips them too.


Expansion
---------

For a global state `G`, a map `m`, an instance ID written `id`, and a local edge
`S --> T : e(a…) ; g ; p`:

```
G --> G : e(id, a…) ; id ∈ dom m ∧ m(id) ∈ 〈S〉 ∧ g ; m' = m ⊕ {id ↦ 〈T〉} ∧ p ∧ FRAME
```

with `id ∉ dom m` and `∪` for a creation, and `⩤` for a deletion. `FRAME` says
that the *other* state variables of `G` are unchanged. The map itself is not
framed: `⊕`, `∪` and `⩤` already say what happens to every key but `id`.

`sync e : m₁(p₁), m₂(p₂)` merges the edges each map contributes to `e` — the
cartesian product, since a guard on one side can pick out any state on the other
— into one edge per combination, over the states where all of them move. The
merged event's arguments are the instance IDs in the order the directive names
them, then each side's own arguments with duplicate names merged into one. Two
arguments that happen to share a name and mean different things would be merged
wrongly, so name them apart. In the states where the sync applies, the maps do
not expand the event on their own.

`constrain e(q…) ; c` conjoins `c` onto the guard of every generated edge that
matches `e` in name and arity, creations and merged edges included. This is
where a condition on another map goes; the local diagram cannot see one.
Hand-written edges are left alone: they already say everything they were meant
to.

A constrain reaches **every** matching generated edge, in every state. The block
its note points at is a drawing: it tells a reader which family the condition is
about, and it narrows nothing. A parameter of the constrain that no matching
edge takes is a warning — a mistyped name would otherwise be a free variable
sitting in a guard, and nothing else would notice.

`tau` is promoted without an instance ID — an internal event with an argument is
an observable one — and the instance is implicitly existentially quantified.
Syncing `tau` is an error. A promoted `tau` is a self-loop, so
`csdflivelockfree` will not call the result structurally livelock-free and will
emit a predicate obligation instead. That is the right answer, not a defect: the
obligation to discharge is that the number of instances in `〈S〉` goes down.

Predicates are copied **verbatim**. A local variable `v` denotes `m(id).v` after
promotion, but it is not rewritten: a predicate is opaque text, so a textual
substitution could only guess, and a wrong guess in a predicate is invisible.
The bare `v` that survives is readable in context thanks to the origin comment
printed above the edge.


Wording
-------

The generated clauses are symbolic by default, which belongs to no natural
language. `-template` replaces them clause by clause with a Go `text/template`
file:

| Template   | Fields              | Default                    |
|:-----------|:--------------------|:---------------------------|
| `inDom`    | `Map`, `ID`         | `id ∈ dom m`               |
| `notInDom` | `Map`, `ID`         | `id ∉ dom m`               |
| `atState`  | `Map`, `ID`, `Src`  | `m(id) ∈ 〈S〉`              |
| `update`   | `Map`, `ID`, `Dst`  | `m' = m ⊕ {id ↦ 〈T〉}`      |
| `create`   | `Map`, `ID`, `Dst`  | `m' = m ∪ {id ↦ 〈T〉}`      |
| `delete`   | `Map`, `ID`         | `m' = {id} ⩤ m`            |
| `frame`    | `Var`               | `v' = v`                   |
| `and`      | —                   | ` ∧ `                      |

A clause the file does not define keeps its default. Every clause is rendered
once when the file is read, so a template that cannot render is refused rather
than turning into an opaque predicate. A Japanese set ships as
`examples/promote/templates/ja.tmpl`.


Output
------

Plain Composable State Diagram syntax: the directives, the `!include`s, the
notes and the preprocessor lines are gone, and the composite states are
flattened into ordinary ones. Hand-written states and edges are kept as they
were, and every edge is printed in the canonical order. A line comment above
each generated edge names the local edge it came from; `-no-comments` leaves
them out.

The expanded form is **not** drawn. It is one state with a bundle of self-loops,
which is unreadable and slow to lay out. A repository whose CI expects a sibling
PNG for every `*.puml` should narrow that to the hand-written ones.


Diagnostics
-----------

An `error` leaves the diagram unprinted, so an unsound expansion never reaches
the tools downstream. A `warning` and an `info` go to standard error and let the
expansion through; `-Werror` makes a warning fatal. `-lint-only` checks without
printing. They are reported in source order.

Errors:

| What                                                                    | Why                                                                            |
|:------------------------------------------------------------------------|:--------------------------------------------------------------------------------|
| A `<<promote>>` title that is not `<map> : <ID> ⇸ <Type>`                | There is no map to promote into.                                                |
| A `<<promote>>` body that is not one `!include`                          | A block names one local diagram or none.                                        |
| A `<<promote>>` block outside any state                                  | The state it is written in is where the family moves.                           |
| A map the state holds no variable for                                    | There is nowhere for the instances to live.                                     |
| The same map promoted twice in one state                                 | The second block says nothing the first does not.                               |
| No `!include`, or two, across a map's blocks                             | One local diagram per map; two copies collide on every state ID.                 |
| Blocks of one map that disagree on the ID or the type                    | They are one family, so they are one shape.                                     |
| A composite state nested in another                                      | Only a `<<promote>>` block may be nested.                                       |
| A local `S₀` that holds state variables                                  | `S₀` means that the instance does not exist.                                    |
| A local end edge                                                         | Completion is a state with no transition out of it.                             |
| A local `S₀` self-loop                                                   | It is an event of an instance that is not there.                                |
| A local creation edge on `tau`                                           | Its instance is only existentially quantified, so instances would appear freely. |
| One state ID declared by two local diagrams                              | `!include` drops them into one namespace.                                       |
| A `sync` of `tau`                                                        | An internal event two families take together is an observable one.              |
| A `sync` naming one map twice                                            | One edge moves one key of one map.                                              |
| A `sync` of a map that is not promoted, or of an event a side lacks      | There is nothing to merge.                                                      |
| A `sync` whose maps never move in the same state                         | The event they take together can never happen.                                  |
| A `constrain` no generated edge matches in name and arity                | It constrains nothing.                                                          |
| A note holding a second directive                                        | One note, one directive.                                                        |
| A directive body that does not parse                                     | —                                                                               |

Warnings:

| What                                                                    | Why                                                                              |
|:------------------------------------------------------------------------|:----------------------------------------------------------------------------------|
| A post on the local start edge, or on a deletion edge                    | It is dropped: there is no instance for it to be about.                           |
| An event two local diagrams have that no `sync` names                    | Two families that name an event alike usually mean to take it together.           |
| A `sync` side whose edges do not all take the same arguments             | The merged event's arity would depend on the combination.                         |
| A `constrain` argument the matching edges do not take                    | A mistyped name would sit in the guard as a free variable.                        |
| A `constrain` guard naming none of its own arguments                     | It says nothing about the instance.                                               |
| A note pointing at a block of a map its `sync` does not name             | The line drawn is misleading.                                                     |
| A `<<promote>>` type the state variable does not mention                 | One of the two is out of date.                                                    |
| A note whose first word is a directive's name in another case            | `Sync` is not `sync`, and it would be drawn as prose.                             |

Info:

| What                                                                    | Why                                                                              |
|:------------------------------------------------------------------------|:----------------------------------------------------------------------------------|
| A state holding a map with no `<<promote>>` block for it                 | The family is frozen there, which may well be meant.                              |
| An `!include` outside a `<<promote>>` block                              | It names no local diagram and is dropped.                                         |

Every other tool refuses a diagram that still holds directives: the core grammar
has no rule for them, and the parse error says to run `csdfpromote` first.
