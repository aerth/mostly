package mllm_test

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"testing"

	"github.com/aerth/mostly/mllm"
)

func TestVision(t *testing.T) {
	_ = getTestImage(t)
	client := mllm.Default()
	ctx := t.Context()
	img, err := os.ReadFile("tmp/placeholder.png")
	if err != nil {
		t.Skip(err.Error())
		return
	}
	convo := mllm.Messages{
		{Role: "user", Content: "Whats this image say", Images: [][]byte{img}},
	}
	got, err := client.Get(ctx, convo, false)
	if err != nil {
		t.Fatal(err)
	}
	_ = got
}

func getTestImage(t *testing.T) (imageb []byte) {
	if _, err := os.Stat("tmp/placeholder.png"); err == nil {
		b, err := os.ReadFile("tmp/placeholder.png")
		if err != nil {
			panic(err.Error())
		}
		return b
	}
	placeholderURL := "https://picsum.photos/id/26/4209/2769"
	r, err := http.Get(placeholderURL)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Body.Close() }()
	b, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll("tmp", 0o755); err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile("tmp/placeholder.png", b, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestVisionJSON(t *testing.T) {
	imageb := getTestImage(t)
	msg := mllm.Message{
		Role:    "user",
		Content: "what are the objects [./tmp/placeholder.png]",
		Images:  [][]byte{imageb},
	}
	js, err := msg.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	var msg2 mllm.Message

	err = msg2.UnmarshalJSON(js)
	if err != nil {
		t.Fatal(err)
	}
	if len(msg2.Images) != 1 {
		t.Fatal("not image", len(msg2.Images))
	}
	if !bytes.Equal(msg2.Images[0], imageb) {
		t.Fatal("image modified")
	}
}
