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

// E (Endpoint) holds information for future requests
//
// Writer for streaming output with ThinkPre and ThinkPost.
type E struct {
	Host  url.URL // the LLM API (required)
	Model string  // the model name (required)

	// optional

	Client  *http.Client      // custom http client (optional)
	Headers map[string]string // custom headers (optional)
	Tools   []json.Marshaler  // Bring your own tools (optional)
	Options *Options          // e options (optional)

	// Writer if non-nil will be automatically added as a stream handler to [[E.Get]] and [[E.Stream]]
	Writer    io.Writer
	ThinkPre  string // optional, default used in [NewEndpoint]
	ThinkPost string // optional, default used in [NewEndpoint]

	// Handlers added to all requests
	Handlers []StreamHandler
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
	DefaultThinkPre  = "Thinking:\n"
	DefaultThinkPost = "\n\n... done thinking.\n\n"
)

var (
	EnvModel = []string{"MLLM_MODEL", "OLLAMA_MODEL", "LLM_MODEL"}
	EnvHost  = []string{"MLLM_HOST", "OLLAMA_HOST", "LLM_HOST"}
)

var (
	StopAtFirstHandlerError = true
	ErrNoLines              = errors.New("no lines read")
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

// NewEndpoint reads env and returns a new reusable Endpoint
//
// See [E.StreamBG] or [E.Stream] or [E.Get]
func NewEndpoint(options *Options, tools []json.Marshaler) E {
	model := firstEnv(EnvModel)
	host := firstEnv(EnvHost)
	var hostURL url.URL
	if host != "" {
		u, _ := url.Parse(host)
		if u != nil {
			hostURL = *u
		} else {
			Slog().Error("parsing url failed, check env: " + EnvModel[0]) // dont print err
		}
	}
	return E{Host: hostURL, Model: model, Client: http.DefaultClient, Options: options, Tools: tools, Writer: DefaultWriter, ThinkPre: DefaultThinkPre, ThinkPost: DefaultThinkPost, Headers: nil, Handlers: nil}
}

// DefaultWriter added to all new endpoints
var DefaultWriter io.Writer = nil

// Default creates a new reusable LLM endpoint from env using [NewEndpoint] with no tools no options.
var Default = func() E {
	e := NewE(nil, nil)
	return e
}

var (
	Errorf = stackerr.Errorf
	Slog   = slog.Default
)

// NewE custom NewEndpoint
var NewE = NewEndpoint

func (ee E) WithoutTools() E {
	ee.Tools = nil
	return ee
}

func (ee E) WithTools(tools ...json.Marshaler) E {
	ee.Tools = tools
	return ee
}

func (ee E) AddHandlers(h ...StreamHandler) E {
	ee.Handlers = append(slices.Clone(ee.Handlers), h...)
	return ee
}

func (ee E) WithHandlers(h ...StreamHandler) E {
	ee.Handlers = h
	return ee
}

func (ee E) WithOptions(options *Options) E {
	ee.Options = options
	return ee
}

// Think Value should be one of: nil,"low","medium","high",true,false
type Think struct {
	Value any
}

// MarshalJSON implements [json.Marshaler] for sending request
func (t Think) MarshalJSON() ([]byte, error) {
	tv := t.Value
	if tv == nil {
		return json.Marshal(nil)
	}
	if v, ok := tv.(string); ok {
		return json.Marshal(v)
	}
	if v, ok := tv.(bool); ok {
		return json.Marshal(v)
	}
	return nil, fmt.Errorf("unknown think type %T", t.Value)
}

// ChatRequest sent to LLM
type ChatRequest struct {
	Model     string           `json:"model,omitempty"`
	Messages  []Message        `json:"messages,omitempty"`
	Stream    bool             `json:"stream"`
	Think     *Think           `json:"think,omitempty"`
	KeepAlive int              `json:"keep_alive,omitempty"`
	Tools     []json.Marshaler `json:"tools,omitempty"`
	Options   *Options         `json:"options,omitempty"`
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
	Debug bool `json:"-"` // internal, for printing full chatrequest before sending
}

// ChatResponse streamable has [ChatResponse.Handler]
//
// Example:
//
//	var cr mllm.ChatResponse
//	err := <-mllm.Default().StreamBG(ctx, mllm.Messages{{Role: "user", Content: "hi"}}, false, cr.Handler)
//	println("Bot:", cr.Message.Content)
type ChatResponse struct {
	Model      string          `json:"model"`
	Created    json.RawMessage `json:"created_at,omitempty"`
	Done       bool            `json:"done"` // for stream
	DoneReason string          `json:"done_reason,omitempty"`
	Message    Message         `json:"message"` // for stream, handlers
	Stats      `gorm:"embedded"`
}

// Stats has [Stats.String]
type Stats struct {
	LoadDuration       time.Duration `json:"load_duration,omitempty"`
	PromptEvalDuration time.Duration `json:"prompt_eval_duration,omitempty"`
	EvalDuration       time.Duration `json:"eval_duration,omitempty"`
	TotalDuration      time.Duration `json:"total_duration,omitempty"`
	PromptEvalCount    int           `json:"prompt_eval_count,omitempty"`
	EvalCount          int           `json:"eval_count,omitempty"`
}

type Message struct {
	Role       string            `json:"role,omitempty"`
	Content    string            `json:"content,omitempty"`
	Reasoning  string            `json:"thinking,omitempty" gorm:"default:default:null"`
	ToolCalls  []json.RawMessage `json:"tool_calls,omitempty" gorm:"type:jsonb;serializer:json;default:null;"`
	ToolName   string            `json:"tool_name,omitempty" gorm:"default:null"`
	ToolCallID string            `json:"tool_call_id,omitempty" gorm:"default:null"`
	Images     [][]byte          `json:"images,omitempty" gorm:"serializer:json;type:jsonb;default:null;"`
}
type Messages []Message

// ChatMessage is a [Message] but with additional metadata (for a multi-participant conversation)
type ChatMessage struct {
	UUID uuid.UUID `json:"uuid,omitempty" gorm:"uniqueIndex"`
	Message
	From      string    `json:"from,omitempty" gorm:"default:null"`
	CreatedAt time.Time `json:"created_at"`
}

// ChatMessages like [Messages] but with additional metadata (for a multi-participant conversation)
type ChatMessages []ChatMessage

func (cm ChatMessages) ToMessages() []Message {
	a := make([]Message, len(cm))
	for i := range cm {
		a[i] = cm[i].Message
	}
	return a
}

// ChatMessageFormatter may skip a message by returning false
//
// The returned formatted messages are NEVER stored only sent
type ChatMessageFormatter = func(cm ChatMessage) ([]Message, bool)

// ToMessagesFormat returns a **temporary** convo for llm to produce a single new [Message]
// The returned convo should NEVER be stored (only sent to llm) as the Content is modified by formatter.
func (cm ChatMessages) ToMessagesFormat(fn ChatMessageFormatter) []Message {
	if fn == nil {
		fn = DefaultChatMessageFormatter
	}
	var a []Message
	for i := range cm {
		m, ok := fn(cm[i])
		if ok {
			a = append(a, m...)
		}
	}
	return a
}

// DefaultChatMessageFormatter pass through [ChatMessageFormatter]
// In a custom implementation, this might add text to Content, replace Role, or insert messages before/after.
var DefaultChatMessageFormatter = func(cm ChatMessage) ([]Message, bool) {
	return []Message{cm.Message}, true
}
var _ ChatMessageFormatter = DefaultChatMessageFormatter

// NewChatMessage from.
func NewChatMessage(from string, role string, text string) ChatMessage {
	m := Message{Role: role, Content: text}
	return m.ToChatMessage(from)
}

// ToChatMessage with new UUID,from,timestamp. should be called immediately after stream is finished..
//
// [ChatMessage] MAY be stored (NOT plain [Message])
func (m Message) ToChatMessage(from string) ChatMessage {
	m2 := ChatMessage{
		UUID:      uuid.New(),
		Message:   m,
		From:      from,
		CreatedAt: time.Now().Truncate(time.Millisecond),
	}
	return m2
}

// Add fields of c2 to c for gathering streamed chunks
//
// Used in [ChatResponse.Handler]
func (c Message) Add(c2 Message) Message {
	if c2.Role != "" {
		c.Role = c2.Role
	}
	c.Content += c2.Content
	c.Reasoning += c2.Reasoning
	if len(c2.ToolCalls) != 0 {
		c.ToolCalls = append(c.ToolCalls, c2.ToolCalls...)
	}
	if c2.ToolName != "" {
		c.ToolName = c2.ToolName
	}
	if c2.ToolCallID != "" {
		c.ToolCallID = c2.ToolCallID
	}
	if len(c2.Images) != 0 {
		c.Images = append(c.Images, c2.Images...)
	}
	return c
}

func (c Message) IsEmpty() bool {
	return c.Content == "" && c.Reasoning == "" && c.Role == "" && len(c.ToolCalls) == 0
}

func (c Message) Equal(c2 Message) bool {
	return c.Content == c2.Content && c.Reasoning == c2.Reasoning && c.Role == c2.Role &&
		c.ToolName == c2.ToolName && c.ToolCallID == c2.ToolCallID &&
		slices.EqualFunc(c.ToolCalls, c2.ToolCalls, bytesEqual) &&
		slices.EqualFunc(c.Images, c2.Images, bytesEqual)
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

var HideThink = false

// NewHandlerOut plain 🍔
func NewHandlerOut(w io.Writer, thinkpre string, thinkpost string) StreamHandler {
	if w == nil {
		return func(cr ChatResponse) error { return nil }
	}
	reasoning := false
	hidethink := HideThink
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
		}
		if content != "" {
			if reasoning {
				reasoning = false
				_, _ = fmt.Fprint(w, thinkpost)
				fflush(w)
			}
			_, _ = fmt.Fprint(w, content)
			fflush(w)
		}
		if done {
			if reasoning { // rare but, could be 'cancelled' or other done with no content TODO: check done reason
				_, _ = fmt.Fprint(w, thinkpost)
			}
			_, _ = fmt.Fprint(w, "\n")
			fflush(w)
		}

		return nil
	}
}

func fflush(w io.Writer) {
	if flusher, ok := w.(interface{ Flush() error }); ok {
		if err := flusher.Flush(); err != nil {
			panic(err.Error())
		}
	}
}

// Handler for [E.StreamBG] and [E.Stream]
func (c *ChatResponse) Handler(in ChatResponse) error {
	c.Done = in.Done
	c.DoneReason = in.DoneReason
	if in.Stats != (Stats{}) {
		c.Stats = in.Stats
	}
	c.Message = c.Message.Add(in.Message)
	return nil
}

// StreamBG uses Stream, closing ch after streaming is finished, sending non-nil error to the channel
func (ee E) StreamBG(ctx context.Context, messages []Message, think any, handlers ...StreamHandler) chan error {
	ch := make(chan error)
	go func() {
		err := ee.Stream(ctx, messages, think, handlers...)
		ch <- err // may be nil
		close(ch)
	}()
	return ch
}

func (ee E) NewRequest(messages []Message, think any) *ChatRequest {
	reqBody := ChatRequest{
		Model:     ee.Model,
		Messages:  messages,
		Stream:    true,
		Think:     &Think{Value: think},
		KeepAlive: -1,
		Options:   ee.Options,
		Tools:     ee.Tools,
	}
	return &reqBody
}

// Stream with handlers(only) or to ee.Writer
func (ee E) Stream(ctx context.Context, messages []Message, think any, handlers ...StreamHandler) error {
	if e := ee.ValidateConfig(think); e != nil {
		return e
	}
	if len(handlers) == 0 && ee.Writer != nil {
		handlers = []StreamHandler{NewHandlerOut(ee.Writer, ee.ThinkPre, ee.ThinkPost)}
	}
	if len(handlers) == 0 && len(messages) != 0 { // allow no handlers for a ping
		return Errorf("no stream handlers were given")
	}
	// t1 := time.Now()

	creq := *ee.NewRequest(messages, think)
	return ee.Do(ctx, creq, handlers)
}

func (ee E) Do(ctx context.Context, creq ChatRequest, handlers []StreamHandler) error {
	bodyBytes, err := json.Marshal(creq)
	if err != nil {
		return Errorf("failed to marshal request: %w", err)
	}
	if ee.Options != nil && ee.Options.Debug {
		slog.Debug("sending body", "js", json.RawMessage(bodyBytes))
	}

	req, err := http.NewRequestWithContext(ctx, "POST", ee.Host.String()+"/api/chat", bytes.NewBuffer(bodyBytes))
	if err != nil {
		return Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	for k, v := range ee.Headers {
		req.Header.Set(k, v)
	}

	// send it
	resp, err := ee.Client.Do(req)
	if err != nil {
		return Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return Errorf("unexpected status code: %d", resp.StatusCode)
	}

	reader := bufio.NewReader(resp.Body)

	err = ee.Read(reader, handlers...)
	if err != nil {
		return Errorf("%w", err)
	}
	return nil
}

func (ee E) Read(reader *bufio.Reader, handlers ...StreamHandler) error {
	lines := 0
	var errs error
	// stream
	for {

		line, err := reader.ReadBytes('\n')
		//println("LINE:", string(line))
		isEOF := errors.Is(err, io.EOF)
		if isEOF {
			err = nil
		}
		if isEOF && len(line) == 0 {
			break
		}

		lines++
		//slog.Info("got line", "line", json.RawMessage(line), "err", err)
		if err != nil {
			return fmt.Errorf("error reading stream: %w", err)
		}
		trimmedLine := bytes.TrimSpace(line)
		if len(trimmedLine) == 0 {
			continue
		}

		var chunk ChatResponse
		if err := json.Unmarshal(trimmedLine, &chunk); err != nil {
			slog.Error("decode cr", "err", err)
			errs = errors.Join(errs, Errorf("%w", err))
			continue
		}
		for i, handler := range handlers {
			if err := handler(chunk); err != nil {
				errs = errors.Join(errs, Errorf("handler %d/%d: %w", i+1, len(handlers), err))
				if StopAtFirstHandlerError {
					return errs
				}
			}
		}
	}
	if lines == 0 {
		return Errorf("%w", ErrNoLines)
	}
	return errs
}

func (ee E) ValidateConfig(think any) error {
	var errs error
	if ee.Model == "" {
		errs = errors.Join(errs, fmt.Errorf("missing MODEL"))
	}
	if ee.Host.Scheme == "" || ee.Host.Host == "" {
		errs = errors.Join(errs, fmt.Errorf("missing HOST"))
	}
	_, thinkB := think.(bool)
	_, thinkS := think.(string)
	if !thinkB && !thinkS && think != nil {
		errs = errors.Join(errs, Errorf("weird think type: %T", think))
	}
	return errs
}

// Get calls [E.Stream] and waits until finished, returning the full ChatResponse
//
// Uses [E.Stream], writes stream text to [[E.Writer]] if non-nil, returns full [ChatResponse]
func (ee E) Get(ctx context.Context, messages []Message, think any) (ChatResponse, error) {
	if e := ee.ValidateConfig(think); e != nil {
		return ChatResponse{}, Errorf("%w", e)
	}
	//slog := Slog()
	var chatResp ChatResponse
	var handlers []StreamHandler
	if ee.Writer != nil {
		handlers = []StreamHandler{NewHandlerOut(ee.Writer, ee.ThinkPre, ee.ThinkPost), chatResp.Handler}
	} else {
		handlers = []StreamHandler{chatResp.Handler}
	}
	if len(ee.Handlers) != 0 {
		handlers = append(handlers, ee.Handlers...)
	}
	err := ee.Stream(ctx, messages, think, handlers...)
	if err != nil {
		return chatResp, Errorf("%w", err)
	}
	return chatResp, nil
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
			return err
		}
	}
	if st.LoadDuration != 0 {
		_, err = fmt.Fprintf(w, "load duration:        %v\n", st.LoadDuration)
		if err != nil {
			return err
		}
	}
	if st.PromptEvalCount != 0 {
		_, err = fmt.Fprintf(w, "prompt eval count:    %d token(s)\n", st.PromptEvalCount)
		if err != nil {
			return err
		}
	}
	if st.PromptEvalDuration != 0 {
		_, err = fmt.Fprintf(w, "prompt eval duration: %s\n", st.PromptEvalDuration)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(w, "prompt eval rate:     %.2f tokens/s\n", float64(st.PromptEvalCount)/st.PromptEvalDuration.Seconds())
		if err != nil {
			return err
		}
	}
	if st.EvalCount != 0 {
		_, err = fmt.Fprintf(w, "eval count:           %d token(s)\n", st.EvalCount)
		if err != nil {
			return err
		}
	}
	if st.EvalDuration != 0 {
		_, err = fmt.Fprintf(w, "eval duration:        %s\n", st.EvalDuration)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(w, "eval rate:            %.2f tokens/s\n", float64(st.EvalCount)/st.EvalDuration.Seconds())
		if err != nil {
			return err
		}
	}
	return nil
}
