Glossary
========

Composable State Diagram Format
-------------------------------
A representation format for state transition diagrams.
Refer to SYNTAX.md for grammar rules.
It is a subset of PlantUML's state diagram notation.

Promotion
---------
Writing a family of like instances as one state variable holding a partial map
from instance IDs to local states, and every local transition as a self-loop on
the global state that updates one key of that map.
The term is Z's; the CSP reading is `||| id : dom m @ Local(m(id))[[e <- e(id)]]`.
`csdfpromote` generates that form from the diagram of one instance.
Refer to PROMOTION.md.

Global diagram
--------------
The diagram that holds the promotion directives and the state variables the
families live in. It is hand-written and it renders.

Local diagram
-------------
The Composable State Diagram of one instance, named by a `<<promote>>` block's
`!include`. Its start state means that the instance does not exist.
Its grammar is the ordinary one.

Expanded form
-------------
What `csdfpromote` prints: plain Composable State Diagram syntax with every
directive consumed. It is what the other tools read, and it is never drawn.
