package infrastructure

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
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

var dockerValuePropOnce sync.Once

// printDockerValuePropOnce prints a one-line contrast vs raw docker run (TTY stderr only).
func printDockerValuePropOnce(stderr *os.File) {
	if stderr == nil {
		stderr = os.Stderr
	}
	dockerValuePropOnce.Do(func() {
		fd, ok := fdInt(stderr)
		if !ok || !isTerminalDockerFn(fd) {
			return
		}
		fmt.Fprintln(stderr, "[dockpipe] Disposable run: project at /work, your uid/gid, --rm — no manual docker run flags.")
	})
}

func printDockerRunFailureHints(stderr *os.File, dockerStderr string) {
	if stderr == nil {
		stderr = os.Stderr
	}
	s := strings.ToLower(dockerStderr)
	if s == "" {
		return
	}
	if strings.Contains(s, "permission denied") && (strings.Contains(s, "mount") || strings.Contains(s, "bind") || strings.Contains(s, "/work")) {
		fmt.Fprintln(stderr, "[dockpipe] Bind mount: check the host path exists, permissions, and Docker Desktop “File sharing” / drive access (Windows/macOS). WSL: docs/wsl-windows.md")
	}
	if strings.Contains(s, "invalid mount") || strings.Contains(s, "not a directory") {
		fmt.Fprintln(stderr, "[dockpipe] Mount: verify `-v` paths and that the directory exists on the host.")
	}
	if strings.Contains(s, "cannot connect to the docker daemon") || strings.Contains(s, "is the docker daemon running") {
		fmt.Fprintln(stderr, "[dockpipe] Docker daemon not reachable — start Docker Desktop or `sudo systemctl start docker` (Linux).")
	}
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
	dockerCmd := dockerCommandName()
	df := filepath.Join(dockerfileDir, "Dockerfile")
	baseName := image
	if i := strings.LastIndex(image, ":"); i >= 0 {
		baseName = image[:i]
	}
	if baseName == "dockpipe-dev" {
		inspect := execCommandFn(dockerCmd, "image", "inspect", "dockpipe-base-dev:latest")
		inspect.Env = dockerCommandEnv(os.Environ(), dockerCmd)
		if err := inspect.Run(); err != nil {
			cmd := execCommandFn(dockerCmd, "build", "-q", "-t", "dockpipe-base-dev",
				"-f", filepath.Join(CoreDir(contextDir), "assets", "images", "base-dev", "Dockerfile"), contextDir)
			cmd.Env = dockerBuildEnv(os.Environ(), dockerCmd)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("build dockpipe-base-dev: %w", err)
			}
		}
	}
	cmd := execCommandFn(dockerCmd, "build", "-q", "-t", image, "-f", df, contextDir)
	cmd.Env = dockerBuildEnv(os.Environ(), dockerCmd)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func dockerBuildEnv(env []string, dockerCmd string) []string {
	env = dockerCommandEnv(env, dockerCmd)
	for _, entry := range env {
		if strings.HasPrefix(strings.ToUpper(entry), "DOCKER_BUILDKIT=") {
			return env
		}
	}
	return append(env, "DOCKER_BUILDKIT=1")
}

func dockerCommandEnv(env []string, dockerCmd string) []string {
	dir := strings.TrimSpace(filepath.Dir(dockerCmd))
	if dir == "" || dir == "." || !filepath.IsAbs(dir) {
		return env
	}
	return prependEnvPathDir(env, dir)
}

func prependEnvPathDir(env []string, dir string) []string {
	if strings.TrimSpace(dir) == "" {
		return env
	}
	out := append([]string(nil), env...)
	pathKey := "PATH"
	pathIdx := -1
	for i, entry := range out {
		key, value, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if strings.EqualFold(key, "PATH") {
			pathKey = key
			pathIdx = i
			if pathContainsDir(value, dir) {
				return out
			}
			out[i] = pathKey + "=" + dir + string(os.PathListSeparator) + value
			return out
		}
	}
	if pathIdx < 0 {
		out = append(out, pathKey+"="+dir)
	}
	return out
}

func pathContainsDir(pathValue, dir string) bool {
	want := normalizePathForCompare(dir)
	for _, part := range filepath.SplitList(pathValue) {
		if normalizePathForCompare(part) == want {
			return true
		}
	}
	return false
}

func normalizePathForCompare(path string) string {
	cleaned := filepath.Clean(strings.TrimSpace(path))
	if runtime.GOOS == "windows" {
		cleaned = strings.ToLower(cleaned)
	}
	return cleaned
}

// DockerImageExists reports whether a local docker image reference currently resolves.
func DockerImageExists(image string) (bool, error) {
	image = strings.TrimSpace(image)
	if image == "" {
		return false, fmt.Errorf("image is empty")
	}
	dockerCmd := dockerCommandName()
	cmd := execCommandFn(dockerCmd, "image", "inspect", image)
	cmd.Env = dockerCommandEnv(os.Environ(), dockerCmd)
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	if _, ok := err.(*exec.ExitError); ok {
		return false, nil
	}
	return false, err
}

// DockerPull pulls a normal OCI image reference into the local daemon.
func DockerPull(image string) error {
	image = strings.TrimSpace(image)
	if image == "" {
		return fmt.Errorf("image is empty")
	}
	dockerCmd := dockerCommandName()
	cmd := execCommandFn(dockerCmd, "pull", image)
	cmd.Env = dockerCommandEnv(os.Environ(), dockerCmd)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// RunOpts is passed to RunContainer.
type RunOpts struct {
	Image       string
	WorkdirHost string
	// WorkdirVolume mounts a Docker named volume at /work instead of bind-mounting WorkdirHost.
	// When WorkdirHost is set, the runner overlays host -> volume before the container run and
	// volume -> host after it so runtime-owned Git checkpoints can still operate on the host worktree.
	WorkdirVolume string
	// SkipVolumeWorkspaceSync leaves a mounted WorkdirVolume authoritative for the run and skips
	// host<->volume copy steps. Use this only when the caller ensures all required workspace state
	// is already present in the volume and post-run lifecycle does not depend on a synced host tree.
	SkipVolumeWorkspaceSync bool
	WorkPath                string
	ActionPath              string
	ExtraMounts             []string
	ExtraEnv                []string
	// ContainerUser overrides the default user mapping when set (for example "0:0").
	ContainerUser string
	// ReadOnlyRootFS enables docker --read-only for the container root filesystem.
	ReadOnlyRootFS bool
	// TmpfsPaths adds docker --tmpfs mounts for writable ephemeral paths.
	TmpfsPaths []string
	// SecurityOpt passes docker --security-opt entries such as no-new-privileges.
	SecurityOpt []string
	// CapDrop / CapAdd pass docker capability adjustments.
	CapDrop []string
	CapAdd  []string
	// PIDLimit, CPULimit, and MemoryLimit enforce container resource restrictions when set.
	PIDLimit    int
	CPULimit    string
	MemoryLimit string
	// NetworkMode: if non-empty, passed to docker run as --network <mode> (e.g. host when bridge IPv6 is broken).
	NetworkMode   string
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
	// StdoutTeePath: if set, container stdout is also written to this host file (in addition to Stdout).
	StdoutTeePath string
}

// DockerNetworkModeFromEnv returns DOCKPIPE_DOCKER_NETWORK from m and removes it so it is not passed with docker -e.
func DockerNetworkModeFromEnv(m map[string]string) string {
	if m == nil {
		return ""
	}
	v := strings.TrimSpace(m["DOCKPIPE_DOCKER_NETWORK"])
	if v != "" {
		delete(m, "DOCKPIPE_DOCKER_NETWORK")
	}
	return v
}

// RunContainer runs docker attach/detach with logging on failure (same contract as the historical bash runner).
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
	workVolume := strings.TrimSpace(o.WorkdirVolume)
	stdoutFile := o.Stdout
	if stdoutFile == nil {
		stdoutFile = os.Stdout
	}
	stdoutOut := io.Writer(stdoutFile)
	if p := strings.TrimSpace(o.StdoutTeePath); p != "" {
		if err := mkdirAllDockerFn(filepath.Dir(p), 0o755); err != nil {
			return 1, err
		}
		f, err := os.OpenFile(p, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if err != nil {
			return 1, fmt.Errorf("stdout tee: %w", err)
		}
		defer f.Close()
		stdoutOut = io.MultiWriter(stdoutOut, f)
	}
	stderr := o.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}
	stdin := o.Stdin
	if stdin == nil {
		stdin = os.Stdin
	}
	if workVolume != "" && !o.SkipVolumeWorkspaceSync {
		fmt.Fprintln(stderr, "[dockpipe] Syncing managed workspace into session volume…")
		if err := dockerSyncWorkspaceIntoVolume(o.Image, workHost, workVolume); err != nil {
			return 1, err
		}
		fmt.Fprintln(stderr, "[dockpipe] Managed workspace volume ready.")
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
	if nw := strings.TrimSpace(o.NetworkMode); nw != "" {
		args = append(args, "--network", nw)
	}
	// Map container user to the host user on Unix so bind mounts have sane ownership unless a
	// compiled runtime policy explicitly overrides it.
	if u := strings.TrimSpace(o.ContainerUser); u != "" {
		args = append(args, "-u", u)
	} else if runtime.GOOS == "windows" {
		// On Windows, optional -u via DOCKPIPE_WINDOWS_CONTAINER_USER only (see windowsDockerUserSpec).
		if u := windowsDockerUserSpec(); u != "" {
			args = append(args, "-u", u)
		}
	} else {
		args = append(args, "-u", unixDockerUserSpec(o.Image, stderr))
	}
	if o.ReadOnlyRootFS {
		args = append(args, "--read-only")
	}
	for _, p := range o.TmpfsPaths {
		p = strings.TrimSpace(p)
		if p != "" {
			args = append(args, "--tmpfs", p)
		}
	}
	for _, s := range o.SecurityOpt {
		s = strings.TrimSpace(s)
		if s != "" {
			args = append(args, "--security-opt", s)
		}
	}
	for _, c := range o.CapDrop {
		c = strings.TrimSpace(c)
		if c != "" {
			args = append(args, "--cap-drop", c)
		}
	}
	for _, c := range o.CapAdd {
		c = strings.TrimSpace(c)
		if c != "" {
			args = append(args, "--cap-add", c)
		}
	}
	if o.PIDLimit > 0 {
		args = append(args, "--pids-limit", strconv.Itoa(o.PIDLimit))
	}
	if cpu := strings.TrimSpace(o.CPULimit); cpu != "" {
		args = append(args, "--cpus", cpu)
	}
	if mem := strings.TrimSpace(o.MemoryLimit); mem != "" {
		args = append(args, "--memory", mem)
	}
	workMount := workHost + ":" + containerWorkMount
	if workVolume != "" {
		workMount = workVolume + ":" + containerWorkMount
	}
	args = append(args,
		"-v", workMount,
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
		dockerCmd := dockerCommandName()
		cmd := execCommandFn(dockerCmd, "volume", "rm", o.DataVolume)
		cmd.Env = dockerCommandEnv(os.Environ(), dockerCmd)
		_ = cmd.Run()
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
			dockerCmd := dockerCommandName()
			ch := execCommandFn(dockerCmd, "run", "--rm", "-v", dataDir+":/dockpipe-data", "-u", "0", o.Image, "sh", "-c", chown)
			ch.Env = dockerCommandEnv(os.Environ(), dockerCmd)
			_ = ch.Run()
		}
	} else if o.DataVolume != "" {
		args = append(args,
			"-v", o.DataVolume+":/dockpipe-data",
			"-e", "DOCKPIPE_DATA=/dockpipe-data",
			"-e", "HOME=/dockpipe-data",
		)
		if runtime.GOOS != "windows" {
			dockerCmd := dockerCommandName()
			ch := execCommandFn(dockerCmd, "run", "--rm", "-v", o.DataVolume+":/dockpipe-data", "-u", "0", o.Image, "sh", "-c", chown)
			ch.Env = dockerCommandEnv(os.Environ(), dockerCmd)
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
		printDockerValuePropOnce(stderr)
		args = append(args, "-d", "--rm")
		args = append(args, o.Image)
		args = append(args, argv...)
		dockerCmd := dockerCommandName()
		cmd := execCommandFn(dockerCmd, args...)
		cmd.Env = dockerCommandEnv(os.Environ(), dockerCmd)
		cmd.Stdin = stdin
		var cidBuf bytes.Buffer
		cmd.Stdout = io.MultiWriter(stdoutOut, &cidBuf)
		var errBuf bytes.Buffer
		cmd.Stderr = io.MultiWriter(stderr, &errBuf)
		stopSpin := StartLineSpinner(stderr, "Starting container…")
		err := cmd.Run()
		stopSpin()
		if err != nil {
			printDockerRunFailureHints(stderr, errBuf.String())
			return 1, err
		}
		cid := strings.TrimSpace(cidBuf.String())
		if cid != "" {
			fmt.Fprintf(stderr, "[dockpipe] Detached: container %s (--rm removes it when the process exits). Logs: docker logs -f %s\n", cid, cid)
		}
		return 0, nil
	}

	if useDockerInteractiveTTY(stdin, stdoutFile, stderr) {
		args = append(args, "-it")
	} else {
		args = append(args, "-i")
	}

	cid := fmt.Sprintf("dockpipe-%d-%d", os.Getpid(), timeNowDockerFn().UnixNano()%1e9)
	args = append(args, "--name", cid, o.Image)
	args = append(args, argv...)

	// User-visible checkpoint: docker run can block on pull, VM, or volume (especially on Windows).
	printDockerValuePropOnce(stderr)
	fmt.Fprintln(stderr, "[dockpipe] Starting container…")

	start := timeNowDockerFn()
	dockerCmd := dockerCommandName()
	cmd := execCommandFn(dockerCmd, args...)
	cmd.Env = dockerCommandEnv(os.Environ(), dockerCmd)
	cmd.Stdin = stdin
	cmd.Stdout = stdoutOut
	var runErrBuf bytes.Buffer
	cmd.Stderr = io.MultiWriter(stderr, &runErrBuf)
	if err := cmd.Start(); err != nil {
		return 1, err
	}
	err := waitCommandWithSignalForward(cmd)
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
			printDockerRunFailureHints(stderr, runErrBuf.String())
			fmt.Fprintf(stderr, "  Container exited with code %d. Full container output:\n", rc)
		} else {
			fmt.Fprintf(stderr, "  Container exited quickly (%s). Full container output:\n", elapsed.Truncate(time.Second))
		}
		fmt.Fprintln(stderr, "  ---")
		dockerCmd := dockerCommandName()
		logs := execCommandFn(dockerCmd, "logs", cid)
		logs.Env = dockerCommandEnv(os.Environ(), dockerCmd)
		logOut, _ := logs.CombinedOutput()
		for _, line := range strings.Split(string(logOut), "\n") {
			if line != "" {
				fmt.Fprintf(stderr, "  %s\n", line)
			}
		}
		fmt.Fprintln(stderr, "  ---")
	}
	{
		dockerCmd := dockerCommandName()
		rm := execCommandFn(dockerCmd, "rm", cid)
		rm.Env = dockerCommandEnv(os.Environ(), dockerCmd)
		_ = rm.Run()
	}
	if workVolume != "" && !o.SkipVolumeWorkspaceSync {
		fmt.Fprintln(stderr, "[dockpipe] Syncing managed workspace changes back from session volume…")
		if err := dockerSyncWorkspaceFromVolume(o.Image, workVolume, workHost); err != nil && rc == 0 {
			return 1, err
		}
		fmt.Fprintln(stderr, "[dockpipe] Managed workspace sync-back complete.")
	}

	if o.CommitOnHost {
		if rc != 0 {
			fmt.Fprintln(stderr, "[dockpipe] Skipping host commit (container exited with non-zero code).")
		} else {
			_ = commitOnHostFn(workHost, o.CommitMessage, o.BundleOut, o.BundleAll)
		}
	}
	return rc, nil
}

func dockerSyncWorkspaceIntoVolume(image, hostDir, volume string) error {
	if top, err := GitTopLevel(hostDir); err == nil && samePathForDocker(top, hostDir) && hasStandaloneGitDir(hostDir) {
		return dockerBootstrapGitWorkspaceVolume(hostDir, volume)
	}
	return dockerSyncWorkspace(dockerWorkspaceHelperImage(image), hostDir+":/dockpipe-sync-src:ro", volume+":/dockpipe-sync-dst", "cd /dockpipe-sync-src && tar --exclude=.git --exclude=bin/.dockpipe --exclude=.dorkpipe -cf - . | tar xf - -C /dockpipe-sync-dst")
}

func dockerSyncWorkspaceFromVolume(image, volume, hostDir string) error {
	if top, err := GitTopLevel(hostDir); err == nil && samePathForDocker(top, hostDir) && hasStandaloneGitDir(hostDir) {
		return dockerApplyGitWorkspaceVolume(volume, hostDir)
	}
	return dockerSyncWorkspace(dockerWorkspaceHelperImage(image), volume+":/dockpipe-sync-src:ro", hostDir+":/dockpipe-sync-dst", "cd /dockpipe-sync-src && tar --exclude=.git --exclude=bin/.dockpipe --exclude=.dorkpipe -cf - . | tar xf - -C /dockpipe-sync-dst")
}

func dockerSyncWorkspace(image, srcMount, dstMount, script string) error {
	image = strings.TrimSpace(image)
	if image == "" {
		return fmt.Errorf("workspace volume sync requires an image")
	}
	dockerCmd := dockerCommandName()
	args := []string{"run", "--rm", "--entrypoint", "sh", "-v", srcMount, "-v", dstMount, "-u", "0", image, "-c", script}
	cmd := execCommandFn(dockerCmd, args...)
	cmd.Env = dockerCommandEnv(os.Environ(), dockerCmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("sync workspace docker volume: %w\n%s", err, out)
	}
	return nil
}

func dockerBootstrapGitWorkspaceVolume(hostDir, volume string) error {
	image := dockerWorkspaceHelperImage("")
	script := strings.Join([]string{
		`set -e`,
		`branch="$(git -C /dockpipe-sync-src rev-parse --abbrev-ref HEAD)"`,
		`head_commit="$(git -C /dockpipe-sync-src rev-parse HEAD)"`,
		`if [ ! -d /dockpipe-sync-dst/.git ]; then`,
		`  find /dockpipe-sync-dst -mindepth 1 -maxdepth 1 -exec rm -rf {} + 2>/dev/null || true`,
		`  git clone /dockpipe-sync-src /dockpipe-sync-dst >/dev/null 2>&1`,
		`fi`,
		`git -C /dockpipe-sync-dst remote remove dockpipe-host >/dev/null 2>&1 || true`,
		`git -C /dockpipe-sync-dst remote add dockpipe-host /dockpipe-sync-src`,
		`git -C /dockpipe-sync-dst fetch --prune dockpipe-host >/dev/null 2>&1`,
		`if [ "$branch" = "HEAD" ]; then`,
		`  git -C /dockpipe-sync-dst checkout --detach "$head_commit" >/dev/null 2>&1`,
		`  git -C /dockpipe-sync-dst reset --hard "$head_commit" >/dev/null 2>&1`,
		`else`,
		`  git -C /dockpipe-sync-dst checkout -B "$branch" "dockpipe-host/$branch" >/dev/null 2>&1`,
		`  git -C /dockpipe-sync-dst reset --hard "dockpipe-host/$branch" >/dev/null 2>&1`,
		`fi`,
		`git -C /dockpipe-sync-dst clean -fd >/dev/null 2>&1`,
	}, "\n")
	return dockerSyncWorkspace(image, hostDir+":/dockpipe-sync-src:ro", volume+":/dockpipe-sync-dst", script)
}

func dockerApplyGitWorkspaceVolume(volume, hostDir string) error {
	image := dockerWorkspaceHelperImage("")
	script := strings.Join([]string{
		`set -e`,
		`git -C /dockpipe-sync-src add -A`,
		`if git -C /dockpipe-sync-src diff --cached --quiet --no-ext-diff; then`,
		`  exit 0`,
		`fi`,
		`git -C /dockpipe-sync-src diff --cached --binary | git -C /dockpipe-sync-dst apply --whitespace=nowarn -`,
	}, "\n")
	return dockerSyncWorkspace(image, volume+":/dockpipe-sync-src", hostDir+":/dockpipe-sync-dst", script)
}

func dockerWorkspaceHelperImage(runImage string) string {
	if helper := strings.TrimSpace(os.Getenv("DOCKPIPE_RUNTIME_HELPER_IMAGE")); helper != "" {
		return helper
	}
	if helper := strings.TrimSpace(os.Getenv("DOCKPIPE_GIT_HELPER_IMAGE")); helper != "" {
		return helper
	}
	if strings.TrimSpace(runImage) != "" {
		return runImage
	}
	return "alpine/git:latest"
}

func hasStandaloneGitDir(hostDir string) bool {
	info, err := osStatDockerFn(filepath.Join(strings.TrimSpace(hostDir), ".git"))
	if err != nil {
		return false
	}
	return info.IsDir()
}

func samePathForDocker(a, b string) bool {
	a = filepath.Clean(strings.TrimSpace(a))
	b = filepath.Clean(strings.TrimSpace(b))
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
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

// RenderBannerForTerminal returns the launch/help banner variant for the current terminal width.
// Unlike PrintLaunchBanner, this always returns a string so callers can embed it in other output.
func RenderBannerForTerminal(stdout, stderr *os.File) string {
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}
	outFd, outOK := fdInt(stdout)
	errFd, errOK := fdInt(stderr)
	width := 0
	if outOK || errOK {
		width = terminalWidthForBanner(outFd, errFd)
	}
	return renderBannerForWidth(width)
}

func shouldShowSpinner(width int) bool {
	// Spinner uses carriage-return updates; hide it in narrow terminals to avoid messy wraps.
	return width >= 60
}
