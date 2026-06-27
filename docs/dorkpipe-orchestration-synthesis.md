# DorkPipe Orchestration Synthesis

## Task Summaries

- `contract_brain` (ollama): Live Ollama worker output captured in response.md
- `workflow_brain` (ollama): Live Ollama worker output captured in response.md
- `repo_shape` (ollama): Live Ollama worker output captured in response.md
- `package_contracts` (ollama): Live Ollama worker output captured in response.md
- `safety_model` (ollama): Live Ollama worker output captured in response.md

## Worker Outputs

### contract_brain

* DorkPipe orchestration contract must preserve bounded task artifacts that include:
  * `task_id`
  * `status`
  * `provider_requested`
  * `provider_actual`
  * `lane_id`
  * `lane_selection`
  * `used_live_model`
  * `budget_halt`
  * `estimated_input_tokens`
  * `estimated_output_tokens`
  * `estimated_total_tokens`
  * `summary`
  * `claims`
  * `artifacts`
  * `citations`
  * `confidence`
  * `issues`
  * `next_actions`

* DorkPipe orchestration contract must preserve normalized worker result artifacts that include:
  * `task_id`
  * `status`
  * `provider_requested`
  * `provider_actual`
  * `lane_id`
  * `lane_selection`
  * `used_live_model`
  * `budget_halt`
  * `estimated_input_tokens`
  * `estimated_output_tokens`
  * `estimated_total_tokens`
  * `summary`
  * `claims`
  * `artifacts`
  * `citations`
  * `confidence`
  * `issues`
  * `next_actions`

* DorkPipe orchestration contract must preserve model lane catalog artifacts that include:
  * Lane descriptions
  * Model context metadata
  * Capabilities
  * Availability checks
  * Budget policy
  * Training/exploration hints

* DorkPipe orchestration contract must preserve merge/verify primitives, including:
  * Merge and verify comparison of task outputs
  * Verify checking coverage, conflicts, and escalation risk

### workflow_brain

The workflow declares that downstream workers should:

* Preserve the orchestration contract primitives
* Own the merged result output
* Not be responsible for verification or approval over budget costs

These are the primary declaration and ownership aspects of the workflow, which will be further refined by downstream workers based on the provided guidance.

### repo_shape

# DorkPipe Orchestration Contract Primitive Guidance

The core orchestration contract primitive in DorkPipe must preserve key task graph artifacts.

### Preserving Task Graph Artifacts

DorkPipe must preserve `task_id`, `status`, and `lane_id` across all bounded task artifacts, ensuring that the downstream worker can correctly navigate the task graph.

### Preserving Worker Result Artifacts

The contract must also preserve normalized worker result artifacts, including `provider_requested`, `provider_actual`, and `budget_halt`, to accurately track task execution and provide insights into model performance.

### Model Lane Catalog Preservation

DorkPipe orchestration contracts must preserve model lane catalog artifacts, which include:

* Lane descriptions
* Model context metadata
* Capabilities
* Availability checks
* Budget policy
* Training/exploration hints

This ensures that the downstream worker can select suitable lanes for task execution and make informed decisions about resource allocation.

### Merge/Verify Primitive Preservation

The contract must preserve merge/verify primitives, including:

* Merge and verify comparison of task outputs
* Verify checking coverage, conflicts, and escalation risk

By preserving these primitives, DorkPipe ensures that downstream workers can accurately assess the quality and reliability of task outputs and make informed decisions about further processing or approval.

### Uncertainty Note

If any specific guidance or details are unclear from the referenced files, it is uncertain whether the orchestration contract primitive should be adjusted accordingly.

### package_contracts

*packages/agent owns* The workflow surface includes:
  - Request text
  - Task list artifacts (task.json)
  - Prompts
  - Concurrency settings
  - Apply outputs

*packages/dorkpipe owns* The contract surface includes:
  - Orchestration artifacts (plan, task-graph, cloud-usage)
  - Lane catalog artifacts (model-lanes/catalog.yml, model-lanes/baseline-policy.yml)
  - Worker result artifacts (result.json)
  - Merge and verify primitives
  - Budget and halt primitives

*Boundary rule*: The workflow governs the artifact contract by declaring what is owned and preserved. Downstream workers should follow these guidelines to ensure a coherent and accurate final output.

### safety_model

Here is a markdown summary of verification and approval needs:

**Safety Model Summary**

The safety model ensures that tasks are executed within approved boundaries and budgets. It verifies that:

* Task inputs are valid and meet constraints
* Output meets expected standards
* Budget is not exceeded
* Verification and approval processes are followed

**Verification Boundaries**

The safety model verifies the following:

* Task output meets task constraints
* Model lane selection aligns with task intent and availability checks
* Budget policy is respected
* Training/exploration hints are used as needed

**Approval Needs**

The safety model requires explicit approval before applying or publishing changes. This includes verifying that:

* Task inputs meet requirements
* Output meets standards
* Budget is sufficient
* Model lane selection aligns with task intent and availability checks

**Uncertainty**

There is uncertainty in the model's ability to accurately predict task outcomes. Further training and testing are required to improve model performance.

This summary should provide a concise overview of the safety model's verification and approval needs, while keeping claims grounded in referenced files.

