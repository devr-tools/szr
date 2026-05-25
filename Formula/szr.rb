class Szr < Formula
  desc "Token-aware CLI proxy that trims noisy command output for LLM workflows"
  homepage "https://github.com/devr-tools/szr"
  license "Apache-2.0"
  version "0.2.0"
  url "https://github.com/devr-tools/szr/archive/refs/tags/v0.2.0.tar.gz"
  sha256 "1a47b3a12e2b712e252067425e23c1474f001109a6c365eda00478d1a54a72c3"
  head "https://github.com/devr-tools/szr.git", branch: "main"

  depends_on "go" => :build

  def install
    version_value = build.head? ? "HEAD" : version.to_s
    ldflags = ["-s", "-w", "-X", "main.version=#{version_value}"]

    system "go", "build", *std_go_args(ldflags: ldflags, output: bin/"szr"), "./cmd/szr"
  end

  test do
    expected_version = build.head? ? "HEAD" : version.to_s

    assert_match "szr #{expected_version}", shell_output("#{bin}/szr --version")
    assert_match "install target:", shell_output("#{bin}/szr self doctor")
  end
end
