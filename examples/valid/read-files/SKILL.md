---
name: read-files
description: Gives the executor, tester and reviewer the one way to read a file tree in this project — a glob that names the files, a depth and count bound, an early stop, large files streamed line by line or read by offset — and the growth test that proves the bound. Use when a change adds or alters code that walks directories or reads logs, ledgers or other files that grow.
license: MIT
compatibility: The project's toolchain and a temporary directory the test fills.
metadata:
  saphan.origin: project
  saphan.default: "off"
  saphan.roles: "executor, tester, review"
  saphan.instruments: "E16, R16, T14"
  saphan.param.growth_files: "files in the growth test's tree; default: 50000"
  saphan.param.growth_bytes: "size of the growth test's largest file; default: 256 MiB"
  saphan.param.deadline: "deadline for one read; default: 5 s"
  saphan.conflicts: "none"
  saphan.upstream: "none — written for this project from the advisor design of 2026-10-26"
---

# Reading files

## What this adds

The file-system craft for **E16**, **R16** and **T14**: directory trees, logs, ledgers and JSONL files grow
without anyone noticing, and a reader that loads them whole stops one day.

## Use when

- Code walks a directory, globs files, or reads a log, ledger, JSONL, record or any file that grows.

## Procedure

1. **Name the files.** A glob or a known path selects what is read; a walk has a root, a maximum depth and
   a maximum count, and skips directories it does not need (`fs.SkipDir`).
2. **Stop early.** A search for one file or one line returns at the first match.
3. **Stream, never slurp, a file that grows.** `bufio.Scanner` (with its buffer raised for long lines) or a
   line reader; never `os.ReadFile`/`readFileSync` on a log, ledger or JSONL.
4. **Read the end by offset.** "The last N lines" seeks from the end; "what is new" keeps the byte offset
   it stopped at and reads from there.
5. **An index beats a scan.** A reader that looks one item up by key many times keeps or builds an index
   (a manifest, a sorted file, a store table), never a walk per lookup.
6. **A deadline per read** (`deadline`) through the context; a cancelled walk returns its error.

## The growth test

Fill a temporary tree with `growth_files` files and one file of `growth_bytes`. Run the reader and assert:
files opened and bytes read are bounded by the answer (count them through a wrapping file system or the
reader's own counters), memory stays bounded (no buffer the size of the file), and the read finishes
inside `deadline`. **Negative control:** replace the stream with a whole-file read or the glob with a full
walk — the test turns red.

## Evidence to return

- The glob, bounds and streaming call used.
- The growth test: tree and file size, files opened, bytes read, time, and the red of its negative control.

## Red flags

- `ReadFile`/`readFileSync`/`ReadAll` on a file that grows; a walk with no depth or count bound.
- A walk inside a loop; a search that continues after its match.

## What this does not do

- It does not decide where files live or how they rotate; that is the owning track's design.
