package infrastructure

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"golang.org/x/term"
)

// fdInt converts *os.File Fd() to int for term.IsTerminal / GetSize without G115 overflow on sane platforms.
func fdInt(f *os.File) (int, bool) {
	fd := f.Fd()
	if fd > uintptr(math.MaxInt) {
		return 0, false
	}
	return int(fd), true
}

// useDockerInteractiveTTY chooses docker run -it vs -i for the attached (non-detach) path.
// Interactive CLIs need a TTY; stdin must be a terminal. Some Windows setups report stdout as
// non-TTY while stderr is still the console — without -t those tools can hang or show no UI.
func useDockerInteractiveTTY(stdin, stdout, stderr *os.File) bool {
	inFd, inOK := fdInt(stdin)
	if !inOK || !isTerminalDockerFn(inFd) {
		return false
	}
	outFd, outOK := fdInt(stdout)
	if outOK && isTerminalDockerFn(outFd) {
		return true
	}
	errFd, errOK := fdInt(stderr)
	return errOK && isTerminalDockerFn(errFd)
}

const containerWorkMount = "/work"

// extraEnvHasKey reports whether pairs contains KEY=... (case-insensitive key).
func extraEnvHasKey(pairs []string, key string) bool {
	prefix := strings.ToLower(key) + "="
	for _, e := range pairs {
		e = strings.TrimSpace(e)
		if e == "" {
			continue
		}
		if strings.HasPrefix(strings.ToLower(e), prefix) {
			return true
		}
	}
	return false
}

var (
	execCommandFn      = exec.Command
	getwdDockerFn      = os.Getwd
	filepathAbsDocker  = filepath.Abs
	osStatDockerFn     = os.Stat
	mkdirAllDockerFn   = os.MkdirAll
	getuidDockerFn     = os.Getuid
	getgidDockerFn     = os.Getgid
	isTerminalDockerFn = term.IsTerminal
	timeNowDockerFn    = time.Now
	commitOnHostFn     = CommitOnHost
)

// DockerBuild runs docker build -q -t image -f Dir/Dockerfile context.
func DockerBuild(image, dockerfileDir, contextDir string) error {
	df := filepath.Join(dockerfileDir, "Dockerfile")
	baseName := image
	if i := strings.LastIndex(image, ":"); i >= 0 {
		baseName = image[:i]
	}
	if baseName == "dockpipe-dev" {
		if err := execCommandFn("docker", "image", "inspect", "dockpipe-base-dev:latest").Run(); err != nil {
			cmd := execCommandFn("docker", "build", "-q", "-t", "dockpipe-base-dev",
				"-f", filepath.Join(contextDir, "images/base-dev/Dockerfile"), contextDir)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("build dockpipe-base-dev: %w", err)
			}
		}
	}
	cmd := execCommandFn("docker", "build", "-q", "-t", image, "-f", df, contextDir)
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
	BundleAll     bool
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
		wd, err := getwdDockerFn()
		if err != nil {
			return 1, err
		}
		workHost = wd
	}
	// Git Bash exports DOCKPIPE_WORKDIR as /c/Users/...; filepath.Abs on Windows does not treat
	// that as absolute, so docker -v can bind the wrong path and /work looks empty.
	if runtime.GOOS == "windows" {
		workHost = HostPathForGit(workHost)
	}
	var absErr error
	workHost, absErr = filepathAbsDocker(workHost)
	if absErr != nil {
		return 1, absErr
	}

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

	cwdInContainer := containerWorkMount
	if o.WorkPath != "" {
		wp := strings.TrimPrefix(o.WorkPath, "/")
		cwdInContainer = filepath.Join(containerWorkMount, wp)
	}

	args := []string{
		"run",
		"--init",
		"--hostname", "dockpipe",
	}
	// Map container user to the host user on Unix so bind mounts have sane ownership.
	// On Windows, optional -u via DOCKPIPE_WINDOWS_CONTAINER_USER only (see windowsDockerUserSpec).
	if runtime.GOOS == "windows" {
		if u := windowsDockerUserSpec(); u != "" {
			args = append(args, "-u", u)
		}
	} else {
		args = append(args, "-u", unixDockerUserSpec(o.Image, stderr))
	}
	args = append(args,
		"-v", workHost+":"+containerWorkMount,
		"-w", cwdInContainer,
		"-e", "DOCKPIPE_CONTAINER_WORKDIR="+containerWorkMount,
	)
	// Claude Code: IS_SANDBOX=1 must be present even if the image lacks our entrypoint (rebuild still recommended).
	if os.Getenv("DOCKPIPE_NO_SANDBOX_ENV") == "" && !extraEnvHasKey(o.ExtraEnv, "IS_SANDBOX") {
		args = append(args, "-e", "IS_SANDBOX=1")
	}

	if o.ActionPath != "" {
		ap := o.ActionPath
		if runtime.GOOS == "windows" {
			ap = HostPathForGit(ap)
		}
		ap, err := filepathAbsDocker(ap)
		if err != nil {
			return 1, err
		}
		if st, err := osStatDockerFn(ap); err == nil && !st.IsDir() {
			args = append(args, "-v", ap+":/dockpipe-action.sh:ro", "-e", "DOCKPIPE_ACTION=/dockpipe-action.sh")
		}
	}

	if o.Reinit && o.DataVolume != "" {
		fmt.Fprintf(stderr, "  ⚠  REINIT: This will permanently delete all data in volume '%s' (login, cache, repos).\n", o.DataVolume)
		if !o.Force {
			reinitInFd, reinitInOK := fdInt(stdin)
			if !reinitInOK || !isTerminalDockerFn(reinitInFd) {
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
		_ = execCommandFn("docker", "volume", "rm", o.DataVolume).Run()
		fmt.Fprintln(stderr, "  Done. Starting with a fresh volume.")
	}

	uid := strconv.Itoa(getuidDockerFn())
	gid := strconv.Itoa(getgidDockerFn())
	chown := "chown -R " + uid + ":" + gid + " /dockpipe-data 2>/dev/null || true"

	if o.DataDir != "" {
		dataDir := o.DataDir
		if runtime.GOOS == "windows" {
			dataDir = HostPathForGit(dataDir)
		}
		dataDir, absErr = filepathAbsDocker(dataDir)
		if absErr != nil {
			return 1, absErr
		}
		_ = mkdirAllDockerFn(dataDir, 0o755)
		args = append(args,
			"-v", dataDir+":/dockpipe-data",
			"-e", "DOCKPIPE_DATA=/dockpipe-data",
			"-e", "HOME=/dockpipe-data",
		)
		// POSIX chown inside the container is meaningless for host uid/gid on Windows.
		if runtime.GOOS != "windows" {
			ch := execCommandFn("docker", "run", "--rm", "-v", dataDir+":/dockpipe-data", "-u", "0", o.Image, "sh", "-c", chown)
			_ = ch.Run()
		}
	} else if o.DataVolume != "" {
		args = append(args,
			"-v", o.DataVolume+":/dockpipe-data",
			"-e", "DOCKPIPE_DATA=/dockpipe-data",
			"-e", "HOME=/dockpipe-data",
		)
		if runtime.GOOS != "windows" {
			ch := execCommandFn("docker", "run", "--rm", "-v", o.DataVolume+":/dockpipe-data", "-u", "0", o.Image, "sh", "-c", chown)
			_ = ch.Run()
		}
	}

	for _, m := range o.ExtraMounts {
		m = strings.TrimSpace(m)
		if m != "" {
			if runtime.GOOS == "windows" {
				m = normalizeDockerBindMountWindows(m)
			}
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
		cmd := execCommandFn("docker", args...)
		cmd.Stdin = stdin
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		stopSpin := StartLineSpinner(stderr, "Starting container…")
		err := cmd.Run()
		stopSpin()
		if err != nil {
			return 1, err
		}
		return 0, nil
	}

	if useDockerInteractiveTTY(stdin, stdout, stderr) {
		args = append(args, "-it")
	} else {
		args = append(args, "-i")
	}

	cid := fmt.Sprintf("dockpipe-%d-%d", os.Getpid(), timeNowDockerFn().UnixNano()%1e9)
	args = append(args, "--name", cid, o.Image)
	args = append(args, argv...)

	// User-visible checkpoint: docker run can block on pull, VM, or volume (especially on Windows).
	fmt.Fprintln(stderr, "[dockpipe] Starting container…")

	start := timeNowDockerFn()
	cmd := execCommandFn("docker", args...)
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
	elapsed := timeNowDockerFn().Sub(start)

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
		logs := execCommandFn("docker", "logs", cid)
		logOut, _ := logs.CombinedOutput()
		for _, line := range strings.Split(string(logOut), "\n") {
			if line != "" {
				fmt.Fprintf(stderr, "  %s\n", line)
			}
		}
		fmt.Fprintln(stderr, "  ---")
	}
	_ = execCommandFn("docker", "rm", cid).Run()

	if o.CommitOnHost {
		_ = commitOnHostFn(workHost, o.CommitMessage, o.BundleOut, o.BundleAll)
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

// terminalWidthForBanner prefers a non-zero GetSize from stdout or stderr; on Windows if both
// are zero but we're on a TTY, assume a wide console so the full ASCII banner matches Linux.
func terminalWidthForBanner(outFd, errFd int) int {
	for _, fd := range []int{outFd, errFd} {
		if fd <= 0 {
			continue
		}
		if !isTerminalDockerFn(fd) {
			continue
		}
		if w := terminalWidth(fd); w > 0 {
			return w
		}
	}
	if runtime.GOOS == "windows" {
		return 120
	}
	return 0
}

// PrintLaunchBanner prints the ASCII banner to stderr when stdout or stderr is a TTY.
// The application calls this once at CLI launch (before host work). Spinners for long-running
// work run separately via StartLineSpinner.
func PrintLaunchBanner(stdout, stderr *os.File) {
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}
	outFd, outOK := fdInt(stdout)
	errFd, errOK := fdInt(stderr)
	stdoutTTY := outOK && isTerminalDockerFn(outFd)
	stderrTTY := errOK && isTerminalDockerFn(errFd)
	if !stdoutTTY && !stderrTTY {
		return
	}
	width := terminalWidthForBanner(outFd, errFd)
	fmt.Fprint(stderr, renderBannerForWidth(width))
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
