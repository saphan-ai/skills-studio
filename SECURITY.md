# Security policy

## Reporting a vulnerability

Please report privately, not in a public issue. Use GitHub's private
vulnerability reporting for this repository: open the **Security** tab of
`saphan-ai/skills-studio`, choose **Report a vulnerability**, and describe what
you found.

Include the version or commit you tested, the smallest skill directory or
command that shows the problem, what you expected and what happened.

We acknowledge a report within 5 working days and tell you what we will do about
it. We ask that you give us a reasonable time to fix a problem before you
publish it, and we credit reporters who want credit.

## What is in scope

The specification, the JSON Schema and `skills verify`. In particular, report:

- a skill directory `skills verify` accepts although the specification refuses it
  (for example a `scripts/` entry, a symbolic link or an unlisted file that gets
  through);
- two different canonical texts that have the same canonical hash, or one
  canonical text that gets two hashes;
- a signature that verifies when it should not, or a verification that reaches
  the network;
- a crash, a hang or unbounded memory use on a crafted skill directory;
- a place where the specification says one thing and the schema or verifier
  another.

## What is not in scope

- What a skill instructs an agent to do. This repository checks the form of a
  skill and a signature over its text; it makes no statement about behaviour.
- The Skills Studio engine, which is not in this repository.
- A key that its holder lost or leaked: the verifier checks a signature against
  the key it is given.

## Supported versions

The latest commit on `main`, and the latest tagged release when releases exist.
