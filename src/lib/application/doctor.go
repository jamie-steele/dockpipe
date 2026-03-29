package application

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
)

func cmdDoctor(argv []string) error {
	if len(argv) > 0 && (argv[0] == "-h" || argv[0] == "--help") {
		fmt.Print(`dockpipe doctor — verify bash, Docker, and bundled assets

Quick checks before a real run. Does not start a project container.

`)
		return nil
	}

	fmt.Fprintf(os.Stderr, "[dockpipe] doctor: host checks\n\n")

	var errs []error

	if p, err := exec.LookPath("bash"); err == nil {
		fmt.Fprintf(os.Stderr, "[dockpipe] bash: ok (%s)\n", p)
	} else {
		fmt.Fprintf(os.Stderr, "[dockpipe] bash: not found in PATH\n")
		errs = append(errs, fmt.Errorf("bash not found in PATH"))
	}

	if err := infrastructure.DockerDoctorCheck(os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "[dockpipe] docker: %v\n", err)
		errs = append(errs, err)
	} else {
		fmt.Fprintln(os.Stderr, "[dockpipe] docker: ok (daemon reachable)")
	}

	rr, err := infrastructure.RepoRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[dockpipe] bundled assets: could not resolve (%v)\n", err)
	} else {
		if wfPath, err := infrastructure.ResolveWorkflowConfigPath(rr, "run"); err != nil {
			fmt.Fprintf(os.Stderr, "[dockpipe] bundled assets: incomplete (%s)\n", rr)
		} else {
			fmt.Fprintf(os.Stderr, "[dockpipe] bundled assets: ok (%s)\n", wfPath)
		}
	}

	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[dockpipe] project config: could not get working directory (%v)\n", err)
	} else {
		pc, err := domain.LoadDockpipeProjectConfig(wd)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[dockpipe] project config: %v\n", err)
		} else if pc == nil {
			fmt.Fprintf(os.Stderr, "[dockpipe] project config: %s not in this directory (optional)\n", domain.DockpipeProjectConfigFileName)
		} else {
			fmt.Fprintf(os.Stderr, "[dockpipe] project config: ok (%s)\n", filepath.Join(wd, domain.DockpipeProjectConfigFileName))
			if p, ok := domain.ResolveOpInjectTemplatePath(pc, wd); ok {
				if st, err := os.Stat(p); err == nil && !st.IsDir() {
					fmt.Fprintf(os.Stderr, "[dockpipe] secrets op inject template: ok (%s)\n", p)
				} else {
					fmt.Fprintf(os.Stderr, "[dockpipe] secrets op inject template: missing (%s)\n", p)
				}
			}
			if pc.Secrets.Notes != nil && strings.TrimSpace(*pc.Secrets.Notes) != "" {
				fmt.Fprintf(os.Stderr, "[dockpipe] secrets notes: %s\n", strings.TrimSpace(*pc.Secrets.Notes))
			}
		}
	}

	fmt.Fprintln(os.Stderr, "")
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	fmt.Fprintln(os.Stderr, "[dockpipe] doctor: all required checks passed")
	return nil
}
