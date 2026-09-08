Promotion
=========

A system made of many instances of one thing — an account per customer, a trade per
order, a report per business day — has no readable single diagram. Written as one
Composable State Diagram, the control state of each instance is absorbed into a state
variable holding a partial map from instance IDs to local states, and the diagram
collapses to one global state with a bundle of self-loops. That form is what the
analyses want. It is not what a reader wants.

The readable form is the diagram of one instance. The two cannot be the same artefact,
so the author writes the local diagrams and a few directives, and `csdfpromote`
generates the global one.

The generation runs that way round for a reason. A local diagram is already a
structured Composable State Diagram, so expanding it into the global form needs no
structure that is not there. Projecting a global diagram back onto one instance would
instead need conventions about map names and state names inside the predicates — and a
predicate in this format is opaque natural language, which is exactly what such a
convention would have to break.


What the words mean
-------------------

The vocabulary is Z's. Where CSP says the same thing, this is the correspondence:

| Directive   | Z                                                                                 | CSP                                                     |
|:------------|:----------------------------------------------------------------------------------|:---------------------------------------------------------|
| `promote`   | free promotion: the framing schema `θLocal = m(id)`, `m' = m ⊕ {id ↦ θLocal'}`     | the symbolic form of `\|\|\| id : dom m @ Local(m(id))[[e ← e(id)]]` |
| `sync`      | the conjunction of two framing schemas                                             | alphabetised parallel, `[\| {e} \|]`                      |
| `constrain` | constrained promotion                                                              | nothing: a constraint process that reads every instance's state is the global diagram itself |

`sync` and `constrain` are not decoration. Coupling — a guard that reaches across maps,
an event two kinds of instance take together — is the substance of a specification.
Without somewhere to write it, promotion produces the direct product of independent
maps, which says almost nothing.


The directives
--------------

The directives are written in the **global** diagram. The grammar is in SYNTAX.md; the
local diagram's grammar does not change.

```plantuml
@startuml ACCOUNTS
state "稼働中" as running
running : accounts ; 口座ID ⇸ Account

[*] --> running : accounts は空

promote local/ACCOUNT.puml as Account via accounts(口座ID)
@enduml
```

### `promote <path> as <Type> via <map>(<idParam>) [in <state>, ...]`

Expands the local diagram at `<path>` into the global states of the `in` clause,
through the state variable `<map>`. `<idParam>` becomes the first argument of every
promoted event.

`<path>` is resolved against the directory of the global diagram, or the current
directory when it is read from standard input; `-base` overrides that. `<Type>` names
the local state type. It is only checked against the free text of the map's type as a
warning, because a state-variable type is free text.

Omitting `in` means the destination of the start edge alone. Naming several states
copies the same expansion into each of them, which is how one map is driven in several
operating modes. A state that holds the map but is not in the `in` clause has the map
**frozen**: nothing the promotion generates moves it there. That is legitimate — a
maintenance mode may be exactly that — so it is read back as an `info`, not a warning.

The edges that switch between such modes, and their frames ("every map is unchanged"),
are hand-written. They are outside the promotion and `csdfpromote` does not generate
them.

### `sync <event> : <map1>(<p1>), <map2>(<p2>), ...`

Makes the named local event one event of several instances instead of one event of
each. See "Synchronised events" below.

### `constrain <event>(<p1>, ...) ; <guard>`

Conjoins `<guard>` onto the guard of every expanded edge whose event matches in name
and in number of arguments. The event is written in its **promoted** form, so its first
argument is the instance ID. This is where a condition on another map goes — the local
diagram cannot see one.

`constrain` does not depend on the global state, and it applies to creation edges,
deletion edges and synchronised edges alike. It does not touch an edge the author wrote
by hand: such an edge already says everything it was meant to.


What a local diagram must be
----------------------------

The start state of a local diagram means **this instance does not exist**. Everything
else follows:

* it holds no state variables (there is nothing to hold) — an error if it does;
* an edge out of it creates an instance, adding a key to the map;
* an edge into it deletes one, removing the key.

An end edge (`--> [*]`) cannot be promoted and is an error. An instance that is finished
is a state with no outgoing edge, not a diagram that terminates.


How an edge is expanded
-----------------------

Write `G` for a global state of the `in` clause, `m` for the map, `S₀` for the local
start state, and `id` for the instance ID parameter. A local edge
`S --> T : e(a…) ; g ; p` becomes one global self-loop on `G`:

```
G --> G : e(id, a…) ; id ∈ dom m ∧ m(id) ∈ 〈S〉 ∧ g ; m' = m ⊕ {id ↦ 〈T〉} ∧ p ∧ FRAME
```

with two special cases:

* **creation** (`S` is `S₀`): the guard is `id ∉ dom m ∧ g`, and the post is
  `m' = m ∪ {id ↦ 〈T〉} ∧ p ∧ FRAME`. The post of the local start edge — the initial
  value of a fresh instance — is conjoined too.
* **deletion** (`T` is `S₀`): the post is `m' = {id} ⩤ m ∧ FRAME`. The local post is
  **discarded**: the absent state holds nothing for it to talk about. Writing one is a
  warning, not an error.

`FRAME` says that the other state variables of `G` do not move. The map itself needs no
clause of its own: `⊕`, `∪` and `⩤` already say what happens to every key but `id`.

Several edges out of the same state on the same event stay several edges. Ordinary
non-determinism is preserved.

### Predicates are copied verbatim

`g` and `p` are embedded as written. A local variable `v` denotes `m(id).v` after
promotion, but it is **not** rewritten. A predicate is opaque text, so a textual
substitution could only guess, and a wrong guess in a predicate is invisible. The bare
`v` that survives is readable in context: the line comment above each generated edge
names the local diagram and the two local states it runs between.

### τ

A local `tau` edge is promoted as `tau`, with the same guard and post. It is **not**
given an instance ID: an internal event with an argument is an observable one, which is
the opposite of what hiding means. The `id` is implicitly existentially quantified —
"for some instance…".

A promoted `tau` is a self-loop, so `csdflivelockfree` cannot call the diagram
structurally livelock-free, and it will emit a predicate obligation instead. That is
the right answer, not a defect: the post does move `m(id)` on, so a local diagram with
no τ-cycle does not diverge, and the obligation to discharge is that the number of
instances in `〈S〉` decreases.


Synchronised events
-------------------

`sync e : m₁(p₁), m₂(p₂)` takes the group of local `e` edges of each map and emits one
global edge for **every combination** — the cartesian product — on every state the two
promotions share:

```
G --> G : e(p₁, p₂, args…) ; GUARD₁ ∧ GUARD₂ ; POST₁ ∧ POST₂ ∧ FRAME
```

The product is needed because a guard on one side can pick out any state on the other.
The parameters of the `sync` directive, not those of the `promote` directives, name the
instance IDs of the merged edge.

A map cannot be synced with itself: both sides would frame the whole map, so the two
updates could only be satisfied by one instance being the other. Neither can `tau` be
synced: an internal event is by definition not shared. Both are errors.

The states are the intersection of the `in` clauses of the promotions. An empty
intersection is an error: the event could never happen.

Arguments of the same **name** are merged into one argument — the instances agree on
the value the shared event carries. Two arguments that happen to share a name and mean
different things would be merged wrongly, so name them apart.

An event synced through a map is not expanded independently for that map. An event that
appears in two local diagrams and is *not* synced is a warning, and so is one that is
synced for some of the maps that have it but not for all: taking an event independently
is legitimate but is more often an oversight.

One event takes one `sync` directive. A second directive for the same event would emit
the merged edges twice, and each would keep the other's maps from expanding on their
own, so it is an error. A directive that names a single map is a warning: it is what
promoting that map alone already does.


Generated phrases
-----------------

The phrases the expansion generates (`id ∈ dom m`, `m(id) ∈ 〈S〉`, `m' = m ⊕ …`, the
frame) are symbolic by default so that they belong to no natural language. `-template`
replaces them clause by clause with a Go `text/template` file; a clause the file does
not redefine keeps its symbolic form. A clause that parses but cannot be rendered — a
field that does not exist, say — is an error, not a phrase: a fault buried in an opaque
predicate is one nothing downstream can catch. The clauses are `exists`, `absent`, `at`,
`insert`, `update`, `delete`, `keep` and `unchanged`, and each is given `.Map`, `.ID`,
`.Src`, `.Dst`, `.Guard`, `.Post`, `.Other` and `.OtherMaps`. A Japanese version ships
as `examples/promote/templates/ja.tmpl`.


Checks
------

`csdfpromote` checks the structure as it expands. Errors leave the diagram unprinted;
warnings do not, unless `-Werror`; info never affects the exit status. `-lint-only`
checks without printing.

| Severity | Condition                                                                             |
|:---------|:---------------------------------------------------------------------------------------|
| error    | the promoted map is not a state variable of a state in the `in` clause                 |
| error    | the `in` clause names a state the diagram does not have                                |
| error    | the local diagram cannot be read                                                       |
| error    | the local start state holds state variables                                            |
| error    | the local diagram has an end edge                                                      |
| error    | the same map is promoted twice                                                         |
| error    | a synced map is not promoted                                                           |
| error    | a synced event is missing from one of the local diagrams                               |
| error    | the promotions of the synced maps share no state                                       |
| error    | a map is synced with itself                                                            |
| error    | `tau` is synced                                                                        |
| error    | two `sync` directives name the same event                                              |
| error    | no expanded edge matches a `constrain` in event name and number of arguments           |
| error    | a clause template cannot be rendered                                                   |
| warning  | an event is in two local diagrams and is not synced, or is synced for only some of them |
| warning  | a `sync` directive names a single map                                                  |
| warning  | the promoted type does not appear in the type of the map                               |
| warning  | a deletion edge has a local post, which is discarded                                   |
| warning  | a `constrain` guard mentions none of its parameters                                    |
| info     | a state holds the map but is not in the `in` clause, so the map is frozen there        |


Using it
--------

```
$ csdfpromote 02_spec/ACCOUNTS.puml > build/ACCOUNTS.expanded.puml
$ csdflivelockfree build/ACCOUNTS.expanded.puml
$ csdfrepl        build/ACCOUNTS.expanded.puml
```

PlantUML cannot read the directives, so render the output rather than the input. Worked
examples, with their expansions recorded next to them, are under `examples/promote`.
