package orchestrationhelper

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestLoadTaskPackSelectsExactStepAgentDeclaration(t *testing.T) {
	root := t.TempDir()
	workflowPath := filepath.Join(root, "workflows", "software-dev", "config.yml")
	if err := os.MkdirAll(filepath.Dir(workflowPath), 0o755); err != nil {
		t.Fatal(err)
	}
	workflow := `name: repo.software-dev
steps:
  - id: software_dev_preview
    workflow: software.dev
    package: dockpipeproject
    agent:
      orchestration:
        request:
          text: Do not select this step.
  - id: software_dev
    workflow: software.dev
    package: dockpipeproject
    vars:
      IGNORED_BY_TASK_PACK_LOADER: "true"
    agent:
      startup_prompt: Apply this repo's source-of-truth rules.
      include_agents_md: true
      orchestration:
        request:
          text: Implement the bounded request.
        plan:
          goal: Produce a verified repo change.
        tasks: []
`
	if err := os.WriteFile(workflowPath, []byte(workflow), 0o644); err != nil {
		t.Fatal(err)
	}

	agent, err := loadTaskPack(root, "workflows/software-dev/config.yml", "software_dev")
	if err != nil {
		t.Fatal(err)
	}
	if got := stringValue(agent["startup_prompt"]); got != "Apply this repo's source-of-truth rules." {
		t.Fatalf("startup_prompt = %q", got)
	}
	orchestration := mapValue(agent["orchestration"])
	if got := stringValue(mapValue(orchestration["request"])["text"]); got != "Implement the bounded request." {
		t.Fatalf("request.text = %q", got)
	}
	for _, excluded := range []string{"id", "workflow", "package", "vars"} {
		if _, ok := agent[excluded]; ok {
			t.Fatalf("selected task-pack agent unexpectedly includes step field %q", excluded)
		}
	}
}

func TestLoadTaskPackRejectsInvalidPathOrStepIdentity(t *testing.T) {
	root := t.TempDir()
	workflowPath := filepath.Join(root, "workflows", "task-pack.yml")
	if err := os.MkdirAll(filepath.Dir(workflowPath), 0o755); err != nil {
		t.Fatal(err)
	}
	workflow := `name: repo.task-pack
steps:
  - id: valid
    agent:
      orchestration:
        request:
          text: Valid task pack.
  - id: duplicate
    agent:
      orchestration: {}
  - id: duplicate
    agent:
      orchestration: {}
  - id: missing_orchestration
    agent:
      startup_prompt: Incomplete declaration.
`
	if err := os.WriteFile(workflowPath, []byte(workflow), 0o644); err != nil {
		t.Fatal(err)
	}
	outsidePath := filepath.Join(filepath.Dir(root), "outside-task-pack.yml")

	tests := []struct {
		name    string
		path    string
		stepID  string
		wantErr string
	}{
		{name: "empty path", path: "", stepID: "valid", wantErr: "task pack path is required"},
		{name: "missing path", path: "workflows/missing.yml", stepID: "valid", wantErr: `task pack path "workflows/missing.yml" does not exist`},
		{name: "absolute path", path: outsidePath, stepID: "valid", wantErr: fmt.Sprintf("task pack path %q must be relative to the consumer repo", outsidePath)},
		{name: "escaping path", path: "../outside-task-pack.yml", stepID: "valid", wantErr: `task pack path "../outside-task-pack.yml" escapes the consumer repo`},
		{name: "missing step id", path: "workflows/task-pack.yml", stepID: "", wantErr: `task pack step id is required for "workflows/task-pack.yml"`},
		{name: "unknown step id", path: "workflows/task-pack.yml", stepID: "unknown", wantErr: `workflows/task-pack.yml: task pack step id "unknown" was not found`},
		{name: "ambiguous step id", path: "workflows/task-pack.yml", stepID: "duplicate", wantErr: `workflows/task-pack.yml: task pack step id "duplicate" is ambiguous (2 matches)`},
		{name: "missing orchestration", path: "workflows/task-pack.yml", stepID: "missing_orchestration", wantErr: `workflows/task-pack.yml: task pack step id "missing_orchestration" has no agent.orchestration declaration`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := loadTaskPack(root, tt.path, tt.stepID)
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("loadTaskPack() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestNormalizeTaskPackContractPrecedenceAndValidGraphs(t *testing.T) {
	tests := []struct {
		name            string
		proposal        func() map[string]any
		prepare         func(map[string]any, map[string]any, map[string]any)
		wantPrompt      string
		wantGoal        string
		wantConstraints []string
		wantFloors      []string
		wantRoles       []string
		wantTasks       []string
		wantRead        []string
		wantBudget      int
	}{
		{
			name:            "static repo graph",
			wantPrompt:      "repo prompt",
			wantGoal:        "repo goal",
			wantConstraints: []string{"package rule", "shared rule", "repo rule"},
			wantFloors:      []string{"base.md", "repo.md"},
			wantRoles:       []string{"base", "repo_role", "zeta"},
			wantTasks:       []string{"repo_write", "repo_verify"},
			wantRead:        []string{"/work/src"},
			wantBudget:      300,
		},
		{
			name:            "proposal selected graph",
			proposal:        validContractProposal,
			wantPrompt:      "proposal prompt",
			wantGoal:        "proposal goal",
			wantConstraints: []string{"package rule", "shared rule", "repo rule", "proposal rule"},
			wantFloors:      []string{"base.md", "repo.md", "extra.md"},
			wantRoles:       []string{"aaa", "base", "repo_role", "zeta"},
			wantTasks:       []string{"proposal_write", "proposal_verify"},
			wantRead:        []string{"/work/src/pkg"},
			wantBudget:      250,
		},
		{
			name: "package seed fallback",
			prepare: func(_ map[string]any, repo, _ map[string]any) {
				delete(contractOrchestration(repo), "tasks")
				contractApply(repo)["required_artifacts"] = []any{}
			},
			wantPrompt:      "repo prompt",
			wantGoal:        "repo goal",
			wantConstraints: []string{"package rule", "shared rule", "repo rule"},
			wantFloors:      []string{"base.md"},
			wantRoles:       []string{"base", "repo_role", "zeta"},
			wantTasks:       []string{"package_seed"},
			wantRead:        []string{"/work/src"},
			wantBudget:      300,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			packageDefaults := validContractPackageDefaults()
			repo := validContractRepoTaskPack()
			proposal := map[string]any(nil)
			if tt.proposal != nil {
				proposal = tt.proposal()
			}
			if tt.prepare != nil {
				tt.prepare(packageDefaults, repo, proposal)
			}
			packageBefore := mustJSON(packageDefaults, nil)
			repoBefore := mustJSON(repo, nil)
			proposalBefore := mustJSON(proposal, nil)

			normalized, err := normalizeTaskPackContract(packageDefaults, repo, proposal)
			if err != nil {
				t.Fatal(err)
			}
			if got := stringValue(normalized["startup_prompt"]); got != tt.wantPrompt {
				t.Fatalf("startup_prompt = %q, want %q", got, tt.wantPrompt)
			}
			if got := stringValue(mapValue(normalized["plan"])["goal"]); got != tt.wantGoal {
				t.Fatalf("plan.goal = %q, want %q", got, tt.wantGoal)
			}
			if got := stringValue(mapValue(normalized["request"])["text"]); got != strings.Replace(tt.wantGoal, "goal", "request", 1) {
				t.Fatalf("request.text = %q", got)
			}
			assertStringOrder(t, "constraints", stringList(normalized["constraints"]), tt.wantConstraints)
			assertStringOrder(t, "required_outputs", stringList(normalized["required_outputs"]), tt.wantFloors)
			assertStringOrder(t, "access.read", stringList(mapValue(normalized["access"])["read"]), tt.wantRead)
			wantDeny := []string{"/work/.git", "/work/secrets", "/work/generated"}
			if tt.proposal != nil {
				wantDeny = append(wantDeny, "/work/src/tmp")
			}
			assertStringOrder(t, "access.deny", stringList(mapValue(normalized["access"])["deny"]), wantDeny)
			if got := intFromAny(mapValue(normalized["cloud_budget"])["max_task_tokens"]); got != tt.wantBudget {
				t.Fatalf("cloud_budget.max_task_tokens = %d, want %d", got, tt.wantBudget)
			}
			if !boolAny(normalized["approval_required"]) || boolAny(normalized["publish"]) || boolAny(normalized["sync"]) {
				t.Fatalf("hard authority = approval:%v publish:%v sync:%v", normalized["approval_required"], normalized["publish"], normalized["sync"])
			}
			if got := stringValue(normalized["apply_target"]); got != "docs/generated" {
				t.Fatalf("apply_target = %q, want repo-selected target", got)
			}
			assertStringOrder(t, "roles", contractIDs(listValue(normalized["roles"])), tt.wantRoles)
			assertStringOrder(t, "tasks", contractIDs(listValue(normalized["tasks"])), tt.wantTasks)

			baseRole := contractItemByID(listValue(normalized["roles"]), "base")
			wantRole := "repo base"
			wantRoleConstraints := []string{"package role rule", "repo role rule"}
			if tt.proposal != nil {
				wantRole = "proposal base"
				wantRoleConstraints = append(wantRoleConstraints, "proposal role rule")
			}
			if got := stringValue(baseRole["role"]); got != wantRole {
				t.Fatalf("base role = %q, want %q", got, wantRole)
			}
			assertStringOrder(t, "base role constraints", stringList(baseRole["constraints"]), wantRoleConstraints)

			if got := mustJSON(packageDefaults, nil); got != packageBefore {
				t.Fatal("package defaults were mutated")
			}
			if got := mustJSON(repo, nil); got != repoBefore {
				t.Fatal("repo task pack was mutated")
			}
			if got := mustJSON(proposal, nil); got != proposalBefore {
				t.Fatal("proposal was mutated")
			}
		})
	}
}

func TestNormalizeTaskPackContractRejectsAuthorityWidening(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(map[string]any, map[string]any, map[string]any)
		wantErr string
	}{
		{
			name: "read outside package ceiling",
			mutate: func(_ map[string]any, repo, _ map[string]any) {
				repo["access"] = map[string]any{"read": []any{"/etc"}}
			},
			wantErr: `repo.access.read path "/etc" is outside the current package authority ceiling`,
		},
		{
			name: "write outside package ceiling",
			mutate: func(_ map[string]any, repo, _ map[string]any) {
				repo["access"] = map[string]any{"write": []any{"/DesignNotes"}}
			},
			wantErr: `repo.access.write path "/DesignNotes" is outside the current package authority ceiling`,
		},
		{
			name: "proposal role access widens narrowed run authority",
			mutate: func(_ map[string]any, _ map[string]any, proposal map[string]any) {
				mapValue(mapValue(contractOrchestration(proposal)["agents"])["aaa"])["access"] = map[string]any{"read": []any{"/work/other"}}
			},
			wantErr: `proposal.roles["aaa"].access.read path "/work/other" is outside the current package authority ceiling`,
		},
		{
			name: "remove package deny rule",
			mutate: func(_ map[string]any, repo, _ map[string]any) {
				repo["access"] = map[string]any{"remove_deny": []any{"/work/.git"}}
			},
			wantErr: `repo.access.remove_deny cannot remove package deny rule "/work/.git"`,
		},
		{
			name: "cloud budget above package ceiling",
			mutate: func(_ map[string]any, repo, _ map[string]any) {
				repo["cloud_budget"] = map[string]any{"max_total_tokens": 1001}
			},
			wantErr: `repo.cloud_budget.max_total_tokens value 1001 exceeds package ceiling 1000`,
		},
		{
			name: "task budget above package ceiling",
			mutate: func(_ map[string]any, repo, proposal map[string]any) {
				for key := range proposal {
					delete(proposal, key)
				}
				mapValue(listValue(contractOrchestration(repo)["tasks"])[0])["max_cloud_tokens"] = 401
			},
			wantErr: `repo.tasks["repo_write"].max_cloud_tokens value 401 exceeds package ceiling 300`,
		},
		{
			name: "selected graph exceeds task count ceiling",
			mutate: func(_ map[string]any, repo, proposal map[string]any) {
				for key := range proposal {
					delete(proposal, key)
				}
				repo["cloud_budget"] = map[string]any{"max_tasks": 1}
			},
			wantErr: `repo.tasks count 2 exceeds cloud_budget.max_tasks ceiling 1`,
		},
		{
			name: "disable mandatory approval",
			mutate: func(_ map[string]any, repo, _ map[string]any) {
				contractApply(repo)["require_approval"] = false
			},
			wantErr: `repo.apply.require_approval cannot disable mandatory package approval`,
		},
		{
			name:    "enable publish from task pack",
			mutate:  func(_ map[string]any, repo, _ map[string]any) { repo["publish"] = true },
			wantErr: `repo.publish cannot enable package-owned publish`,
		},
		{
			name:    "enable sync from proposal",
			mutate:  func(_ map[string]any, _ map[string]any, proposal map[string]any) { proposal["sync"] = true },
			wantErr: `proposal.sync cannot enable package-owned sync`,
		},
		{
			name: "change repo apply target",
			mutate: func(_ map[string]any, _ map[string]any, proposal map[string]any) {
				contractApply(proposal)["target_root"] = "elsewhere"
			},
			wantErr: `proposal.apply.target_root "elsewhere" cannot change repo-selected target "docs/generated"`,
		},
		{
			name: "rename required output floor",
			mutate: func(_ map[string]any, _ map[string]any, proposal map[string]any) {
				contractApply(proposal)["required_artifacts"] = []any{"renamed.md", "repo.md", "extra.md"}
			},
			wantErr: `proposal.apply.required_artifacts cannot remove or rename required output "base.md"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			packageDefaults := validContractPackageDefaults()
			repo := validContractRepoTaskPack()
			proposal := validContractProposal()
			tt.mutate(packageDefaults, repo, proposal)
			_, err := normalizeTaskPackContract(packageDefaults, repo, proposal)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("normalizeTaskPackContract() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestNormalizeTaskPackContractRejectsInvalidGraphsDeterministically(t *testing.T) {
	tests := []struct {
		name    string
		tasks   []any
		wantErr string
	}{
		{
			name: "duplicate task ids",
			tasks: []any{
				contractTask("dup", nil, "base.md"),
				contractTask("dup", nil, "repo.md"),
			},
			wantErr: `tasks["dup"].id is duplicate`,
		},
		{
			name: "missing dependency",
			tasks: []any{
				contractTask("a", []string{"missing"}, "base.md", "repo.md"),
			},
			wantErr: `tasks["a"].depends_on dependency "missing" does not exist`,
		},
		{
			name: "dependency cycle",
			tasks: []any{
				contractTask("a", []string{"b"}, "base.md"),
				contractTask("b", []string{"a"}, "repo.md"),
			},
			wantErr: `tasks["b"].depends_on dependency "a" creates cycle a -> b -> a`,
		},
		{
			name: "duplicate output producers",
			tasks: []any{
				contractTask("a", nil, "base.md", "repo.md"),
				contractTask("b", nil, "repo.md"),
			},
			wantErr: `materialized output "repo.md" has duplicate producers tasks["a"] and tasks["b"]`,
		},
		{
			name: "required floor without producer",
			tasks: []any{
				contractTask("a", nil, "base.md"),
			},
			wantErr: `required output floor "repo.md" has no producer`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			packageDefaults := validContractPackageDefaults()
			repo := validContractRepoTaskPack()
			contractOrchestration(repo)["tasks"] = tt.tasks
			_, firstErr := normalizeTaskPackContract(packageDefaults, repo, nil)
			_, secondErr := normalizeTaskPackContract(packageDefaults, repo, nil)
			if firstErr == nil || !strings.Contains(firstErr.Error(), tt.wantErr) {
				t.Fatalf("normalizeTaskPackContract() error = %v, want %q", firstErr, tt.wantErr)
			}
			if secondErr == nil || firstErr.Error() != secondErr.Error() {
				t.Fatalf("errors are not deterministic:\nfirst:  %v\nsecond: %v", firstErr, secondErr)
			}
		})
	}
}

func TestParsePlannerProposalAcceptsStructuredJSONAndYAML(t *testing.T) {
	tests := []struct {
		name       string
		raw        string
		wantFormat string
	}{
		{
			name:       "json",
			raw:        mustJSON(validExecutableProposal(), nil),
			wantFormat: "json",
		},
		{
			name: "yaml",
			raw: `startup_prompt: proposal prompt
constraints:
  - proposal rule
orchestration:
  agents:
    base:
      role: proposal base
      constraints:
        - proposal role rule
  tasks:
    - id: proposal_write
      agent: base
      goal: Write the bounded output.
      brief: Preserve the repository contract.
      context:
        required_artifacts:
          - shared/request.md
        seed_paths:
          - packages/dorkpipe
        source_roots:
          - packages/dorkpipe
      constraints:
        - Do not widen authority.
      expected_output: A reviewable artifact bundle.
      max_cloud_tokens: 200
      depends_on: []
      materialize_outputs:
        - id: base
          path: base.md
`,
			wantFormat: "yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proposal, err := parsePlannerProposal([]byte(tt.raw))
			if err != nil {
				t.Fatal(err)
			}
			if proposal.Format != tt.wantFormat {
				t.Fatalf("format = %q, want %q", proposal.Format, tt.wantFormat)
			}
			tasks := listValue(contractOrchestration(proposal.Declaration)["tasks"])
			if len(tasks) == 0 {
				t.Fatal("parsed proposal has no tasks")
			}
			first := mapValue(tasks[0])
			if stringValue(first["id"]) != "proposal_write" || stringValue(first["agent"]) != "base" {
				t.Fatalf("first task identity = %#v", first)
			}
			if tt.name == "yaml" {
				assertStringOrder(t, "context.required_artifacts", stringList(mapValue(first["context"])["required_artifacts"]), []string{"shared/request.md"})
				assertStringOrder(t, "materialize_outputs", []string{stringValue(mapValue(listValue(first["materialize_outputs"])[0])["path"])}, []string{"base.md"})
			}
		})
	}
}

func TestParsePlannerProposalRejectsUnstructuredOrInvalidInput(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr string
	}{
		{name: "empty", raw: " \n\t", wantErr: "planner proposal is empty"},
		{name: "malformed json", raw: `{`, wantErr: "planner proposal is not valid JSON or YAML"},
		{name: "malformed yaml", raw: "orchestration:\n  tasks: [", wantErr: "planner proposal is not valid JSON or YAML"},
		{name: "wrong root", raw: "- id: task\n", wantErr: "planner proposal root must be an object"},
		{name: "narrated", raw: "Here is the proposal:\n  orchestration:\n    tasks:\n      - id: task\n", wantErr: `planner proposal root field "Here is the proposal" is not allowed`},
		{name: "fenced", raw: "```yaml\norchestration:\n  tasks:\n    - id: task\n```\n", wantErr: "planner proposal is not valid JSON or YAML"},
		{name: "multiple documents", raw: "orchestration:\n  tasks:\n    - id: first\n---\norchestration:\n  tasks:\n    - id: second\n", wantErr: "planner proposal must contain exactly one structured document"},
		{name: "duplicate keys", raw: "orchestration:\n  tasks:\n    - id: first\n  tasks:\n    - id: second\n", wantErr: "planner proposal is not valid JSON or YAML"},
		{name: "missing orchestration", raw: "constraints: [bounded]\n", wantErr: "planner proposal.orchestration is required"},
		{name: "wrong task root", raw: "orchestration:\n  tasks: task\n", wantErr: "planner proposal.orchestration.tasks must be an array"},
		{name: "empty tasks", raw: "orchestration:\n  tasks: []\n", wantErr: "planner proposal.orchestration.tasks must contain at least one task"},
		{name: "missing task id", raw: "orchestration:\n  tasks:\n    - goal: missing id\n", wantErr: "planner proposal.orchestration.tasks[0].id is required"},
		{name: "wrong dependencies", raw: "orchestration:\n  tasks:\n    - id: task\n      depends_on: other\n", wantErr: "planner proposal.orchestration.tasks[0].depends_on must be an array of strings"},
		{name: "wrong context", raw: "orchestration:\n  tasks:\n    - id: task\n      context: source.md\n", wantErr: "planner proposal.orchestration.tasks[0].context must be an object"},
		{name: "missing output path", raw: "orchestration:\n  tasks:\n    - id: task\n      materialize_outputs:\n        - id: output\n", wantErr: "planner proposal.orchestration.tasks[0].materialize_outputs[0].path is required"},
		{name: "unknown task field", raw: "orchestration:\n  tasks:\n    - id: task\n      publish: true\n", wantErr: `planner proposal.orchestration.tasks[0] field "publish" is not allowed`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proposal, err := parsePlannerProposal([]byte(tt.raw))
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("parsePlannerProposal() = %#v, %v, want %q", proposal, err, tt.wantErr)
			}
			if proposal != nil {
				t.Fatalf("parse failure returned partial proposal: %#v", proposal)
			}
		})
	}
}

func TestCompileExecutableContractBuildsDeterministicArtifacts(t *testing.T) {
	tests := []struct {
		name         string
		prepare      func(map[string]any, map[string]any) *parsedPlannerProposal
		wantTaskIDs  []string
		wantProposal bool
	}{
		{
			name:        "static repo graph",
			wantTaskIDs: []string{"repo_write", "repo_verify"},
		},
		{
			name: "proposal selected graph",
			prepare: func(_ map[string]any, _ map[string]any) *parsedPlannerProposal {
				proposal, err := parsePlannerProposal([]byte(mustJSON(validExecutableProposal(), nil)))
				if err != nil {
					t.Fatal(err)
				}
				return proposal
			},
			wantTaskIDs:  []string{"proposal_write", "proposal_verify"},
			wantProposal: true,
		},
		{
			name: "package seed fallback",
			prepare: func(_ map[string]any, repo map[string]any) *parsedPlannerProposal {
				delete(contractOrchestration(repo), "tasks")
				contractApply(repo)["required_artifacts"] = []any{}
				return nil
			},
			wantTaskIDs: []string{"package_seed"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			packageDefaults := validContractPackageDefaults()
			repo := validContractRepoTaskPack()
			addExecutableTaskFields(packageDefaults, repo)
			var proposal *parsedPlannerProposal
			if tt.prepare != nil {
				proposal = tt.prepare(packageDefaults, repo)
			}

			compiled, err := compileExecutableContract(packageDefaults, repo, proposal)
			if err != nil {
				t.Fatal(err)
			}
			assertStringOrder(t, "graph tasks", contractIDs(listValue(compiled.TaskGraph["tasks"])), tt.wantTaskIDs)
			assertStringOrder(t, "task artifacts", contractIDs(compiled.TaskArtifacts), tt.wantTaskIDs)
			if got := boolAny(compiled.ProposalMetadata["present"]); got != tt.wantProposal {
				t.Fatalf("proposal metadata present = %v, want %v", got, tt.wantProposal)
			}
			if boolAny(compiled.Plan["publish"]) || boolAny(compiled.Plan["sync"]) || !boolAny(compiled.Plan["approval_required"]) {
				t.Fatalf("compiled hard authority = approval:%v publish:%v sync:%v", compiled.Plan["approval_required"], compiled.Plan["publish"], compiled.Plan["sync"])
			}

			if tt.wantProposal {
				first := mapValue(compiled.TaskArtifacts[0])
				if stringValue(first["agent"]) != "base" || stringValue(first["goal"]) != "proposal goal for writer" || stringValue(first["brief"]) != "proposal writer brief" {
					t.Fatalf("compiled proposal task fields = %#v", first)
				}
				assertStringOrder(t, "compiled dependencies", stringList(mapValue(compiled.TaskArtifacts[1])["depends_on"]), []string{"proposal_write"})
				assertStringOrder(t, "compiled context", stringList(mapValue(first["context"])["required_artifacts"]), []string{"shared/request.md", "shared/repo.md"})
				assertStringOrder(t, "compiled constraints", stringList(first["constraints"]), []string{"package role rule", "repo role rule", "proposal role rule", "writer constraint", "shared task constraint"})
				assertStringOrder(t, "compiled outputs", []string{
					stringValue(mapValue(listValue(first["materialize_outputs"])[0])["path"]),
					stringValue(mapValue(listValue(first["materialize_outputs"])[1])["path"]),
					stringValue(mapValue(listValue(first["materialize_outputs"])[2])["path"]),
				}, []string{"base.md", "repo.md", "extra.md"})
				if intFromAny(first["max_cloud_tokens"]) != 200 || stringValue(first["expected_output"]) != "proposal writer output" {
					t.Fatalf("compiled output contract = %#v", first)
				}
				assertStringOrder(t, "proposal role metadata", stringList(compiled.ProposalMetadata["role_ids"]), []string{"aaa", "base"})
				assertStringOrder(t, "proposal task metadata", stringList(compiled.ProposalMetadata["task_ids"]), tt.wantTaskIDs)
			}

			second, err := compileExecutableContract(packageDefaults, repo, proposal)
			if err != nil {
				t.Fatal(err)
			}
			if mustJSON(compiled, nil) != mustJSON(second, nil) {
				t.Fatalf("compiled output is not deterministic:\nfirst:  %s\nsecond: %s", mustJSON(compiled, nil), mustJSON(second, nil))
			}
		})
	}
}

func TestMaterializeSoftwareDevContractWritesDeterministicRuntimeLayout(t *testing.T) {
	packageDefaults := validContractPackageDefaults()
	repo := validContractRepoTaskPack()
	addExecutableTaskFields(packageDefaults, repo)
	compiled, err := compileExecutableContract(packageDefaults, repo, nil)
	if err != nil {
		t.Fatal(err)
	}

	firstRoot := t.TempDir()
	secondRoot := t.TempDir()
	if err := materializeSoftwareDevContract(firstRoot, firstRoot, compiled, nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := materializeSoftwareDevContract(secondRoot, secondRoot, compiled, nil, nil); err != nil {
		t.Fatal(err)
	}
	for _, relativePath := range []string{
		"request.json",
		"plan.json",
		"task-graph.json",
		"proposal/metadata.json",
		"tasks/repo_write/task.json",
		"tasks/repo_write/prompt.md",
		"tasks/repo_verify/task.json",
		"tasks/repo_verify/prompt.md",
	} {
		first, err := os.ReadFile(filepath.Join(firstRoot, filepath.FromSlash(relativePath)))
		if err != nil {
			t.Fatalf("read first %s: %v", relativePath, err)
		}
		second, err := os.ReadFile(filepath.Join(secondRoot, filepath.FromSlash(relativePath)))
		if err != nil {
			t.Fatalf("read second %s: %v", relativePath, err)
		}
		if !bytes.Equal(first, second) {
			t.Fatalf("materialized artifact %s is not deterministic", relativePath)
		}
	}

	task := readJSONMap(filepath.Join(firstRoot, "tasks", "repo_write", "task.json"))
	if !boolAny(mapValue(task["lane"])["available"]) || stringValue(task["resolver_hint"]) == "" {
		t.Fatalf("compiled runtime task lacks an executable lane: %#v", task)
	}
	promptRaw, err := os.ReadFile(filepath.Join(firstRoot, "tasks", "repo_write", "prompt.md"))
	if err != nil {
		t.Fatal(err)
	}
	prompt := string(promptRaw)
	if !strings.Contains(prompt, "Execute only this compiled task") || !strings.Contains(prompt, "dorkpipe:file") {
		t.Fatalf("compiled runtime prompt lacks bounded execution or materialized-output contract:\n%s", prompt)
	}
}

func TestCompileExecutableContractRejectsInvalidContractsWithoutPartialArtifacts(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(map[string]any, map[string]any, *parsedPlannerProposal)
		wantErr string
	}{
		{
			name: "unknown role reference",
			mutate: func(_ map[string]any, _ map[string]any, proposal *parsedPlannerProposal) {
				mapValue(listValue(contractOrchestration(proposal.Declaration)["tasks"])[0])["agent"] = "missing_role"
			},
			wantErr: `proposal.tasks["proposal_write"].agent references unknown normalized role "missing_role"`,
		},
		{
			name: "invalid dependency",
			mutate: func(_ map[string]any, _ map[string]any, proposal *parsedPlannerProposal) {
				mapValue(listValue(contractOrchestration(proposal.Declaration)["tasks"])[0])["depends_on"] = []any{"missing"}
			},
			wantErr: `tasks["proposal_write"].depends_on dependency "missing" does not exist`,
		},
		{
			name: "authority widening",
			mutate: func(_ map[string]any, _ map[string]any, proposal *parsedPlannerProposal) {
				proposal.Declaration["access"] = map[string]any{"read": []any{"/etc"}}
			},
			wantErr: `proposal.access.read path "/etc" is outside the current package authority ceiling`,
		},
		{
			name: "output floor",
			mutate: func(_ map[string]any, _ map[string]any, proposal *parsedPlannerProposal) {
				first := mapValue(listValue(contractOrchestration(proposal.Declaration)["tasks"])[0])
				first["materialize_outputs"] = []any{map[string]any{"path": "extra.md"}}
			},
			wantErr: `required output floor "base.md" has no producer`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			packageDefaults := validContractPackageDefaults()
			repo := validContractRepoTaskPack()
			addExecutableTaskFields(packageDefaults, repo)
			proposal, err := parsePlannerProposal([]byte(mustJSON(validExecutableProposal(), nil)))
			if err != nil {
				t.Fatal(err)
			}
			tt.mutate(packageDefaults, repo, proposal)

			compiled, firstErr := compileExecutableContract(packageDefaults, repo, proposal)
			second, secondErr := compileExecutableContract(packageDefaults, repo, proposal)
			if firstErr == nil || !strings.Contains(firstErr.Error(), tt.wantErr) {
				t.Fatalf("compileExecutableContract() = %#v, %v, want %q", compiled, firstErr, tt.wantErr)
			}
			if compiled != nil {
				t.Fatalf("compile failure returned partial executable artifacts: %#v", compiled)
			}
			if second != nil || secondErr == nil || firstErr.Error() != secondErr.Error() {
				t.Fatalf("compile errors are not deterministic:\nfirst:  %#v, %v\nsecond: %#v, %v", compiled, firstErr, second, secondErr)
			}
		})
	}
}

func validExecutableProposal() map[string]any {
	proposal := validContractProposal()
	tasks := listValue(contractOrchestration(proposal)["tasks"])
	writer := mapValue(tasks[0])
	writer["agent"] = "base"
	writer["goal"] = "proposal goal for writer"
	writer["brief"] = "proposal writer brief"
	writer["context"] = map[string]any{
		"required_artifacts": []any{"shared/request.md", "shared/repo.md"},
		"seed_paths":         []any{"packages/dorkpipe"},
		"source_roots":       []any{"packages/dorkpipe/lib"},
	}
	writer["constraints"] = []any{"writer constraint", "shared task constraint"}
	writer["expected_output"] = "proposal writer output"
	verifier := mapValue(tasks[1])
	verifier["agent"] = "aaa"
	verifier["goal"] = "proposal goal for verifier"
	verifier["brief"] = "proposal verifier brief"
	verifier["context"] = map[string]any{"required_artifacts": []any{"tasks/proposal_write/result.json"}}
	verifier["constraints"] = []any{"verifier constraint"}
	verifier["expected_output"] = "proposal verifier output"
	return proposal
}

func addExecutableTaskFields(packageDefaults, repo map[string]any) {
	packageTask := mapValue(listValue(contractOrchestration(packageDefaults)["tasks"])[0])
	packageTask["agent"] = "base"
	packageTask["goal"] = "package seed goal"
	packageTask["brief"] = "package seed brief"
	packageTask["context"] = map[string]any{}
	packageTask["constraints"] = []any{"package task constraint"}
	packageTask["expected_output"] = "package seed output"

	repoTasks := listValue(contractOrchestration(repo)["tasks"])
	for index, raw := range repoTasks {
		task := mapValue(raw)
		if index == 0 {
			task["agent"] = "base"
		} else {
			task["agent"] = "repo_role"
		}
		task["goal"] = fmt.Sprintf("repo task %d goal", index)
		task["brief"] = fmt.Sprintf("repo task %d brief", index)
		task["context"] = map[string]any{}
		task["constraints"] = []any{fmt.Sprintf("repo task %d constraint", index)}
		task["expected_output"] = fmt.Sprintf("repo task %d output", index)
	}
}

func validContractPackageDefaults() map[string]any {
	return map[string]any{
		"startup_prompt": "package prompt",
		"access": map[string]any{
			"read":  []any{"/work", "/DesignNotes"},
			"write": []any{"/work"},
			"deny":  []any{"/work/.git", "/work/secrets"},
		},
		"cloud_budget": map[string]any{"max_total_tokens": 1000, "max_task_tokens": 400, "max_tasks": 4},
		"constraints":  []any{"package rule", "shared rule"},
		"publish":      false,
		"sync":         false,
		"orchestration": map[string]any{
			"request": map[string]any{"text": "package request"},
			"plan":    map[string]any{"goal": "package goal"},
			"agents": map[string]any{
				"zeta": map[string]any{"role": "package zeta"},
				"base": map[string]any{"role": "package base", "constraints": []any{"package role rule"}},
			},
			"tasks": []any{contractTask("package_seed", nil, "base.md")},
			"apply": map[string]any{
				"require_approval":   true,
				"target_root":        "package/default",
				"required_artifacts": []any{"base.md"},
			},
		},
	}
}

func validContractRepoTaskPack() map[string]any {
	return map[string]any{
		"startup_prompt": "repo prompt",
		"access": map[string]any{
			"read":  []any{"/work/src"},
			"write": []any{"/work/src"},
			"deny":  []any{"/work/generated", "/work/secrets"},
		},
		"cloud_budget": map[string]any{"max_total_tokens": 800, "max_task_tokens": 300, "max_tasks": 3},
		"constraints":  []any{"repo rule", "shared rule"},
		"orchestration": map[string]any{
			"request": map[string]any{"text": "repo request"},
			"plan":    map[string]any{"goal": "repo goal"},
			"agents": map[string]any{
				"repo_role": map[string]any{"role": "repo only"},
				"base":      map[string]any{"role": "repo base", "constraints": []any{"repo role rule"}},
			},
			"tasks": []any{
				contractTask("repo_write", nil, "base.md", "repo.md"),
				contractTask("repo_verify", []string{"repo_write"}),
			},
			"apply": map[string]any{
				"require_approval":   true,
				"target_root":        "docs/generated",
				"required_artifacts": []any{"repo.md"},
			},
		},
	}
}

func validContractProposal() map[string]any {
	return map[string]any{
		"startup_prompt": "proposal prompt",
		"access": map[string]any{
			"read":  []any{"/work/src/pkg"},
			"write": []any{"/work/src/pkg"},
			"deny":  []any{"/work/src/tmp"},
		},
		"cloud_budget": map[string]any{"max_total_tokens": 700, "max_task_tokens": 250, "max_tasks": 2},
		"constraints":  []any{"proposal rule", "repo rule"},
		"orchestration": map[string]any{
			"request": map[string]any{"text": "proposal request"},
			"plan":    map[string]any{"goal": "proposal goal"},
			"agents": map[string]any{
				"aaa":  map[string]any{"role": "proposal only"},
				"base": map[string]any{"role": "proposal base", "constraints": []any{"proposal role rule"}},
			},
			"tasks": []any{
				contractTask("proposal_write", nil, "base.md", "repo.md", "extra.md"),
				contractTask("proposal_verify", []string{"proposal_write"}),
			},
			"apply": map[string]any{
				"require_approval":   true,
				"target_root":        "docs/generated",
				"required_artifacts": []any{"base.md", "repo.md", "extra.md"},
			},
		},
	}
}

func contractTask(id string, dependencies []string, outputs ...string) map[string]any {
	materialized := make([]any, 0, len(outputs))
	for _, output := range outputs {
		materialized = append(materialized, map[string]any{"path": output})
	}
	return map[string]any{
		"id":                  id,
		"depends_on":          anySlice(dependencies),
		"max_cloud_tokens":    200,
		"materialize_outputs": materialized,
	}
}

func contractIDs(items []any) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, stringValue(mapValue(item)["id"]))
	}
	return out
}

func contractItemByID(items []any, id string) map[string]any {
	for _, item := range items {
		if value := mapValue(item); stringValue(value["id"]) == id {
			return value
		}
	}
	return map[string]any{}
}

func assertStringOrder(t *testing.T, field string, got, want []string) {
	t.Helper()
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("%s = %#v, want %#v", field, got, want)
	}
}

func TestLoadAgentsConfigUsesNearestWorkflowParentAndSiblingOverride(t *testing.T) {
	root := t.TempDir()
	workflowPath := filepath.Join(root, "workflows", "agent", "review", "config.yml")
	writeAgentsConfig := func(path, role string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		content := "agents:\n  reviewer:\n    role: " + role + "\n"
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	writeAgentsConfig(filepath.Join(root, "agents.yml"), "outside-workflow-root")
	writeAgentsConfig(filepath.Join(root, "workflows", "agent", "agents.yml"), "shared-reviewer")
	if got := stringValue(mapValue(loadAgentsConfig(workflowPath)["reviewer"])["role"]); got != "shared-reviewer" {
		t.Fatalf("parent role = %q, want shared-reviewer", got)
	}

	writeAgentsConfig(filepath.Join(filepath.Dir(workflowPath), "agents.yml"), "local-reviewer")
	if got := stringValue(mapValue(loadAgentsConfig(workflowPath)["reviewer"])["role"]); got != "local-reviewer" {
		t.Fatalf("sibling role = %q, want local-reviewer", got)
	}
}

func TestLoadAgentsConfigUsesPackageAuthoringRoot(t *testing.T) {
	root := t.TempDir()
	packageRoot := filepath.Join(root, "packages", "example")
	workflowPath := filepath.Join(packageRoot, "workflows", "review", "config.yml")
	if err := os.MkdirAll(filepath.Dir(workflowPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(packageRoot, "package.yml"), []byte("name: example\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "agents.yml"), []byte("agents:\n  reviewer:\n    role: outside-package-root\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(packageRoot, "agents.yml"), []byte("agents:\n  reviewer:\n    role: package-reviewer\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := stringValue(mapValue(loadAgentsConfig(workflowPath)["reviewer"])["role"]); got != "package-reviewer" {
		t.Fatalf("package role = %q, want package-reviewer", got)
	}
}

func TestPlanOrchestrationPreservesApplyContract(t *testing.T) {
	tests := []struct {
		name              string
		applyYAML         string
		requireApproval   bool
		wantOutputs       []map[string]string
		wantTargetRoot    string
		wantRequiredPaths []string
	}{
		{
			name: "explicit and inferred fields",
			applyYAML: `          require_approval: false
          outputs:
            - source: merge/summary.md
              path: docs/summary.md
          target_root: docs/generated
          required_artifacts:
            - overview.md
            - decisions.md
`,
			requireApproval: false,
			wantOutputs: []map[string]string{
				{"source": "merge/summary.md", "path": "docs/summary.md"},
			},
			wantTargetRoot:    "docs/generated",
			wantRequiredPaths: []string{"overview.md", "decisions.md"},
		},
		{
			name: "inferred outputs only",
			applyYAML: `          require_approval: true
          target_root: docs/agents/brain
          required_artifacts:
            - index.md
            - source-of-truth-rules.md
            - repo-knowledge.md
`,
			requireApproval:   true,
			wantTargetRoot:    "docs/agents/brain",
			wantRequiredPaths: []string{"index.md", "source-of-truth-rules.md", "repo-knowledge.md"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			artifactRoot := filepath.Join(root, "artifacts")
			sharedDir := filepath.Join(artifactRoot, "shared")
			tasksDir := filepath.Join(artifactRoot, "tasks")
			for _, dir := range []string{sharedDir, tasksDir} {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
			}

			workflowPath := filepath.Join(root, "config.yml")
			workflow := `name: plan-apply-contract
steps:
  - id: orchestrate
    agent:
      include_agents_md: false
      orchestration:
        request:
          text: Preserve the authored apply contract.
        tasks:
          - id: writer
            goal: Produce one bounded output.
            expected_output: A concise artifact.
        apply:
` + tt.applyYAML
			if err := os.WriteFile(workflowPath, []byte(workflow), 0o644); err != nil {
				t.Fatal(err)
			}

			planPath := filepath.Join(artifactRoot, "plan.json")
			env := map[string]string{
				"ROOT":                                  root,
				"DORKPIPE_ORCH_SHARED_DIR":              sharedDir,
				"DORKPIPE_ORCH_TASKS_DIR":               tasksDir,
				"DORKPIPE_ORCH_REQUEST_JSON":            filepath.Join(artifactRoot, "request.json"),
				"DORKPIPE_ORCH_PLAN_JSON":               planPath,
				"DORKPIPE_ORCH_GRAPH_JSON":              filepath.Join(artifactRoot, "task-graph.json"),
				"DORKPIPE_ORCH_LANE_PLAN_JSON":          filepath.Join(artifactRoot, "lane-plan.json"),
				"DORKPIPE_ORCH_MODEL_CATALOG":           filepath.Join(root, "missing-model-catalog.yml"),
				"DORKPIPE_ORCH_BASELINE_POLICY":         filepath.Join(root, "missing-baseline-policy.yml"),
				"DORKPIPE_ORCH_GLOBAL_TRAINING_METRICS": filepath.Join(root, "missing-training-metrics.jsonl"),
				"DORKPIPE_ORCH_WORKFLOW":                "plan-apply-contract",
				"DORKPIPE_ORCH_ROOT":                    artifactRoot,
			}
			if err := planOrchestration(workflowPath, "orchestrate", env); err != nil {
				t.Fatal(err)
			}

			applyPlan := mapValue(readJSONMap(planPath)["apply"])
			if got := boolAny(applyPlan["require_approval"]); got != tt.requireApproval {
				t.Fatalf("apply.require_approval = %t, want %t", got, tt.requireApproval)
			}
			outputs := listValue(applyPlan["outputs"])
			if len(outputs) != len(tt.wantOutputs) {
				t.Fatalf("apply.outputs length = %d, want %d", len(outputs), len(tt.wantOutputs))
			}
			for index, want := range tt.wantOutputs {
				got := mapValue(outputs[index])
				if stringValue(got["source"]) != want["source"] || stringValue(got["path"]) != want["path"] {
					t.Fatalf("apply.outputs[%d] = %#v, want %#v", index, got, want)
				}
			}
			if got := stringValue(applyPlan["target_root"]); got != tt.wantTargetRoot {
				t.Fatalf("apply.target_root = %q, want %q", got, tt.wantTargetRoot)
			}
			if got := stringList(applyPlan["required_artifacts"]); strings.Join(got, "\n") != strings.Join(tt.wantRequiredPaths, "\n") {
				t.Fatalf("apply.required_artifacts = %#v, want %#v", got, tt.wantRequiredPaths)
			}
		})
	}
}

func TestRenderExecutionLanePromptContextIncludesSelectionAndPolicy(t *testing.T) {
	got := renderExecutionLanePromptContext(map[string]any{
		"requested": "auto",
		"lane_id":   "codex.strong",
		"provider":  "codex",
		"model":     "gpt-5.6",
		"reasons":   []string{"high-authority task favors a strong lane", "available"},
	}, map[string]any{
		"task_class": map[string]any{"name": "architecture", "authority": "high"},
		"model_policy": map[string]any{
			"attempt": map[string]any{"preference": "strong"},
		},
	})
	for _, want := range []string{
		"Execution lane (operational run metadata):",
		"Requested lane: auto",
		"Selected lane: codex.strong",
		"Provider: codex",
		"Model: gpt-5.6",
		"Work class: architecture",
		"Authority: high",
		"Selection rationale: high-authority task favors a strong lane; available",
		"Model policy: `{\"attempt\":{\"preference\":\"strong\"}}`",
		"current run only",
		"Do not substitute lane selection for source evidence",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("prompt context missing %q:\n%s", want, got)
		}
	}
}

func TestRenderSourcePacketUsesOnlyAllowedTextEvidence(t *testing.T) {
	root := t.TempDir()
	write := func(rel, content string) {
		t.Helper()
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("docs/a.md", "allowed A\n")
	write("docs/z.go", "package docs\n")
	write("secrets/hidden.md", "do not expose\n")
	write(".git/config", "ignored\n")
	write("docs/image.png", "\x00binary")

	packet, err := renderSourcePacket(root, []string{"."}, []string{"."}, []string{"secrets"}, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"docs/a.md", "allowed A", "docs/z.go", "package docs", "access.read"} {
		if !strings.Contains(packet, want) {
			t.Fatalf("packet missing %q:\n%s", want, packet)
		}
	}
	for _, forbidden := range []string{"hidden.md", "do not expose", ".git/config", "image.png"} {
		if strings.Contains(packet, forbidden) {
			t.Fatalf("packet leaked %q:\n%s", forbidden, packet)
		}
	}
}

func TestRenderSourcePacketRejectsRootOutsideReadAccess(t *testing.T) {
	root := t.TempDir()
	for _, rel := range []string{"allowed/a.md", "outside/b.md"} {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("evidence\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	_, err := renderSourcePacket(root, []string{"outside"}, []string{"allowed"}, nil, "")
	if err == nil || !strings.Contains(err.Error(), "outside access.read") {
		t.Fatalf("expected access failure, got %v", err)
	}
}

func TestRenderSourcePacketUsesGuestPathsForExternalMount(t *testing.T) {
	root := t.TempDir()
	externalRoot := t.TempDir()
	path := filepath.Join(externalRoot, "reference", "design.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("external design evidence\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mountEnv := externalRoot + ":/DesignNotes:ro"

	packet, err := renderSourcePacket(root, []string{"/DesignNotes/reference"}, []string{"/DesignNotes"}, nil, mountEnv)
	if err != nil {
		t.Fatal(err)
	}
	hostDeclaredPacket, err := renderSourcePacket(root, []string{filepath.Join(externalRoot, "reference")}, []string{externalRoot}, nil, mountEnv)
	if err != nil {
		t.Fatal(err)
	}
	if hostDeclaredPacket != packet {
		t.Fatalf("host-resolved and guest-declared packets differ:\nguest:\n%s\nhost-resolved:\n%s", packet, hostDeclaredPacket)
	}
	for _, want := range []string{"`/DesignNotes/reference`", "## /DesignNotes/reference/design.md", "external design evidence"} {
		if !strings.Contains(packet, want) {
			t.Fatalf("packet missing guest-oriented path %q:\n%s", want, packet)
		}
	}
	for _, hostPath := range []string{externalRoot, filepath.ToSlash(externalRoot)} {
		if strings.Contains(packet, hostPath) {
			t.Fatalf("packet disclosed host path %q:\n%s", hostPath, packet)
		}
	}
}

func TestRenderSourcePacketPreservesDeclaredRootAndLexicalFileOrder(t *testing.T) {
	root := t.TempDir()
	write := func(rel string) {
		t.Helper()
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(rel+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("first/z.md")
	write("first/a.md")
	write("second/b.md")
	write("second/a.md")

	packet, err := renderSourcePacket(root, []string{"second", "first"}, []string{"."}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	ordered := []string{"## second/a.md", "## second/b.md", "## first/a.md", "## first/z.md"}
	previous := -1
	for _, heading := range ordered {
		index := strings.Index(packet, heading)
		if index < 0 {
			t.Fatalf("packet missing %q:\n%s", heading, packet)
		}
		if index <= previous {
			t.Fatalf("packet order is not declared-root then lexical: %q\n%s", heading, packet)
		}
		previous = index
	}
}

func TestRenderSourcePacketExcludesBinaryFilesAndSymlinks(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, "docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docs, "visible.md"), []byte("visible evidence\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docs, "binary.md"), []byte("binary\x00payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "outside.md")
	if err := os.WriteFile(target, []byte("symlink target must stay out\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	symlinkPath := filepath.Join(docs, "linked.md")
	symlinkCreated := true
	if err := os.Symlink(target, symlinkPath); err != nil {
		symlinkCreated = false
		t.Logf("symlink fixture unavailable on this platform: %v", err)
	}

	packet, err := renderSourcePacket(root, []string{"docs"}, []string{"."}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(packet, "visible evidence") {
		t.Fatalf("packet omitted visible text evidence:\n%s", packet)
	}
	for _, forbidden := range []string{"binary.md", "binary\\x00payload"} {
		if strings.Contains(packet, forbidden) {
			t.Fatalf("packet included binary evidence %q:\n%s", forbidden, packet)
		}
	}
	if symlinkCreated {
		for _, forbidden := range []string{"linked.md", "symlink target must stay out"} {
			if strings.Contains(packet, forbidden) {
				t.Fatalf("packet followed symlink evidence %q:\n%s", forbidden, packet)
			}
		}
	}
}

func TestRenderSourcePacketEnforcesPacketBounds(t *testing.T) {
	t.Run("file count", func(t *testing.T) {
		root := t.TempDir()
		docs := filepath.Join(root, "docs")
		if err := os.MkdirAll(docs, 0o755); err != nil {
			t.Fatal(err)
		}
		for index := 0; index <= sourcePacketMaxFiles; index++ {
			name := fmt.Sprintf("%02d.md", index)
			if err := os.WriteFile(filepath.Join(docs, name), []byte(name+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}

		packet, err := renderSourcePacket(root, []string{"docs"}, []string{"."}, nil, "")
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.Count(packet, "## docs/"); got != sourcePacketMaxFiles {
			t.Fatalf("packet included %d files, want %d:\n%s", got, sourcePacketMaxFiles, packet)
		}
		if strings.Contains(packet, fmt.Sprintf("## docs/%02d.md", sourcePacketMaxFiles)) {
			t.Fatalf("packet included a file beyond its count bound:\n%s", packet)
		}
		if !strings.Contains(packet, "Packet bounds reached") {
			t.Fatalf("packet did not report its file-count bound:\n%s", packet)
		}
	})

	t.Run("per file and total bytes", func(t *testing.T) {
		root := t.TempDir()
		docs := filepath.Join(root, "docs")
		if err := os.MkdirAll(docs, 0o755); err != nil {
			t.Fatal(err)
		}
		fileCount := sourcePacketMaxBytes/sourcePacketMaxFileBytes + 1
		for index := 0; index < fileCount; index++ {
			content := strings.Repeat(string(rune('a'+index)), sourcePacketMaxFileBytes)
			if index == 0 {
				content += "AFTER_FILE_BOUND"
			}
			name := fmt.Sprintf("%02d.md", index)
			if err := os.WriteFile(filepath.Join(docs, name), []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		}

		packet, err := renderSourcePacket(root, []string{"docs"}, []string{"."}, nil, "")
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(packet, "AFTER_FILE_BOUND") {
			t.Fatalf("packet exceeded its per-file byte bound:\n%s", packet)
		}
		lastName := fmt.Sprintf("## docs/%02d.md", fileCount-1)
		if strings.Contains(packet, lastName) {
			t.Fatalf("packet exceeded its total source-byte bound:\n%s", packet)
		}
		if !strings.Contains(packet, "Packet bounds reached") {
			t.Fatalf("packet did not report its total byte bound:\n%s", packet)
		}
	})
}

func TestMaterializePromptBriefPersistsBoundedContextForLocalLane(t *testing.T) {
	root := t.TempDir()
	taskDir := filepath.Join(root, "artifacts", "tasks", "inventory")
	for rel, content := range map[string]string{
		"docs/z.md": "z evidence\n",
		"docs/a.md": "a evidence that is longer than the configured per-file limit\n",
	} {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(taskDir, 0o755); err != nil {
		t.Fatal(err)
	}

	metadata, brief, err := materializePromptBrief(taskDir, []string{"docs/z.md", "docs/a.md"}, "ollama", root, filepath.Join(root, "artifacts"), filepath.Join(root, "artifacts", "shared"), true, 12, 20)
	if err != nil {
		t.Fatal(err)
	}
	if got := stringValue(metadata["path"]); got != "tasks/inventory/prompt-brief.md" {
		t.Fatalf("brief path = %q", got)
	}
	if !strings.Contains(brief, "### docs/a.md") || strings.Index(brief, "### docs/a.md") > strings.Index(brief, "### docs/z.md") {
		t.Fatalf("brief ordering is not deterministic:\n%s", brief)
	}
	if !strings.Contains(brief, "[truncated]") || !strings.Contains(brief, "Local model lane guidance:") {
		t.Fatalf("brief did not preserve bounded local guidance:\n%s", brief)
	}
	persisted, err := os.ReadFile(filepath.Join(taskDir, "prompt-brief.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(persisted) != brief {
		t.Fatalf("persisted brief differs from prompt evidence")
	}
}

func TestMaterializePromptBriefSkipsDisabledOrMissingContext(t *testing.T) {
	taskDir := t.TempDir()
	metadata, brief, err := materializePromptBrief(taskDir, []string{"missing.md"}, "ollama", taskDir, filepath.Join(taskDir, "artifacts"), filepath.Join(taskDir, "shared"), false, 100, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(metadata) != 0 || brief != "" {
		t.Fatalf("disabled context should not materialize a brief: %#v %q", metadata, brief)
	}
	if _, err := os.Stat(filepath.Join(taskDir, "prompt-brief.md")); !os.IsNotExist(err) {
		t.Fatalf("disabled context unexpectedly wrote a brief: %v", err)
	}
}

func TestApplyTaskWorkerProfileDefaultsToPrefer(t *testing.T) {
	task, err := applyTaskWorkerProfile(map[string]any{
		"id":     "patch",
		"worker": "codex",
	}, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	if got := stringValue(task["resolver_hint"]); got != "" {
		t.Fatalf("resolver_hint should stay empty for seeded worker profiles, got %q", got)
	}
	if got := workerPolicyMode(task); got != "prefer" {
		t.Fatalf("worker policy mode = %q, want prefer", got)
	}
	if got := stringValue(task["worker_preferred_resolver_hint"]); got != "codex" {
		t.Fatalf("worker preferred resolver = %q, want codex", got)
	}
}

func TestApplyTaskWorkerProfileEditModeDefaultsToRequire(t *testing.T) {
	task, err := applyTaskWorkerProfile(map[string]any{
		"id":        "patch",
		"worker":    "codex",
		"work_mode": "edit",
	}, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	if got := workerPolicyMode(task); got != "require" {
		t.Fatalf("worker policy mode = %q, want require for edit mode", got)
	}
}

func TestSelectLaneWorkerPreferAllowsFallback(t *testing.T) {
	task, err := applyTaskWorkerProfile(map[string]any{
		"id":     "analysis",
		"worker": "codex",
		"model_policy": map[string]any{
			"attempt": map[string]any{
				"preference": "cheap-first",
			},
		},
	}, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	lanes := []map[string]any{
		{
			"id":            "ollama.local.default",
			"provider":      "ollama",
			"resolver_hint": "ollama",
			"local":         true,
			"available":     true,
		},
		{
			"id":            "codex.cloud.default",
			"provider":      "codex",
			"resolver_hint": "codex",
			"cloud":         true,
			"available":     true,
		},
	}
	selection := selectLane(task, mapValue(task["model_policy"]), "", "", "", map[string]string{}, lanes, map[string]any{
		"local_first_bonus":       15.0,
		"cloud_cost_penalty":      2.0,
		"worker_preference_bonus": 10.0,
	}, map[string]any{}, map[string]trainingEntry{}, false, nil)
	if got := stringValue(selection["provider"]); got != "ollama" {
		t.Fatalf("provider = %q, want ollama fallback under prefer policy", got)
	}
}

func TestSelectLanePlanningWorkerPreferPinsDeclaredLane(t *testing.T) {
	task, err := applyTaskWorkerProfile(map[string]any{
		"id":          "contract_brain",
		"worker":      "ollama",
		"worker_type": "planning",
		"goal":        "distill an architecture contract",
	}, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	lanes := []map[string]any{
		{"id": "ollama.local.default", "provider": "ollama", "resolver_hint": "ollama", "local": true, "available": true},
		{"id": "codex.cloud.default", "provider": "codex", "resolver_hint": "codex", "cloud": true, "available": true, "capabilities": []any{"strong_validation"}},
	}
	selection := selectLane(task, map[string]any{"validate": map[string]any{"preference": "strongest_available"}}, "", "", "", map[string]string{}, lanes, map[string]any{
		"local_first_bonus":          15.0,
		"cloud_cost_penalty":         2.0,
		"worker_preference_bonus":    10.0,
		"strong_validation_bonus":    8.0,
		"authority_cloud_bonus":      8.0,
		"local_architecture_penalty": 18.0,
	}, map[string]any{}, map[string]trainingEntry{}, true, nil)
	if got := stringValue(selection["provider"]); got != "ollama" {
		t.Fatalf("provider = %q, want declared planning lane ollama", got)
	}
	if got := stringValue(selection["requested"]); got != "ollama" {
		t.Fatalf("requested = %q, want ollama", got)
	}
}

func TestSelectLaneWorkerRequirePinsPreferredLane(t *testing.T) {
	task, err := applyTaskWorkerProfile(map[string]any{
		"id":     "patch",
		"worker": "codex",
		"worker_policy": map[string]any{
			"mode": "require",
		},
		"model_policy": map[string]any{
			"attempt": map[string]any{
				"preference": "cheap-first",
			},
		},
	}, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	lanes := []map[string]any{
		{
			"id":            "ollama.local.default",
			"provider":      "ollama",
			"resolver_hint": "ollama",
			"local":         true,
			"available":     true,
		},
		{
			"id":            "codex.cloud.default",
			"provider":      "codex",
			"resolver_hint": "codex",
			"cloud":         true,
			"available":     true,
		},
	}
	selection := selectLane(task, mapValue(task["model_policy"]), "", "", "", map[string]string{}, lanes, map[string]any{
		"local_first_bonus":       15.0,
		"cloud_cost_penalty":      2.0,
		"worker_preference_bonus": 10.0,
	}, map[string]any{}, map[string]trainingEntry{}, true, nil)
	if got := stringValue(selection["provider"]); got != "codex" {
		t.Fatalf("provider = %q, want codex under require policy", got)
	}
	if got := stringValue(selection["requested"]); got != "codex" {
		t.Fatalf("requested = %q, want codex", got)
	}
}

func TestEmitTaskEnvIncludesProviderPoolModelPolicy(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "task.json")
	if err := writeJSONFile(path, map[string]any{
		"id":   "review",
		"goal": "review the change",
		"model_policy": map[string]any{
			"execution_mode":        "provider_pool",
			"role":                  "reviewer",
			"session_scope":         "workflow",
			"max_active":            1,
			"queue_timeout_seconds": 2,
		},
	}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := emitTaskEnv(path, &out); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{
		"TASK_MODEL_POLICY_EXECUTION_MODE='provider_pool'",
		"TASK_PROVIDER_POOL_ROLE='reviewer'",
		"TASK_PROVIDER_POOL_SESSION_SCOPE='workflow'",
		"TASK_PROVIDER_POOL_MAX_ACTIVE='1'",
		"TASK_PROVIDER_POOL_QUEUE_TIMEOUT_SECONDS='2'",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("task env missing %s in:\n%s", want, text)
		}
	}
}

func TestEmitProviderPoolResponseEnvWritesResponseAndMetadata(t *testing.T) {
	dir := t.TempDir()
	responseJSON := filepath.Join(dir, "provider-pool-response.json")
	responseMD := filepath.Join(dir, "response.md")
	if err := writeJSONFile(responseJSON, map[string]any{
		"state":     "ready",
		"status":    "ready",
		"text":      "pooled response",
		"exit_code": 0,
		"metadata": map[string]any{
			"session_id":     "workflow:run:node:review",
			"worker_id":      "worker-1",
			"prompt_turn_id": "turn-1",
		},
	}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := emitProviderPoolResponseEnv(responseJSON, responseMD, &out); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(responseMD)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "pooled response" {
		t.Fatalf("response.md = %q", string(raw))
	}
	text := out.String()
	for _, want := range []string{
		"PROVIDER_POOL_STATE='ready'",
		"PROVIDER_POOL_USED_LIVE_MODEL='true'",
		"PROVIDER_POOL_PROVIDER_SESSION_ID='workflow:run:node:review'",
		"PROVIDER_POOL_WORKER_ID='worker-1'",
		"PROVIDER_POOL_PROMPT_TURN_ID='turn-1'",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("provider pool env missing %s in:\n%s", want, text)
		}
	}
}

func TestSelectLaneDoesNotUseMismatchedRoleModel(t *testing.T) {
	task, err := applyTaskWorkerProfile(map[string]any{
		"id":     "contract_brain",
		"worker": "ollama",
		"model": map[string]any{
			"provider": "ollama",
			"model":    "llama3.2",
		},
	}, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	lanes := []map[string]any{
		{
			"id":            "ollama.local.default",
			"provider":      "ollama",
			"resolver_hint": "ollama",
			"model":         "llama3.2",
			"local":         true,
			"available":     true,
		},
		{
			"id":            "codex.cli.default",
			"provider":      "codex",
			"resolver_hint": "codex",
			"model":         "cli",
			"cloud":         true,
			"available":     true,
		},
	}
	selection := selectLane(task, mapValue(task["model_policy"]), "codex", "", "", map[string]string{}, lanes, map[string]any{}, map[string]any{}, map[string]trainingEntry{}, true, nil)
	if got := stringValue(selection["provider"]); got != "codex" {
		t.Fatalf("provider = %q, want codex", got)
	}
	if got := stringValue(selection["model"]); got != "cli" {
		t.Fatalf("model = %q, want selected codex lane model", got)
	}
	effective := taskModelForLane(mapValue(task["model"]), selection)
	if got := stringValue(effective["provider"]); got != "codex" {
		t.Fatalf("effective provider = %q, want codex", got)
	}
	if got := stringValue(effective["model"]); got != "cli" {
		t.Fatalf("effective model = %q, want cli", got)
	}
}

func TestSelectLaneArchitectureTaskPrefersCloudOverCheapLocal(t *testing.T) {
	task, err := applyTaskWorkerProfile(map[string]any{
		"id":              "brain_contract",
		"worker":          "claude",
		"worker_type":     "architecture",
		"goal":            "Define the architecture contract and acceptance criteria",
		"expected_output": "A durable contract and routing policy",
		"model_policy": map[string]any{
			"attempt": map[string]any{
				"preference": "cheap-first",
			},
		},
	}, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	lanes := []map[string]any{
		{
			"id":            "ollama.local.default",
			"provider":      "ollama",
			"resolver_hint": "ollama",
			"model":         "qwen2.5:7b",
			"local":         true,
			"available":     true,
		},
		{
			"id":            "claude.cloud.default",
			"provider":      "claude",
			"resolver_hint": "claude",
			"model":         "cli",
			"cloud":         true,
			"available":     true,
			"capabilities":  []any{"review", "safety", "strong_validation"},
		},
	}
	selection := selectLane(task, mapValue(task["model_policy"]), "", "", "", map[string]string{
		"DORKPIPE_ORCH_HOST_MEMORY_GB":      "16",
		"DORKPIPE_ORCH_HOST_CPU_CORES":      "8",
		"DORKPIPE_ORCH_LOCAL_ACCELERATION":  "cpu",
		"DORKPIPE_ORCH_LOCAL_HARDWARE_TIER": "low",
	}, lanes, map[string]any{
		"local_first_bonus":                 15.0,
		"cloud_cost_penalty":                2.0,
		"worker_preference_bonus":           10.0,
		"authority_cloud_bonus":             8.0,
		"local_architecture_penalty":        18.0,
		"low_tier_local_authority_penalty":  10.0,
		"architecture_keywords":             []any{"architecture", "contract", "acceptance criteria"},
		"cloud_score_threshold":             14.0,
		"high_risk_cloud_score_threshold":   10.0,
		"explicit_hint_bypasses_cloud_gate": true,
	}, map[string]any{}, map[string]trainingEntry{}, true, nil)
	if got := stringValue(selection["provider"]); got != "claude" {
		t.Fatalf("provider = %q, want claude for architecture authority", got)
	}
}

func TestSelectLaneExtractionTaskKeepsLocalWhenModelFitsHost(t *testing.T) {
	task, err := applyTaskWorkerProfile(map[string]any{
		"id":              "repo_fact_packet",
		"worker":          "ollama",
		"worker_type":     "extraction",
		"goal":            "Extract narrow repo facts only",
		"expected_output": "A compact fact packet",
		"constraints":     []any{"facts only", "extract path groups"},
		"model_policy": map[string]any{
			"attempt": map[string]any{
				"preference": "cheap-first",
			},
		},
	}, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	lanes := []map[string]any{
		{
			"id":            "ollama.local.default",
			"provider":      "ollama",
			"resolver_hint": "ollama",
			"model":         "qwen2.5:7b",
			"local":         true,
			"available":     true,
		},
		{
			"id":            "codex.cloud.default",
			"provider":      "codex",
			"resolver_hint": "codex",
			"model":         "cli",
			"cloud":         true,
			"available":     true,
			"capabilities":  []any{"code", "strong_validation"},
		},
	}
	selection := selectLane(task, mapValue(task["model_policy"]), "", "", "", map[string]string{
		"DORKPIPE_ORCH_HOST_MEMORY_GB":     "64",
		"DORKPIPE_ORCH_HOST_CPU_CORES":     "16",
		"DORKPIPE_ORCH_LOCAL_ACCELERATION": "gpu",
	}, lanes, map[string]any{
		"local_first_bonus":          15.0,
		"cloud_cost_penalty":         2.0,
		"worker_preference_bonus":    10.0,
		"extraction_local_bonus":     8.0,
		"local_model_fit_bonus":      3.0,
		"gpu_local_extraction_bonus": 2.0,
		"extraction_keywords":        []any{"extract", "facts only", "fact packet", "path groups"},
	}, map[string]any{}, map[string]trainingEntry{}, true, nil)
	if got := stringValue(selection["provider"]); got != "ollama" {
		t.Fatalf("provider = %q, want ollama for bounded extraction", got)
	}
}

func TestSelectLaneOversizedLocalModelLosesOnSmallHost(t *testing.T) {
	task, err := applyTaskWorkerProfile(map[string]any{
		"id":              "design_inventory",
		"worker":          "ollama",
		"worker_type":     "extraction",
		"goal":            "Extract design inventory only",
		"expected_output": "A compact inventory packet",
		"constraints":     []any{"extract path groups"},
		"model_policy": map[string]any{
			"attempt": map[string]any{
				"preference": "cheap-first",
			},
		},
	}, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	lanes := []map[string]any{
		{
			"id":            "ollama.local.default",
			"provider":      "ollama",
			"resolver_hint": "ollama",
			"model":         "qwen2.5:32b",
			"local":         true,
			"available":     true,
		},
		{
			"id":            "claude.cloud.default",
			"provider":      "claude",
			"resolver_hint": "claude",
			"model":         "cli",
			"cloud":         true,
			"available":     true,
			"capabilities":  []any{"review", "strong_validation"},
		},
	}
	selection := selectLane(task, mapValue(task["model_policy"]), "", "", "", map[string]string{
		"DORKPIPE_ORCH_HOST_MEMORY_GB":     "16",
		"DORKPIPE_ORCH_HOST_CPU_CORES":     "8",
		"DORKPIPE_ORCH_LOCAL_ACCELERATION": "cpu",
	}, lanes, map[string]any{
		"local_first_bonus":             15.0,
		"cloud_cost_penalty":            2.0,
		"worker_preference_bonus":       10.0,
		"extraction_local_bonus":        8.0,
		"oversized_local_model_penalty": 30.0,
		"extraction_keywords":           []any{"extract", "inventory", "path groups"},
	}, map[string]any{}, map[string]trainingEntry{}, true, nil)
	if got := stringValue(selection["provider"]); got != "claude" {
		t.Fatalf("provider = %q, want claude when local model is oversized for host", got)
	}
}

func TestComparisonDisabledForRequiredWorker(t *testing.T) {
	task, err := applyTaskWorkerProfile(map[string]any{
		"id":     "patch",
		"worker": "codex",
		"worker_policy": map[string]any{
			"mode": "require",
		},
	}, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	if comparisonEnabledForTask(task, []string{"codex", "claude"}, "auto") {
		t.Fatal("comparison should be disabled when worker_policy.mode=require")
	}
}

func TestEmitTaskEnvIncludesWorkMode(t *testing.T) {
	taskPath := filepath.Join(t.TempDir(), "task.json")
	if err := os.WriteFile(taskPath, []byte(`{"id":"patch","worker":"codex","work_mode":"edit","output_path":"/work/docs/brain.md"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	if err := emitTaskEnv(taskPath, &stdout); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "TASK_WORK_MODE='edit'") {
		t.Fatalf("task env missing work mode:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "TASK_OUTPUT_PATH='/work/docs/brain.md'") {
		t.Fatalf("task env missing output path:\n%s", stdout.String())
	}
}

func TestEmitTaskEnvIncludesLaneAvailability(t *testing.T) {
	taskPath := filepath.Join(t.TempDir(), "task.json")
	task := `{
		"id": "patch",
		"worker": "codex",
		"lane": {
			"lane_id": "codex.cli.default",
			"provider": "codex",
			"available": false,
			"missing_commands": ["codex"],
			"setup_hint": "Install and sign in to the Codex CLI.",
			"auth_hint": "Codex CLI must be authenticated."
		}
	}`
	if err := os.WriteFile(taskPath, []byte(task), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	if err := emitTaskEnv(taskPath, &stdout); err != nil {
		t.Fatal(err)
	}
	got := stdout.String()
	for _, want := range []string{
		"TASK_LANE_AVAILABLE='false'",
		`TASK_LANE_MISSING_COMMANDS_JSON='["codex"]'`,
		"TASK_LANE_SETUP_HINT='Install and sign in to the Codex CLI.'",
		"TASK_LANE_AUTH_HINT='Codex CLI must be authenticated.'",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("task env missing %q:\n%s", want, got)
		}
	}
}

func TestWriteTaskResultIncludesTraceOnlyWorkerSession(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "result.json")
	err := writeTaskResult(outPath, map[string]string{
		"task_id":                 "author_index",
		"status":                  "ok",
		"resolver_hint":           "codex",
		"provider":                "codex",
		"selected_model":          "cli",
		"lane_id":                 "codex.cloud.default",
		"provider_session_id":     "abc123",
		"used_live_model":         "true",
		"budget_halt":             "false",
		"estimated_input_tokens":  "10",
		"estimated_output_tokens": "5",
		"estimated_total_tokens":  "15",
		"confidence":              "0.72",
		"issues_json":             "[]",
		"next_actions_json":       "[]",
	})
	if err != nil {
		t.Fatal(err)
	}
	result := readJSONMap(outPath)
	session := mapValue(result["worker_session"])
	if got := stringValue(session["session_id"]); got != "abc123" {
		t.Fatalf("session_id = %q want abc123", got)
	}
	if got := stringValue(session["mode"]); got != "trace_only" {
		t.Fatalf("session mode = %q want trace_only", got)
	}
}

func TestMaterializeTaskOutputsExtractsDeclaredBlocks(t *testing.T) {
	dir := t.TempDir()
	responsePath := filepath.Join(dir, "response.md")
	resultPath := filepath.Join(dir, "materialized-result.json")
	response := strings.Join([]string{
		`<!-- dorkpipe:file path="index.md" -->`,
		"# Index",
		"",
		"Start here.",
		`<!-- /dorkpipe:file -->`,
		"",
		`<!-- dorkpipe:file path="index.yaml" -->`,
		"schema: test",
		`<!-- /dorkpipe:file -->`,
	}, "\n")
	if err := os.WriteFile(responsePath, []byte(response), 0o644); err != nil {
		t.Fatal(err)
	}
	outputs := `[{"path":"index.md"},{"path":"index.yaml"}]`
	if err := materializeTaskOutputs(responsePath, dir, outputs, resultPath, dir, ""); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(mustReadFile(t, filepath.Join(dir, "materialized", "index.md")))); got != "# Index\n\nStart here." {
		t.Fatalf("index.md = %q", got)
	}
	if got := stringValue(readJSONMap(resultPath)["status"]); got != "materialized" {
		t.Fatalf("status = %q want materialized", got)
	}
}

func TestRenderMaterializeOutputContractShowsExactBlocks(t *testing.T) {
	got := renderMaterializeOutputContract([]any{
		map[string]any{"path": "index.yaml"},
		map[string]any{"path": "design-corpus-index.yaml"},
	})
	for _, want := range []string{
		"DorkPipe materialized output contract:",
		"Required output paths: index.yaml, design-corpus-index.yaml",
		`<!-- dorkpipe:file path="index.yaml" -->`,
		`<!-- dorkpipe:file path="design-corpus-index.yaml" -->`,
		"Do not use YAML bundle/list wrappers around the file blocks.",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("contract missing %q in:\n%s", want, got)
		}
	}
}

func TestMaterializeTaskOutputsRejectsEscapingPath(t *testing.T) {
	dir := t.TempDir()
	responsePath := filepath.Join(dir, "response.md")
	if err := os.WriteFile(responsePath, []byte(`<!-- dorkpipe:file path="../bad.md" -->bad<!-- /dorkpipe:file -->`), 0o644); err != nil {
		t.Fatal(err)
	}
	err := materializeTaskOutputs(responsePath, dir, `[{"path":"../bad.md"}]`, filepath.Join(dir, "result.json"), dir, "")
	if err == nil {
		t.Fatal("expected escaping materialized path to fail")
	}
}

func TestExampleBrainBaselineEligibilityInMixedTaskPack(t *testing.T) {
	shared := []any{
		map[string]any{"path": "repo-map.md", "collector": "repo_map"},
		map[string]any{"path": "baseline-rules.md", "collector": "example_brain_baseline"},
		map[string]any{"path": "todo-pattern.md", "collector": "literal"},
	}
	ordered := orderedNativeGuidanceShared(shared)
	if got := stringValue(mapValue(ordered[0])["collector"]); got != "example_brain_baseline" {
		t.Fatalf("first shared collector = %q", got)
	}
	baseline := nativeGuidanceBaselineContextPath(ordered)
	if baseline != "shared/baseline-rules.md" {
		t.Fatalf("baseline context path = %q", baseline)
	}
	tests := []struct {
		name    string
		context []string
		want    []string
	}{
		{
			name:    "durable guidance writer opts in",
			context: []string{"shared/repo-map.md", baseline, "docs/README.md"},
			want:    []string{baseline, "shared/repo-map.md", "docs/README.md"},
		},
		{
			name:    "planning dependency opts in",
			context: []string{"shared/request.md", baseline, "shared/repo-map.md"},
			want:    []string{baseline, "shared/request.md", "shared/repo-map.md"},
		},
		{
			name:    "general implementation task does not opt in",
			context: []string{"shared/request.md", "src/app.go", "docs/README.md"},
			want:    []string{"shared/request.md", "src/app.go", "docs/README.md"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := orderNativeGuidanceTaskContext(tt.context, baseline)
			if fmt.Sprint(got) != fmt.Sprint(tt.want) {
				t.Fatalf("context order = %#v want %#v", got, tt.want)
			}
		})
	}
}

func TestExampleBrainUnchangedConfigurationProvesTaskPackContract(t *testing.T) {
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not locate orchestrationhelper test source")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(testFile), "..", "..", "..", ".."))
	workflowPath := filepath.Join(repoRoot, "packages", "dorkpipe", "workflows", "example.brain", "config.yml")
	agentsPath := filepath.Join(filepath.Dir(workflowPath), "agents.yml")
	for _, path := range []string{workflowPath, agentsPath} {
		if info, err := os.Stat(path); err != nil || info.IsDir() {
			t.Fatalf("required Example Brain contract source %s is unavailable: %v", path, err)
		}
	}

	workflow := readYAMLMap(workflowPath)
	if stringValue(workflow["name"]) != "example.brain" {
		t.Fatalf("workflow name = %q", stringValue(workflow["name"]))
	}
	steps := listValue(workflow["steps"])
	stepByID := func(id string) map[string]any {
		t.Helper()
		for _, raw := range steps {
			step := mapValue(raw)
			if stringValue(step["id"]) == id {
				return step
			}
		}
		t.Fatalf("missing workflow step %q", id)
		return nil
	}
	wantStepOrder := []string{"plan", "auth_preflight", "stack_up", "run_workers", "merge", "verify", "approval", "apply"}
	gotStepOrder := make([]string, 0, len(steps))
	for _, raw := range steps {
		gotStepOrder = append(gotStepOrder, stringValue(mapValue(raw)["id"]))
	}
	assertStringOrder(t, "Example Brain hard-stage order", gotStepOrder, wantStepOrder)
	if stringValue(stepByID("approval")["run"]) != "scripts/dorkpipe/orchestrate-approve.sh" || stringValue(stepByID("apply")["run"]) != "scripts/dorkpipe/orchestrate-apply-results.sh" {
		t.Fatal("approval and apply must remain separate package-owned mechanics")
	}
	for _, forbidden := range []string{"publish", "sync"} {
		for _, id := range gotStepOrder {
			if id == forbidden {
				t.Fatalf("Example Brain must not define an implicit %s step", forbidden)
			}
		}
	}
	finally := listValue(workflow["finally"])
	if len(finally) != 1 || stringValue(mapValue(finally[0])["id"]) != "stack_down" {
		t.Fatalf("finally stages = %#v, want stack_down", finally)
	}

	vars := mapValue(workflow["vars"])
	for key, want := range map[string]string{
		"DORKPIPE_ORCH_MAX_TOTAL_CLOUD_TOKENS":  "120000",
		"DORKPIPE_ORCH_MAX_TASK_CLOUD_TOKENS":   "40000",
		"DORKPIPE_ORCH_STOP_ON_BUDGET_EXCEEDED": "true",
		"DORKPIPE_ORCH_CLOUD_LANES":             "true",
		"DORKPIPE_ORCH_BRAIN_PROVIDER":          "ollama",
	} {
		if got := stringValue(vars[key]); got != want {
			t.Fatalf("vars.%s = %q want %q", key, got, want)
		}
	}

	planStep := stepByID("plan")
	agent := mapValue(planStep["agent"])
	if !boolAny(agent["include_agents_md"]) || !strings.Contains(stringValue(agent["startup_prompt"]), "durable repo-native guidance") {
		t.Fatal("repo-defined startup guidance or AGENTS.md inclusion changed")
	}
	access := mapValue(agent["access"])
	assertStringOrder(t, "access.read", stringList(access["read"]), []string{"AGENTS.md", "docs", "src", "packages", "workflows"})
	assertStringOrder(t, "access.write", stringList(access["write"]), []string{"scope:artifacts:orchestrate"})
	assertStringOrder(t, "access.deny", stringList(access["deny"]), []string{".env", ".env.*", "**/*secret*", "**/*token*"})

	orchestration := mapValue(agent["orchestration"])
	if strings.TrimSpace(stringValue(mapValue(orchestration["request"])["text"])) == "" || len(listValue(mapValue(orchestration["plan"])["steps"])) != 4 {
		t.Fatal("repo-defined request and plan guidance must remain populated")
	}
	if _, present := orchestration["publish"]; present {
		t.Fatal("Example Brain orchestration must not define publish behavior")
	}
	if _, present := orchestration["sync"]; present {
		t.Fatal("Example Brain orchestration must not define sync behavior")
	}

	shared := listValue(orchestration["shared"])
	if len(shared) != 3 {
		t.Fatalf("shared collector count = %d want 3", len(shared))
	}
	assertStringOrder(t, "shared collectors", []string{
		stringValue(mapValue(shared[0])["collector"]),
		stringValue(mapValue(shared[1])["collector"]),
		stringValue(mapValue(shared[2])["collector"]),
	}, []string{"example_brain_baseline", "repo_map", "literal"})
	baseline := nativeGuidanceBaselineContextPath(shared)
	if baseline != "shared/baseline-rules.md" {
		t.Fatalf("baseline context path = %q", baseline)
	}

	agents := loadAgentsConfig(workflowPath)
	if len(agents) != 3 {
		t.Fatalf("sibling agents.yml role count = %d want 3", len(agents))
	}
	for id, workerType := range map[string]string{
		"planner_brain":         "planning",
		"guidance_architect":    "documentation",
		"repo_inventory_writer": "documentation",
	} {
		role := mapValue(agents[id])
		if stringValue(role["worker_type"]) != workerType || stringValue(role["work_mode"]) != "artifact" {
			t.Fatalf("role %s = %#v", id, role)
		}
	}

	tasks := listValue(orchestration["tasks"])
	if len(tasks) != 3 {
		t.Fatalf("task count = %d want 3", len(tasks))
	}
	taskByID := func(id string) map[string]any {
		t.Helper()
		for _, raw := range tasks {
			task := mapValue(raw)
			if stringValue(task["id"]) == id {
				return task
			}
		}
		t.Fatalf("missing Example Brain task %q", id)
		return nil
	}
	planner := taskByID("planner_brain")
	rulesWriter := taskByID("rules_writer")
	inventoryWriter := taskByID("inventory_writer")
	if len(listValue(planner["materialize_outputs"])) != 0 || !strings.Contains(stringValue(planner["expected_output"]), "Three concise bullets") {
		t.Fatal("planner_brain must remain a bounded planning response, not a materialized graph proposal")
	}
	for _, task := range []map[string]any{rulesWriter, inventoryWriter} {
		assertStringOrder(t, stringValue(task["id"])+" dependencies", stringList(task["depends_on"]), []string{"planner_brain"})
		if !containsString(taskContextPaths(task), "tasks/planner_brain/response.md") {
			t.Fatalf("task %s does not consume the bounded planner artifact", stringValue(task["id"]))
		}
	}

	contextTests := []struct {
		task map[string]any
		want []string
	}{
		{planner, []string{baseline, "shared/repo-map.md", "shared/todo-pattern.md"}},
		{rulesWriter, []string{baseline, "shared/repo-map.md", "tasks/planner_brain/response.md"}},
		{inventoryWriter, []string{baseline, "shared/repo-map.md", "shared/todo-pattern.md", "tasks/planner_brain/response.md", "AGENTS.md", "docs/README.md", "src/lib/domain/workflow.go", "packages/dorkpipe/package.yml", "workflows/README.md"}},
	}
	for _, tt := range contextTests {
		got := orderNativeGuidanceTaskContext(taskContextPaths(tt.task), baseline)
		assertStringOrder(t, stringValue(tt.task["id"])+" effective context", got, tt.want)
	}

	materializedPaths := []string{}
	for _, task := range []map[string]any{rulesWriter, inventoryWriter} {
		for _, raw := range listValue(task["materialize_outputs"]) {
			materializedPaths = append(materializedPaths, stringValue(mapValue(raw)["path"]))
		}
	}
	apply := mapValue(orchestration["apply"])
	if !boolAny(apply["require_approval"]) || stringValue(apply["target_root"]) != "docs/agents/brain" {
		t.Fatalf("apply contract = %#v", apply)
	}
	required := stringList(apply["required_artifacts"])
	assertStringOrder(t, "materialized files", materializedPaths, []string{"index.md", "source-of-truth-rules.md", "repo-knowledge.md", "open-gaps.md"})
	assertStringOrder(t, "required artifact floor", required, materializedPaths)

	artifactRoot := t.TempDir()
	writeMaterialized := func(taskID, rel string) {
		t.Helper()
		path := filepath.Join(artifactRoot, "tasks", taskID, "materialized", filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(rel), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeMaterialized("rules_writer", "index.md")
	writeMaterialized("rules_writer", "source-of-truth-rules.md")
	writeMaterialized("inventory_writer", "repo-knowledge.md")
	writeMaterialized("inventory_writer", "open-gaps.md")
	writeMaterialized("inventory_writer", "supplemental.md")
	inferred, err := inferApplyOutputs(repoRoot, artifactRoot, stringValue(apply["target_root"]), listValue(apply["required_artifacts"]))
	if err != nil {
		t.Fatal(err)
	}
	if len(inferred) != 5 {
		t.Fatalf("inferred apply output count = %d want 5", len(inferred))
	}
	wantTargets := []string{
		"docs/agents/brain/index.md",
		"docs/agents/brain/open-gaps.md",
		"docs/agents/brain/repo-knowledge.md",
		"docs/agents/brain/source-of-truth-rules.md",
		"docs/agents/brain/supplemental.md",
	}
	gotTargets := make([]string, 0, len(inferred))
	for _, raw := range inferred {
		gotTargets = append(gotTargets, stringValue(mapValue(raw)["path"]))
	}
	assertStringOrder(t, "required floor plus inferred unique extra", gotTargets, wantTargets)

	merge := mapValue(orchestration["merge"])
	verify := mapValue(orchestration["verify"])
	if stringValue(merge["title"]) != "Example Brain Starter" || len(listValue(merge["summary_points"])) != 3 || strings.TrimSpace(stringValue(verify["next_action_default"])) == "" {
		t.Fatal("merge or verification guidance changed")
	}
}

func TestRenderSharedExampleBrainBaselinePreservesSourcePrecedence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "baseline-rules.md")
	want := "# Baseline\n\n- direct code before summary prose\n- intended direction stays separate\n"
	if err := os.WriteFile(path, []byte(want), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := renderShared(map[string]any{"collector": "example_brain_baseline"}, dir, map[string]string{
		"DORKPIPE_ORCH_EXAMPLE_BRAIN_BASELINE": path,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("baseline = %q want %q", got, want)
	}
}

func TestNormalizeDurableOutputPreservesRepoNativeReferences(t *testing.T) {
	root := t.TempDir()
	want := "See `docs/architecture.md`, `/api/workflows`, and https://example.com/home/guide.md.\n"
	got, err := normalizeDurableOutput(want, root, "")
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("normalized output = %q want %q", got, want)
	}
}

func TestNormalizeDurableOutputRewritesMappedRuntimeReferences(t *testing.T) {
	root := t.TempDir()
	docsRoot := filepath.Join(root, "docs")
	if err := os.MkdirAll(docsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	mounts := docsRoot + ":/RepoDocs:ro"
	input := "Use `/work/AGENTS.md` first, then `/RepoDocs/architecture.md`.\n"
	got, err := normalizeDurableOutput(input, root, mounts)
	if err != nil {
		t.Fatal(err)
	}
	want := "Use `AGENTS.md` first, then `docs/architecture.md`.\n"
	if got != want {
		t.Fatalf("normalized output = %q want %q", got, want)
	}
}

func TestNormalizeDurableOutputRejectsAmbiguousRuntimeReference(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "docs", "first")
	second := filepath.Join(root, "docs", "second")
	mounts := first + ":/DesignNotes:ro\n" + second + ":/DesignNotes:ro"
	if _, err := normalizeDurableOutput("See `/DesignNotes/decision.md`.\n", root, mounts); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("expected ambiguous runtime reference failure, got %v", err)
	}
}

func TestNormalizeDurableOutputRejectsExternalMountReference(t *testing.T) {
	root := t.TempDir()
	external := t.TempDir()
	mounts := external + ":/DesignNotes:ro"
	if _, err := normalizeDurableOutput("See `/DesignNotes/decision.md`.\n", root, mounts); err == nil || !strings.Contains(err.Error(), "outside the consumer repository") {
		t.Fatalf("expected external mount failure, got %v", err)
	}
}

func TestNormalizeDurableOutputRejectsMachineHostPathDisclosure(t *testing.T) {
	root := t.TempDir()
	for _, external := range []string{filepath.Join(t.TempDir(), "decision.md"), filepath.Join(t.TempDir(), "private")} {
		input := fmt.Sprintf("See `%s`.\n", external)
		if _, err := normalizeDurableOutput(input, root, ""); err == nil || !strings.Contains(err.Error(), "machine host path") {
			t.Fatalf("expected machine host path failure for %q, got %v", external, err)
		}
	}
}

func TestNormalizeDurableOutputRejectsOrchestrationTerminology(t *testing.T) {
	root := t.TempDir()
	for _, input := range []string{
		"The provider lane produced this page.\n",
		"Copy the worker artifact into the artifact root.\n",
		"The source packet is authoritative.\n",
	} {
		if _, err := normalizeDurableOutput(input, root, ""); err == nil || !strings.Contains(err.Error(), "orchestration-only terminology") {
			t.Fatalf("expected terminology failure for %q, got %v", input, err)
		}
	}
}

func TestMaterializeTaskOutputsPreflightsDurablePolicyBeforeWriting(t *testing.T) {
	dir := t.TempDir()
	responsePath := filepath.Join(dir, "response.md")
	response := strings.Join([]string{
		`<!-- dorkpipe:file path="first.md" -->`,
		"# Valid",
		`<!-- /dorkpipe:file -->`,
		`<!-- dorkpipe:file path="second.md" -->`,
		"The provider lane produced this page.",
		`<!-- /dorkpipe:file -->`,
	}, "\n")
	if err := os.WriteFile(responsePath, []byte(response), 0o644); err != nil {
		t.Fatal(err)
	}
	err := materializeTaskOutputs(responsePath, dir, `[{"path":"first.md"},{"path":"second.md"}]`, filepath.Join(dir, "result.json"), dir, "")
	if err == nil {
		t.Fatal("expected durable output policy failure")
	}
	if _, statErr := os.Stat(filepath.Join(dir, "materialized", "first.md")); !os.IsNotExist(statErr) {
		t.Fatalf("first output should not be written before all durable outputs pass, err=%v", statErr)
	}
}

func TestInferTaskOutputPathPrefersExplicitThenExpectedOutput(t *testing.T) {
	if got := inferTaskOutputPath(map[string]any{
		"output_path":     "/work/docs/explicit.md",
		"expected_output": "Write /work/docs/fallback.md",
	}); got != "/work/docs/explicit.md" {
		t.Fatalf("explicit output path = %q", got)
	}
	if got := inferTaskOutputPath(map[string]any{
		"expected_output": "Update canonical doc at /work/docs/agents/index.md and keep links valid.",
	}); got != "/work/docs/agents/index.md" {
		t.Fatalf("inferred output path = %q", got)
	}
}

func TestEmitRequiredAuthProviders(t *testing.T) {
	tasksDir := filepath.Join(t.TempDir(), "tasks")
	if err := os.MkdirAll(filepath.Join(tasksDir, "need-claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tasksDir, "need-codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tasksDir, "prefer-claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tasksDir, "local-ollama"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tasksDir, "need-claude", "task.json"), []byte(`{"worker":"claude","worker_policy":{"mode":"require"},"lane":{"provider":"claude"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tasksDir, "need-codex", "task.json"), []byte(`{"worker":"codex","worker_policy":{"mode":"require"},"lane":{"provider":"codex"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tasksDir, "prefer-claude", "task.json"), []byte(`{"worker":"claude","worker_policy":{"mode":"prefer"},"lane":{"provider":"claude"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tasksDir, "local-ollama", "task.json"), []byte(`{"worker":"ollama","worker_policy":{"mode":"require"},"lane":{"provider":"ollama"}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	if err := emitRequiredAuthProviders(tasksDir, &stdout); err != nil {
		t.Fatal(err)
	}
	got := strings.Fields(stdout.String())
	want := []string{"claude", "codex"}
	if len(got) != len(want) {
		t.Fatalf("required providers = %#v want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("required providers = %#v want %#v", got, want)
		}
	}
}

func TestResolveDockpipeCommandPrefersEnv(t *testing.T) {
	got := resolveDockpipeCommand(t.TempDir(), map[string]string{"DOCKPIPE_BIN": "/custom/dockpipe"})
	if got != "/custom/dockpipe" {
		t.Fatalf("resolveDockpipeCommand() = %q", got)
	}
}

func TestResolveDockpipeCommandFallsBackToRepoLocalBinary(t *testing.T) {
	root := t.TempDir()
	binDir := filepath.Join(root, "src", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(binDir, "dockpipe")
	if runtime.GOOS == "windows" {
		want += ".exe"
	}
	if err := os.WriteFile(want, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	got := resolveDockpipeCommand(root, map[string]string{})
	if got != want {
		t.Fatalf("resolveDockpipeCommand() = %q want %q", got, want)
	}
}

func TestResolveApplyTargetPathMapsAllowedGuestMount(t *testing.T) {
	root := t.TempDir()
	uniteHere := filepath.Join(root, "UniteHere")
	designNotes := filepath.Join(root, "DesignNotes")
	t.Setenv("DOCKPIPE_CONTAINER_MOUNTS", uniteHere+":/UniteHere:ro\n"+designNotes+":/DesignNotes:ro")
	t.Setenv("DORKPIPE_ORCH_APPLY_ALLOWED_GUEST_ROOTS", "/UniteHere")

	gotPath, gotRoot, err := resolveApplyTargetPath(root, "/UniteHere/docs/agents/plans/brain.md")
	if err != nil {
		t.Fatal(err)
	}
	wantPath := filepath.Join(uniteHere, "docs", "agents", "plans", "brain.md")
	if gotPath != wantPath {
		t.Fatalf("target path = %q want %q", gotPath, wantPath)
	}
	if gotRoot != uniteHere {
		t.Fatalf("target root = %q want %q", gotRoot, uniteHere)
	}
}

func TestResolveApplyTargetPathAllowsGitBashConvertedGuestRoot(t *testing.T) {
	root := t.TempDir()
	uniteHere := filepath.Join(root, "UniteHere")
	t.Setenv("DOCKPIPE_CONTAINER_MOUNTS", uniteHere+":/UniteHere:ro")
	t.Setenv("DORKPIPE_ORCH_APPLY_ALLOWED_GUEST_ROOTS", "C:/Program Files/Git/UniteHere")

	gotPath, gotRoot, err := resolveApplyTargetPath(root, "/UniteHere/docs/agents/plans/brain.md")
	if err != nil {
		t.Fatal(err)
	}
	wantPath := filepath.Join(uniteHere, "docs", "agents", "plans", "brain.md")
	if gotPath != wantPath {
		t.Fatalf("target path = %q want %q", gotPath, wantPath)
	}
	if gotRoot != uniteHere {
		t.Fatalf("target root = %q want %q", gotRoot, uniteHere)
	}
}

func TestResolveApplyTargetPathRejectsDisallowedGuestMount(t *testing.T) {
	root := t.TempDir()
	t.Setenv("DOCKPIPE_CONTAINER_MOUNTS", filepath.Join(root, "UniteHere")+":/UniteHere:ro\n"+filepath.Join(root, "DesignNotes")+":/DesignNotes:ro")
	t.Setenv("DORKPIPE_ORCH_APPLY_ALLOWED_GUEST_ROOTS", "/UniteHere")

	if _, _, err := resolveApplyTargetPath(root, "/DesignNotes/planning/generated.md"); err == nil {
		t.Fatal("expected disallowed guest mount apply target to fail")
	}
}

func TestResolveApplyTargetPathFallsBackToWorkflowRootForWorkGuestPath(t *testing.T) {
	root := t.TempDir()
	t.Setenv("DOCKPIPE_CONTAINER_MOUNTS", filepath.Join(root, "DesignNotes")+":/DesignNotes:ro")
	t.Setenv("DORKPIPE_ORCH_APPLY_ALLOWED_GUEST_ROOTS", "/work")

	gotPath, gotRoot, err := resolveApplyTargetPath(root, "/work/docs/agents/brain/index.md")
	if err != nil {
		t.Fatal(err)
	}
	wantPath := filepath.Join(root, "docs", "agents", "brain", "index.md")
	if gotPath != wantPath {
		t.Fatalf("target path = %q want %q", gotPath, wantPath)
	}
	if gotRoot != root {
		t.Fatalf("target root = %q want %q", gotRoot, root)
	}
}

func TestMountedGuestRootNotesKeepsHostPathsOutOfGuidance(t *testing.T) {
	notes := mountedGuestRootNotes("C:\\docs\\UniteHere\\UH - SePuede - Design Notes:/DesignNotes:ro\nC:\\Source\\UniteHere:/work:rw")
	got := strings.Join(notes, "\n")
	for _, want := range []string{
		"`/DesignNotes` is a stable source-packet label",
		"Durable docs must cite a repo-native reference proven by an explicit source mapping",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("mounted guest root notes missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "C:\\docs") || strings.Contains(got, "Host path") {
		t.Fatalf("mounted guest root notes disclosed a machine host path:\n%s", got)
	}
	if strings.Contains(got, "`/work` is an external mounted") {
		t.Fatalf("mounted guest root notes should not emit a standalone /work mount note:\n%s", got)
	}
}

func TestHasSchedulerOutputConflictMatchesSameTarget(t *testing.T) {
	running := map[string]schedulerTask{
		"author_index": {ID: "author_index", OutputPath: "/work/docs/agents/index.md"},
	}
	if !hasSchedulerOutputConflict(schedulerTask{ID: "finalize_index", OutputPath: "/work/docs/agents/index.md"}, running) {
		t.Fatal("expected same output path to conflict")
	}
	if hasSchedulerOutputConflict(schedulerTask{ID: "author_repo", OutputPath: "/work/docs/agents/repo.md"}, running) {
		t.Fatal("did not expect different output path to conflict")
	}
}

func TestApplyResultsPreflightsAllSourcesBeforeWriting(t *testing.T) {
	root := t.TempDir()
	artifactRoot := filepath.Join(root, "artifacts")
	if err := os.MkdirAll(filepath.Join(artifactRoot, "merge"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(artifactRoot, "merge", "first.md"), []byte("first"), 0o644); err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(artifactRoot, "plan.json")
	approvalPath := filepath.Join(artifactRoot, "approval.md")
	resultPath := filepath.Join(artifactRoot, "apply.json")
	if err := os.WriteFile(planPath, []byte(`{"apply":{"require_approval":true,"outputs":[{"source":"merge/first.md","path":"out/first.md"},{"source":"merge/missing.md","path":"out/missing.md"}]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(approvalPath, []byte("- Approved: yes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := applyResults(root, artifactRoot, planPath, approvalPath, resultPath); err == nil {
		t.Fatal("expected missing second source to fail")
	}
	if _, err := os.Stat(filepath.Join(root, "out", "first.md")); !os.IsNotExist(err) {
		t.Fatalf("first output should not be written before all sources preflight, err=%v", err)
	}
}

func TestApplyResultsAllowsReviewForWorkspaceDiff(t *testing.T) {
	root := t.TempDir()
	artifactRoot := filepath.Join(root, "artifacts")
	if err := os.MkdirAll(filepath.Join(artifactRoot, "merge"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(artifactRoot, "merge", "first.md"), []byte("first"), 0o644); err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(artifactRoot, "plan.json")
	approvalPath := filepath.Join(artifactRoot, "approval.md")
	verifyPath := filepath.Join(artifactRoot, "verify.json")
	resultPath := filepath.Join(artifactRoot, "apply.json")
	if err := os.WriteFile(planPath, []byte(`{"apply":{"require_approval":true,"outputs":[{"source":"merge/first.md","path":"out/first.md"}]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(approvalPath, []byte("- Approved: yes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(verifyPath, []byte(`{"status":"review"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := applyResultsWithVerify(root, artifactRoot, planPath, approvalPath, resultPath, verifyPath, false); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(root, "out", "first.md")); err != nil || string(got) != "first" {
		t.Fatalf("output = %q err=%v", string(got), err)
	}
	result := readJSONMap(resultPath)
	if got := stringValue(result["status"]); got != "applied" {
		t.Fatalf("status = %q want applied", got)
	}
	if got := stringValue(result["verify_status"]); got != "review" {
		t.Fatalf("verify_status = %q want review", got)
	}
	if !boolAny(result["requires_human_review"]) {
		t.Fatal("expected requires_human_review for review-status apply")
	}
	if boolAny(result["publish_allowed"]) {
		t.Fatal("review-status apply should not mark publish_allowed")
	}
}

func TestApplyResultsSkipsWhenVerifyStatusIsFail(t *testing.T) {
	root := t.TempDir()
	artifactRoot := filepath.Join(root, "artifacts")
	if err := os.MkdirAll(filepath.Join(artifactRoot, "merge"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(artifactRoot, "merge", "first.md"), []byte("first"), 0o644); err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(artifactRoot, "plan.json")
	approvalPath := filepath.Join(artifactRoot, "approval.md")
	verifyPath := filepath.Join(artifactRoot, "verify.json")
	resultPath := filepath.Join(artifactRoot, "apply.json")
	if err := os.WriteFile(planPath, []byte(`{"apply":{"require_approval":true,"outputs":[{"source":"merge/first.md","path":"out/first.md"}]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(approvalPath, []byte("- Approved: yes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(verifyPath, []byte(`{"status":"fail"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := applyResultsWithVerify(root, artifactRoot, planPath, approvalPath, resultPath, verifyPath, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "out", "first.md")); !os.IsNotExist(err) {
		t.Fatalf("output should not be written when verify status is fail, err=%v", err)
	}
	result := readJSONMap(resultPath)
	if got := stringValue(result["status"]); got != "skipped" {
		t.Fatalf("status = %q want skipped", got)
	}
	if got := stringValue(result["verify_status"]); got != "fail" {
		t.Fatalf("verify_status = %q want fail", got)
	}
}

func TestApplyResultsInfersMaterializedOutputsFromTargetRoot(t *testing.T) {
	root := t.TempDir()
	artifactRoot := filepath.Join(root, "artifacts")
	materializedDir := filepath.Join(artifactRoot, "tasks", "writer", "materialized")
	if err := os.MkdirAll(materializedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(materializedDir, "index.md"), []byte("index"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(materializedDir, "open-gaps.md"), []byte("gaps"), 0o644); err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(artifactRoot, "plan.json")
	approvalPath := filepath.Join(artifactRoot, "approval.md")
	resultPath := filepath.Join(artifactRoot, "apply.json")
	if err := os.WriteFile(planPath, []byte(`{"apply":{"require_approval":true,"target_root":"docs/agents/brain","required_artifacts":["index.md"]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(approvalPath, []byte("- Approved: yes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := applyResults(root, artifactRoot, planPath, approvalPath, resultPath); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(root, "docs", "agents", "brain", "index.md")); err != nil || string(got) != "index" {
		t.Fatalf("index.md = %q err=%v", string(got), err)
	}
	if got, err := os.ReadFile(filepath.Join(root, "docs", "agents", "brain", "open-gaps.md")); err != nil || string(got) != "gaps" {
		t.Fatalf("open-gaps.md = %q err=%v", string(got), err)
	}
}

func TestApplyResultsFailsWhenRequiredInferredArtifactMissing(t *testing.T) {
	root := t.TempDir()
	artifactRoot := filepath.Join(root, "artifacts")
	materializedDir := filepath.Join(artifactRoot, "tasks", "writer", "materialized")
	if err := os.MkdirAll(materializedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(materializedDir, "index.md"), []byte("index"), 0o644); err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(artifactRoot, "plan.json")
	approvalPath := filepath.Join(artifactRoot, "approval.md")
	resultPath := filepath.Join(artifactRoot, "apply.json")
	if err := os.WriteFile(planPath, []byte(`{"apply":{"require_approval":true,"target_root":"docs/agents/brain","required_artifacts":["index.md","source-of-truth-rules.md"]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(approvalPath, []byte("- Approved: yes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := applyResults(root, artifactRoot, planPath, approvalPath, resultPath); err == nil {
		t.Fatal("expected missing required inferred artifact to fail")
	}
}

func TestEmitVerifyApplyCoherenceFlagsBrokenMarkdownAndYamlTargets(t *testing.T) {
	root := t.TempDir()
	artifactRoot := filepath.Join(root, "artifacts")
	if err := os.MkdirAll(filepath.Join(artifactRoot, "merge"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(artifactRoot, "merge", "index.md"), []byte("[Missing](./missing.md)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(artifactRoot, "merge", "index.yaml"), []byte("canonical: ./missing.md\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(artifactRoot, "plan.json")
	if err := os.WriteFile(planPath, []byte(`{"apply":{"outputs":[{"source":"merge/index.md","path":"docs/index.md"},{"source":"merge/index.yaml","path":"docs/index.yaml"}]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	if err := emitVerifyApplyCoherence(root, artifactRoot, planPath, `[]`, &out); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "VERIFY_APPLY_STATUS='fail'") {
		t.Fatalf("expected fail status, got:\n%s", got)
	}
	for _, want := range []string{"markdown link target is missing", "yaml reference target is missing"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in:\n%s", want, got)
		}
	}
}

func TestEmitVerifyApplyCoherencePreservesPriorReviewWithoutBlockingApply(t *testing.T) {
	root := t.TempDir()
	artifactRoot := filepath.Join(root, "artifacts")
	if err := os.MkdirAll(filepath.Join(artifactRoot, "merge"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(artifactRoot, "merge", "index.md"), []byte("# Valid\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(artifactRoot, "plan.json")
	if err := os.WriteFile(planPath, []byte(`{"apply":{"outputs":[{"source":"merge/index.md","path":"docs/index.md"}]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	if err := emitVerifyApplyCoherence(root, artifactRoot, planPath, `["heuristic review"]`, &out); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "VERIFY_APPLY_STATUS='pass'") {
		t.Fatalf("expected pass status for valid staged output, got:\n%s", got)
	}
	if !strings.Contains(got, "heuristic review") {
		t.Fatalf("expected inherited issue to be preserved, got:\n%s", got)
	}
}

func TestEmitVerifyApplyCoherenceFlagsContradictoryValidationClaim(t *testing.T) {
	root := t.TempDir()
	artifactRoot := filepath.Join(root, "artifacts")
	if err := os.MkdirAll(filepath.Join(artifactRoot, "merge"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "source-of-truth.md"), []byte("still here\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(artifactRoot, "merge", "validation.md"), []byte("- **Removed `source-of-truth.md`**\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(artifactRoot, "plan.json")
	if err := os.WriteFile(planPath, []byte(`{"apply":{"outputs":[{"source":"merge/validation.md","path":"docs/validation.md"}]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	if err := emitVerifyApplyCoherence(root, artifactRoot, planPath, `[]`, &out); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "still exists") {
		t.Fatalf("expected contradictory validation claim issue, got:\n%s", got)
	}
}

func TestBuildMergeResultUsesTaskResultObjects(t *testing.T) {
	dir := t.TempDir()
	mainPath := filepath.Join(dir, "main.json")
	planPath := filepath.Join(dir, "planning.json")
	outPath := filepath.Join(dir, "merge.json")
	if err := os.WriteFile(mainPath, []byte(`{"task_id":"codex_brain_plan","provider_actual":"codex","summary":"done","confidence":0.8,"estimated_input_tokens":10,"estimated_output_tokens":5,"estimated_total_tokens":15,"duration_ms":100}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(planPath, []byte(`{"task_id":"repo_knowledge","provider_actual":"ollama","summary":"planned","confidence":0.6,"estimated_input_tokens":4,"estimated_output_tokens":2,"estimated_total_tokens":6,"duration_ms":20}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := buildMergeResult(outPath, []string{mainPath, "--planning", planPath}); err != nil {
		t.Fatal(err)
	}
	result := readJSONMap(outPath)
	tasks := listValue(result["tasks"])
	if len(tasks) != 1 {
		t.Fatalf("tasks length = %d want 1", len(tasks))
	}
	task := mapValue(tasks[0])
	if got := stringValue(task["task_id"]); got != "codex_brain_plan" {
		t.Fatalf("task_id = %q", got)
	}
	if got := intAny(result["total_estimated_task_tokens"]); got != 15 {
		t.Fatalf("total_estimated_task_tokens = %d", got)
	}
	planning := listValue(result["planning_tasks"])
	if len(planning) != 1 {
		t.Fatalf("planning length = %d want 1", len(planning))
	}
}

func TestBuildVerifyResultAddsValueBarAndRerunTargets(t *testing.T) {
	dir := t.TempDir()
	planPath := filepath.Join(dir, "plan.json")
	graphPath := filepath.Join(dir, "graph.json")
	mergePath := filepath.Join(dir, "merge.json")
	usagePath := filepath.Join(dir, "cloud-usage.json")
	haltPath := filepath.Join(dir, "halt.json")
	outPath := filepath.Join(dir, "verify.json")
	if err := writeJSONFile(planPath, map[string]any{
		"apply": map[string]any{
			"require_approval": true,
			"outputs": []map[string]any{
				{"source": "tasks/author/response.md", "path": "/work/docs/brain.md"},
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := writeJSONFile(graphPath, map[string]any{
		"tasks": []map[string]any{
			{"id": "extract", "worker_type": "extraction", "provider": "ollama"},
			{"id": "architect", "worker_type": "architecture", "provider": "claude", "depends_on": []string{"extract"}},
			{"id": "author", "worker_type": "authoring", "provider": "codex", "depends_on": []string{"architect"}, "output_path": "/work/docs/brain.md"},
			{"id": "validator", "worker_type": "validation", "provider": "claude", "depends_on": []string{"author"}},
			{"id": "merge_final", "worker_type": "merge", "depends_on": []string{"validator"}},
			{"id": "verify_final", "worker_type": "verify", "depends_on": []string{"merge_final"}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := writeJSONFile(mergePath, map[string]any{
		"average_confidence": 0.71,
		"tasks": []map[string]any{
			{"task_id": "extract", "status": "ok", "provider_actual": "ollama", "used_live_model": true, "confidence": 0.7},
			{"task_id": "architect", "status": "ok", "provider_actual": "claude", "used_live_model": true, "confidence": 0.8},
			{"task_id": "author", "status": "ok", "provider_actual": "codex", "used_live_model": true, "confidence": 0.75},
			{"task_id": "validator", "status": "ok", "provider_actual": "claude", "used_live_model": true, "confidence": 0.7},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := writeJSONFile(usagePath, map[string]any{"total_estimated_tokens": 4200}); err != nil {
		t.Fatal(err)
	}

	if err := buildVerifyResult(outPath, planPath, graphPath, mergePath, usagePath, haltPath, "pass", "0.71", `["author: markdown link target is missing: missing.md"]`, "review links", map[string]string{}); err != nil {
		t.Fatal(err)
	}
	result := readJSONMap(outPath)
	if got := stringValue(result["status"]); got != "review" {
		t.Fatalf("status = %q want review", got)
	}
	if got := stringValue(result["failure_class"]); got != "broken_references" {
		t.Fatalf("failure_class = %q want broken_references", got)
	}
	rerun := stringList(result["recommended_rerun_tasks"])
	if len(rerun) != 1 || rerun[0] != "author" {
		t.Fatalf("recommended_rerun_tasks = %#v want [author]", rerun)
	}
	valueBar := mapValue(result["value_bar"])
	if got := stringValue(valueBar["verdict"]); got != "strong_orchestration_value" {
		t.Fatalf("value_bar verdict = %q", got)
	}
	baseline := mapValue(result["direct_worker_baseline"])
	if got := stringValue(baseline["verdict"]); got != "orchestration_adds_value" {
		t.Fatalf("baseline verdict = %q", got)
	}
}

func TestBuildVerifyResultFlagsLowValueSerialGraph(t *testing.T) {
	dir := t.TempDir()
	planPath := filepath.Join(dir, "plan.json")
	graphPath := filepath.Join(dir, "graph.json")
	mergePath := filepath.Join(dir, "merge.json")
	usagePath := filepath.Join(dir, "cloud-usage.json")
	haltPath := filepath.Join(dir, "halt.json")
	outPath := filepath.Join(dir, "verify.json")
	if err := writeJSONFile(planPath, map[string]any{"apply": map[string]any{"require_approval": false}}); err != nil {
		t.Fatal(err)
	}
	if err := writeJSONFile(graphPath, map[string]any{
		"tasks": []map[string]any{
			{"id": "a", "worker_type": "analysis", "provider": "codex"},
			{"id": "b", "worker_type": "analysis", "provider": "codex", "depends_on": []string{"a"}},
			{"id": "c", "worker_type": "analysis", "provider": "codex", "depends_on": []string{"b"}},
			{"id": "d", "worker_type": "analysis", "provider": "codex", "depends_on": []string{"c"}},
			{"id": "merge_final", "worker_type": "merge", "depends_on": []string{"d"}},
			{"id": "verify_final", "worker_type": "verify", "depends_on": []string{"merge_final"}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := writeJSONFile(mergePath, map[string]any{
		"average_confidence": 0.82,
		"tasks": []map[string]any{
			{"task_id": "a", "status": "ok", "provider_actual": "codex", "used_live_model": true, "confidence": 0.8},
			{"task_id": "b", "status": "ok", "provider_actual": "codex", "used_live_model": true, "confidence": 0.8},
			{"task_id": "c", "status": "ok", "provider_actual": "codex", "used_live_model": true, "confidence": 0.8},
			{"task_id": "d", "status": "ok", "provider_actual": "codex", "used_live_model": true, "confidence": 0.8},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := writeJSONFile(usagePath, map[string]any{"total_estimated_tokens": 9000}); err != nil {
		t.Fatal(err)
	}

	if err := buildVerifyResult(outPath, planPath, graphPath, mergePath, usagePath, haltPath, "pass", "0.82", `[]`, "review", map[string]string{}); err != nil {
		t.Fatal(err)
	}
	result := readJSONMap(outPath)
	if got := stringValue(result["status"]); got != "review" {
		t.Fatalf("status = %q want review", got)
	}
	if got := stringValue(result["failure_class"]); got != "low_value_graph" {
		t.Fatalf("failure_class = %q want low_value_graph", got)
	}
	baseline := mapValue(result["direct_worker_baseline"])
	if got := stringValue(baseline["verdict"]); got != "direct_worker_likely_better" {
		t.Fatalf("baseline verdict = %q", got)
	}
	graphLint := mapValue(result["graph_lint"])
	if got := stringValue(graphLint["status"]); got != "review" {
		t.Fatalf("graph_lint status = %q", got)
	}
}

func TestOllamaChatRequestAndResponseHelpers(t *testing.T) {
	dir := t.TempDir()
	promptPath := filepath.Join(dir, "prompt.md")
	requestPath := filepath.Join(dir, "request.json")
	responsePath := filepath.Join(dir, "response.json")
	outPath := filepath.Join(dir, "response.md")
	if err := os.WriteFile(promptPath, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeOllamaChatRequest("llama-test", promptPath, requestPath); err != nil {
		t.Fatal(err)
	}
	request := readJSONMap(requestPath)
	if got := stringValue(request["model"]); got != "llama-test" {
		t.Fatalf("model = %q", got)
	}
	if err := os.WriteFile(responsePath, []byte(`{"message":{"content":"useful response"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeOllamaChatResponse(responsePath, outPath); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(mustReadFile(t, outPath))); got != "useful response" {
		t.Fatalf("response = %q", got)
	}
}

func TestEmitVerifySummaryEnvCountsFallbackTasks(t *testing.T) {
	dir := t.TempDir()
	mergePath := filepath.Join(dir, "merge.json")
	if err := os.WriteFile(mergePath, []byte(`{"average_confidence":0.55,"tasks":[{"used_live_model":false},{"used_live_model":true},{"used_live_model":false}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	if err := emitVerifySummaryEnv(mergePath, &out); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{"VERIFY_LIVE_COUNT='1'", "VERIFY_FALLBACK_COUNT='2'", "VERIFY_AVG_CONFIDENCE='0.55'"} {
		if !strings.Contains(got, want) {
			t.Fatalf("verify env missing %q in:\n%s", want, got)
		}
	}
}

func TestExecutableSearchPathEntriesNormalizesGitBashWindowsPath(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Git Bash PATH normalization is Windows-specific")
	}
	entries := executableSearchPathEntries("/c/Program Files/Docker/Docker/resources/bin:/usr/bin")
	if len(entries) == 0 || entries[0] != `C:\Program Files\Docker\Docker\resources\bin` {
		t.Fatalf("entries = %#v", entries)
	}
}

func TestWindowsExecutableFallbackDirsIncludesDockerDesktop(t *testing.T) {
	dirs := windowsExecutableFallbackDirs("docker")
	if len(dirs) == 0 {
		t.Fatal("expected docker fallback dirs")
	}
	if dirs[0] != `C:\Program Files\Docker\Docker\resources\bin` {
		t.Fatalf("first fallback dir = %q", dirs[0])
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
