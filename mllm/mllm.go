// Package mllm is simple llm client, part of aerth/mostly project
package mllm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"slices"
	"time"
	"uuid"

	"github.com/aerth/mostly/stackerr"
)

type Client = E

// E (Endpoint) holds information for future requests
//
// Writer for streaming output with ThinkPre and ThinkPost.
type E struct {
	// minimally

	Host  url.URL // the LLM API (required)
	Model string  // the model name (required)

	// optional

	Client  *http.Client      `json:"-"` // custom http client (optional)
	Headers map[string]string `json:"-"` // custom headers (optional)
	Tools   []json.Marshaler  // Bring your own tools (optional)
	Options *Options          // Options (optional) added to each NewRequest

	Writer       io.Writer // used in [E.Get] as StreamWriter if no StreamWriter
	StreamWriter W1        `json:"-" gorm:"-"` // used as only handler **IF no handlers given**

	PreRequest func(request *ChatRequest) error `json:"-"`

	// Handlers **added** to all requests
	Handlers []StreamHandler `json:"-"`

	//
	From     string
	ThreadID uuid.UUID
}

func ToMarshalers(v ...json.Marshaler) []json.Marshaler {
	return v
}

func (t Tools) ToMarshalers() []json.Marshaler {
	return t.ToJSON()
}

func _() {
	println("OK")
	ToMarshalers(json.RawMessage(""))
}

type (
	W  interface{ New() StreamHandler }
	W1 struct {
		// Writer if non-nil will be automatically added as a stream handler to [[E.Get]] and [[E.Stream]]
		Writer    io.Writer `json:"-"`
		ThinkPre  string    // optional, default used in [NewEndpoint]
		ThinkPost string    // optional, default used in [NewEndpoint]
	}
)

var _ W = W1{}

func Noop(cr ChatResponse) error { return nil }
func (w W1) New() StreamHandler {
	if w.Writer == nil || w.Writer == io.Discard {
		return Noop
	}
	return NewHandlerOut(w.Writer, w.ThinkPre, w.ThinkPost)
}

type (
	StreamHandler func(cr ChatResponse) error
	ChatHandlerT  interface {
		HandleChat(cr ChatResponse) error
	}
)

func (c StreamHandler) HandleChat(cr ChatResponse) error {
	return c(cr)
}

var (
	DefaultThinkPre         = "Thinking:\n"
	DefaultThinkPost        = "\n... done thinking.\n"
	ThinkRequestType        = false
	StopAtFirstHandlerError = true
	ErrNoLines              = errors.New("no lines read")
	HideThink               = false
	HideTools               = false
	EnvModel                = []string{"MLLM_MODEL", "OLLAMA_MODEL", "LLM_MODEL"}
	EnvHost                 = []string{"MLLM_HOST", "OLLAMA_HOST", "LLM_HOST"}
	EnvKey                  = []string{"MLLM_API_KEY", "OLLAMA_API_KEY", "LLM_API_KEY"}
	DebugLLM2               = os.Getenv("DEBUG_LLM") == "2"
	DebugLLM                = os.Getenv("DEBUG_LLM") == "1" || DebugLLM2
	ErrNoConvo              = errors.New("no messages in conversation")
	Quiet                   = false // quiet the debug
)

func firstEnv(k []string) string {
	for i := range k {
		s := os.Getenv(k[i])
		if s != "" {
			return s
		}
	}
	return ""
}

var (
	DefaultOptions = Options{
		Temperature: new(1.0),
		Seed:        -1,
	}
	DefaultModel string
	DefaultHost  string
	ForcePath    = false
)

var ToolWriter = toolWriter

// NewEndpoint reads env and returns a new reusable Endpoint
//
// See [E.StreamBG] or [E.Stream] or [E.Get]
func NewEndpoint(options *Options, tools []json.Marshaler) E {
	if options == nil {
		options = new(DefaultOptions)
	}
	if DebugLLM2 && (options == nil) {
		options = &Options{Debug: 2}
	}
	model := firstEnv(EnvModel)
	host := firstEnv(EnvHost)
	var hostURL url.URL
	if host != "" {
		u, _ := url.Parse(host)
		if u != nil {
			hostURL = *u
			if !ForcePath && (hostURL.Path == "" || hostURL.Path == "/") {
				hostURL.Path = "/v1/chat/completions"
			}
		} else {
			Slog().Error("parsing url failed, check env: " + EnvModel[0]) // dont print err
		}
	}
	var hdr map[string]string
	k := firstEnv(EnvKey)
	if k != "" {
		hdr = map[string]string{
			"Authorization": "Bearer " + k,
		}
	}
	var w1 W1
	if DefaultWriter != nil {
		w1 = W1{Writer: DefaultWriter, ThinkPre: DefaultThinkPre, ThinkPost: DefaultThinkPost}
	}
	return E{
		Host: hostURL, Model: model, Client: http.DefaultClient, Options: options, Tools: tools,
		StreamWriter: w1,
		Headers:      hdr, Handlers: nil,
		ThreadID: DefaultThread,
		Writer:   DefaultWriter,
		From:     DefaultFrom,
	}
}

// DefaultWriter added to all new endpoints
var DefaultWriter io.Writer = os.Stderr

var DefaultFrom string

// Default creates a new reusable LLM endpoint from env using [NewEndpoint] with no tools no options.
var Default = func() E {
	e := NewEndpoint(nil, nil)
	return e
}

var (
	Errorf = stackerr.Errorf
	Slog   = slog.Default
)

// NewE custom NewEndpoint
var NewE = NewEndpoint

func (e E) WithoutTools() E {
	e.Tools = nil
	return e
}

func (e E) WithTools(tools ...json.Marshaler) E {
	if tools == nil || len(tools) == 0 && tools[0] == nil {
		e.Tools = nil
	} else {
		e.Tools = tools
	}
	return e
}

func (e E) AddHandlers(h ...StreamHandler) E {
	e.Handlers = append(slices.Clone(e.Handlers), h...)
	return e
}

func (e E) WithHandlers(h ...StreamHandler) E {
	e.Handlers = h
	return e
}

func (e E) WithOptions(options *Options) E {
	e.Options = options
	return e
}

// Think Value should be one of: nil,"low","medium","high",true,false
type Think struct {
	Value `json:"-"`
}
type Value = any

// MarshalJSON implements [json.Marshaler] for sending request
func (t Think) MarshalJSON() ([]byte, error) {
	tv := t.Value
	if tv == nil {
		return json.Marshal(nil)
	}
	if _, ok := tv.(string); ok {
	} else if v, ok := tv.(bool); ok {
		if !v {
			tv = "none"
		}
	} else {
		return nil, fmt.Errorf("unknown think type %T", t.Value)
	}
	if ThinkRequestType {
		return json.Marshal(map[string]any{"effort": tv})
	} else {
		return json.Marshal(tv)
	}
}

// ChatRequest sent to LLM (/v1/chat/completions or compatible)
type ChatRequest struct {
	Model     string           `json:"model,omitempty"`
	Messages  []Message        `json:"messages,omitempty"`
	Stream    bool             `json:"stream"`
	Think     *Think           `json:"reasoning_effort,omitempty"` // ollama endpoint "think" but /v1/chat/completions is "thinking"
	KeepAlive int              `json:"keep_alive,omitempty"`
	Tools     []json.Marshaler `json:"tools,omitempty"`
	*Options  `gorm:"type:jsonb;serializer:json;default:null"`
}

// Options for ChatRequest
type Options struct {
	Seed             int64    `json:"seed,omitempty"`
	Temperature      *float64 `json:"temperature,omitempty"` // 0.0 to 2.0
	NumCtx           int      `json:"num_ctx,omitempty"`
	TopP             *float64 `json:"top_p,omitempty"`             // 0.0 to 1.0
	TopK             *int     `json:"top_k,omitempty"`             // 1 to ???
	FrequencyPenalty *float64 `json:"frequency_penalty,omitempty"` // -2.0 to 2.0
	PresencePenalty  *float64 `json:"presence_penalty,omitempty"`  // -2.0 to 2.0
	//
	Debug int `json:"-"` // internal, for printing full chatrequest before sending
}

// ChatResponse streamable has [ChatResponse.Handler]
//
// See [LMessage]
//
// Example:
//
//	var cr mllm.ChatResponse
//	err := <-mllm.Default().StreamBG(ctx, mllm.Messages{{Role: "user", Content: "hi"}}, false, cr.Handler)
//	println("Bot:", cr.Message.Content)
type ChatResponse struct {
	LLMID string `json:"id,omitempty" gorm:"-"`
	//	Object string
	Model       string          `json:"model"`
	Created     json.RawMessage `json:"created,omitempty"`
	Done        bool            `json:"done"` // for stream
	DoneReason  string          `json:"done_reason,omitempty"`
	DoneReason2 string          `json:"finish_reason,omitempty" gorm:"-"` // INTERNAL do not use
	Message     Message         `json:"message"`                          // for stream, handlers
	Stats       `gorm:"embedded"`
	Choices     []Choice `json:"choices,omitempty"` // INTERNAL do not use
}

type Choice struct {
	Index        int     `json:"index"`
	FinishReason *string `json:"finish_reason,omitempty"`
	Delta        Message `json:"delta"`
}

// Stats has [Stats.String]
type Stats struct {
	LoadDuration       time.Duration `json:"load_duration,omitempty"`
	PromptEvalDuration time.Duration `json:"prompt_eval_duration,omitempty"`
	EvalDuration       time.Duration `json:"eval_duration,omitempty"`
	TotalDuration      time.Duration `json:"total_duration,omitempty"`
	PromptEvalCount    int           `json:"prompt_eval_count,omitempty"`
	EvalCount          int           `json:"eval_count,omitempty"`
	Usage
}
type Usage struct {
	TotalTokens  int `json:"total_tokens,omitempty"`
	PromptTokens int `json:"prompt_tokens,omitempty"`
}

// Message from [LMessage]
type Message struct {
	Role       string            `json:"role,omitempty"`
	Content    string            `json:"content"`
	Reasoning  string            `json:"thinking,omitempty" gorm:"default:null"`
	ToolCalls  []json.RawMessage `json:"tool_calls,omitempty" gorm:"type:jsonb;serializer:json;default:null;"`
	ToolName   string            `json:"tool_name,omitempty" gorm:"default:null"`
	ToolCallID string            `json:"tool_call_id,omitempty" gorm:"default:null"`
	Images     [][]byte          `json:"images,omitempty" gorm:"serializer:json;type:jsonb;default:null;"`
	message2   `gorm:"-"`
}
type message2 struct {
	// DO NOT USE
	// ToolCalls2  *Call[json.RawMessage] `json:"tool_call,omitempty" gorm:"-"`         // Internal, DO NOT USE
	ToolCallID2 string `json:"id,omitempty" gorm:"-"`                // Internal, DO NOT USE
	Reasoning2  string `json:"reasoning,omitempty" gorm:"-"`         // Internal, DO NOT USE
	Reasoning3  string `json:"reasoning_content,omitempty" gorm:"-"` // Internal, DO NOT USE
}
type Messages []Message

func (m Messages) Top() Message {
	l := len(m)
	if l == 0 {
		return Message{}
	}
	return m[l-1]
}

func (m Messages) TopN(n int) Messages {
	l := len(m)
	if l == 0 {
		return nil
	}
	if l <= n {
		return m
	}
	return m[l-n:]
}

func (m Messages) TopRole(role string) Message {
	l := len(m)
	if l == 0 {
		return Message{}
	}
	for i := range m {
		if m[l-i].Role == role {
			return m[i]
		}
	}
	return Message{}
}

// Add fields of c2 to c for gathering streamed chunks
//
// Used in [ChatResponse.Handler]
func (m Message) Add(m2 Message) Message {
	if m2.Role != "" {
		m.Role = m2.Role
	}
	m.Content += m2.Content
	m.Reasoning += m2.Reasoning
	if m2.Reasoning2 != "" {
		panic("oops")
	}
	if len(m2.ToolCalls) != 0 {
		m.ToolCalls = append(m.ToolCalls, m2.ToolCalls...)
	}
	if m2.ToolName != "" {
		m.ToolName = m2.ToolName
	}
	if m2.ToolCallID != "" {
		m.ToolCallID = m2.ToolCallID
	}
	if len(m2.Images) != 0 {
		m.Images = append(m.Images, m2.Images...)
	}
	return m
}

// IsEmpty DOES NOT check m.Aux -- see also: [Message.IsBlank]
func (m Message) IsEmpty() bool {
	return m.Content == "" && m.Reasoning == "" && m.Role == "" && len(m.ToolCalls) == 0
}

// IsBlank DOES NOT check m.Aux or m.Role -- see also: [Message.IsEmpty]
func (m Message) IsBlank() bool {
	return m.Content == "" && m.Reasoning == "" && len(m.ToolCalls) == 0
}

func (m Message) Equal(m2 Message) bool {
	return m.Content == m2.Content && m.Reasoning == m2.Reasoning && m.Role == m2.Role &&
		m.ToolName == m2.ToolName && m.ToolCallID == m2.ToolCallID &&
		//	slices.EqualFunc(m.ToolCalls, m2.ToolCalls, bytesEqual) &&
		slices.EqualFunc(m.Images, m2.Images, bytesEqual)
}

func bytesEqual[T ~[]byte](a, b T) bool {
	return bytes.Equal(a, b)
}

// StderrHandler plain 🍔
func StderrHandler(cr ChatResponse) error {
	return NewHandlerOut(os.Stderr, DefaultThinkPre, DefaultThinkPost).HandleChat(cr)
}

// StdoutHandler plain 🍔
func StdoutHandler(cr ChatResponse) error {
	return NewHandlerOut(os.Stdout, DefaultThinkPre, DefaultThinkPost).HandleChat(cr)
}

// NewHandlerOut plain 🍔
func NewHandlerOut(w io.Writer, thinkpre string, thinkpost string) StreamHandler {
	if w == nil {
		return func(cr ChatResponse) error { return nil }
	}
	reasoning := false
	hidethink := HideThink
	hidetools := HideTools
	wrote := false
	var calls []json.RawMessage
	return func(cr ChatResponse) error {
		msg := cr.Message
		content := msg.Content
		think := msg.Reasoning
		done := cr.Done

		if think != "" && !hidethink {
			if !reasoning {
				reasoning = true
				_, _ = fmt.Fprint(w, thinkpre)
				fflush(w)
			}
			_, _ = fmt.Fprint(w, think)
			fflush(w)
			wrote = true
		}
		if content != "" {
			if reasoning {
				reasoning = false
				_, _ = fmt.Fprint(w, thinkpost)
				fflush(w)
			}
			_, _ = fmt.Fprint(w, content)
			fflush(w)
			wrote = true
		}
		if !hidetools && len(cr.Message.ToolCalls) != 0 {
			calls = append(calls, cr.Message.ToolCalls...)
		}
		if done {
			if reasoning { // rare but, could be 'cancelled' or other done with no content TODO: check done reason
				_, _ = fmt.Fprint(w, thinkpost)
			}
			if len(calls) != 0 && ToolWriter != nil {
				for i := range calls {
					_ = ToolWriter(w, calls[i])
				}
			} else if wrote { // toolwriter should print new line
				_, _ = fmt.Fprint(w, "\n")
			}
			fflush(w)

			// be kind rewind
			calls = nil
			wrote = false
			reasoning = false
			hidethink = HideThink
			hidetools = HideTools
		}

		return nil
	}
}

func toolWriter(w io.Writer, rawMessage json.RawMessage) (err error) {
	return json.NewEncoder(w).Encode(map[string]any{"mllm_tool_call": rawMessage})
}

func fflush(w io.Writer) {
	if flusher, ok := w.(interface{ Flush() error }); ok {
		if err := flusher.Flush(); err != nil {
			panic(err.Error())
		}
	}
}

// Handler for [E.StreamBG] and [E.Stream]
//
// # SWAPS
//
// Is a [StreamHandler] to gather [Message]
func (c *ChatResponse) Handler(in ChatResponse) error {
	c.Done = in.Done
	c.DoneReason = in.DoneReason
	if in.Stats != (Stats{}) {
		c.Stats = in.Stats
	}
	msg := &c.Message
	*msg = msg.Add(in.Message)
	return nil
}

// NewRequest creates a [ChatRequest] adds e.Model, e.Tools, and e.Options
func (e E) NewRequest(messages []Message, think any) *ChatRequest {
	reqBody := ChatRequest{
		Model:     e.Model,
		Messages:  messages,
		Stream:    true,
		Think:     &Think{Value: think},
		KeepAlive: -1,
		Options:   e.Options,
		Tools: slices.DeleteFunc(slices.Clone(e.Tools), func(v json.Marshaler) bool {
			return v == nil
		}),
	}
	return &reqBody
}

// Do sends a [ChatRequest] and streams response in [E.Read]
func (e E) Do(ctx context.Context, creq ChatRequest, handlers []StreamHandler) error {
	if len(handlers) == 0 && e.StreamWriter.Writer != nil {
		handlers = []StreamHandler{e.StreamWriter.New()}
	}
	if len(handlers) == 0 && e.Writer != nil {
		handlers = []StreamHandler{W1{Writer: e.Writer, ThinkPre: DefaultThinkPre, ThinkPost: DefaultThinkPost}.New()}
	}
	if len(handlers) == 0 {
		return Errorf("no stream handlers were given")
	}
	bodyBytes, err := json.Marshal(creq)
	if err != nil {
		return Errorf("failed to marshal request: %w", err)
	}
	if creq.Options != nil && creq.Debug > 1 {
		Slog().Info("sending body js: " + string(bodyBytes))
	}

	req, err := http.NewRequestWithContext(ctx, "POST", e.Host.String(), bytes.NewBuffer(bodyBytes))
	if err != nil {
		return Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	for k, v := range e.Headers {
		req.Header.Set(k, v)
	}

	// send it
	client := e.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return Errorf("unexpected status code: %d %s", resp.StatusCode, string(b))
	}

	var copybuf bytes.Buffer
	reader := bufio.NewReader(io.TeeReader(resp.Body, &copybuf))

	err = e.Read(ctx, reader, handlers...)
	if err != nil {
		if !Quiet {
			if e.Options == nil || e.Options.Debug < 2 {
				Slog().Info("sent request body", "js", json.RawMessage(bodyBytes))
			}
			Slog().Info("full response body", "js", copybuf.String())
		}
		slog.Error("errorreading", "err", err)
		return Errorf("%w", err)
	}
	return nil
}

type (
	Calls[T any] []Call[T]
	StringCalls  []Call[string] // "arguments": "{"
)

// Combined fragments of tool calls into final calls
// (one per call index, which usually 1) instead of bunch of fragments
//
// Used in [E.Read] to finish a stream
//
// Should remove [Call] from this TODO:
func (c StringCalls) Combined() Calls[string] {
	var a StringCalls
	for i := range c {
		if len(a) == 0 || c[i].Index != a[len(a)-1].Index {
			a = append(a, c[i])
		} else {
			a[len(a)-1].Function.Arguments += c[i].Function.Arguments
		}
	}
	return Calls[string](a)
}

// ToJSON used in [E.Read] to recombine to unparsed RawMessages
func (c Calls[T]) ToJSON() []json.RawMessage {
	if len(c) == 0 {
		return nil
	}
	a := make([]json.RawMessage, len(c))
	for i := range c {
		a[i] = c[i].ToJSON()
	}
	return a
}

type minireader struct {
	nlines      int
	errors      []error
	modeP1      bool
	callsparsed StringCalls
}

func (m *minireader) ReadStreamFrame(line []byte, handlers []StreamHandler) error {
	m.nlines++
	trimmedLine := bytes.TrimSpace(line)
	if len(trimmedLine) == 0 {
		return nil
	}
	if post, ok := bytes.CutPrefix(trimmedLine, []byte("data: ")); ok {
		trimmedLine = bytes.TrimSpace(post)
		m.modeP1 = true
	}

	if len(trimmedLine) == 0 {
		return nil
	}
	var chunk ChatResponse
	if post, ok := bytes.CutPrefix(trimmedLine, []byte("[DONE]")); ok {
		trimmedLine = bytes.TrimSpace(post)
	}
	if len(trimmedLine) == 0 {
		return nil
	}
	//	println("GOT LINE", len(trimmedLine), string(trimmedLine))
	if err := json.Unmarshal(trimmedLine, &chunk); err != nil { // chunk
		Slog().Error("decode cr", "err", err, "chunk", string(trimmedLine))
		return Errorf("Read: %w", err)
	}
	if chunk.DoneReason2 != "" {
		chunk.Done = true
		chunk.DoneReason = chunk.DoneReason2
		chunk.DoneReason2 = ""
	}
	if chunk.Message.Reasoning2 != "" {
		chunk.Message.Reasoning = chunk.Message.Reasoning2
		chunk.Message.Reasoning2 = ""
	}
	if chunk.Message.Reasoning3 != "" {
		chunk.Message.Reasoning = chunk.Message.Reasoning3
		chunk.Message.Reasoning3 = ""
	}
	if x := chunk.Message.ToolCallID2; x != "" {
		chunk.Message.ToolCallID = x
		chunk.Message.ToolCallID2 = ""
	}
	for i := range chunk.Choices {
		choice := &chunk.Choices[i]
		if l := len(choice.Delta.ToolCalls); l != 0 {
			if callsfound, err := chunkCollectCall(choice); err != nil {
				return Errorf("Read.chunkCollectCall: %w", err)
			} else if len(callsfound) != 0 {
				m.callsparsed = append(m.callsparsed, callsfound...)
				chunk.Message.ToolCalls = nil // yank from chunk/choice so StreamHandlers dont read partial tools
				choice.Delta.ToolCalls = nil
			}
		}
		if chunk.DoneReason2 != "" {
			chunk.Done = true
			chunk.DoneReason = chunk.DoneReason2
			chunk.DoneReason2 = ""
		}
		if choice.Delta.Reasoning3 != "" {
			choice.Delta.Reasoning = choice.Delta.Reasoning3
		} else if choice.Delta.Reasoning2 != "" {
			choice.Delta.Reasoning = choice.Delta.Reasoning2
		}

		choice.Delta.Reasoning2 = ""
		if ptr := choice.FinishReason; ptr != nil {
			chunk.Done = true
			chunk.DoneReason = *ptr
		}
		if x := chunk.Message.ToolCallID2; x != "" {
			panic("oops")
		}
		chunk.Message = chunk.Message.Add(choice.Delta)
	}
	chunk.Choices = nil
	// keep emptiness if empty
	if !chunk.Done && len(m.callsparsed) == 0 && chunk.Message.IsEmpty() {
		Slog().Warn("empty message streamed, possible decode issue", "raw", string(trimmedLine))
	} else if chunk.Message.Role == "" {
		chunk.Message.Role = "assistant"
	}
	if chunk.Done {
		// send tool calls in the last frame
		chunk.Message.ToolCalls = m.callsparsed.Combined().ToJSON()
	}
	for i, handler := range handlers {
		if err := handler(chunk); err != nil {
			if StopAtFirstHandlerError {
				return Errorf("%w", err)
			}
			m.errors = append(m.errors, Errorf("handler %d/%d: %w", i+1, len(handlers), err))
		}
	}
	return nil
}

// Read a stream, collecting for StreamHandlers, stopping immediately at first error.
//
// Nothing is printed, only error is returned. At least one StreamHandler must be given.
//
// NOTE: To modify Read so that, after an error, **all** stream handlers are
// given each frame, caller should set [StopAtFirstHandlerError] to false,
func (e E) Read(ctx context.Context, reader *bufio.Reader, handlers ...StreamHandler) error {
	if len(handlers) == 0 {
		return Errorf("Read: no handlers")
	}
	var mini minireader
	for {
		if ctx.Err() != nil {
			return context.Cause(ctx)
		}
		line, err := reader.ReadBytes('\n')
		//println("LINE:", string(line))
		isEOF := errors.Is(err, io.EOF)
		if isEOF && mini.nlines == 0 {
			err = nil
		}
		if isEOF && len(line) == 0 {
			break
		}
		if err != nil {
			return fmt.Errorf("Read: error reading stream: %w", err)
		}
		if err := mini.ReadStreamFrame(line, handlers); err != nil {
			return err
		}
	} // stream loop
	errs := mini.errors
	lines := mini.nlines
	if errs != nil {
		return Errorf("%w", errs)
	}
	if lines == 0 {
		return Errorf("%w", ErrNoLines)
	}
	return nil
}

func chunkCollectCall(choice *Choice) ([]Call[string], error) {
	var a []Call[string]
	for i := range choice.Delta.ToolCalls {
		var x Call[string]
		if err := json.Unmarshal(choice.Delta.ToolCalls[i], &x); err != nil {
			return nil, err
		}
		a = append(a, x)
	}
	return a, nil
}

func (e *E) ValidateConfig(think any) error {
	var errs error
	if e.Model == "" {
		e.Model = DefaultModel
	}
	if e.Model == "" {
		errs = errors.Join(errs, fmt.Errorf("missing LLM_MODEL"))
	}
	if (e.Host.Scheme == "" || e.Host.Host == "") && DefaultHost != "" {
		u, err := url.Parse(DefaultHost)
		if err != nil {
			return err
		}
		e.Host = *u
	}
	if e.Host.Scheme == "" || e.Host.Host == "" {
		errs = errors.Join(errs, fmt.Errorf("missing LLM_HOST"))
	}
	_, thinkB := think.(bool)
	_, thinkS := think.(string)
	if !thinkB && !thinkS && think != nil {
		errs = errors.Join(errs, Errorf("weird think type: %T", think))
	}
	if errs != nil {
		return Errorf("%w", errs)
	}
	return errs
}

// Summary in well-known format
func (st Stats) Summary(w io.Writer) error {
	return Summary(w, st)
}

// SummaryString collects [ChatResponse.Stats] into a string
func (c ChatResponse) SummaryString() string {
	return c.Stats.SummaryString()
}

// SummaryString collects [ChatResponse.Stats] into a string
func (st Stats) SummaryString() string {
	var buf bytes.Buffer
	_ = Summary(&buf, st)
	return buf.String()
}

// Summary in well-known format
func Summary(w io.Writer, st Stats) (err error) {
	if st.TotalDuration != 0 {
		_, err = fmt.Fprintf(w, "total duration:       %v\n", st.TotalDuration)
		if err != nil {
			return Errorf("%w", err)
		}
	}
	if st.LoadDuration != 0 {
		_, err = fmt.Fprintf(w, "load duration:        %v\n", st.LoadDuration)
		if err != nil {
			return Errorf("%w", err)
		}
	}
	if st.PromptEvalCount != 0 {
		_, err = fmt.Fprintf(w, "prompt eval count:    %d token(s)\n", st.PromptEvalCount)
		if err != nil {
			return Errorf("%w", err)
		}
	}
	if st.PromptEvalDuration != 0 {
		_, err = fmt.Fprintf(w, "prompt eval duration: %s\n", st.PromptEvalDuration)
		if err != nil {
			return Errorf("%w", err)
		}
		_, err = fmt.Fprintf(w, "prompt eval rate:     %.2f tokens/s\n", float64(st.PromptEvalCount)/st.PromptEvalDuration.Seconds())
		if err != nil {
			return Errorf("%w", err)
		}
	}
	if st.EvalCount != 0 {
		_, err = fmt.Fprintf(w, "eval count:           %d token(s)\n", st.EvalCount)
		if err != nil {
			return Errorf("%w", err)
		}
	}
	if st.EvalDuration != 0 {
		_, err = fmt.Fprintf(w, "eval duration:        %s\n", st.EvalDuration)
		if err != nil {
			return Errorf("%w", err)
		}
		_, err = fmt.Fprintf(w, "eval rate:            %.2f tokens/s\n", float64(st.EvalCount)/st.EvalDuration.Seconds())
		if err != nil {
			return Errorf("%w", err)
		}
	}
	return nil
}
