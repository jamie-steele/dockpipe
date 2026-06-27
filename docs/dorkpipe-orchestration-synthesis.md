# DorkPipe Orchestration Synthesis

## Task Summaries

- `repo_shape` (ollama): Live Ollama worker output captured in response.md
- `package_contracts` (ollama): Live Ollama worker output captured in response.md
- `safety_model` (claude): Fallback worker output for safety_model

## Worker Outputs

### repo_shape

## Task Graph Artifact Primitive Explanation

The task graph artifact primitive is a stronger approach than generic agent branding because it encapsulates the orchestration logic within the artifacts themselves, rather than relying on separate agents or workflows.

**Definition**: A task graph artifact is a self-contained, directed graph representing the relationships between tasks and their dependencies. This graph can be used to define the execution order of tasks, ensuring that each task completes before its successors.

**Advantages over Generic Agent Branding**

1.  **Decoupling**: By encapsulating orchestration logic within artifacts, we decouple the worker's behavior from specific agent or workflow implementations.
2.  **Composability**: Task graph artifacts enable easy combination and reuse of workflows across different environments and workers.
3.  **Readability and Maintenance**: The task graph provides a clear and concise representation of the execution flow, making it easier to understand and maintain.

**Example**

Consider an orchestration that runs a Docker container for a web server with a specific application. Instead of defining separate agents or workflows for each task, we can create a single task graph artifact that represents the entire process:

*   `web-server-run` (task)
    *   `docker-run` (dependency)
        *   `image-pull` (dependency)
            *   `registry-auth` (dependency)

In this example, the task graph artifact defines a clear execution order: first, pull the image from the registry, then authenticate with the registry, and finally run the Docker container.

**Conclusion**

By leveraging task graph artifacts, we can create more composable, maintainable, and readable orchestration graphs. This approach provides a stronger foundation for building modular, scalable, and flexible workflows.

### package_contracts

To orchestrate a DorkPipe run for summarizing how packages/agent and packages/dorkpipe should divide workflow surface vs contract surface, we will follow these steps:

1. Load the relevant docs:
   - `docs/agents/index.yaml`
   - `docs/agents/repo-map.md`
   - `docs/agents/core-package-model.md`
2. Identify task type in `docs/agents/index.yaml`.
3. Check whether the work is engine, workflow, package, resolver, strategy, or generated artifact.
4. Determine how packages/agent and packages/dorkpipe divide workflow surface vs contract surface.

**Doc Loading**: Loaded from accessible files, including:

- docs/agents/index.yaml
- docs/agents/repo-map.md
- docs/agents/core-package-model.md

**Task Type Identification**: Task type is identified as `package_contracts` in `docs/agents/index.yaml`.

**Work Classification**: Work is classified as a workflow modification, specifically focusing on package boundaries.

**Boundary Summary**: Package boundaries should divide workflow surface and contract surface as follows:

- Workflow Surface: Reserved for general workflow management and orchestration. Packages can use this surface to interact with the engine or CLI.
- Contract Surface: Dedicated to package-specific behavior and interactions. This includes package metadata, commands, and APIs.

**Uncertainty Note**: There is some uncertainty regarding how packages/agent and packages/dorkpipe should handle specific cases, such as resolved vault templates or cache builds. Further clarification from relevant stakeholders would be beneficial.

**Expected Output**: A concise markdown summary of the package boundaries:

```markdown
Package Boundaries Summary

* Workflow Surface:
  Reserved for general workflow management and orchestration.
* Contract Surface:
  Dedicated to package-specific behavior and interactions, including metadata, commands, and APIs.
```

This output will serve as a clear starting point for further development and refinement of package boundaries in the DorkPipe orchestration graph.

### safety_model

# safety_model

Fallback worker output for provider `claude`.

- Goal: Summarize the safety and approval model for multi-worker orchestration.
- Expected output: A short markdown summary of verification and approval needs.
- Task stayed bounded and artifact-driven.

