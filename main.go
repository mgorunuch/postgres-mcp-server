package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/lib/pq"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Version information set by build flags
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// DB is the global database connection
var DB *sql.DB

// MCPConfig holds the configuration for the MCP server
type MCPConfig struct {
	Type    string            `json:"type"`
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
}

func main() {
	// Define command line flags
	jsonFlag := flag.Bool("json", false, "Output MCP configuration as JSON")
	versionFlag := flag.Bool("version", false, "Display version information")
	connectionStringPtr := flag.String("connection-string", "", "PostgreSQL connection string")
	flag.Parse()

	// Check if version information is requested
	if *versionFlag {
		fmt.Printf("PostgreSQL MCP Server\nVersion: %s\nCommit: %s\nBuild date: %s\n", version, commit, date)
		return
	}

	// Check if JSON output is requested
	if *jsonFlag {
		config := MCPConfig{
			Type:    "stdio",
			Command: getExecutablePath(),
		}

		// Add connection string as argument if provided
		if *connectionStringPtr != "" {
			config.Args = []string{"--connection-string", *connectionStringPtr}
		}

		// Output JSON configuration
		jsonOutput, err := json.Marshal(config)
		if err != nil {
			log.Fatalf("Failed to generate JSON: %v", err)
		}
		fmt.Println(string(jsonOutput))
		return
	}

	// Normal server operation mode
	// Determine connection string from sources in order of priority:
	// 1. Connection string flag
	// 2. Environment variable
	// 3. Default value
	connStr := "postgresql://postgres:postgres@localhost:5432/postgres?sslmode=disable"

	if *connectionStringPtr != "" {
		// Use connection string from flag if provided
		connStr = *connectionStringPtr
	} else {
		// Use connection string from environment variable if provided
		envConnStr := os.Getenv("POSTGRES_CONNECTION_STRING")
		if envConnStr != "" {
			connStr = envConnStr
		}
	}

	// Connect to the PostgreSQL database
	var err error
	DB, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer DB.Close()

	// Test the connection
	if err = DB.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	log.Println("Connected to PostgreSQL database")

	// Create a new MCP server
	s := server.NewMCPServer(
		"PostgreSQL MCP Server",
		version, // Version from build flags
		server.WithToolCapabilities(true),
		server.WithRecovery(),
	)

	// Add PostgreSQL query tool
	queryTool := mcp.NewTool("pg_query",
		mcp.WithDescription("Execute a PostgreSQL query"),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("The SQL query to execute"),
		),
		mcp.WithBoolean("unsafe",
			mcp.Description("Set to true to allow potentially unsafe queries (use with caution)"),
		),
	)

	// Add the query tool handler
	s.AddTool(queryTool, handleQuery)

	// Add PostgreSQL schema information tool
	schemaInfoTool := mcp.NewTool("pg_schema_info",
		mcp.WithDescription("Get schema information about database tables"),
		mcp.WithString("table",
			mcp.Description("Specific table to get schema for (leave empty for all tables)"),
		),
	)

	// Add the schema info tool handler
	s.AddTool(schemaInfoTool, handleSchemaInfo)

	// Start the server using stdio
	log.Println("Starting PostgreSQL MCP Server...")
	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

// handleQuery executes a PostgreSQL query and returns the result
func handleQuery(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query, ok := request.Params.Arguments["query"].(string)
	if !ok {
		return mcp.NewToolResultError("Query parameter is required"), nil
	}

	// Check if unsafe is enabled
	unsafe := false
	if val, ok := request.Params.Arguments["unsafe"].(bool); ok {
		unsafe = val
	}

	// Basic safety check for non-unsafe queries
	if !unsafe {
		lowerQuery := strings.ToLower(query)
		if strings.Contains(lowerQuery, "drop ") ||
			strings.Contains(lowerQuery, "truncate ") ||
			strings.Contains(lowerQuery, "delete ") ||
			strings.Contains(lowerQuery, "update ") ||
			strings.Contains(lowerQuery, "alter ") ||
			strings.Contains(lowerQuery, "create ") ||
			strings.Contains(lowerQuery, "insert ") {
			return mcp.NewToolResultError("Potentially unsafe query detected. Set 'unsafe' to true to execute."), nil
		}
	}

	// Execute the query
	rows, err := DB.QueryContext(ctx, query)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Query execution failed: %v", err)), nil
	}
	defer rows.Close()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get column names: %v", err)), nil
	}

	// Prepare the result
	var result []map[string]interface{}
	colTypes, _ := rows.ColumnTypes()

	// Process each row
	for rows.Next() {
		// Create a slice of interface{} to hold the values
		values := make([]interface{}, len(columns))
		valuePointers := make([]interface{}, len(columns))

		// Initialize pointers
		for i := range values {
			valuePointers[i] = &values[i]
		}

		// Scan the result into the pointers
		if err := rows.Scan(valuePointers...); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error scanning row: %v", err)), nil
		}

		// Create a map for this row
		row := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]

			// Handle null values
			if val == nil {
				row[col] = nil
				continue
			}

			// Convert to appropriate type based on column type
			switch colTypes[i].DatabaseTypeName() {
			case "INT4", "INT8":
				if v, ok := val.(int64); ok {
					row[col] = v
				} else {
					row[col] = val
				}
			case "FLOAT4", "FLOAT8":
				if v, ok := val.(float64); ok {
					row[col] = v
				} else {
					row[col] = val
				}
			default:
				// Convert []byte to string for text types
				if b, ok := val.([]byte); ok {
					row[col] = string(b)
				} else {
					row[col] = val
				}
			}
		}

		result = append(result, row)
	}

	// Check for errors during iteration
	if err = rows.Err(); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Error iterating rows: %v", err)), nil
	}

	// If no rows were returned
	if len(result) == 0 {
		jsonData := map[string]interface{}{
			"message": "Query executed successfully with no rows returned",
			"columns": columns,
		}
		return mcp.NewToolResultText(fmt.Sprintf("%v", jsonData)), nil
	}

	// Return the result
	jsonData := map[string]interface{}{
		"columns": columns,
		"rows":    result,
		"count":   len(result),
	}
	return mcp.NewToolResultText(fmt.Sprintf("%v", jsonData)), nil
}

// handleSchemaInfo returns information about database tables
func handleSchemaInfo(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Get table parameter (optional)
	var tableName string
	if val, ok := request.Params.Arguments["table"].(string); ok {
		tableName = val
	}

	// Construct the query based on whether a specific table was requested
	query := `
		SELECT 
			t.table_name, 
			c.column_name, 
			c.data_type, 
			c.is_nullable,
			c.column_default,
			tc.constraint_type
		FROM 
			information_schema.tables t
		JOIN 
			information_schema.columns c ON t.table_name = c.table_name
		LEFT JOIN 
			information_schema.key_column_usage kcu ON c.column_name = kcu.column_name AND c.table_name = kcu.table_name
		LEFT JOIN 
			information_schema.table_constraints tc ON kcu.constraint_name = tc.constraint_name
		WHERE 
			t.table_schema = 'public'
	`

	// Add table filter if specified
	if tableName != "" {
		query += fmt.Sprintf(" AND t.table_name = '%s'", tableName)
	}

	query += " ORDER BY t.table_name, c.ordinal_position"

	// Execute the query
	rows, err := DB.QueryContext(ctx, query)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get schema information: %v", err)), nil
	}
	defer rows.Close()

	// Process the results
	tableMap := make(map[string][]map[string]interface{})

	for rows.Next() {
		var tableName, columnName, dataType, isNullable, columnDefault sql.NullString
		var constraintType sql.NullString

		if err := rows.Scan(&tableName, &columnName, &dataType, &isNullable, &columnDefault, &constraintType); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error scanning row: %v", err)), nil
		}

		columnInfo := map[string]interface{}{
			"column_name": columnName.String,
			"data_type":   dataType.String,
			"is_nullable": isNullable.String == "YES",
		}

		if columnDefault.Valid {
			columnInfo["default_value"] = columnDefault.String
		}

		if constraintType.Valid {
			columnInfo["constraint"] = constraintType.String
		}

		tableMap[tableName.String] = append(tableMap[tableName.String], columnInfo)
	}

	if err = rows.Err(); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Error iterating rows: %v", err)), nil
	}

	// Convert to a list of tables with columns
	var tables []map[string]interface{}
	for tableName, columns := range tableMap {
		tables = append(tables, map[string]interface{}{
			"table_name": tableName,
			"columns":    columns,
		})
	}

	// If no tables were found
	if len(tables) == 0 {
		if tableName == "" {
			return mcp.NewToolResultText("No tables found in the database."), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Table '%s' not found.", tableName)), nil
	}

	// Return the schema information
	jsonData := map[string]interface{}{
		"tables": tables,
	}
	return mcp.NewToolResultText(fmt.Sprintf("%v", jsonData)), nil
}

// getEnv returns the value of an environment variable or a default value if not set
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// getExecutablePath returns the full path to the current executable
func getExecutablePath() string {
	execPath, err := os.Executable()
	if err != nil {
		// Fall back to just the binary name if we can't get the path
		return filepath.Base(os.Args[0])
	}
	return execPath
}
