---
name: changelog-helper
description: >-
  Turns a list of merged changes into a short changelog entry. Use when
  the user asks for release notes, a changelog entry or a summary of what
  changed between two versions.
license: Proprietary. LICENSE.txt has complete terms
version: "1.2"
---

# Changelog helper

1. Read the list of changes the user gives you.
2. Group them under Added, Changed and Fixed.
3. Run `scripts/count.sh` on the list when it is long, to see how many lines
   each group holds.
4. Write one line per change, in the past tense.
