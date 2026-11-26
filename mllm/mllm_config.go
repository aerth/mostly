package mllm

import (
	"crypto/sha256"
	"flag"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"uuid"

	"github.com/aerth/mostly/stackerr"
)

/* PER-APPLICATION CONFIG */

// ThreadHasher for easy thread UUID by "name"
//
// Set to 'nil' if the "--thread" flag should strictly parse UUID.
var ThreadHasher = func(v string) uuid.UUID {
	h := sha256.New()
	_, err := h.Write([]byte(v))
	if err != nil {
		panic(err.Error())
	}
	b := h.Sum(nil)
	var id uuid.UUID
	copy(id[:], b)
	return id
}

// DefaultKeepAlive used in [E.NewRequest]
var DefaultKeepAlive = -1

var NoLogFlags bool

// DefaultThread for all [Message.ToChatMessage] calls.
//
// Applications may use this for chat rooms, conversations, etc
var DefaultThread = uuid.Nil()

var (
	DefaultOptions = Options{
		Temperature: new(1.0),
		Seed:        -1,
	}
	DefaultModel      string
	DefaultHost       string
	DefaultHTTPClient *http.Client
	ForcePath         = false
)

// ToolWriter for call log
var ToolWriter = toolWriter

// DefaultWriter added to all new endpoints
var DefaultWriter io.Writer = os.Stderr

// DefaultFrom name (bot name used in ToChatMessage, ToThreadMessage, MultiChat, MultiChat2)
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

// DefaultEmbedOptions is added to the embed requests
//
// Untested
var DefaultEmbedOptions map[string]any // nil

// ThreadFlag name can be renamed if collision with other flag name. If empty, it will not be used at all.
var ThreadFlag = "thread"

var threadFlagFn = func(s string) error {
	var id uuid.UUID
	if err := id.UnmarshalText([]byte(s)); err != nil {
		if ThreadHasher == nil {
			return err
		}
		id = ThreadHasher(s)
	}
	DefaultThread = id
	return nil
}

// Flags useful for programs to use, but not necessary. If -v or -q are already flags, set [NoLogFlags] to true.
func Flags() {
	flag.IntVar(&DefaultOptions.NumCtx, "llm-ctx", DefaultOptions.NumCtx, "llm num_ctx (0 does not send to api)")
	flag.Int64Var(&DefaultOptions.Seed, "llm-seed", DefaultOptions.Seed, "llm seed if necessary (0 does not send to api)")
	flag.Float64Var(DefaultOptions.Temperature, "temperature", *DefaultOptions.Temperature, "temperature")
	flag.StringVar(&DefaultModel, "model", DefaultModel, "model (ONLY if no env var)")
	flag.StringVar(&DefaultHost, "host", DefaultHost, "host URL (ONLY if no env var)")
	if ThreadFlag != "" {
		flag.Func(ThreadFlag, "conversation thread ID (if applicable)", threadFlagFn)
	}
	if !NoLogFlags {
		logFlags()
	}
}

func logFlags() {
	flag.BoolFunc("v", "enable log", func(s string) error {
		if s != "" {
			x, _ := strconv.ParseBool(s)
			if !x {
				return nil
			}
		}
		sl := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}))
		DefaultWriter = os.Stderr
		Slog = func() *slog.Logger {
			return sl
		}
		return nil
	})
	flag.BoolFunc("q", "disable log", func(s string) error {
		if s != "" {
			x, _ := strconv.ParseBool(s)
			if !x {
				return nil
			}
		}
		sl := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelError,
		}))
		DefaultWriter = io.Discard
		Slog = func() *slog.Logger {
			return sl
		}
		return nil
	})
}
