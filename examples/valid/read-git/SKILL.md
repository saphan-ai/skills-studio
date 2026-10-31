---
name: read-git
description: Gives the executor, tester and reviewer the one way to read a git repository in this project — ranges, --since, pathspecs and --format fields, counts from rev-list, refs by pattern, objects by name — and the growth test that proves the bound. Use when a change adds or alters code that reads history, refs or objects of a git repository.
license: MIT
compatibility: git of the version the project pins, and a disposable repository the test builds.
metadata:
  saphan.origin: project
  saphan.default: "off"
  saphan.roles: "executor, tester, review"
  saphan.instruments: "E16, R16, T14"
  saphan.param.growth_commits: "commits in the growth test's repository; default: 50000"
  saphan.param.growth_refs: "refs in the growth test's repository; default: 5000"
  saphan.param.deadline: "deadline per git call; default: 10 s"
  saphan.conflicts: "none"
  saphan.upstream: "none — written for this project from the advisor design of 2026-10-26"
---

# Reading git

## What this adds

The git craft for **E16**, **R16** and **T14**. The bridge alone holds tens of thousands of commits and
files; a read that walks all of it is a defect today.

## Use when

- Code runs `git log`, `rev-list`, `for-each-ref`, `ls-files`, `ls-tree`, `cat-file` or reads `.git`
  directly, or a library does the same.

## Procedure

1. **Bound the walk in the command.** `git log` takes a range (`A..B`), `--since`, `-n`, and a pathspec
   (`-- <path>`); never the whole history to find one commit.
2. **Ask for the fields.** `--format=` with the placeholders used, separated by `%x00`; never parse the
   default output.
3. **Count with git.** `git rev-list --count A..B`; never list commits to count them.
4. **Refs by pattern.** `git for-each-ref --format=… refs/heads/<prefix>*`, or `rev-parse --verify` for one
   ref; never every ref then filter.
5. **Objects by name, many in one process.** `git cat-file --batch` (or `--batch-check`) fed the names;
   never one `git show` per object in a loop.
6. **Files by pathspec.** `git ls-files -- <glob>`, `git diff --name-only A B -- <path>`; never the whole
   tree then filter.
7. **A deadline per call** (`deadline`) through the process context; a timed-out git is an error.
8. **Stream the output.** Read stdout line by line and stop early when the caller has its answer; never
   buffer a whole log in memory.

## The growth test

Build a disposable repository with `growth_commits` commits and `growth_refs` refs (a script with
`fast-import` takes seconds). Run the reader and assert: the git argv carries the range, `--since` or
pathspec it should (captured from the runner), the commits or refs read are bounded by the answer, not by
the repository, and the call finishes inside `deadline`. **Negative control:** drop the range or pattern
and filter in the language — the test turns red.

## Evidence to return

- The git commands as run.
- The growth test: repository size, items read, time, and the red of its negative control.

## Red flags

- `git log` with no range, `-n` or `--since`; `for-each-ref` with no pattern.
- A loop running one git process per commit or object.
- Reading the whole output into a string before parsing.

## What this does not do

- It does not write to a repository; writes follow the project's git conventions.
