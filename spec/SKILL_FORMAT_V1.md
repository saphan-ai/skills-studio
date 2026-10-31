# Skill format, version 1

Status: published specification, version 1. Licence: see `LICENSE-docs`.

The key words MUST, MUST NOT, SHOULD and MAY are used as in RFC 2119.

## 1. What this document is

This document defines how a **skill** is laid out on disk, what its `SKILL.md`
file contains, how a single **canonical hash** is computed over it, and what the
word **verified** means when a tool says it about a skill.

It defines a file format. It says nothing about what a skill instructs an agent
to do, and a skill that passes every rule here has been checked for form and for
nothing else (section 10).

The format is pinned to the Agent Skills specification as that page was read on
2026-10-30 (`https://agentskills.io/specification`; the page carries no version
or date). Section 5 states the facts taken from it. Where this document and a
later version of that page differ, this document governs files claiming
conformance to *skill format v1*.

The published JSON Schema for the front matter is
[`skill-format-v1.schema.json`](skill-format-v1.schema.json) (JSON Schema draft
2020-12). The reference verifier is `skills verify` in this repository.

## 2. The skill directory

A skill is a directory. Its name is the skill's name.

```
<name>/
  SKILL.md        required
  references/     optional: further text the skill points to
  assets/         optional: fixed data the skill points to
```

Rules for the directory:

- It MUST contain a regular file named `SKILL.md` (`SF-SKILLMD-MISSING`).
- Its only other entries MAY be `references/` and `assets/`, each a directory
  (`SF-FILE-UNLISTED`). Directories nested inside them are allowed.
- It MUST NOT contain a `scripts/` entry (`SF-FILE-SCRIPT`). See section 7.
- Nothing in it, at any depth, MAY be a symbolic link (`SF-FILE-SYMLINK`).
- The format does not read the files under `references/` and `assets/`: it does
  not hash them, parse them or limit their types. Section 8 says what the hash
  therefore does not cover.

## 3. The text of SKILL.md

`SKILL.md` is UTF-8 text. Its line ends are a single LF byte. It has no byte
order mark, no CR byte and no NUL byte (`SF-TEXT-ENCODING`).

### 3.1 The projection header

A system that stores a skill may write a **decision-projection header** at the
very top of the file. A header:

- opens with a first line that is exactly `<!-- saphan-decision-projection-v1`;
- runs through the first following line that is exactly `-->`;
- is followed by exactly one blank line.

A file whose first line is not exactly the opening line has no header: a line
that merely begins with those characters, or a header-like block lower in the
file, is ordinary text. A header that is never closed, or whose closing line is
not followed by one blank line, makes the file invalid (`SF-HEADER-MALFORMED`).
Only one header is removed.

The header is not part of the skill. Everything after the blank line is the
**canonical text** (section 8). A file without a header is its own canonical
text.

## 4. Front matter

The canonical text MUST begin with a front matter block: a line `---`, the
fields, and a closing line `---` (`SF-FRONTMATTER-MISSING`). Text after the
closing line is the body and is free-form Markdown.

The front matter is read into the JSON data model (objects, strings, numbers,
booleans, null) and that value is what the schema describes. A reader accepts
the following subset of YAML and nothing more (`SF-FRONTMATTER-SYNTAX`):

- One field per line, `key: value`, with a space after the colon. A key is made
  of letters, digits, `.`, `_` and `-`. A key MUST NOT appear twice in a block.
- One nested block: a field with an empty value followed by lines indented by
  exactly two spaces, each `key: value`, is read as an object. Nesting goes no
  deeper. Indent with spaces, never tabs.
- Lines that are blank, or that begin with `#`, are ignored.
- A value is one of:
  - a **double-quoted** string, with the escapes of a JSON string;
  - a **single-quoted** string, in which a quote is written twice;
  - a **plain** scalar, which runs to the end of the line. A plain scalar MUST
    NOT contain `: ` or ` #`, and MUST NOT begin with any of
    `[ { & * ! | > % @ ` - ?`. A plain `true` or `false` is a boolean, `null` or
    `~` is null, a decimal number such as `3` or `-1.5` is a number, and
    anything else is a string. The plain words `y`, `n`, `yes`, `no`, `on` and
    `off`, in any letter case, are refused, because some YAML readers turn them
    into booleans: write them in quotes.
- Lists, inline maps, anchors, tags, multi-line and folded (`|`, `>`) scalars
  are not in the subset. Quote the text instead.

The subset exists so that every conforming reader obtains the same value from the
same bytes.

## 5. Fields

| field | required | type | constraint | rule |
|---|---|---|---|---|
| `name` | yes | string | only `a-z`, `0-9` and `-`; at most 64 characters; equal to the directory name | `SF-NAME-REQUIRED`, `SF-NAME-PATTERN`, `SF-NAME-LENGTH`, `SF-NAME-DIR-MISMATCH` |
| `description` | yes | string | 1 to 1024 characters | `SF-DESCRIPTION-REQUIRED`, `SF-DESCRIPTION-LENGTH` |
| `license` | no | string | none | `SF-FIELD-TYPE` |
| `compatibility` | no | string | at most 500 characters | `SF-COMPATIBILITY-LENGTH` |
| `metadata` | no | map from string to string | every value a string | `SF-METADATA-TYPE`, `SF-METADATA-VALUE-TYPE` |
| `allowed-tools` | no | string | space-separated tool names; marked experimental by the page of section 1 | `SF-FIELD-TYPE` |

A value of the wrong type for `name`, `description`, `license`, `compatibility`
or `allowed-tools` is refused by `SF-FIELD-TYPE`. A top-level key not in this
table is refused by `SF-KEY-UNKNOWN`; extensions go inside `metadata`.
Length is counted in characters (Unicode code points), as JSON Schema counts it.

### 5.1 Keys this format reads inside `metadata`

Keys beginning `saphan.` are the format's own. Every `metadata` value is a
string. Two keys have a closed set of values; the others are free text.

| key | value |
|---|---|
| `saphan.origin` | `builtin` or `project` (`SF-SAPHAN-ORIGIN`) |
| `saphan.default` | `on` or `off`, written in quotes (`SF-SAPHAN-DEFAULT`) |
| `saphan.roles` | free text: the roles the skill is for, comma-separated |
| `saphan.instruments` | free text: craft identifiers, such as `C1, E1` |
| `saphan.conflicts` | free text: skills that do not combine with this one, or `none` |
| `saphan.upstream` | free text: `<owner>/<repo>@<sha>` of the text it was derived from, or `none` |
| `saphan.param.<name>` | free text: a parameter the skill takes, with its default |

Other keys, including other `saphan.` keys, are carried and not interpreted.

## 6. What a skill may not carry

The schema and the layout rules together are the whole of what this version
checks. Anything the format does not list is refused, not ignored: an unknown
front matter key, an unlisted file, a symbolic link.

## 7. Scripts

A script never enters a skill. `scripts/` is not an allowed entry, and the
verifier refuses a skill that has one (`SF-FILE-SCRIPT`). Executable behaviour
belongs in a **tool**: a separate artefact, admitted on its own terms and cited
by identifier from the skill's text. The skill says which tool to call; it does
not contain the tool.

The format does not inspect the contents or file modes under `references/` and
`assets/`. Those directories are for text and data; a file there is not a
sanctioned place for a script, and this version does not detect one.

## 8. The canonical hash

A skill's identity for trust purposes is one value: the **canonical hash**.

**Algorithm (normative).**

1. Read `SKILL.md` as bytes. Refuse it under section 3 if it is not text of the
   required form.
2. If the first line is exactly `<!-- saphan-decision-projection-v1`, remove the
   header: every byte up to and including the LF that ends the first line that
   is exactly `-->`, and then the one empty line after it (one further LF). If
   the file has that first line and no valid close, refuse it
   (`SF-HEADER-MALFORMED`). If the first line is anything else, remove nothing.
3. The remaining bytes are the canonical text. No other normalisation is
   applied: no trimming, no re-flowing, no Unicode normalisation.
4. The canonical hash is the SHA-256 of the canonical text, written as 64
   lowercase hexadecimal digits.

**What the hash covers.** The canonical text only. It does not cover
`references/`, `assets/`, the file's name or modification time, or the header.
A change to any byte of the canonical text changes the hash.

**Worked example 1: a file without a header.** The file
`examples/valid/read-files/SKILL.md` has no header, so its canonical text is the
whole file, and

```
sha256 = 7273c8112a081b0c93ac5b4f7a33f8dc4b6b50df2b55f0ae0dfdd8ff7000db2c
```

**Worked example 2: a file with a header.** The file
`examples/valid/projected-skill/SKILL.md` begins:

```
<!-- saphan-decision-projection-v1
_id: example--projected-skill
status: RATIFIED
A header like this one is written around a skill by the system that stores it.
It is not part of the skill: the canonical hash is taken without it.
-->

---
name: projected-skill
```

The first line is the opening line, the line `-->` closes the header and one
blank line follows it. The canonical text starts at the line `---`, and

```
sha256 = 3331f0dbd1a8b966b4323771692e75ef6dd71aa545c0fa041668a5e747f2d668
```

The same skill stored without its header has the same hash.

`skills verify <dir>` prints the hash; `skills verify` of the two directories
above must print the two values shown.

## 9. Rules

A verifier refuses with a named rule. The name is stable within version 1.
`skills verify` exits 1 and prints every rule that applies, one line each.

| rule | refuses |
|---|---|
| `SF-SKILLMD-MISSING` | a directory with no `SKILL.md` |
| `SF-FILE-SCRIPT` | a `scripts/` entry |
| `SF-FILE-UNLISTED` | an entry other than `SKILL.md`, `references/`, `assets/` |
| `SF-FILE-SYMLINK` | a symbolic link at any depth |
| `SF-TEXT-ENCODING` | `SKILL.md` that is not UTF-8 with LF line ends, no BOM, no NUL |
| `SF-HEADER-MALFORMED` | a projection header that is not closed, or not followed by one blank line |
| `SF-FRONTMATTER-MISSING` | no front matter block at the head of the canonical text |
| `SF-FRONTMATTER-SYNTAX` | front matter outside the subset of section 4 |
| `SF-KEY-UNKNOWN` | a top-level key not in section 5 |
| `SF-NAME-REQUIRED` | no `name` |
| `SF-NAME-PATTERN` | a `name` that is not `a-z`, `0-9`, `-` only |
| `SF-NAME-LENGTH` | a `name` of more than 64 characters |
| `SF-NAME-DIR-MISMATCH` | a `name` different from the directory name |
| `SF-DESCRIPTION-REQUIRED` | no `description` |
| `SF-DESCRIPTION-LENGTH` | a `description` that is empty or longer than 1024 characters |
| `SF-FIELD-TYPE` | a non-string `name`, `description`, `license`, `compatibility` or `allowed-tools` |
| `SF-COMPATIBILITY-LENGTH` | a `compatibility` of more than 500 characters |
| `SF-METADATA-TYPE` | a `metadata` that is not a map |
| `SF-METADATA-VALUE-TYPE` | a `metadata` value that is not a string |
| `SF-SAPHAN-ORIGIN` | `saphan.origin` other than `builtin` or `project` |
| `SF-SAPHAN-DEFAULT` | `saphan.default` other than `on` or `off` |
| `SF-SIGNATURE-MALFORMED` | a signature file that does not decode to 64 bytes |
| `SF-PUBKEY-MALFORMED` | a public key file that does not decode to an ed25519 key |
| `SF-SIGNATURE-INVALID` | a signature that does not verify over the canonical hash |

Every rule above except the last three, and except `SF-FILE-SYMLINK` (a link
cannot be committed portably), has a skill in `examples/invalid/<rule>/` that is
refused by that rule and no other. The schema carries the rule name of each of
its constraints in an `x-rule` annotation; the annotation does not change which
documents are accepted.

## 10. What "verified" means

A skill is **verified** when an ed25519 signature over its canonical hash
checks against a public key the verifier was given. That is all.

- The signed message is the 32 raw bytes of the SHA-256 digest, not the hex text.
- A signature file holds the 64 signature bytes raw, or as 128 hex digits, or as
  base64. A public key file holds 32 raw bytes, 64 hex digits, base64, or a PEM
  `PUBLIC KEY` block.
- Verification is offline. It reads the two files and the skill directory and
  contacts nothing.

What verification establishes: the canonical text is byte for byte the text the
holder of that key signed. What it does not establish: who holds the key, that
the key should be trusted, that the signer read the skill, that the skill
behaves in any particular way on any model, or anything about `references/` and
`assets/`. A claim about behaviour needs its own evidence — the checks run, the
models they ran on and the trace they produced — and is not made by this format.

## 11. Conformance and versions

A file conforms to skill format v1 when `skills verify` of its directory exits 0
at the commit of this specification. An independent verifier conforms when it
agrees with the reference verifier on every directory under `examples/`.

Changes that accept a document v1 refuses, or refuse one it accepts, are a new
major version with a new schema file. Additions that change no verdict, such as
a corrected sentence, keep the version.
