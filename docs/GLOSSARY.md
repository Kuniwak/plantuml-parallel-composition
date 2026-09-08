Glossary
========

Composable State Diagram Format
-------------------------------
A representation format for state transition diagrams.
Refer to SYNTAX.md for grammar rules.
It is a subset of PlantUML's state diagram notation.


Promotion
---------
Writing a system of many instances of one thing as a single Composable State Diagram,
by holding the local state of each instance in a state variable that maps instance IDs
to local states. The term is Z's. The `promote`, `sync` and `constrain` directives
declare it; `csdfpromote` expands them into ordinary edges.
Refer to PROMOTION.md.

Local diagram
-------------
The Composable State Diagram of a single instance, which a `promote` directive refers
to. Its start state means that no such instance exists.

Global diagram
--------------
The Composable State Diagram that holds the maps and the promotion directives. It is
what the analyses read, once `csdfpromote` has expanded it.

Promotion directive
-------------------
A `promote`, `sync` or `constrain` line in a global diagram. Every tool but
`csdfpromote` and `csdfparse` refuses a diagram that still carries one.
