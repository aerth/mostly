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

	Client  *http.Client      `json:"-"` // custom http client (optional)
	Headers map[string]string `json:"-"` // custom headers (optional)
	Tools   []json.Marshaler  // Bring your own tools (optional)
	Options *Options          // e options (optional)

	// Writer if non-nil will be automatically added as a stream handler to [[E.Get]] and [[E.Stream]]
	Writer    io.Writer `json:"-"`
	ThinkPre  string    // optional, default used in [NewEndpoint]
	ThinkPost string    // optional, default used in [NewEndpoint]

	// Handlers added to all requests
	Handlers []StreamHandler `json:"-"`
}

type Tool struct {
	Spec json.RawMessage
	Exec any `json:"-"`
}

type RawToolCall = Call[json.RawMessage]

// Call is for a tool-runner to decode arguments. It handles the case where Arguments json object is passed as a json string.
type Call[T any] struct {
	ID       string `json:"id"`
	Index    int    `json:"index"`
	Type     string `json:"type"` // "function" always?
	Function struct {
		Name      string `json:"name"`
		Arguments T      `json:"arguments,omitempty"`
	} `json:"function"`
}

func (t Tool) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]any{"type": "function", "function": t.Spec})
}

func (c *Call[T]) UnmarshalJSON(b []byte) error {
	type plain Call[json.RawMessage]
	var x plain
	err := json.Unmarshal(b, &x)
	if err != nil {
		return err
	}
	var args T
	if x.Function.Arguments[0] != '"' {
		type plain2 Call[T]
		var x2 plain2
		err := json.Unmarshal(b, &x2)
		if err != nil {
			return err
		}
		*c = Call[T](x2)
		return nil
	}
	{
		var z string
		err = json.Unmarshal(x.Function.Arguments, &z)
		if err != nil {
			return err
		}
		x.Function.Arguments = json.RawMessage(z)
	}
	err = json.Unmarshal(x.Function.Arguments, &args)
	if err != nil {
		return err
	}
	*c = Call[T]{
		ID:    x.ID,
		Index: x.Index,
		Type:  x.Type,
	}
	c.Function.Name = x.Function.Name
	c.Function.Arguments = args
	return nil
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
	DefaultThinkPost = "\n... done thinking.\n"
)

var (
	EnvModel = []string{"MLLM_MODEL", "OLLAMA_MODEL", "LLM_MODEL"}
	EnvHost  = []string{"MLLM_HOST", "OLLAMA_HOST", "LLM_HOST"}
	EnvKey   = []string{"MLLM_API_KEY", "OLLAMA_API_KEY", "LLM_API_KEY"}
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

var ForcePath = false

// NewEndpoint reads env and returns a new reusable Endpoint
//
// See [E.StreamBG] or [E.Stream] or [E.Get]
func NewEndpoint(options *Options, tools []json.Marshaler) E {
	model := firstEnv(EnvModel)
	host := firstEnv(EnvHost)
	k := firstEnv(EnvKey)
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
	if k != "" {
		hdr = map[string]string{
			"Authorization": "Bearer " + k,
		}
	}
	return E{Host: hostURL, Model: model, Client: http.DefaultClient, Options: options, Tools: tools, Writer: DefaultWriter, ThinkPre: DefaultThinkPre, ThinkPost: DefaultThinkPost, Headers: hdr, Handlers: nil}
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
	if v, ok := tv.(string); ok {
		return json.Marshal(map[string]any{"effort": v})
	}
	if v, ok := tv.(bool); ok {
		if !v {
			return json.Marshal(map[string]any{"effort": "none"})
		}
		return json.Marshal(map[string]any{"effort": v})
	}
	return nil, fmt.Errorf("unknown think type %T", t.Value)
}

// ChatRequest sent to LLM
type ChatRequest struct {
	Model     string           `json:"model,omitempty"`
	Messages  []Message        `json:"messages,omitempty"`
	Stream    bool             `json:"stream"`
	Think     *Think           `json:"reasoning,omitempty"` // ollama endpoint "think" but /v1/chat/completions is "thinking"
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

type Message struct {
	Role       string            `json:"role,omitempty"`
	Content    string            `json:"content,omitempty"`
	Reasoning  string            `json:"thinking,omitempty" gorm:"default:null"`
	Reasoning2 string            `json:"reasoning,omitempty" gorm:"-"` // Internal, DO NOT USE
	ToolCalls  []json.RawMessage `json:"tool_calls,omitempty" gorm:"type:jsonb;serializer:json;default:null;"`
	ToolName   string            `json:"tool_name,omitempty" gorm:"default:null"`
	ToolCallID string            `json:"tool_call_id,omitempty" gorm:"default:null"`
	Images     [][]byte          `json:"images,omitempty" gorm:"serializer:json;type:jsonb;default:null;"`
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

func (cm ChatMessages) Top() ChatMessage {
	l := len(cm)
	if l == 0 {
		return ChatMessage{}
	}
	return cm[l-1]
}

func (cm ChatMessages) TopN(n int) ChatMessages {
	l := len(cm)
	if l == 0 {
		return nil
	}
	if l <= n {
		return cm
	}
	return cm[l-n:]
}

func (cm ChatMessages) TopRole(role string) ChatMessage {
	l := len(cm)
	if l == 0 {
		return ChatMessage{}
	}
	l -= 1
	for i := range cm {
		if cm[l-i].Role == role {
			return cm[i]
		}
	}
	return ChatMessage{}
}

// ChatMessage is a [Message] but with additional metadata (for a multi-participant conversation)
type ChatMessage struct {
	MessageID uuid.UUID `gorm:"type:uuid;primaryKey;default:null;" json:"message_id,omitempty"` // MessageID is unique identifier for each message
	Message
	From      string    `json:"from,omitempty" gorm:"default:null"`
	CreatedAt time.Time `json:"created_at"`
}

// ChatMessages like [Messages] but with additional metadata (for a multi-participant conversation)
type ChatMessages []ChatMessage

func (cm ChatMessages) ToMessages() Messages {
	a := make(Messages, len(cm))
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

// NewChatMessage from (see also [Message.ToChatMessage])
func NewChatMessage(from string, role string, text string) ChatMessage {
	m := Message{Role: role, Content: text}
	return m.ToChatMessage(from)
}

// ToChatMessage with new UUID,from,timestamp. should be called immediately after stream is finished..
//
// [ChatMessage] MAY be stored (NOT plain [Message])
func (m Message) ToChatMessage(from string) ChatMessage {
	m2 := ChatMessage{
		MessageID: uuid.New(),
		Message:   m,
		From:      from,
		CreatedAt: time.Now().Truncate(time.Millisecond),
	}
	return m2
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

func (m Message) IsEmpty() bool {
	return m.Content == "" && m.Reasoning == "" && m.Role == "" && len(m.ToolCalls) == 0
}

func (m Message) Equal(m2 Message) bool {
	return m.Content == m2.Content && m.Reasoning == m2.Reasoning && m.Role == m2.Role &&
		m.ToolName == m2.ToolName && m.ToolCallID == m2.ToolCallID &&
		slices.EqualFunc(m.ToolCalls, m2.ToolCalls, bytesEqual) &&
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

var (
	HideThink = false
	HideTools = false
	DebugLLM  = os.Getenv("DEBUG_LLM") == "1"
)

// NewHandlerOut plain 🍔
func NewHandlerOut(w io.Writer, thinkpre string, thinkpost string) StreamHandler {
	if w == nil {
		return func(cr ChatResponse) error { return nil }
	}
	reasoning := false
	hidethink := HideThink
	hidetools := HideTools
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
			} else { // toolwriter should print new line
				_, _ = fmt.Fprint(w, "\n")
			}
			fflush(w)
		}

		return nil
	}
}

var ToolWriter = toolWriter

func toolWriter(w io.Writer, rawMessage json.RawMessage) (err error) {
	return json.NewEncoder(w).Encode(map[string]any{"tool_call": rawMessage})
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
// Is a [StreamHandler] to gather [Message]
func (c *ChatResponse) Handler(in ChatResponse) error {
	c.Done = in.Done
	c.DoneReason = in.DoneReason
	if in.Stats != (Stats{}) {
		c.Stats = in.Stats
	}
	c.Message = c.Message.Add(in.Message)
	return nil
}

// StreamBG uses [E.Stream], closing ch after streaming is finished, sending non-nil error to the channel
func (e E) StreamBG(ctx context.Context, messages []Message, think any, handlers ...StreamHandler) chan error {
	ch := make(chan error)
	go func() {
		err := e.Stream(ctx, messages, think, handlers...)
		ch <- err // may be nil
		close(ch)
	}()
	return ch
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

var ErrNoConvo = errors.New("no messages in conversation")

// Stream with handlers(only) or to [E.Writer], calls [E.Do]
func (e E) Stream(ctx context.Context, messages []Message, think any, handlers ...StreamHandler) error {
	if len(messages) == 0 {
		return Errorf("%w", ErrNoConvo)
	}
	if e := e.ValidateConfig(think); e != nil {
		return e
	}
	if len(handlers) == 0 && e.Writer != nil {
		handlers = []StreamHandler{NewHandlerOut(e.Writer, e.ThinkPre, e.ThinkPost)}
	}
	if len(handlers) == 0 && len(messages) != 0 { // allow no handlers for a ping
		return Errorf("no stream handlers were given")
	}
	// t1 := time.Now()

	creq := *e.NewRequest(messages, think)
	return e.Do(ctx, creq, handlers)
}

// Do sends a [ChatRequest] and streams response in [E.Read]
func (e E) Do(ctx context.Context, creq ChatRequest, handlers []StreamHandler) error {
	bodyBytes, err := json.Marshal(creq)
	if err != nil {
		return Errorf("failed to marshal request: %w", err)
	}
	if e.Options != nil && e.Options.Debug {
		slog.Info("sending body", "js", json.RawMessage(bodyBytes))
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
	resp, err := e.Client.Do(req)
	if err != nil {
		return Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return Errorf("unexpected status code: %d %s", resp.StatusCode, string(b))
	}

	reader := bufio.NewReader(resp.Body)

	err = e.Read(reader, handlers...)
	if err != nil {
		return Errorf("%w", err)
	}
	return nil
}

func (e E) Read(reader *bufio.Reader, handlers ...StreamHandler) error {
	lines := 0
	var errs error
	modeP1 := false
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
		if err != nil {
			return fmt.Errorf("error reading stream: %w", err)
		}
		trimmedLine := bytes.TrimSpace(line)
		if len(trimmedLine) == 0 {
			continue
		}
		if post, ok := bytes.CutPrefix(trimmedLine, []byte("data: ")); ok {
			trimmedLine = post
			modeP1 = true
		}

		var chunk ChatResponse
		_ = modeP1 // TODO: use
		if bytes.HasPrefix(trimmedLine, []byte("[DONE]")) {
			continue
		} else {
			if err := json.Unmarshal(trimmedLine, &chunk); err != nil {
				slog.Error("decode cr", "err", err, "chunk", string(trimmedLine))
				errs = errors.Join(errs, Errorf("%w", err))
				continue
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
			for i := range chunk.Choices {
				choice := &chunk.Choices[i]
				if chunk.DoneReason2 != "" {
					chunk.Done = true
					chunk.DoneReason = chunk.DoneReason2
					chunk.DoneReason2 = ""
				}
				//	slog.Info("handling delta", "d", chunk.Choices[i].Delta)
				choice.Delta.Reasoning = chunk.Choices[i].Delta.Reasoning2

				choice.Delta.Reasoning2 = ""
				if ptr := choice.FinishReason; ptr != nil {
					chunk.Done = true
					chunk.DoneReason = *ptr
				}
				chunk.Message = chunk.Message.Add(choice.Delta)
				//	slog.Info("added delta", "c", chunk.Choices[i].Delta)
				if !chunk.Done && chunk.Message.IsEmpty() {
					slog.Info("handling delta", "d", chunk.Choices[i].Delta, "line", string(trimmedLine), "chunk", chunk)
					return fmt.Errorf("could not ")
				}
			}
			chunk.Choices = nil
		}
		if !chunk.Done && chunk.Message.IsEmpty() {
			slog.Warn("empty", "from", string(trimmedLine))
		} else if chunk.Message.Role == "" {
			chunk.Message.Role = "assistant"
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

func (e E) ValidateConfig(think any) error {
	var errs error
	if e.Model == "" {
		errs = errors.Join(errs, fmt.Errorf("missing MODEL"))
	}
	if e.Host.Scheme == "" || e.Host.Host == "" {
		errs = errors.Join(errs, fmt.Errorf("missing HOST"))
	}
	_, thinkB := think.(bool)
	_, thinkS := think.(string)
	if !thinkB && !thinkS && think != nil {
		errs = errors.Join(errs, Errorf("weird think type: %T", think))
	}
	return errs
}

func (e E) Chat(ctx context.Context, messages ChatMessages, think any) (ChatMessage, error) {
	x, err := e.Get(ctx, messages.ToMessages(), think)
	return x.Message.ToChatMessage(""), err
}

// Get calls [E.Stream] and waits until finished, returning the full ChatResponse
//
// Uses [E.Stream], writes stream text to [[E.Writer]] if non-nil, returns full [ChatResponse]
func (e E) Get(ctx context.Context, messages []Message, think any) (ChatResponse, error) {
	if e := e.ValidateConfig(think); e != nil {
		return ChatResponse{}, Errorf("validate: %w", e)
	}
	//slog := Slog()
	var chatResp ChatResponse
	var handlers []StreamHandler
	if e.Writer != nil {
		handlers = []StreamHandler{NewHandlerOut(e.Writer, e.ThinkPre, e.ThinkPost), chatResp.Handler}
	} else {
		handlers = []StreamHandler{chatResp.Handler}
	}
	if len(e.Handlers) != 0 {
		handlers = append(handlers, e.Handlers...)
	}
	err := e.Stream(ctx, messages, think, handlers...)
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
