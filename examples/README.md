# Examples

- `valid/` — skills `skills verify` accepts. `read-files` and `read-git` are two
  real skills; `HASHES.txt` states the canonical hash beside each, and a test
  checks the verifier prints the same value.
- `invalid/<rule>/<skill>/` — one skill per rule, refused by that rule and no
  other. The rule is the directory name, in lowercase.
- `agentskills/` — skills in the shape published Agent Skills have: a `scripts/`
  directory, a `LICENSE.txt`, a description over several lines, keys the
  profile does not define, a name outside `a-z`. `skills verify` reads each as
  level 1 (valid Agent Skills) and prints every rule of the Saphan profile that
  keeps it out of level 2; it exits 3. A test holds each list to its exact rules.
  The text of every example was written for this repository.
- `export/<format>/` — what `skills export examples/valid/all-fields --format <format>`
  writes; a test compares each with its adapter. `adapter/` is a worked example of
  a format anyone can add: one Go file and its test (see `docs/ADAPTERS.md`).
- `signing/` — a demonstration of `--signature` and `--pubkey`:

  ```
  skills verify examples/valid/read-files \
    --signature examples/signing/read-files.sig.b64 \
    --pubkey examples/signing/demo-public-key.b64
  ```

  The key is a **demonstration key whose seed is in the test source**. A
  signature made with it proves nothing about who signed. Do not sign a real
  skill with it.
