# Contributing

Thank you for helping. This repository holds a specification, so a change here
is judged first by what it does to the documents that pass and fail.

## Before you start

Open an issue for anything that changes a verdict: a new rule, a changed limit,
a new field. A change that accepts a document version 1 refuses, or refuses one
it accepts, is a new major version of the format (see section 11 of the
specification) and is decided by the maintainers.

Corrections, clearer wording, new examples and fixes to the verifier that bring
it in line with the specification need no prior discussion.

## How a rule is added

A rule lives in four places that must agree, and `go test ./...` fails if they
do not:

1. the rule list in `internal/skillfmt/rules.go`;
2. the specification, section 9 of `spec/SKILL_FORMAT_V1.md`;
3. the schema, as an `x-rule` annotation, when the rule is a schema constraint;
4. one skill under `examples/invalid/<rule, lowercase>/<skill>/` that is refused
   by that rule and by no other.

## Tests

- Run `go test ./...`. No test may need the network.
- A test that guards a refusal changes exactly the field the rule guards and
  nothing else. Show that it fails when the rule is removed.
- Do not weaken a test or an example to make a change pass.
- Standard library only; a new dependency needs an issue first.

## Wording

The specification and the README say what the format is and what the tools check.
They do not claim that a skill is fit to run, and they do not describe how the
closed part of Skills Studio works. A change that adds such a claim will be asked
to remove it.

## Commits and pull requests

One logical change per commit. Write the subject as `<area>: <what>` and use the
body to say why. Describe in the pull request what changed, why, and how you
checked it, including the commands you ran.

By contributing you agree that your contribution is licensed under the Apache
License 2.0 (`LICENSE`) for code and `LICENSE-docs` for documentation.
