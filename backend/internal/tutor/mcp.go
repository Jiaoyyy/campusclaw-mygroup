package tutor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"campusclaw/backend/internal/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MCP is configured by the server operator. Only explicitly allowlisted tools
// can be discovered or invoked; a remote server's annotations are not authority.
type MCPTool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema any    `json:"input_schema"`
}

func allowedMCPTools(cfg config.Config) map[string]bool {
	allowed := map[string]bool{}
	for _, name := range strings.Split(cfg.MCPAllowedTools, ",") {
		name = strings.TrimSpace(name)
		if name != "" {
			allowed[name] = true
		}
	}
	return allowed
}

func mcpConfigured(cfg config.Config) bool {
	u, err := url.Parse(cfg.MCPServerURL)
	return err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Host != "" && len(allowedMCPTools(cfg)) > 0
}

type mcpAuthTransport struct {
	base http.RoundTripper
	key  string
}

func (t mcpAuthTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	copy := request.Clone(request.Context())
	copy.Header.Set("Authorization", "Bearer "+t.key)
	return t.base.RoundTrip(copy)
}

func connectMCP(ctx context.Context, cfg config.Config) (*mcp.ClientSession, error) {
	if !mcpConfigured(cfg) {
		return nil, errors.New("MCP connection is not configured")
	}
	transport := http.DefaultTransport
	if cfg.MCPAPIKey != "" {
		transport = mcpAuthTransport{base: transport, key: cfg.MCPAPIKey}
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "campusclaw-tutor", Version: "1.0.0"}, nil)
	return client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint: cfg.MCPServerURL, HTTPClient: &http.Client{Transport: transport, Timeout: 12 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }},
		DisableStandaloneSSE: true, MaxRetries: -1,
	}, nil)
}

func discoverMCPTools(ctx context.Context, cfg config.Config) ([]MCPTool, error) {
	session, err := connectMCP(ctx, cfg)
	if err != nil {
		return nil, err
	}
	defer session.Close()
	allowed := allowedMCPTools(cfg)
	tools := []MCPTool{}
	cursor := ""
	for page := 0; page < 10; page++ {
		result, err := session.ListTools(ctx, &mcp.ListToolsParams{Cursor: cursor})
		if err != nil {
			return nil, err
		}
		for _, tool := range result.Tools {
			if tool == nil || !allowed[tool.Name] {
				continue
			}
			schema, err := json.Marshal(tool.InputSchema)
			if err != nil || len(schema) > 16<<10 {
				continue
			}
			tools = append(tools, MCPTool{Name: tool.Name, Description: tool.Description, InputSchema: tool.InputSchema})
		}
		if result.NextCursor == "" {
			return tools, nil
		}
		cursor = result.NextCursor
	}
	return nil, errors.New("MCP tool list exceeded page limit")
}

func callMCPTool(ctx context.Context, cfg config.Config, name string, arguments map[string]any) (string, error) {
	if !allowedMCPTools(cfg)[name] {
		return "", errors.New("MCP tool is not allowed")
	}
	session, err := connectMCP(ctx, cfg)
	if err != nil {
		return "", err
	}
	defer session.Close()
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: arguments})
	if err != nil {
		return "", err
	}
	if result.IsError {
		return "", errors.New("MCP tool reported an error")
	}
	var output strings.Builder
	for _, content := range result.Content {
		item, ok := content.(*mcp.TextContent)
		if !ok || item.Text == "" {
			continue // Do not forward images, resource links, or other active content to the model.
		}
		output.WriteString(item.Text)
		output.WriteByte('\n')
		if output.Len() > 8<<10 {
			break
		}
	}
	text := strings.TrimSpace(output.String())
	if text == "" {
		return "", errors.New("MCP tool returned no text")
	}
	runes := []rune(text)
	if len(runes) > 4000 {
		text = string(runes[:4000])
	}
	return text, nil
}

// chooseMCPTool makes one non-streaming, tool-choice request without sending
// class materials, conversation history, or access tokens to the model.
func chooseMCPTool(ctx context.Context, cfg config.Config, question string, tools []MCPTool) (string, map[string]any, error) {
	if len(tools) == 0 {
		return "", nil, nil
	}
	definitions := make([]any, 0, len(tools))
	for _, tool := range tools {
		definitions = append(definitions, map[string]any{"type": "function", "function": map[string]any{
			"name": tool.Name, "description": tool.Description, "parameters": tool.InputSchema,
		}})
	}
	body, _ := json.Marshal(map[string]any{
		"model": cfg.ChatModel, "stream": false, "temperature": 0, "tool_choice": "auto", "tools": definitions,
		"messages": []ChatMessage{
			{Role: "system", Content: "仅当用户问题确实需要外部信息时，从提供的只读工具中最多选择一个；否则直接回答无需工具。不要在工具参数中加入班级资料、历史消息或凭据。"},
			{Role: "user", Content: question},
		},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(cfg.ChatBaseURL, "/")+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.ChatAPIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.ChatAPIKey)
	}
	response, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return "", nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", nil, fmt.Errorf("tool selection status %d", response.StatusCode)
	}
	var payload struct {
		Choices []struct {
			Message struct {
				ToolCalls []struct {
					Type     string `json:"type"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&payload); err != nil || len(payload.Choices) == 0 {
		return "", nil, errors.New("invalid tool selection response")
	}
	calls := payload.Choices[0].Message.ToolCalls
	if len(calls) == 0 {
		return "", nil, nil
	}
	if len(calls) != 1 || calls[0].Type != "function" {
		return "", nil, errors.New("unexpected tool selection")
	}
	selected := calls[0].Function.Name
	valid := false
	for _, tool := range tools {
		if selected == tool.Name {
			valid = true
			break
		}
	}
	if !valid {
		return "", nil, errors.New("tool not bound to agent")
	}
	var arguments map[string]any
	if err := json.Unmarshal([]byte(calls[0].Function.Arguments), &arguments); err != nil || arguments == nil {
		return "", nil, errors.New("invalid tool arguments")
	}
	return selected, arguments, nil
}
