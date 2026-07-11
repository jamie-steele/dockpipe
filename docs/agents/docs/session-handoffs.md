# Session Handoffs

At a completed slice:

1. Report the completed scope, checks, risks, and generated artifacts.
2. Ask whether the user wants the current branch committed. Commit only after explicit approval; never commit/push by default.
3. If work remains, offer one compact next-slice prompt. Otherwise ask what to do next.

## Next-slice prompt

Write a copy/paste-ready continuation request, not a status sentence or link-only summary. It must state:

- requested outcome and exact boundary;
- current evidence, completed proof, and the specific unresolved proof;
- linked task plus the smallest relevant routing docs;
- model lane, attempts already used, allowed remaining attempts, and no-fallback rule when agentic work applies;
- explicit approval/cost gate, token or budget limit, halt behavior, and whether a new cloud turn is authorized;
- artifact location/redaction expectation and focused validation command;
- explicit non-goals and safety boundaries.

Keep it compact, but concrete enough that a fresh agent can execute it without reopening the whole conversation. Do not replace these facts with generic references. For example:

> Continue `<task-id>` only: `<outcome>`. Current evidence: `<facts>`; still unproven: `<fact>`. Read `<task>` and `<focused docs>`. Model policy: `<lane/attempt state/no fallback>`; cloud work: `<approval/budget/halt>`. Write `<redacted artifact scope>` and validate with `<command>`. Do not `<non-goals>`.