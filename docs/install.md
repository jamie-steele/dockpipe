# Installing dockpipe

## Install the .deb (recommended)

1. Download the latest `.deb` from [Releases](https://github.com/jamie-steele/dockpipe/releases).
2. Install:

   ```bash
   sudo dpkg -i dockpipe_*_all.deb
   ```

3. If `dpkg` reports missing dependencies (e.g. Docker):

   ```bash
   sudo apt-get install -f
   ```

**Upgrades:** download the new .deb and run `sudo dpkg -i dockpipe_*_all.deb` again.

**Requirements:** Bash and Docker (`docker.io` or `docker-ce`). Install Docker if needed:

```bash
sudo apt-get install docker.io
```

---

## Or run from source (no root)

```bash
git clone https://github.com/jamie-steele/dockpipe.git
export PATH="$PATH:$(pwd)/dockpipe/bin"
dockpipe -- ls -la
```

---

## Building the .deb (for maintainers)

From the repo root:

```bash
./packaging/build-deb.sh 0.1.0
# Output: packaging/build/dockpipe_0.1.0_all.deb
```

Attach that file to a GitHub Release. If we add a proper APT repo later, we’ll document it here.
