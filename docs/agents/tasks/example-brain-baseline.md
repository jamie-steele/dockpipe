# TASK-006 Example Brain Baseline

## Current State

- The DorkPipe package owns a consumer-repository baseline under
  `packages/dorkpipe/resolvers/dorkpipe/assets/docs/example-brain/`.
- `example.brain` seeds equivalent baseline guidance before its workers and materializes a repository
  index, source-of-truth guidance, repository knowledge, and indexed open gaps. Its prompts already
  define source precedence, conflict handling, repo-native wording, and bounded source roots.
- Prompt and verifier guidance discourage execution-only terminology, but the shared baseline is not
  yet the default seed for every native guidance workflow. Durable output also has no deterministic
  implementation-side rejection or normalization policy for execution-only paths and labels.

## Still Open

- Make the package-owned baseline a reusable default for native guidance workflows instead of
  duplicating equivalent literals in individual prompts.
- Choose and document the deterministic durable-output policy: reject ambiguous execution-only
  references fail-closed, or rewrite only references backed by explicit source mappings.
- Add focused fixtures for runtime mount labels such as `/work` and `/DesignNotes`, artifact/lane/
  provider terminology, valid repo-native references, and ambiguous references that must be rejected.
- Reconcile TASK-007 only after the shared baseline and durable-output policy are proven.
