package infrastructure

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/term"
)

const containerWorkMount = "/work"

// DockerBuild runs docker build -q -t image -f Dir/Dockerfile context.
func DockerBuild(image, dockerfileDir, contextDir string) error {
	df := filepath.Join(dockerfileDir, "Dockerfile")
	baseName := image
	if i := strings.LastIndex(image, ":"); i >= 0 {
		baseName = image[:i]
	}
	if baseName == "dockpipe-dev" {
		if err := exec.Command("docker", "image", "inspect", "dockpipe-base-dev:latest").Run(); err != nil {
			cmd := exec.Command("docker", "build", "-q", "-t", "dockpipe-base-dev",
				"-f", filepath.Join(contextDir, "images/base-dev/Dockerfile"), contextDir)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("build dockpipe-base-dev: %w", err)
			}
		}
	}
	cmd := exec.Command("docker", "build", "-q", "-t", image, "-f", df, contextDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// RunOpts is passed to RunContainer.
type RunOpts struct {
	Image         string
	WorkdirHost   string
	WorkPath      string
	ActionPath    string
	ExtraMounts   []string
	ExtraEnv      []string
	DataVolume    string
	DataDir       string
	Reinit        bool
	Force         bool
	Detach        bool
	CommitOnHost  bool
	CommitMessage string
	BundleOut     string
	Stdin         *os.File
	Stdout        *os.File
	Stderr        *os.File
}

// RunContainer mirrors lib/runner.sh dockpipe_run (attached path with logs on failure).
func RunContainer(o RunOpts, argv []string) (int, error) {
	if o.Image == "" {
		return 1, fmt.Errorf("DOCKPIPE_IMAGE is required")
	}
	workHost := o.WorkdirHost
	if workHost == "" {
		wd, err := os.Getwd()
		if err != nil {
			return 1, err
		}
		workHost = wd
	}
	workHost, _ = filepath.Abs(workHost)

	stdout := o.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stderr := o.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}
	stdin := o.Stdin
	if stdin == nil {
		stdin = os.Stdin
	}

	if !o.Detach && term.IsTerminal(int(stdout.Fd())) {
		width := terminalWidth(int(stdout.Fd()))
		fmt.Fprint(stderr, renderBannerForWidth(width))
		if shouldShowSpinner(width) {
			spin(stderr)
		}
	}

	cwdInContainer := containerWorkMount
	if o.WorkPath != "" {
		wp := strings.TrimPrefix(o.WorkPath, "/")
		cwdInContainer = filepath.Join(containerWorkMount, wp)
	}

	args := []string{
		"run",
		"--init",
		"--hostname", "dockpipe",
		"-u", strconv.Itoa(os.Getuid()) + ":" + strconv.Itoa(os.Getgid()),
		"-v", workHost + ":" + containerWorkMount,
		"-w", cwdInContainer,
		"-e", "DOCKPIPE_CONTAINER_WORKDIR=" + containerWorkMount,
	}

	if o.ActionPath != "" {
		ap, err := filepath.Abs(o.ActionPath)
		if err != nil {
			return 1, err
		}
		if st, err := os.Stat(ap); err == nil && !st.IsDir() {
			args = append(args, "-v", ap+":/dockpipe-action.sh:ro", "-e", "DOCKPIPE_ACTION=/dockpipe-action.sh")
		}
	}

	if o.Reinit && o.DataVolume != "" {
		fmt.Fprintf(stderr, "  ⚠  REINIT: This will permanently delete all data in volume '%s' (login, cache, repos).\n", o.DataVolume)
		if !o.Force {
			if !term.IsTerminal(int(stdin.Fd())) {
				return 1, fmt.Errorf("no TTY; use -f to reinit non-interactively")
			}
			fmt.Fprintf(stderr, "  Continue? [y/N] ")
			br := bufio.NewReader(stdin)
			line, _ := br.ReadString('\n')
			line = strings.TrimSpace(strings.ToLower(line))
			if line != "y" && line != "yes" {
				fmt.Fprintln(stderr, "Aborted.")
				return 1, fmt.Errorf("aborted")
			}
		}
		fmt.Fprintf(stderr, "  Removing volume '%s'...\n", o.DataVolume)
		_ = exec.Command("docker", "volume", "rm", o.DataVolume).Run()
		fmt.Fprintln(stderr, "  Done. Starting with a fresh volume.")
	}

	uid := strconv.Itoa(os.Getuid())
	gid := strconv.Itoa(os.Getgid())
	chown := "chown -R " + uid + ":" + gid + " /dockpipe-data 2>/dev/null || true"

	if o.DataDir != "" {
		_ = os.MkdirAll(o.DataDir, 0o755)
		args = append(args,
			"-v", o.DataDir+":/dockpipe-data",
			"-e", "DOCKPIPE_DATA=/dockpipe-data",
			"-e", "HOME=/dockpipe-data",
		)
		ch := exec.Command("docker", "run", "--rm", "-v", o.DataDir+":/dockpipe-data", "-u", "0", o.Image, "sh", "-c", chown)
		_ = ch.Run()
	} else if o.DataVolume != "" {
		args = append(args,
			"-v", o.DataVolume+":/dockpipe-data",
			"-e", "DOCKPIPE_DATA=/dockpipe-data",
			"-e", "HOME=/dockpipe-data",
		)
		ch := exec.Command("docker", "run", "--rm", "-v", o.DataVolume+":/dockpipe-data", "-u", "0", o.Image, "sh", "-c", chown)
		_ = ch.Run()
	}

	for _, m := range o.ExtraMounts {
		m = strings.TrimSpace(m)
		if m != "" {
			args = append(args, "-v", m)
		}
	}
	for _, e := range o.ExtraEnv {
		e = strings.TrimSpace(e)
		if e != "" {
			args = append(args, "-e", e)
		}
	}

	if o.Detach {
		args = append(args, "-d", "--rm")
		args = append(args, o.Image)
		args = append(args, argv...)
		cmd := exec.Command("docker", args...)
		cmd.Stdin = stdin
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		if err := cmd.Run(); err != nil {
			return 1, err
		}
		return 0, nil
	}

	if term.IsTerminal(int(stdin.Fd())) && term.IsTerminal(int(stdout.Fd())) {
		args = append(args, "-it")
	} else {
		args = append(args, "-i")
	}

	cid := fmt.Sprintf("dockpipe-%d-%d", os.Getpid(), time.Now().UnixNano()%1e9)
	args = append(args, "--name", cid, o.Image)
	args = append(args, argv...)

	start := time.Now()
	cmd := exec.Command("docker", args...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err := cmd.Run()
	rc := 0
	if err != nil {
		if x, ok := err.(*exec.ExitError); ok {
			rc = x.ExitCode()
		} else {
			rc = 1
		}
	}
	elapsed := time.Since(start)

	// Keep default output quiet on success. Quick-exit dumps are useful for debugging
	// but noisy in normal/test runs; opt in via DOCKPIPE_LOG_QUICK_EXIT=1.
	logQuickExit := os.Getenv("DOCKPIPE_LOG_QUICK_EXIT") == "1"
	if rc != 0 || (logQuickExit && elapsed < 3*time.Second) {
		fmt.Fprintln(stderr, "")
		if rc != 0 {
			fmt.Fprintf(stderr, "  Container exited with code %d. Full container output:\n", rc)
		} else {
			fmt.Fprintf(stderr, "  Container exited quickly (%s). Full container output:\n", elapsed.Truncate(time.Second))
		}
		fmt.Fprintln(stderr, "  ---")
		logs := exec.Command("docker", "logs", cid)
		logOut, _ := logs.CombinedOutput()
		for _, line := range strings.Split(string(logOut), "\n") {
			if line != "" {
				fmt.Fprintf(stderr, "  %s\n", line)
			}
		}
		fmt.Fprintln(stderr, "  ---")
	}
	_ = exec.Command("docker", "rm", cid).Run()

	if o.CommitOnHost {
		_ = CommitOnHost(workHost, o.CommitMessage, o.BundleOut)
	}
	return rc, nil
}

const banner = `
    ██████╗  ██████╗ ██████╗██╗  ██╗██████╗ ██╗██████╗ ███████╗
    ██╔══██╗██╔═══██╗██╔═══╝██║ ██╔╝██╔══██╗██║██╔══██╗██╔════╝
    ██║  ██║██║   ██║██║    █████╔╝ ██████╔╝██║██████╔╝█████╗
    ██║  ██║██║   ██║██║    ██╔═██╗ ██╔═══╝ ██║██╔═══╝ ██╔══╝
    ██████╔╝╚██████╔╝██████╗██║  ██╗██║     ██║██║     ███████╗
    ╚═════╝  ╚═════╝ ╚═════╝╚═╝  ╚═╝╚═╝     ╚═╝╚═╝     ╚══════╝
                      Run  →  Isolate  →  Act

`

const compactBanner = "dockpipe — Run -> Isolate -> Act\n"

func terminalWidth(fd int) int {
	width, _, err := term.GetSize(fd)
	if err != nil || width <= 0 {
		return 0
	}
	return width
}

func renderBannerForWidth(width int) string {
	// Use a conservative threshold to avoid wrapping/artifacts in split panes.
	const minBannerWidth = 70
	if width < minBannerWidth {
		return compactBanner
	}
	return banner
}

func shouldShowSpinner(width int) bool {
	// Spinner uses carriage-return updates; hide it in narrow terminals to avoid messy wraps.
	return width >= 60
}

func spin(w *os.File) {
	chars := `|/-\`
	for i := 0; i < 8; i++ {
		fmt.Fprintf(w, "\r  Launching container... %c  ", chars[i%4])
		time.Sleep(80 * time.Millisecond)
	}
	fmt.Fprintf(w, "\r  %40s\r", "")
}
