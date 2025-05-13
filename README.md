# PostgreSQL MCP Server

![PostgreSQL MCP Server Banner](banner.png)

A Model Control Protocol (MCP) server for PostgreSQL databases that allows Claude to interact with your PostgreSQL databases.

## Using with Claude Code

The easiest way to use this server with Claude Code:

```bash
# Generate MCP config and add it to Claude
JSON=$(postgres-mcp-server --json --connection-string '...') \
&& claude mcp add-json "$JSON"
```

## Installation

### Using Homebrew

```bash
# Add the tap
brew tap mgorunuch/postgres-mcp-server https://github.com/mgorunuch/postgres-mcp-server

# Install the server
brew install mgorunuch/postgres-mcp-server/postgres-mcp-server
```

### Manual Installation

Download the appropriate binary for your platform from the releases page.

## Setup

There are two ways to use this server:

### 1. Running Directly

1. Set your PostgreSQL connection string as an environment variable (optional):
   ```
   export POSTGRES_CONNECTION_STRING="postgresql://user:password@host:port/dbname?sslmode=disable"
   ```
   If not provided, defaults to `postgresql://postgres:postgres@localhost:5432/postgres?sslmode=disable`

2. Launch the server:
   ```
   postgres-mcp-server
   ```

3. Use with Claude by setting up the MCP connection to this server.

### 2. Using with Claude's MCP Configuration

1. Generate the MCP configuration JSON:
   ```
   postgres-mcp-server --json --connection-string "postgresql://user:password@host:port/dbname?sslmode=disable"
   ```
   This will output a JSON configuration you can use with Claude's MCP tools.

   Example output:
   ```json
   {"type":"stdio","command":"/path/to/postgres-mcp-server","args":["--connection-string","postgresql://user:password@host:port/dbname?sslmode=disable"],"env":{}}
   ```

2. Copy this JSON output and use it to configure the MCP connection in Claude.

## Command Line Options

- `--version`: Displays version information
- `--json`: Outputs MCP configuration JSON for use with Claude
- `--connection-string`: PostgreSQL connection string (overrides environment variable)

## Security

For safety, potentially destructive queries (DROP, TRUNCATE, DELETE, UPDATE, ALTER, CREATE, INSERT) are blocked by default.

To execute these queries, you must explicitly set `unsafe: true` when using the pg_query tool.