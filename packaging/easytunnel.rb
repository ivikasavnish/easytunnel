class Easytunnel < Formula
  desc "Easy SSH Tunnel Manager with web interface"
  homepage "https://github.com/ivikasavnish/easytunnel"
  url "https://github.com/ivikasavnish/easytunnel/releases/download/v1.0.0/easytunnel-v1.0.0-darwin-amd64.tar.gz"
  sha256 "REPLACE_WITH_ACTUAL_SHA256"
  license "MIT"
  version "1.0.0"

  depends_on "openssh"

  on_intel do
    url "https://github.com/ivikasavnish/easytunnel/releases/download/v1.0.0/easytunnel-v1.0.0-darwin-amd64.tar.gz"
    sha256 "REPLACE_WITH_ACTUAL_SHA256_AMD64"
  end

  on_arm do
    url "https://github.com/ivikasavnish/easytunnel/releases/download/v1.0.0/easytunnel-v1.0.0-darwin-arm64.tar.gz"
    sha256 "REPLACE_WITH_ACTUAL_SHA256_ARM64"
  end

  def install
    bin.install "easytunnel"
    bin.install "debug-ssh.sh"
    bin.install "diagnose-tunnel.sh"
    
    # Install documentation
    doc.install "README.md"
    doc.install "QUICKSTART.md"
    doc.install "LICENSE"
  end

  service do
    run [opt_bin/"easytunnel"]
    keep_alive false
    log_path var/"log/easytunnel.log"
    error_log_path var/"log/easytunnel.log"
  end

  test do
    system "#{bin}/easytunnel", "--version"
  end
end
