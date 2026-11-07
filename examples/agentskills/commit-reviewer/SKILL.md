---
name: commit-reviewer
description: |
  Reads the staged changes and says what a reviewer would ask about them.
  Use before a commit, when the user wants a second pair of eyes on a diff.
argument-hint: "[path]"
disable-model-invocation: true
---

# Commit reviewer

Read the staged diff (`git diff --staged`), or the diff of the path the user
gave. List what you would ask the author, most important first. Do not edit
anything.
