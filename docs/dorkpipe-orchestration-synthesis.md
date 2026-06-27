# DorkPipe Orchestration Synthesis

## Task Summaries

- `repo_shape` (ollama): Live Ollama worker output captured in response.md
- `package_contracts` (ollama): Live Ollama worker output captured in response.md
- `safety_model` (ollama): Live Ollama worker output captured in response.md

## Planning Scouts

- `contract_brain` (ollama): Live Ollama worker output captured in response.md
- `workflow_brain` (ollama): Live Ollama worker output captured in response.md
- `planner_brain` (ollama): Live Ollama worker output captured in response.md

## Worker Outputs

### repo_shape

### repo_shape
#### DorkPipe Orchestration Primitive Strength

The orchestration contract is stronger than generic agent branding due to its focus on bounded task artifacts, normalized worker result artifacts, package-owned model lane catalog, and explicit approval before apply/publish.

#### Three Bullet Summary

- The orchestration primitive preserves task graph metadata, ensuring that downstream workers can accurately reconstruct the task graph.
- The primitive ensures that packages/agent own request tasks, prompts, and concurrency, providing a clear understanding of what artifacts are owned and how they should be applied.
- The contract verifies preservation of verification facts, training/exploration hints, approval processes, and explicit budgets for cloud-backed model lanes, ensuring that safety and governance are maintained.

### package_contracts

Here are three terse markdown bullets about the contract primitives downstream workers must preserve:

* packages/agent owns request tasks, prompts, concurrency, apply outputs
* packages/dorkpipe owns artifacts, lanes, merge, verify, budget, halt signals
* Boundary rule: Workflows declare what while DorkPipe materializes and governs the artifact contract; uncertainties remain

### safety_model

Here is a markdown summary of the verification and approval needs:

Verification and Approval Needs
=====================================

### Verification Boundary

* `verify/result.json` must contain accurate task verification results.
* Budget halt signals in `halt.json` are correctly recorded.

### Approval Process

* Explicit approval before apply/publish is enforced through the orchestration contract.
* Cloud-backed lanes must remain behind budget/halt policy.

### Uncertainty

* Model lane availability checks and training metadata can introduce uncertainty.
* Lane scores and confidence values should be cited in the final artifact.

