package tutor

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"campusclaw/backend/internal/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMCPDiscoveryAndCallHonorServerAllowlist(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "test-teaching-tools", Version: "1.0"}, nil)
	called := false
	mcp.AddTool(server, &mcp.Tool{Name: "course_outline", Description: "Read a public course outline"}, func(_ context.Context, _ *mcp.CallToolRequest, input struct {
		Course string `json:"course"`
	}) (*mcp.CallToolResult, any, error) {
		called = true
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "Outline for " + input.Course}}}, nil, nil
	})
	mcp.AddTool(server, &mcp.Tool{Name: "unapproved_tool"}, func(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		t.Fatal("unapproved tool must not be called")
		return nil, nil, nil
	})
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, &mcp.StreamableHTTPOptions{JSONResponse: true})
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer local-test-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		handler.ServeHTTP(w, r)
	}))
	defer httpServer.Close()
	cfg := config.Config{MCPServerURL: httpServer.URL, MCPAPIKey: "local-test-key", MCPAllowedTools: "course_outline"}
	tools, err := discoverMCPTools(context.Background(), cfg)
	if err != nil || len(tools) != 1 || tools[0].Name != "course_outline" {
		t.Fatalf("approved discovery failed: %+v %v", tools, err)
	}
	if _, err = callMCPTool(context.Background(), cfg, "unapproved_tool", nil); err == nil {
		t.Fatal("unapproved tool was accepted")
	}
	result, err := callMCPTool(context.Background(), cfg, "course_outline", map[string]any{"course": "算法"})
	if err != nil || !called || result != "Outline for 算法" {
		t.Fatalf("MCP call failed: result=%q called=%v err=%v", result, called, err)
	}
}

func TestChooseMCPToolAndSeparateEvidence(t *testing.T) {
	chat := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Tools    []any         `json:"tools"`
			Messages []ChatMessage `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if len(request.Tools) != 1 || len(request.Messages) != 2 || strings.Contains(request.Messages[1].Content, "私有材料") {
			t.Fatalf("tool choice leaked class context: %+v", request)
		}
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(request.Messages[1].Content, "bad") {
			_, _ = w.Write([]byte(`{"choices":[{"message":{"tool_calls":[{"type":"function","function":{"name":"unapproved_tool","arguments":"{}"}}]}}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"tool_calls":[{"type":"function","function":{"name":"course_outline","arguments":"{\"course\":\"算法\"}"}}]}}]}`))
	}))
	defer chat.Close()
	cfg := config.Config{ChatBaseURL: chat.URL, ChatModel: "test-model"}
	name, arguments, err := chooseMCPTool(context.Background(), cfg, "算法课程有哪些章节？", []MCPTool{{Name: "course_outline", InputSchema: map[string]any{"type": "object"}}})
	if err != nil || name != "course_outline" || arguments["course"] != "算法" {
		t.Fatalf("tool choice failed: %q %+v %v", name, arguments, err)
	}
	if _, _, err = chooseMCPTool(context.Background(), cfg, "bad", []MCPTool{{Name: "course_outline", InputSchema: map[string]any{"type": "object"}}}); err == nil {
		t.Fatal("model-selected unbound tool was accepted")
	}
	messages := assembleWithMCP(Agent{Prompt: "你是教师"}, nil, "章节？", nil, nil, name, "第一章")
	if strings.Contains(messages[0].Content, "第一章") || !strings.Contains(messages[1].Content, "第一章") || !strings.Contains(messages[0].Content, "不可信数据") {
		t.Fatalf("MCP result crossed prompt boundary: %+v", messages)
	}
}
