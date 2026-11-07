---
name: csv-column-stats
description: Reports the row count and the distinct values of one column of a CSV file. Use when the user wants a quick look at a column before deciding what to do with it.
compatibility: Needs a POSIX shell with cut, sort and uniq.
allowed-tools: Read Bash
metadata:
  author: example
  version: "1.0"
---

# CSV column stats

Run `scripts/column.sh <file> <column number>`. The script prints each distinct
value with its count. `references/csv-notes.md` says what to do with quoted
fields, and `assets/sample.csv` is a file to try it on.
