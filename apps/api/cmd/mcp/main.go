package main

import (
	"context"
	"fmt"
	"github.com/OstKost/avari-p3-express/apps/api/internal/mcpbridge"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"os"
)

func main() {
	base := os.Getenv("AVARI_API_URL")
	if base == "" {
		base = "http://127.0.0.1:4820"
	}
	s, e := mcpbridge.New(base, os.Getenv("AVARI_AGENT_TOKEN"))
	if e == nil {
		e = s.Run(context.Background(), &mcp.StdioTransport{})
	}
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
