# MCP v2 integration demo

Run the automated in-memory stdio lifecycle demo:

```shell
go test ./mcp -run TestMCPIntegrationDemo -v
```

The test starts the MCP server, queries health and capabilities, starts a run,
records and completes a correlated action, retrieves its summary, and verifies
the event hash chain. It writes only to a temporary directory.
