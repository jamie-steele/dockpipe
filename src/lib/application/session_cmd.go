package application

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"text/tabwriter"

	"dockpipe/src/lib/infrastructure"
)

func cmdSession(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Print(sessionUsageText)
		return nil
	}
	switch args[0] {
	case "list":
		return cmdSessionList(args[1:])
	case "inspect":
		return cmdSessionInspect(args[1:])
	case "switch":
		return cmdSessionSwitch(args[1:])
	case "publish":
		return cmdSessionPublish(args[1:])
	default:
		return fmt.Errorf("unknown session subcommand %q (try: list, inspect, switch, or publish)", args[0])
	}
}

func cmdSessionList(args []string) error {
	if sessionArgsHaveHelp(args) {
		fmt.Print(sessionUsageText)
		return nil
	}
	workdir, jsonOut, err := parseSessionCommonFlags(args)
	if err != nil {
		return err
	}
	sessions, err := infrastructure.ListGitSessions(workdir)
	if err != nil {
		return err
	}
	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(sessions)
	}
	if len(sessions) == 0 {
		fmt.Fprintln(os.Stdout, "No DockPipe sessions found.")
		return nil
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tSTATUS\tBRANCH\tWORKSPACE\tUPDATED")
	for _, session := range sessions {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
			session.SessionID,
			firstNonEmpty(session.Status, "-"),
			firstNonEmpty(session.Repo.SessionRef, "-"),
			firstNonEmpty(session.Storage.Workspace, "-"),
			firstNonEmpty(session.UpdatedAt, session.CreatedAt, "-"),
		)
	}
	return tw.Flush()
}

func cmdSessionInspect(args []string) error {
	selector, rest, err := takeSessionSelector("inspect", args)
	if err != nil {
		return err
	}
	if selector == "" {
		return nil
	}
	if sessionArgsHaveHelp(rest) {
		fmt.Print(sessionUsageText)
		return nil
	}
	workdir, jsonOut, err := parseSessionCommonFlags(rest)
	if err != nil {
		return err
	}
	session, err := infrastructure.LoadGitSession(workdir, selector)
	if err != nil {
		return err
	}
	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(session)
	}
	printSessionDetails(session)
	return nil
}

func cmdSessionSwitch(args []string) error {
	selector, rest, err := takeSessionSelector("switch", args)
	if err != nil {
		return err
	}
	if selector == "" {
		return nil
	}
	if sessionArgsHaveHelp(rest) {
		fmt.Print(sessionUsageText)
		return nil
	}
	workdir, jsonOut, err := parseSessionCommonFlags(rest)
	if err != nil {
		return err
	}
	session, err := infrastructure.LoadGitSession(workdir, selector)
	if err != nil {
		return err
	}
	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]string{
			"session_id": session.SessionID,
			"branch":     session.Repo.SessionRef,
			"workspace":  session.Storage.Workspace,
			"command":    shellChangeDirectoryCommand(session.Storage.Workspace),
		})
	}
	fmt.Fprintf(os.Stdout, "Session: %s\n", session.SessionID)
	fmt.Fprintf(os.Stdout, "Branch:  %s\n", session.Repo.SessionRef)
	fmt.Fprintf(os.Stdout, "Workdir: %s\n", session.Storage.Workspace)
	fmt.Fprintf(os.Stdout, "\n%s\n", shellChangeDirectoryCommand(session.Storage.Workspace))
	return nil
}

func cmdSessionPublish(args []string) error {
	selector, rest, err := takeSessionSelector("publish", args)
	if err != nil {
		return err
	}
	if selector == "" {
		return nil
	}
	if sessionArgsHaveHelp(rest) {
		fmt.Print(sessionUsageText)
		return nil
	}
	workdir := ""
	remote := "origin"
	jsonOut := false
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case "--workdir":
			if i+1 >= len(rest) {
				return fmt.Errorf("--workdir requires a path")
			}
			workdir = rest[i+1]
			i++
		case "--remote":
			if i+1 >= len(rest) {
				return fmt.Errorf("--remote requires a name")
			}
			remote = rest[i+1]
			i++
		case "--json":
			jsonOut = true
		default:
			if strings.HasPrefix(rest[i], "-") {
				return fmt.Errorf("unknown option %s", rest[i])
			}
			return fmt.Errorf("unexpected argument %q", rest[i])
		}
	}
	if workdir == "" {
		var err error
		workdir, err = os.Getwd()
		if err != nil {
			return err
		}
	}
	session, err := infrastructure.LoadGitSession(workdir, selector)
	if err != nil {
		return err
	}
	cp, err := infrastructure.CheckpointSession(session, "pre-publish checkpoint")
	if err != nil {
		return fmt.Errorf("checkpoint before publish: %w", err)
	}
	res, err := infrastructure.PublishSession(session, remote)
	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(map[string]any{
			"checkpoint": cp,
			"publish":    res,
		})
	}
	if err != nil {
		return err
	}
	if !jsonOut {
		if cp.Status == "created" {
			fmt.Fprintf(os.Stdout, "Checkpoint: %s %s\n", cp.CheckpointID, cp.Commit)
		} else {
			fmt.Fprintf(os.Stdout, "Checkpoint: clean (%s)\n", cp.CheckpointID)
		}
		fmt.Fprintf(os.Stdout, "Published: %s -> %s/%s\n", session.SessionID, res.Remote, res.Branch)
	}
	return nil
}

func parseSessionCommonFlags(args []string) (string, bool, error) {
	workdir := ""
	jsonOut := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--workdir":
			if i+1 >= len(args) {
				return "", false, fmt.Errorf("--workdir requires a path")
			}
			workdir = args[i+1]
			i++
		case "--json":
			jsonOut = true
		default:
			if strings.HasPrefix(args[i], "-") {
				return "", false, fmt.Errorf("unknown option %s", args[i])
			}
			return "", false, fmt.Errorf("unexpected argument %q", args[i])
		}
	}
	if workdir == "" {
		var err error
		workdir, err = os.Getwd()
		if err != nil {
			return "", false, err
		}
	}
	return workdir, jsonOut, nil
}

func sessionArgsHaveHelp(args []string) bool {
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			return true
		}
	}
	return false
}

func takeSessionSelector(cmd string, args []string) (string, []string, error) {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		if len(args) > 0 {
			fmt.Print(sessionUsageText)
			return "", nil, nil
		}
		return "", nil, fmt.Errorf("usage: dockpipe session %s <id> [--workdir <path>] [--json]", cmd)
	}
	if strings.HasPrefix(args[0], "-") {
		return "", nil, fmt.Errorf("usage: dockpipe session %s <id> [--workdir <path>] [--json]", cmd)
	}
	return args[0], args[1:], nil
}

func printSessionDetails(session *infrastructure.GitSession) {
	fmt.Fprintf(os.Stdout, "Session:   %s\n", session.SessionID)
	fmt.Fprintf(os.Stdout, "Status:    %s\n", session.Status)
	fmt.Fprintf(os.Stdout, "Workspace: %s\n", session.WorkspaceID)
	fmt.Fprintf(os.Stdout, "Branch:    %s\n", session.Repo.SessionRef)
	fmt.Fprintf(os.Stdout, "Base:      %s\n", session.Repo.BaseRef)
	fmt.Fprintf(os.Stdout, "Source:    %s\n", session.Repo.Source)
	fmt.Fprintf(os.Stdout, "Path:      %s\n", session.Storage.Workspace)
	fmt.Fprintf(os.Stdout, "Storage:   %s/%s\n", session.Storage.Mode, session.Storage.Backend)
	if strings.TrimSpace(session.Storage.Volume) != "" {
		fmt.Fprintf(os.Stdout, "Volume:    %s\n", session.Storage.Volume)
	}
	fmt.Fprintf(os.Stdout, "Metadata:  %s\n", session.Storage.Metadata)
	fmt.Fprintf(os.Stdout, "Created:   %s\n", session.CreatedAt)
	fmt.Fprintf(os.Stdout, "Updated:   %s\n", session.UpdatedAt)
	fmt.Fprintf(os.Stdout, "Policy:    checkpoint=%s publish=%s allow_agent_git=%v\n", session.Policy.Checkpoint, session.Policy.Publish, session.Policy.AllowAgentGit)
}

func shellChangeDirectoryCommand(path string) string {
	path = filepath.Clean(path)
	if runtime.GOOS == "windows" {
		return "Set-Location -LiteralPath '" + strings.ReplaceAll(path, "'", "''") + "'"
	}
	return "cd " + shellQuote(path)
}

const sessionUsageText = `dockpipe session — inspect runtime-owned Git workspaces

Usage:
  dockpipe session list [--workdir <path>] [--json]
  dockpipe session inspect <id|latest> [--workdir <path>] [--json]
  dockpipe session switch <id|latest> [--workdir <path>] [--json]
  dockpipe session publish <id|latest> [--workdir <path>] [--remote origin] [--json]

Notes:
  switch prints the managed worktree path and shell cd command; a child process cannot change the parent shell cwd.
  publish creates a pre-publish checkpoint commit when needed, then pushes the session branch. It does not merge.
`
