package application

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
)

var doctorBashLookPathFn = exec.LookPath
var doctorDockerCheckFn = infrastructure.DockerDoctorCheck
var doctorRepoRootFn = infrastructure.RepoRoot
var doctorResolveWorkflowConfigPathFn = infrastructure.ResolveWorkflowConfigPath
var doctorGetwdFn = os.Getwd
var doctorLoadProjectConfigFn = loadDockpipeProjectConfig
var doctorStatFn = os.Stat

func cmdDoctor(argv []string) error {
	if len(argv) > 0 && (argv[0] == "-h" || argv[0] == "--help") {
		fmt.Print(`dockpipe doctor — verify bash, Docker, and bundled assets

Quick checks before a real run. Does not start a project container.

`)
		return nil
	}

	var errs []error
	optionalIssues := 0

	if p, err := doctorBashLookPathFn("bash"); err == nil {
		logDoctorResult("doctor.bash", infrastructure.OperationStatusDone, map[string]string{
			"required": "true",
			"result":   "available",
			"tool":     "bash",
			"path":     p,
		}, "")
	} else {
		bashErr := "bash not found in PATH"
		logDoctorResult("doctor.bash", infrastructure.OperationStatusFail, map[string]string{
			"required": "true",
			"tool":     "bash",
		}, bashErr)
		errs = append(errs, fmt.Errorf("%s", bashErr))
	}

	if err := doctorDockerCheckFn(os.Stderr); err != nil {
		logDoctorResult("doctor.docker", infrastructure.OperationStatusFail, map[string]string{
			"required": "true",
			"tool":     "docker",
		}, err.Error())
		errs = append(errs, err)
	} else {
		logDoctorResult("doctor.docker", infrastructure.OperationStatusDone, map[string]string{
			"required": "true",
			"result":   "reachable",
			"tool":     "docker",
		}, "")
	}

	rr, err := doctorRepoRootFn()
	if err != nil {
		optionalIssues++
		logDoctorResult("doctor.assets", infrastructure.OperationStatusFail, map[string]string{
			"required": "false",
			"asset":    "bundled_workflow",
			"workflow": "run",
		}, err.Error())
	} else {
		if wfPath, err := doctorResolveWorkflowConfigPathFn(rr, "run"); err != nil {
			optionalIssues++
			logDoctorResult("doctor.assets", infrastructure.OperationStatusFail, map[string]string{
				"required":  "false",
				"asset":     "bundled_workflow",
				"repo_root": rr,
				"workflow":  "run",
			}, err.Error())
		} else {
			logDoctorResult("doctor.assets", infrastructure.OperationStatusDone, map[string]string{
				"required":        "false",
				"asset":           "bundled_workflow",
				"repo_root":       rr,
				"result":          "ok",
				"workflow":        "run",
				"workflow_config": wfPath,
			}, "")
		}
	}

	wd, err := doctorGetwdFn()
	if err != nil {
		optionalIssues++
		logDoctorResult("doctor.project_config", infrastructure.OperationStatusFail, map[string]string{
			"required": "false",
		}, err.Error())
	} else {
		configPath := filepath.Join(wd, domain.DockpipeProjectConfigFileName)
		pc, err := doctorLoadProjectConfigFn(wd)
		if err != nil {
			optionalIssues++
			logDoctorResult("doctor.project_config", infrastructure.OperationStatusFail, map[string]string{
				"config_path": configPath,
				"required":    "false",
			}, err.Error())
		} else if pc == nil {
			logDoctorResult("doctor.project_config", infrastructure.OperationStatusDone, map[string]string{
				"config_path": configPath,
				"required":    "false",
				"result":      "missing",
			}, "")
		} else {
			projectIDs := map[string]string{
				"config_path": configPath,
				"required":    "false",
				"result":      "present",
			}
			if pc.Secrets.Vault != nil && strings.TrimSpace(*pc.Secrets.Vault) != "" {
				projectIDs["vault_default"] = strings.TrimSpace(*pc.Secrets.Vault)
			}
			if pc.Secrets.Notes != nil && strings.TrimSpace(*pc.Secrets.Notes) != "" {
				projectIDs["notes"] = "present"
			}
			logDoctorResult("doctor.project_config", infrastructure.OperationStatusDone, projectIDs, "")
			if p, ok := domain.ResolveVaultTemplatePath(pc, wd); ok {
				if st, err := doctorStatFn(p); err == nil && !st.IsDir() {
					logDoctorResult("doctor.vault_template", infrastructure.OperationStatusDone, map[string]string{
						"required": "false",
						"result":   "present",
						"template": p,
					}, "")
					if _, err := opLookPathFn("op"); err == nil {
						logDoctorResult("doctor.vault_cli", infrastructure.OperationStatusDone, map[string]string{
							"required": "false",
							"result":   "available",
							"template": p,
							"tool":     "op",
						}, "")
						fmt.Fprintln(os.Stderr, "[dockpipe] doctor: op inject is available for workflow-start vault resolution")
					} else {
						optionalIssues++
						logDoctorResult("doctor.vault_cli", infrastructure.OperationStatusFail, map[string]string{
							"required": "false",
							"template": p,
							"tool":     "op",
						}, "op not in PATH")
						fmt.Fprintln(os.Stderr, "[dockpipe] doctor: install 1Password CLI for op inject, or set DOCKPIPE_OP_INJECT=0 to skip")
					}
				} else {
					optionalIssues++
					templateErr := "vault template missing"
					if err != nil {
						templateErr = err.Error()
					}
					logDoctorResult("doctor.vault_template", infrastructure.OperationStatusFail, map[string]string{
						"required": "false",
						"template": p,
					}, templateErr)
				}
			}
			if pc.Secrets.Notes != nil && strings.TrimSpace(*pc.Secrets.Notes) != "" {
				fmt.Fprintf(os.Stderr, "[dockpipe] doctor note: %s\n", strings.TrimSpace(*pc.Secrets.Notes))
			}
		}
	}

	summaryStatus := infrastructure.OperationStatusDone
	requiredChecks := "passed"
	if len(errs) > 0 {
		summaryStatus = infrastructure.OperationStatusFail
		requiredChecks = "failed"
	}
	logDoctorResult("doctor.summary", summaryStatus, map[string]string{
		"optional_issues":   strconv.Itoa(optionalIssues),
		"required_checks":   requiredChecks,
		"required_failures": strconv.Itoa(len(errs)),
	}, "")
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func logDoctorResult(unit, status string, ids map[string]string, err string) {
	result := infrastructure.OperationResult{
		Unit:       unit,
		Status:     status,
		DurationMs: 0,
		IDs:        ids,
	}
	if strings.TrimSpace(err) != "" {
		result.Error = strings.TrimSpace(err)
	}
	infrastructure.LogOperationResult(os.Stderr, result)
}
