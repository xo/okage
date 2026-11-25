# D91. lottie ignores expressions, and webfont reads no MicroType Express

Status: Decided, amends D41 and D45.

## Context

Two items of [`../BACKLOG.md`](../BACKLOG.md) waited for work that no user
of okage asked for:

1. Lottie expressions. An expression is a script in JavaScript that computes
   the value of a property in each frame. velato ignores them, and the port
   of velato in `lottie` ignores them too (D41). 29 files of the corpus of
   D41 have expressions. Almost all of them are references of the form
   `thisComp.layer(...)`. The files still pass, because the values of the
   keyframes are close to the values of the expressions.
2. EOT files with MicroType Express compression. `webfont` returns
   `okage.ErrFontUnsupported` for such a file (D45, D51). Only old versions
   of Internet Explorer wrote them.

## The decision

On 2026-11-25 the maintainers decided that okage does not plan either
feature:

1. `lottie` ignores expressions, as velato does. It draws the value of the
   keyframes of a property that has an expression. An evaluator of
   JavaScript in pure Go is a large project, and it is not a part of a port
   of velato.
2. `webfont` does not read an EOT file with MicroType Express compression.
   It returns `okage.ErrFontUnsupported` for it, as it does now.

The two items leave [`../BACKLOG.md`](../BACKLOG.md). If velato adds
expressions, the port follows velato with a new decision.
