package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/aerth/mostly/mllm"
)

func main() {
	log.SetFlags(0)
	apiurl, _ := url.Parse("https://ollama.home.arpa/v1/chat/completions")
	model := "gemma4:26b"
	historyfile := "history.tmp"
	mllm.DefaultOptions.Temperature = new(1.3)
	flag.StringVar(&apiurl.Host, "host", apiurl.Host, "host (not url)")
	flag.BoolFunc("http", "disable https", func(s string) error {
		if s == "" {
			s = "true"
		}
		b, _ := strconv.ParseBool(s)
		if b {
			apiurl.Scheme = "http://"
		}
		return nil
	})
	flag.StringVar(&model, "m", model, "model")
	flag.StringVar(&historyfile, "history", historyfile, "history file so jokes do not repeat (jsonl)")
	flag.Parse()
	streamer := mllm.NewHandlerOut(os.Stdout, mllm.DefaultThinkPre, mllm.DefaultThinkPost)
	client := mllm.E{
		Host:  *apiurl,
		Model: model,
		//	Writer: os.Stdout,
		Options:  &mllm.DefaultOptions, // to simplify flags above
		Client:   http.DefaultClient,
		Handlers: []mllm.StreamHandler{streamer.HandleChat},
	}
	system := "You are a joke creator. No markdown, No other output."
	prompt := "Tell funny short story"

	// history file example for non-conversation
	if historyfile != "" {
		history, _ := os.ReadFile(historyfile)
		if len(history) != 0 {
			prompt += "\nDo not repeat previous jokes: " + string(history)
		}
	}
	out, err := client.Generate(
		context.TODO(),
		system, prompt, false)
	if err != nil {
		log.Fatalln(err)
	}
	if strings.Contains(out.Content, "Wait, ") ||
		strings.Contains(out.Content, "Actually, ") ||
		strings.Contains(out.Content, "STOP") {
		log.Fatalln("fatal: bad generated")
	}
	if historyfile != "" && historyfile != "none" {
		f, err := os.OpenFile(historyfile, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0o644)
		if err == nil {
			err = json.NewEncoder(f).Encode(out.Content)
		}
		if err != nil {
			slog.Error("history file", "err", err)
		}
	}
}
