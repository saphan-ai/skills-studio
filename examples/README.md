# Examples

- `valid/` — skills `skills verify` accepts. `read-files` and `read-git` are two
  real skills; `HASHES.txt` states the canonical hash beside each, and a test
  checks the verifier prints the same value.
- `invalid/<rule>/<skill>/` — one skill per rule, refused by that rule and no
  other. The rule is the directory name, in lowercase.
- `signing/` — a demonstration of `--signature` and `--pubkey`:

  ```
  skills verify examples/valid/read-files \
    --signature examples/signing/read-files.sig.b64 \
    --pubkey examples/signing/demo-public-key.b64
  ```

  The key is a **demonstration key whose seed is in the test source**. A
  signature made with it proves nothing about who signed. Do not sign a real
  skill with it.
