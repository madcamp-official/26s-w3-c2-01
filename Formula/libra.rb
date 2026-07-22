# Homebrew formula for libra, built from this repo's source.
#
# This repo doubles as the tap (Homebrew normally expects a repo named
# `homebrew-<name>`; since this one isn't, tap it with the explicit URL):
#
#   brew tap madcamp-official/26s-w3-c2-01 https://github.com/madcamp-official/26s-w3-c2-01
#   brew install libra
#
# `url`/`tag`/`revision` below point at a released tag. Until a tag exists,
# only `brew install --HEAD libra` (building main) works — see
# scripts/macos/build.sh and the README's macOS install section for details.
class Libra < Formula
  desc "Analyze and manage local developer storage"
  homepage "https://github.com/madcamp-official/26s-w3-c2-01"
  url "https://github.com/madcamp-official/26s-w3-c2-01.git",
      tag:      "v0.1.0",
      revision: "45785b1322a3573c21a31d94f1b040465ebcd30a"
  version "0.1.0"
  head "https://github.com/madcamp-official/26s-w3-c2-01.git", branch: "main"

  depends_on "go" => :build

  def install
    ldflags = %W[
      -s -w
      -X github.com/madcamp-official/26s-w3-c2-01/cmd.Version=#{version}
    ]
    system "go", "build", *std_go_args(ldflags: ldflags)
  end

  test do
    assert_match "libra", shell_output("#{bin}/libra --help")
    assert_match version.to_s, shell_output("#{bin}/libra --version")
  end
end
