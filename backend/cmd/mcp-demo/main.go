// mcp-demo is an optional, read-only Streamable HTTP server for local demos.
// It does not access CampusClaw's database, uploaded materials, or credentials.
package main

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type glossaryInput struct {
	Term string `json:"term" jsonschema:"Course term to look up"`
}

func main() {
	server := mcp.NewServer(&mcp.Implementation{Name: "campusclaw-demo-glossary", Version: "1.0.0"}, nil)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "course_glossary",
		Description: "查询本演示服务的公开课程术语表。适用于 MCP、班级隔离、混合检索、RRF 等术语；不查询私有教学材料。",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, func(_ context.Context, _ *mcp.CallToolRequest, input glossaryInput) (*mcp.CallToolResult, any, error) {
		definitions := map[string]string{
			"mcp":  "MCP（Model Context Protocol）是一种让应用与外部工具、资源交互的协议。教学助手通过它调用独立的工具服务。",
			"班级隔离": "班级隔离要求材料检索与会话访问以服务端认证得到的班级为边界，不能由请求自行指定班级。",
			"混合检索": "混合检索综合关键词匹配与向量相似度，再按融合得分排列候选切片。",
			"rrf":  "RRF（Reciprocal Rank Fusion）按多个候选列表中的名次融合结果，不要求各路原始分数同量纲。",
		}
		term := strings.ToLower(strings.TrimSpace(input.Term))
		answer, ok := definitions[term]
		if !ok {
			answer = "演示术语表中没有“" + input.Term + "”。可查询：MCP、班级隔离、混合检索、RRF。"
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: answer}}}, nil, nil
	})
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, &mcp.StreamableHTTPOptions{JSONResponse: true})
	mux := http.NewServeMux()
	mux.Handle("/mcp", handler)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	httpServer := &http.Server{Addr: ":8090", Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	log.Fatal(httpServer.ListenAndServe())
}
