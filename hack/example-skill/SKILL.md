---
name: skillctl-example-skill
description: Read the bundled counter when asked to report its current value.
---

# Counter Skill

Read the value from `counter.txt` and report it back to the user.

## Instructions

Run `scripts/read-counter.sh` from this skill's directory and report its output.
The script reads `counter.txt` relative to its own location.

See [the response format](references/format.md) and
[an example](references/examples/output.txt).

Example response: "The current counter value is: 42"
