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

const containerWorkMount = "/work"

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
	workHost, _ = filepathAbsDocker(workHost)

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

	outFd, outOK := fdInt(stdout)
	if !o.Detach && outOK && isTerminalDockerFn(outFd) {
		width := terminalWidth(outFd)
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
	}
	// Windows has no POSIX uid/gid; Docker Desktop rejects -u -1:-1. Omit user mapping.
	if runtime.GOOS != "windows" {
		args = append(args, "-u", strconv.Itoa(getuidDockerFn())+":"+strconv.Itoa(getgidDockerFn()))
	}
	args = append(args,
		"-v", workHost+":"+containerWorkMount,
		"-w", cwdInContainer,
		"-e", "DOCKPIPE_CONTAINER_WORKDIR="+containerWorkMount,
	)

	if o.ActionPath != "" {
		ap, err := filepathAbsDocker(o.ActionPath)
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
		_ = mkdirAllDockerFn(o.DataDir, 0o755)
		args = append(args,
			"-v", o.DataDir+":/dockpipe-data",
			"-e", "DOCKPIPE_DATA=/dockpipe-data",
			"-e", "HOME=/dockpipe-data",
		)
		// POSIX chown inside the container is meaningless for host uid/gid on Windows.
		if runtime.GOOS != "windows" {
			ch := execCommandFn("docker", "run", "--rm", "-v", o.DataDir+":/dockpipe-data", "-u", "0", o.Image, "sh", "-c", chown)
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
		if err := cmd.Run(); err != nil {
			return 1, err
		}
		return 0, nil
	}

	inFd, inOK := fdInt(stdin)
	outFd2, outOK2 := fdInt(stdout)
	if inOK && outOK2 && isTerminalDockerFn(inFd) && isTerminalDockerFn(outFd2) {
		args = append(args, "-it")
	} else {
		args = append(args, "-i")
	}

	cid := fmt.Sprintf("dockpipe-%d-%d", os.Getpid(), timeNowDockerFn().UnixNano()%1e9)
	args = append(args, "--name", cid, o.Image)
	args = append(args, argv...)

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
