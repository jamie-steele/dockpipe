class Dockpipe < Formula
  desc "Run -> Isolate -> Act in disposable containers"
  homepage "https://github.com/jamie-steele/dockpipe"
  url "https://github.com/jamie-steele/dockpipe/archive/refs/tags/v0.6.0.tar.gz"
  sha256 "REPLACE_WITH_RELEASE_TARBALL_SHA256"
  license "Apache-2.0"

  depends_on "go" => :build

  def install
    # Install runtime assets next to the binary and pin runtime root
    # via wrapper script (DOCKPIPE_REPO_ROOT).
    libexec.install "lib", "scripts", "images", "templates", "docs"
    (libexec/"version").write version.to_s
    (libexec/"bin").mkpath

    system "go", "build", "-trimpath", "-ldflags", "-s -w", "-o", libexec/"bin/dockpipe", "./cmd/dockpipe"
    bin.write_env_script libexec/"bin/dockpipe", DOCKPIPE_REPO_ROOT: libexec
  end

  test do
    assert_match "dockpipe", shell_output("#{bin}/dockpipe --help")
  end
end
