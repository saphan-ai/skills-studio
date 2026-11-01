# Skills Studio — the open part

Skills Studio helps teams write, check and trust the skills their AI agents
use. A **skill** is a directory with a `SKILL.md` file: a short description and
instructions an agent reads when the description matches the task.

This repository is the open part of Skills Studio, under the Apache License 2.0.

## What is open here

| path | what it is |
|---|---|
| [`spec/SKILL_FORMAT_V1.md`](spec/SKILL_FORMAT_V1.md) | the skill format, version 1, with its canonical hash and the meaning of "verified" |
| [`spec/skill-format-v1.schema.json`](spec/skill-format-v1.schema.json) | the front matter as a JSON Schema (draft 2020-12) |
| [`cmd/skills`](cmd/skills) | `skills verify`, an offline checker for a skill directory |
| [`examples/`](examples) | skills the format accepts, one skill per rule it refuses, and a signed demonstration |

## What is not here

The Skills Studio engine is not in this repository, and nothing here depends on
it. That includes `skills check` and everything beyond it: the checks that look
at what a skill says and does, and the reviews and records built on them. This
repository lets anyone read, write and verify the *format*, and build an
independent implementation of it.

## Use it

```
go install github.com/saphan-ai/skills-studio/cmd/skills@latest

skills verify path/to/my-skill
skills verify path/to/my-skill --signature my-skill.sig --pubkey author.pub
```

`skills verify <dir>` checks the directory against the schema and the layout
rules and prints the skill's canonical hash:

```
skill: read-files
canonical-sha256: 7273c8112a081b0c93ac5b4f7a33f8dc4b6b50df2b55f0ae0dfdd8ff7000db2c
format: skill-format-v1 ok
signature: not checked
```

With `--signature` and `--pubkey` it also checks an ed25519 signature over that
hash, with no network access. Exit codes: `0` pass, `1` refused with a named rule
(printed on stderr), `2` usage.

## What "verified" means, and what it does not

A skill is *verified* when a signature over its canonical hash checks against a
public key you supplied. That tells you the text is the text that key signed.
The projection header a store may write above a skill is not covered by the hash
or by a signature: a consumer MUST give an agent the canonical text, with the
header removed, and never the stored file.

It does not tell you who holds the key, what the skill makes an agent do, or how
it behaves on any model. Skills Studio makes no claim about a skill beyond the
checks it names, run on the models it names, with the trace they produced. This
repository's tools make only the claim in the previous paragraph.

## Development

```
go test ./...
```

The tests hold the rules, the schema, the specification text and the examples
to one another; adding a rule in one place and not the others fails the build.
Contributions: see [CONTRIBUTING.md](CONTRIBUTING.md). Reporting a
vulnerability: see [SECURITY.md](SECURITY.md).

## Licence

Code: Apache-2.0 (`LICENSE`). Documentation and the specification text:
`LICENSE-docs`.
