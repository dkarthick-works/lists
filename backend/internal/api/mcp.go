package api

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"lists/internal/auth"
)

// mcpHandler serves the MCP tools over Streamable HTTP. It must sit behind
// auth.KeyMiddleware: every tool acts as auth.UserID(ctx).
func (s *Server) mcpHandler() http.Handler {
	srv := mcp.NewServer(&mcp.Implementation{Name: "lists", Version: "1.0.0"}, &mcp.ServerOptions{
		Instructions: "The user's lists: a tree of lists whose entries are text, pages or nested lists. " +
			"Find a list's id with search_lists (by name) or list_lists (top level) before get_list or add_entry.",
	})

	// Both create tools share the page rule; state it with the real limits.
	pageRule := fmt.Sprintf("Text over %d words (or over %d characters) becomes a page: an item titled "+
		"with its opening words, which a generated title may replace shortly after, whose body holds the "+
		"full text (at most %d characters).", s.pages.WordThreshold, maxTextLen, s.pages.MaxChars)

	mcp.AddTool(srv, &mcp.Tool{
		Name: "search_lists",
		Description: "Find the user's lists, at any depth, by title. Matches titles containing the query " +
			"and near misses (small typos), prefix matches first, up to 8 results. Each result has the list's " +
			"id (a UUID), its title (text) and its parent list's title (parent_text, null at top level). " +
			"Use it to get a list's id by name before get_list or add_entry.",
	}, tool(func(ctx context.Context, user uuid.UUID, in struct {
		Query string `json:"query" jsonschema:"words from the list's title"`
	}) (any, error) {
		rows, err := s.searchLists(ctx, user, in.Query)
		return map[string]any{"lists": rows}, err
	}))

	mcp.AddTool(srv, &mcp.Tool{
		Name: "list_lists",
		Description: "List all of the user's top-level lists (not nested ones; use search_lists for those). " +
			"Each list has an id (a UUID), its title (text), pinned_at if pinned to the home page, and " +
			"updated_at, which is bumped whenever anything inside it changes.",
	}, tool(func(ctx context.Context, user uuid.UUID, _ struct{}) (any, error) {
		rows, err := s.topLists(ctx, user)
		return map[string]any{"lists": rows}, err
	}))

	mcp.AddTool(srv, &mcp.Tool{
		Name: "get_list",
		Description: "Read one list: the list itself, its entries (open ones in order, then completed " +
			"ones), and its ancestors (the lists above it, outermost first). An entry with is_list=true is itself a list: call get_list with its " +
			"id to read inside it. An entry with a body is a page: text is its title, body its full text. " +
			"completed_at is set on completed entries. Get the id from search_lists or list_lists first.",
	}, tool(func(ctx context.Context, user uuid.UUID, in struct {
		ID string `json:"id" jsonschema:"the list's id, a UUID from search_lists, list_lists or get_list"`
	}) (any, error) {
		id, err := parseID(in.ID, "id")
		if err != nil {
			return nil, err
		}
		return s.openList(ctx, user, id)
	}))

	mcp.AddTool(srv, &mcp.Tool{
		Name: "create_list",
		Description: "Create a new top-level list and return it, including its id (a UUID) for add_entry. " +
			"Check search_lists first to avoid creating a duplicate. " + pageRule +
			" For a long title, the new list gets that short title and holds the page as its only entry.",
	}, tool(func(ctx context.Context, user uuid.UUID, in struct {
		Title string `json:"title" jsonschema:"the list's title"`
	}) (any, error) {
		return s.newList(ctx, user, in.Title)
	}))

	mcp.AddTool(srv, &mcp.Tool{
		Name: "add_entry",
		Description: "Add an entry to the end of a list and return it. With is_list=true the entry is " +
			"itself a list (nested), which can take entries of its own. Get the list's id from search_lists " +
			"or list_lists first. " + pageRule + " With is_list=true, long text makes a nested list holding that page.",
	}, tool(func(ctx context.Context, user uuid.UUID, in struct {
		ListID string `json:"list_id" jsonschema:"id of the list to add to, a UUID from search_lists, list_lists or get_list"`
		Text   string `json:"text" jsonschema:"the entry's text"`
		IsList bool   `json:"is_list,omitempty" jsonschema:"true to add a nested list instead of a plain entry"`
	}) (any, error) {
		id, err := parseID(in.ListID, "list_id")
		if err != nil {
			return nil, err
		}
		return s.addEntry(ctx, user, id, in.Text, in.IsList)
	}))

	h := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv },
		&mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.ServeHTTP(noStore{w}, r)
	})
}

// noStore keeps per-user results out of HTTP caches. The SDK sets its own
// Cache-Control (no-cache), so this overrides it as the headers go out.
type noStore struct{ http.ResponseWriter }

func (w noStore) WriteHeader(code int) {
	w.Header().Set("Cache-Control", "no-store")
	w.ResponseWriter.WriteHeader(code)
}

func (w noStore) Write(b []byte) (int, error) {
	w.Header().Set("Cache-Control", "no-store") // no-op once headers are sent
	return w.ResponseWriter.Write(b)
}

// Unwrap lets http.ResponseController reach Flush on the real writer.
func (w noStore) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// tool adapts fn to the SDK: it passes the key's user, and turns errors into
// tool errors the agent can read. Expected failures keep their message;
// anything else is logged and reported generically.
func tool[In any](fn func(context.Context, uuid.UUID, In) (any, error)) mcp.ToolHandlerFor[In, any] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in In) (*mcp.CallToolResult, any, error) {
		out, err := fn(ctx, auth.UserID(ctx), in)
		var e *errStatus
		if err != nil && !errors.As(err, &e) {
			log.Printf("mcp tool: %v", err)
			err = errors.New("internal error")
		}
		return nil, out, err
	}
}

func parseID(s, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, &errStatus{http.StatusBadRequest, name + " must be a UUID; find it with search_lists or list_lists"}
	}
	return id, nil
}
