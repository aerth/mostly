package mllm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/aerth/mostly/vector256"
)

// EmbedOptions application-wide global options
//
// Apart from Path, all serve as fallback if not present in request/env.
var EmbedOptions = struct {
	Model   string // if req.Model is empty and env is empty
	EnvHost string // the ENV_VAR to check to override [E.Host] (for when using 2 different hosts for llm,embeddings.)
	Path    string // custom endpoint url path (default /api/embed)
}{
	Model:   "nomic-embed-text:v1.5",
	EnvHost: "LLM_EMBED_HOST",
	Path:    "/api/embed",
}

// Embed generates new embeddings
//
// See [NewNomicDocument] and [NewNomicQuery] for model-specific query prefixes
func (e E) Embed(ctx context.Context, req EmbedRequest) (*EmbedResponse, error) {
	if req.Model == "" {
		req.Model = EmbedOptions.Model
	}
	if req.Host == "" {
		endpoint := e.Host.String()
		if embeddingHost := os.Getenv(EmbedOptions.EnvHost); embeddingHost != "" {
			endpoint = embeddingHost
		}
		req.Host = endpoint
	}
	req.Host = strings.TrimSuffix(req.Host, "/")
	if req.Host == "" {
		req.Host = "http://localhost:11434"
	}

	b, err := json.Marshal(req)
	if err != nil {
		return nil, Errorf("%w", err)
	}
	body := bytes.NewReader(b)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, req.Host+EmbedOptions.Path, body)
	if err != nil {
		return nil, Errorf("%w", err)
	}
	got, err := e.Client.Do(request)
	if err != nil {
		return nil, Errorf("%w", err)
	}
	defer func() {
		_ = got.Body.Close()
	}()
	buf, err := io.ReadAll(got.Body)
	if err != nil {
		return nil, Errorf("%w", err)
	}
	resp := new(EmbedResponse)
	err = json.Unmarshal(buf, resp)
	if err != nil {
		return nil, ErrorWithResponse{error: Errorf("%w", err), body: buf}
	}
	if len(resp.Embeddings) == 0 {
		return resp, ErrorWithResponse{error: fmt.Errorf("none"), body: buf}
	}
	return resp, nil
}

type ErrorWithResponse struct {
	error
	body []byte
}

func (e ErrorWithResponse) Body() []byte {
	return e.body
}

// Unwrap for [errors.Unwrap].
func (e ErrorWithResponse) Unwrap() error {
	return e.error
}

// EmbedRequest is the request passed to [E.Embed].
type EmbedRequest struct {
	Model     string   `json:"model"`
	Input     []string `json:"input"`
	KeepAlive int      `json:"keep_alive,omitempty"`
	// Dimensions truncates the output embedding to the specified dimension.
	Dimensions int   `json:"dimensions,omitempty"`
	Truncate   *bool `json:"truncate,omitempty"`
	// Options lists model-specific options.
	Options map[string]any `json:"options,omitempty"`

	// not sent

	Host string `json:"-"` // optional: override E.Host and LLM_EMBED_HOST env
}

// EmbedResponse is the response from [Client.Embed].
type EmbedResponse struct {
	Model      string                `json:"model"`
	Embeddings []vector256.Vector256 `json:"embeddings"` // get resp.Embeddings[0]
	Stats                            // limited for this endpoint
}

func (e *EmbedResponse) GetEmbeddings() vector256.Vector256 {
	if e == nil || len(e.Embeddings) == 0 {
		return nil
	}
	return e.Embeddings[0]
}

var DefaultEmbedOptions map[string]any // nil

// NewEmbed
func NewEmbed(s string) EmbedRequest {
	reqBody := EmbedRequest{
		Model:      "",
		Input:      []string{s},
		Options:    DefaultEmbedOptions,
		Truncate:   new(true),
		Dimensions: vector256.VarLen,
	}
	return reqBody
}

// NewNomicQuery adds `search_query: ` prefix to the given string
func NewNomicQuery(s string) EmbedRequest {
	return NewEmbed("search_query: " + s)
}

// NewNomicDocument adds `search_document: ` prefix to the given string
func NewNomicDocument(s string) EmbedRequest {
	return NewEmbed("search_document: " + s)
}
