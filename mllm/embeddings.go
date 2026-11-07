package mllm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"os"

	"github.com/aerth/mostly/vector256"
)

// EmbedOptions application-wide global options
//
// Apart from Path, all serve as fallback if not present in request/env.
var EmbedOptions = struct {
	URL        url.URL
	Model      string            // if req.Model is empty and env is empty
	Dimensions int               // must match [vector256.VarLen]
	EnvHost    string            // the ENV_VAR to check to override [E.Host] (for when using 2 different hosts for llm,embeddings.)
	Path       string            // custom endpoint url path (default /api/embed)
	Headers    map[string]string // if different than LLM_API_KEY
}{
	Model:   "nomic-embed-text:v1.5",
	EnvHost: "LLM_EMBED_HOST",
	Path:    "/v1/embeddings", // Path is to use same as [E.Host] but different path. NOT used if URL is set
}

var envembedhost = struct {
	Checked bool
	Value   url.URL
}{}

// Embed generates new embeddings
//
// See [NewNomicDocument] and [NewNomicQuery] for model-specific query prefixes
func (e E) Embed(ctx context.Context, req EmbedRequest) (*EmbedResponse, error) {
	if !envembedhost.Checked {
		if embeddingHost := os.Getenv(EmbedOptions.EnvHost); embeddingHost != "" {
			u, err := url.Parse(embeddingHost)
			if err != nil {
				return nil, err
			}
			EmbedOptions.URL = *u
		}
		envembedhost.Checked = true
	}

	if req.Model == "" {
		req.Model = EmbedOptions.Model
	}
	endpoint := e.Host
	if EmbedOptions.URL.Host != "" {
		endpoint = EmbedOptions.URL
	} else if EmbedOptions.Path != "" {
		endpoint.Path = EmbedOptions.Path
	}
	req.Host = endpoint

	b, err := json.Marshal(req)
	if err != nil {
		return nil, Errorf("%w", err)
	}
	body := bytes.NewReader(b)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, req.Host.String(), body)
	if err != nil {
		return nil, Errorf("%w", err)
	}
	for k, v := range e.Headers {
		request.Header.Set(k, v)
	}
	for k, v := range EmbedOptions.Headers {
		request.Header.Set(k, v)
	}
	client := e.Client
	if client == nil {
		client = http.DefaultClient
	}
	got, err := client.Do(request)
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
	if len(resp.Error) > 4 {

		code := 42
		msg := "error"
		if err := json.Unmarshal(resp.Error, &struct {
			Code    *int
			Message *string
		}{Code: &code, Message: &msg}); err != nil {
			slog.Error(fmt.Sprintf("could not decode response error: %s", string(resp.Error)))
		}
		return resp, ErrorWithResponse{error: fmt.Errorf("%d: %s", code, msg), body: buf}
	}
	if len(resp.Data) != 0 && len(resp.Embeddings) == 0 {
		resp.Embeddings = []vector256.Vector256{resp.Data[0].Embedding}
		resp.Data = nil
	}
	if len(resp.Embeddings) == 0 {
		slog.Error("no embeddings in response", "x", resp)
		return resp, ErrorWithResponse{error: fmt.Errorf("none"), body: buf}
	}
	em := resp.Embeddings[0]
	// quirk: llama.cpp no dimensions in request?
	if req.Dimensions != 0 && len(em) > req.Dimensions {
		em = em[:req.Dimensions] // will edit in place
		em = L2normalize(em)
		resp.Embeddings = []vector256.Vector256{em}
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
	Model     string `json:"model"`
	Input     string `json:"input"`
	KeepAlive int    `json:"keep_alive,omitempty"`
	// Dimensions truncates the output embedding to the specified dimension.
	Dimensions int   `json:"dimensions,omitempty"`
	Truncate   *bool `json:"truncate,omitempty"`
	// Options lists model-specific options.
	Options map[string]any `json:"options,omitempty"`

	// not sent

	Host url.URL `json:"-"` // optional: override E.Host and LLM_EMBED_HOST env
}

// EmbedResponse is the response from [Client.Embed].
type EmbedResponse struct {
	Model      string                `json:"model"`
	Embeddings []vector256.Vector256 `json:"embeddings"` // get resp.Embeddings[0]
	Data       []*struct {
		Embedding vector256.Vector256
	} `json:"data,omitempty"`
	Stats                   // limited for this endpoint
	Error   json.RawMessage `json:"error,omitempty"`
	Message json.RawMessage `json:"message,omitempty"`
}

// GetEmbeddings extracts the search embeddings vectors
func (e *EmbedResponse) GetEmbeddings() vector256.Vector256 {
	if e != nil && len(e.Data) != 0 {
		return e.Data[0].Embedding
	}
	if e == nil || len(e.Embeddings) == 0 {
		return nil
	}
	return e.Embeddings[0]
}

// DefaultEmbedOptions is added to the embed requests
//
// Untested
var DefaultEmbedOptions map[string]any // nil

// NewEmbed request
func NewEmbed(s string) EmbedRequest {
	reqBody := EmbedRequest{
		Model:      "",
		Input:      s,
		Options:    DefaultEmbedOptions,
		Truncate:   new(true),
		Dimensions: vector256.VarLen,
		KeepAlive:  -1,
	}
	return reqBody
}

// L2normalize for llama.cpp to truncate dimensions
func L2normalize[T float32 | float64](v []T) []T {
	var sumSquares float64
	for _, val := range v {
		sumSquares += float64(val) * float64(val)
	}
	norm := T(math.Sqrt(sumSquares))
	if norm == 0 {
		return v
	}
	for i := range v {
		v[i] = v[i] / norm
	}
	return v
}

// NewNomicQuery adds `search_query: ` prefix to the given string
func NewNomicQuery(s string) EmbedRequest {
	return NewEmbed("search_query: " + s)
}

// NewNomicDocument adds `search_document: ` prefix to the given string
func NewNomicDocument(s string) EmbedRequest {
	return NewEmbed("search_document: " + s)
}
