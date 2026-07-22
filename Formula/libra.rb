# Homebrew formula for libra, built from this repo's source.
#
# This repo doubles as the tap (Homebrew normally expects a repo named
# `homebrew-<name>`; since this one isn't, tap it with the explicit URL):
#
#   brew tap madcamp-official/26s-w3-c2-01 https://github.com/madcamp-official/26s-w3-c2-01
#   brew install libra
#
# `url`/`tag`/`revision` below point at the v0.1.0 release tag. When cutting
# a new release, update both `tag` and `revision` to match.
class Libra < Formula
  desc "Analyze and manage local developer storage"
  homepage "https://github.com/madcamp-official/26s-w3-c2-01"
  url "https://github.com/madcamp-official/26s-w3-c2-01.git",
      tag:      "v0.1.0",
      revision: "91f009942c37956ce9333a35a608c29f873a3a99"
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
