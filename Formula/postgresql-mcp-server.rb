class PostgresqlMcpServer < Formula
  desc "Model Control Protocol (MCP) server for PostgreSQL databases"
  homepage "https://github.com/mgorunuch/postgres-mcp-server"
  version "0.1.7"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/mgorunuch/postgres-mcp-server/releases/download/v#{version}/postgres-mcp-server_Darwin_arm64.tar.gz"
      sha256 "REPLACE_WITH_ACTUAL_SHA_AFTER_FIRST_RELEASE"
    elsif Hardware::CPU.intel?
      url "https://github.com/mgorunuch/postgres-mcp-server/releases/download/v#{version}/postgres-mcp-server_Darwin_x86_64.tar.gz"
      sha256 "REPLACE_WITH_ACTUAL_SHA_AFTER_FIRST_RELEASE"
    end
  end

  on_linux do
    if Hardware::CPU.arm? && Hardware::CPU.is_64_bit?
      url "https://github.com/mgorunuch/postgres-mcp-server/releases/download/v#{version}/postgres-mcp-server_Linux_arm64.tar.gz"
      sha256 "REPLACE_WITH_ACTUAL_SHA_AFTER_FIRST_RELEASE"
    elsif Hardware::CPU.intel?
      url "https://github.com/mgorunuch/postgres-mcp-server/releases/download/v#{version}/postgres-mcp-server_Linux_x86_64.tar.gz"
      sha256 "REPLACE_WITH_ACTUAL_SHA_AFTER_FIRST_RELEASE"
    end
  end

  def install
    bin.install "postgres-mcp-server"
  end

  test do
    system "#{bin}/postgres-mcp-server", "--version"
  end
end