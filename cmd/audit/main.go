package main

import (
	"context"
	"os"

	"github.com/agent-audit-console/agent-audit-console/internal/cli"
)

func main() { os.Exit(cli.Execute(context.Background())) }
