# Export formats: the adapters

`skills export <dir> --format <name> --out <dir>` writes a skill in the format
another tool reads. One format is one adapter: one Go type behind one small
interface. The formats are not built into the command; they are a registry, and
anyone can add one.

```
skills export examples/valid/all-fields --format cursor-rules --out ./out
```

The command reads the skill with the same reader as `skills verify`, refuses a
skill that is not valid Agent Skills (level 1) by the rule that refuses it,
writes only into `--out`, never overwrites a file that is there, and prints one
`loss:` line for every piece of the skill the format cannot carry. An unknown
`--format` lists the registered names.

## The built-in formats

| `--format` | writes | reads back |
|---|---|---|
| `agentskills` | `<name>/SKILL.md` byte for byte, and every other file | yes (`Import`) |
| `cursor-rules` | `.cursor/rules/<name>.mdc`: front matter `description`, `globs`, `alwaysApply`, then the Markdown | no |
| `agents-md` | `AGENTS.md`: the name as a heading, the description, the body; no front matter | no |
| `copilot-instructions` | `.github/copilot-instructions.md`: the same text as `agents-md` | no |

`examples/export/<format>/` holds what each writes for `examples/valid/all-fields`;
a test compares them with the adapters' output.

**What these four rest on.** The shapes of `cursor-rules`, `agents-md` and
`copilot-instructions` were written from the maintainers' understanding of those
formats on 2026-11-02, not checked against each tool's documentation. The
`cursor-rules` front matter keys (`description`, `globs`, `alwaysApply`) and the
values written (`globs` empty, `alwaysApply` false: the rule applies when the
agent judges the description relevant) are that understanding. If a target
changes or the understanding is wrong, the fix is one adapter and its sample.
`agentskills` is the format the skill already is, so it drops nothing.

**What the loss lines do and do not say.** A loss names a piece of the skill the
format has no place for (`license`, `compatibility`, `allowed-tools`, `metadata`,
a front matter key the Agent Skills page does not define, a file or a directory
such as `references/`, the `canonical-hash`). The text of a skill is carried:
`agents-md` and `copilot-instructions` write the body as it is. A loss line says
nothing about whether the target tool would act on a field it can carry; "losses:
none" says nothing was dropped from what this program read.

## The interface

Package `exportfmt` (`github.com/saphan-ai/skills-studio/exportfmt`):

```go
type Adapter interface {
    Name() string                                   // the value of --format
    Export(s Skill) ([]File, []Loss, error)         // deterministic; no disk
}

type Importer interface {                           // optional
    Adapter
    Import(dir string) (Skill, error)               // reads what Export wrote
}
```

`Skill` is a skill directory that `exportfmt.Load` read and found valid Agent
Skills: `Name`, `Description`, `License`, `Compatibility`, `AllowedTools`,
`Metadata`, `Extra` (every other front matter key), `Body`, `Files` (every file
besides `SKILL.md`), `SkillMD` (the file as read) and `Hash` (the canonical hash;
empty unless the skill is also in the Saphan profile). A `Loss` is an `Item` and
a one-line `Reason`. An `Item` is a front matter key, `canonical-hash`, `projection-header` (the header that may open SKILL.md; `Skill.HasProjectionHeader` says whether it does), a file
path, or a top level directory with a trailing slash.

An adapter never touches the disk. The caller validates what it returned before
a byte is written (`exportfmt.ValidateOutput`): every path is clean, relative,
inside the output directory and unique, so an adapter written by anyone cannot
write outside `--out`.

## Adding a format

One format is one adapter. Adding one is four things, and nothing shared changes.

1. **The type.** One Go type with `Name` and `Export`; if the format can be read
   back, also `Import`. Put a compile-time assertion beside it:
   `var _ exportfmt.Adapter = MyFormat{}` (and `exportfmt.Importer` if it imports).
2. **Its tests.** Run the conformance test, and add the case the adapter refuses
   with the refusal in a sentence:

   ```go
   func TestConformance(t *testing.T) { exporttest.Conformance(t, MyFormat{}) }
   ```

   The conformance test fails an adapter whose output differs between two runs,
   whose output cannot be written, that drops a piece of the skill without
   naming it as a loss, that names a loss for a piece the skill does not have,
   that accepts a skill nobody read, or whose `Import` does not return the skill
   less its named losses.
3. **One registry line.** Add `MyFormat{}` to the list in `Builtins` in
   `exportfmt/builtin.go`. To add a format in your own program instead, pass it
   to `exportfmt.NewRegistry` beside `exportfmt.Builtins()`, as
   `examples/adapter/plaintext_test.go` does. To ship a format with this
   repository, also add a sample under `examples/export/<name>/`
   (`go test ./exportfmt -run TestSamples -update-samples` writes it); the
   sample's test looks the format up in the registry, so it fails when the
   adapter is taken out.
4. **A section here**, under "The built-in formats": what it writes, what it
   reads back, and what you are not sure of about the target.

### A worked example

`examples/adapter/plaintext.go` is one Go file, and `plaintext_test.go` is its
test: an adapter that writes `<name>.txt`, the description and the body as plain
text, and names every other piece as a loss. Copy the pair and change `Export`.

### Rules for `Export`

- Deterministic: the same skill gives the same bytes. Sort anything that came
  from a map (`Skill.MetadataKeys`, `Skill.ExtraKeys` are sorted for you).
- No disk, no network, no clock, no random number.
- Every piece of the skill is either in the output or in the losses. When in
  doubt, name the loss: a loss is a sentence, a silent drop is a defect.
- Refuse, with an error that is one sentence, a skill you cannot write; do not
  write something that looks like the format and is not.
