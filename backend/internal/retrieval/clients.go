package retrieval

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"campusclaw/backend/internal/config"
)

var ErrUnavailable = errors.New("retrieval dependency unavailable")

type VectorPoint struct {
	ID    int64
	Score float64
}

type Embedder interface {
	Embed(context.Context, string) ([]float64, error)
}

type VectorStore interface {
	Upsert(context.Context, int64, []float64, map[string]any) error
	Delete(context.Context, []int64) error
	Query(context.Context, int64, []float64, int) ([]VectorPoint, error)
}

type Answerer interface {
	Answer(context.Context, string, []Hit) (string, error)
}

type modelClient struct {
	http  *http.Client
	base  string
	key   string
	model string
}

var citationPattern = regexp.MustCompile(`\[(\d+)\]`)

func NewEmbedder(cfg config.Config) Embedder {
	return modelClient{http: &http.Client{Timeout: 30 * time.Second}, base: cfg.EmbeddingBaseURL, key: cfg.EmbeddingAPIKey, model: cfg.EmbeddingModel}
}

func NewAnswerer(cfg config.Config) Answerer {
	return modelClient{http: &http.Client{Timeout: 45 * time.Second}, base: cfg.ChatBaseURL, key: cfg.ChatAPIKey, model: cfg.ChatModel}
}

func (c modelClient) call(ctx context.Context, endpoint string, requestBody any, responseBody any) error {
	if c.base == "" || c.model == "" {
		return ErrUnavailable
	}
	base, err := url.Parse(c.base)
	if err != nil || (base.Scheme != "https" && base.Scheme != "http") || base.Host == "" {
		return ErrUnavailable
	}
	data, err := json.Marshal(requestBody)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.base, "/")+endpoint, bytes.NewReader(data))
	if err != nil {
		return ErrUnavailable
	}
	req.Header.Set("Content-Type", "application/json")
	if c.key != "" {
		req.Header.Set("Authorization", "Bearer "+c.key)
	}
	response, err := c.http.Do(req)
	if err != nil {
		return ErrUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return ErrUnavailable
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(responseBody); err != nil {
		return ErrUnavailable
	}
	return nil
}

func (c modelClient) Embed(ctx context.Context, text string) ([]float64, error) {
	var response struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}
	if err := c.call(ctx, "/embeddings", map[string]any{"model": c.model, "input": text}, &response); err != nil {
		return nil, err
	}
	if len(response.Data) != 1 || len(response.Data[0].Embedding) == 0 {
		return nil, ErrUnavailable
	}
	return response.Data[0].Embedding, nil
}

func (c modelClient) Answer(ctx context.Context, question string, hits []Hit) (string, error) {
	var evidence strings.Builder
	for i, hit := range hits {
		fmt.Fprintf(&evidence, "[%d] 材料：%s；切片：%d；正文：%s\n", i+1, hit.Title, hit.ChunkIndex, hit.ChunkText)
	}
	messages := []map[string]string{
		{"role": "system", "content": "只根据用户消息中的本班材料切片回答。回答要简短，并以 [1]、[2] 等编号标出所用依据；不得编造出处或使用外部知识。"},
		{"role": "user", "content": "问题：" + question + "\n本班材料切片：\n" + evidence.String()},
	}
	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := c.call(ctx, "/chat/completions", map[string]any{"model": c.model, "messages": messages, "temperature": 0}, &response); err != nil {
		return "", err
	}
	if len(response.Choices) == 0 || strings.TrimSpace(response.Choices[0].Message.Content) == "" {
		return "", ErrUnavailable
	}
	answer := strings.TrimSpace(response.Choices[0].Message.Content)
	refs := citationPattern.FindAllStringSubmatch(answer, -1)
	if len(refs) == 0 {
		return "", ErrUnavailable
	}
	for _, ref := range refs {
		index, err := strconv.Atoi(ref[1])
		if err != nil || index < 1 || index > len(hits) {
			return "", ErrUnavailable
		}
	}
	return answer, nil
}

type qdrantClient struct {
	http       *http.Client
	base       string
	collection string
}

func NewVectorStore(cfg config.Config) VectorStore {
	return &qdrantClient{http: &http.Client{Timeout: 15 * time.Second}, base: strings.TrimRight(cfg.QdrantURL, "/"), collection: cfg.QdrantCollection}
}

func (c *qdrantClient) path(suffix string) string {
	return c.base + "/collections/" + url.PathEscape(c.collection) + suffix
}

func (c *qdrantClient) do(ctx context.Context, method, path string, body any, out any) (int, error) {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return 0, err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, path, reader)
	if err != nil {
		return 0, ErrUnavailable
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	response, err := c.http.Do(req)
	if err != nil {
		return 0, ErrUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return response.StatusCode, ErrUnavailable
	}
	if out != nil {
		if err := json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(out); err != nil {
			return response.StatusCode, ErrUnavailable
		}
	}
	return response.StatusCode, nil
}

func (c *qdrantClient) ensureCollection(ctx context.Context, dimension int) error {
	var existing struct {
		Result struct {
			Config struct {
				Params struct {
					Vectors struct {
						Size int `json:"size"`
					} `json:"vectors"`
				} `json:"params"`
			} `json:"config"`
		} `json:"result"`
	}
	status, err := c.do(ctx, http.MethodGet, c.path(""), nil, &existing)
	if err == nil {
		if existing.Result.Config.Params.Vectors.Size != dimension {
			return ErrUnavailable
		}
		return nil
	}
	if status != http.StatusNotFound {
		return err
	}
	_, err = c.do(ctx, http.MethodPut, c.path(""), map[string]any{"vectors": map[string]any{"size": dimension, "distance": "Cosine"}}, nil)
	if err != nil {
		status, retryErr := c.do(ctx, http.MethodGet, c.path(""), nil, &existing)
		if retryErr != nil || status != http.StatusOK || existing.Result.Config.Params.Vectors.Size != dimension {
			return ErrUnavailable
		}
	}
	return nil
}

func (c *qdrantClient) Upsert(ctx context.Context, id int64, vector []float64, payload map[string]any) error {
	if len(vector) == 0 {
		return ErrUnavailable
	}
	if err := c.ensureCollection(ctx, len(vector)); err != nil {
		return err
	}
	_, err := c.do(ctx, http.MethodPut, c.path("/points?wait=true"), map[string]any{"points": []any{map[string]any{"id": id, "vector": vector, "payload": payload}}}, nil)
	return err
}

func (c *qdrantClient) Delete(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	status, err := c.do(ctx, http.MethodPost, c.path("/points/delete?wait=true"), map[string]any{"points": ids}, nil)
	if status == http.StatusNotFound {
		return nil
	}
	return err
}

func (c *qdrantClient) Query(ctx context.Context, classID int64, vector []float64, limit int) ([]VectorPoint, error) {
	var response struct {
		Result struct {
			Points []struct {
				ID    int64   `json:"id"`
				Score float64 `json:"score"`
			} `json:"points"`
		} `json:"result"`
	}
	body := map[string]any{"query": vector, "filter": map[string]any{"must": []any{map[string]any{"key": "class_id", "match": map[string]any{"value": classID}}}}, "score_threshold": 0.35, "limit": limit, "with_payload": false}
	status, err := c.do(ctx, http.MethodPost, c.path("/points/query"), body, &response)
	if status == http.StatusNotFound {
		return []VectorPoint{}, nil
	}
	if err != nil {
		return nil, err
	}
	points := make([]VectorPoint, 0, len(response.Result.Points))
	for _, point := range response.Result.Points {
		if point.Score >= 0.35 {
			points = append(points, VectorPoint{ID: point.ID, Score: point.Score})
		}
	}
	return points, nil
}
