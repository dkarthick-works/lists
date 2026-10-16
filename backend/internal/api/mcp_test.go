package api

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestMCP drives the real handler over Streamable HTTP: all five tools are
// listed, and failures come back as tool errors (not protocol errors), with
// expected messages kept and unexpected ones hidden.
func TestMCP(t *testing.T) {
	ctx := context.Background()
	hs := httptest.NewServer((&Server{}).mcpHandler())
	defer hs.Close()
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).
		Connect(ctx, &mcp.StreamableClientTransport{Endpoint: hs.URL, DisableStandaloneSSE: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	tools, err := cs.ListTools(ctx, nil)
	if err != nil || len(tools.Tools) != 5 {
		t.Fatalf("tools/list: %v %v", tools, err)
	}

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "get_list", Arguments: map[string]any{"id": "nope"}})
	if err != nil || !res.IsError || res.Content[0].(*mcp.TextContent).Text != "id must be a UUID; find it with search_lists or list_lists" {
		t.Errorf("bad uuid: %+v %v", res, err)
	}

	for in, want := range map[error]string{errListNotFound: "list not found", errors.New("db down"): "internal error"} {
		_, _, err := tool(func(context.Context, uuid.UUID, struct{}) (any, error) { return nil, in })(ctx, nil, struct{}{})
		if err == nil || err.Error() != want {
			t.Errorf("%v: got %v", in, err)
		}
	}
}
