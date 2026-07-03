package application

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"dockpipe/src/lib/infrastructure"
)

func cmdRuns(argv []string) error {
	for _, a := range argv {
		if a == "-h" || a == "--help" {
			fmt.Print(runsUsageText)
			return nil
		}
	}
	sub := "list"
	if len(argv) > 0 {
		sub = argv[0]
	}
	switch sub {
	case "list", "policy", "decisions", "events":
	default:
		return fmt.Errorf("dockpipe runs: unknown subcommand %q (try: list, policy, or events)", sub)
	}
	publicSub := sub
	if publicSub == "decisions" {
		publicSub = "policy"
	}
	rest := argv[1:]
	workdir := ""
	eventLog := ""
	eventIndex := ""
	jsonOut := false
	policyOpts := infrastructure.RunPolicyListOptions{}
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case "--workdir":
			if i+1 >= len(rest) {
				return fmt.Errorf("dockpipe runs %s: --workdir requires a path", publicSub)
			}
			workdir = rest[i+1]
			i++
		case "--event-log":
			if publicSub != "events" {
				return fmt.Errorf("dockpipe runs %s: --event-log is only valid with events", publicSub)
			}
			if i+1 >= len(rest) {
				return fmt.Errorf("dockpipe runs events: --event-log requires a path")
			}
			eventLog = rest[i+1]
			i++
		case "--index":
			if publicSub != "events" {
				return fmt.Errorf("dockpipe runs %s: --index is only valid with events", publicSub)
			}
			if i+1 < len(rest) && !strings.HasPrefix(rest[i+1], "-") {
				eventIndex = rest[i+1]
				i++
			} else {
				eventIndex = "__default__"
			}
		case "--workflow":
			if publicSub != "policy" {
				return fmt.Errorf("dockpipe runs %s: --workflow is only valid with policy", publicSub)
			}
			if i+1 >= len(rest) {
				return fmt.Errorf("dockpipe runs policy: --workflow requires a value")
			}
			policyOpts.WorkflowName = rest[i+1]
			i++
		case "--step":
			if publicSub != "policy" {
				return fmt.Errorf("dockpipe runs %s: --step is only valid with policy", publicSub)
			}
			if i+1 >= len(rest) {
				return fmt.Errorf("dockpipe runs policy: --step requires a value")
			}
			policyOpts.StepID = rest[i+1]
			i++
		case "--json":
			if publicSub != "policy" && publicSub != "events" {
				return fmt.Errorf("dockpipe runs %s: --json is only valid with policy or events", publicSub)
			}
			if publicSub == "events" {
				jsonOut = true
			} else {
				policyOpts.JSON = true
			}
		default:
			return fmt.Errorf("dockpipe runs %s: unexpected argument %q", publicSub, rest[i])
		}
	}
	if workdir == "" {
		if w := os.Getenv("DOCKPIPE_WORKDIR"); w != "" {
			workdir = w
		} else {
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			workdir = wd
		}
	}
	if publicSub == "policy" {
		return infrastructure.ListRunPolicyRecords(workdir, os.Stdout, policyOpts)
	}
	if publicSub == "events" {
		return cmdRunsEvents(eventLog, eventIndex, jsonOut)
	}
	return infrastructure.ListHostRuns(workdir, os.Stdout)
}

func cmdRunsEvents(eventLog, eventIndex string, jsonOut bool) error {
	eventLog = strings.TrimSpace(eventLog)
	if eventLog == "" {
		eventLog = strings.TrimSpace(os.Getenv(infrastructure.EnvDockpipeEventLog))
	}
	if eventLog == "" {
		return fmt.Errorf("dockpipe runs events: provide --event-log <path> or set %s", infrastructure.EnvDockpipeEventLog)
	}
	eventIndex = strings.TrimSpace(eventIndex)
	if eventIndex != "" {
		if eventIndex == "__default__" {
			eventIndex = strings.TrimSpace(os.Getenv(infrastructure.EnvDockpipeEventIndex))
			if eventIndex == "" {
				return fmt.Errorf("dockpipe runs events: provide --index <path> or set %s", infrastructure.EnvDockpipeEventIndex)
			}
		}
		index, err := infrastructure.BuildOperationEventIndex(eventLog)
		if err != nil {
			return err
		}
		if err := infrastructure.WriteOperationEventIndex(eventIndex, index); err != nil {
			return err
		}
		if jsonOut {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(index)
		}
		fmt.Fprintf(os.Stdout, "Indexed %d operation events -> %s\n", index.EventCount, eventIndex)
		return nil
	}
	events, err := infrastructure.ReadOperationEvents(eventLog)
	if err != nil {
		return err
	}
	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(events)
	}
	if len(events) == 0 {
		fmt.Fprintln(os.Stdout, "No DockPipe operation events found.")
		return nil
	}
	for _, event := range events {
		fmt.Fprintf(os.Stdout, "%s  %-8s  %s", firstNonEmpty(event.Timestamp, "-"), firstNonEmpty(event.Status, "-"), firstNonEmpty(event.Unit, "-"))
		if event.DurationMs != nil {
			fmt.Fprintf(os.Stdout, "  duration_ms=%d", *event.DurationMs)
		}
		for _, key := range sortedRunEventIDKeys(event.IDs) {
			value := strings.TrimSpace(event.IDs[key])
			if value == "" {
				continue
			}
			fmt.Fprintf(os.Stdout, "  %s=%s", key, value)
		}
		if strings.TrimSpace(event.Error) != "" {
			fmt.Fprintf(os.Stdout, "  error=%q", strings.TrimSpace(event.Error))
		}
		fmt.Fprintln(os.Stdout)
	}
	return nil
}

func sortedRunEventIDKeys(ids map[string]string) []string {
	if len(ids) == 0 {
		return nil
	}
	keys := make([]string, 0, len(ids))
	for key := range ids {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
