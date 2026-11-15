package mllm_test

import (
	"os"
	"testing"

	"github.com/aerth/mostly/mllm"
)

func TestVision(t *testing.T) {
	client := mllm.Default()
	ctx := t.Context()
	img, err := os.ReadFile("../screenshot.png")
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
