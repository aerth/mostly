package mllm

import "context"

// Stream with handlers(only) or to [E.Writer], calls [E.Do]
//
// Like all [E] methods, uses stream mode with /v1/chat/completions endpoint.
func (e E) Stream(ctx context.Context, messages []Message, think any, handlers ...StreamHandler) error {
	if len(messages) == 0 {
		return Errorf("%w", ErrNoConvo)
	}
	if e := e.ValidateConfig(think); e != nil {
		return Errorf("%w", e)
	}
	creq := e.NewRequest(messages, think)
	if creq == nil {
		return Errorf("could not create request")
	}
	if e.PreRequest != nil {
		if err := e.PreRequest(creq); err != nil {
			return Errorf("%w", err)
		}
	}
	if err := e.Do(ctx, *creq, handlers); err != nil {
		return Errorf("%w", err)
	}
	return nil
}

// MultiChat returns a threaded ChatMessage (from [E.From] in thread [E.ThreadID])
//
// For Messages input see [E.MultiChat2]
//
// Like all [E] methods, uses stream mode with /v1/chat/completions endpoint.
func (e E) MultiChat(ctx context.Context, messages ChatMessages, think any) (ChatMessage, error) {
	x, err := e.Get(ctx, messages.ToMessages(), think)
	return *x.Message.ToThreadMessage(e.From, e.ThreadID), err
}

// MultiChat2 returns a threaded ChatMessage (from [E.From] in thread [E.ThreadID])
//
// For ChatMessages input see [E.MultiChat]
//
// Like all [E] methods, uses stream mode with /v1/chat/completions endpoint.
func (e E) MultiChat2(ctx context.Context, messages Messages, think any) (ChatMessage, error) {
	x, err := e.Get(ctx, messages, think)
	return *x.Message.ToThreadMessage(e.From, e.ThreadID), err
}

// GetMessage is [E.Get] but only message.
//
// Like all [E] methods, uses stream mode with /v1/chat/completions endpoint.
func (e E) GetMessage(ctx context.Context, messages []Message, think any) (Message, error) {
	x, err := e.Get(ctx, messages, think)
	return x.Message, err
}

// Get calls [E.Stream] and waits until finished, returning the full ChatResponse
//
// Uses [E.Stream], writes stream text to [E.Writer] if non-nil, returns full [ChatResponse]
//
// Like all [E] methods, uses stream mode with /v1/chat/completions endpoint.
func (e E) Get(ctx context.Context, messages []Message, think any) (ChatResponse, error) {
	if e := e.ValidateConfig(think); e != nil {
		return ChatResponse{}, Errorf("validate: %w", e)
	}
	//slog := Slog()
	var chatResp ChatResponse
	var handlers []StreamHandler
	if e.StreamWriter.Writer != nil {
		handlers = []StreamHandler{e.StreamWriter.New()}
	} else if e.Writer != nil {
		handlers = []StreamHandler{W1{e.Writer, DefaultThinkPre, DefaultThinkPost}.New()}
	} else {
		handlers = []StreamHandler{} // no printing
	}
	handlers = append(handlers, chatResp.Handler)
	if len(e.Handlers) != 0 {
		handlers = append(handlers, e.Handlers...)
	}
	err := e.Stream(ctx, messages, think, handlers...)
	if err != nil {
		return chatResp, Errorf("Stream: %w", err)
	}
	return chatResp, nil
}

// Generate helper, uses [E.GetMessage] with a new conversation.
//
// Like all [E] methods, uses stream mode with /v1/chat/completions endpoint.
func (e E) Generate(ctx context.Context, sys string, usr string, think any) (Message, error) {
	if sys == "" {
		sys = "You are a helpful assistant."
	}
	return e.GetMessage(ctx, Messages{
		{Role: "system", Content: sys},
		{Role: "user", Content: usr},
	}, think)
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
