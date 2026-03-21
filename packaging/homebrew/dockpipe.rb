class Dockpipe < Formula
  desc "Run -> Isolate -> Act in disposable containers"
  homepage "https://github.com/jamie-steele/dockpipe"
  url "https://github.com/jamie-steele/dockpipe/archive/refs/tags/v0.6.0.tar.gz"
  sha256 "REPLACE_WITH_RELEASE_TARBALL_SHA256"
  license "Apache-2.0"

  depends_on "go" => :build

  def install
    # Templates/scripts/images are embedded in the binary; unpacked to the user cache on first run.
    system "go", "build", "-trimpath", "-ldflags", "-s -w", "-o", "dockpipe", "./cmd/dockpipe"
    bin.install "dockpipe"
  end

  test do
    assert_match "dockpipe", shell_output("#{bin}/dockpipe --help")
  end
end
