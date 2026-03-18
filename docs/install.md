# Installing dockpipe

**Platforms:** Linux is the primary target (`.deb` package). macOS is supported from source (Bash + Docker). Windows is not supported; use [WSL](https://docs.microsoft.com/en-us/windows/wsl/) and install from source if you need it there.

---

## Install the .deb (Linux)

1. Download the latest `.deb` from [Releases](https://github.com/jamie-steele/dockpipe/releases).
2. Install:

   ```bash
   sudo dpkg -i dockpipe_*_all.deb
   ```

3. If `dpkg` reports missing dependencies (e.g. Docker):

   ```bash
   sudo apt-get install -f
   ```

Using `dpkg -i` avoids apt sandbox warnings when the .deb is in your home directory; `apt install ./file.deb` there can show a permission notice (apt’s `_apt` user can’t read the file).

**Upgrades:** download the new .deb and run `sudo dpkg -i dockpipe_*_all.deb` again.

**Requirements:** Bash and Docker (`docker.io` or `docker-ce`). Install Docker if needed:

```bash
sudo apt-get install docker.io
```

**Persistent data:** By default dockpipe uses a named volume `dockpipe-data` for persistent state (repos, tool config, first-time login), mounted at `/dockpipe-data`. Same volume every run = reusable agent environment. Use `--data-vol <name>` for a different volume, `--data-dir /path` to bind mount a host path, or `--no-data` to disable.

---

## Or run from source (Linux or macOS, no root)

```bash
git clone https://github.com/jamie-steele/dockpipe.git
export PATH="$PATH:$(pwd)/dockpipe/bin"
dockpipe -- ls -la
```

---

## Building the .deb (for maintainers)

From the repo root:

```bash
./packaging/build-deb.sh
# Output: packaging/build/dockpipe_<version>_all.deb
```

Attach that file to a GitHub Release. If we add a proper APT repo later, we’ll document it here.
