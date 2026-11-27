package mllm_test

import (
	"fmt"
	"testing"

	"github.com/aerth/mostly/mllm"
	"github.com/aerth/mostly/ncode"
)

// test [mllm.E.Get]
func TestStream4(t *testing.T) {
	ai := mllm.Default()
	if ai.ValidateConfig(false) != nil {
		t.SkipNow()
		return
	}
	mllm.DefaultKeepAlive = 0
	ai.Options.StreamOptions = map[string]any{"include_usage": true}
	ctx := t.Context()
	got, err := ai.Get(ctx, mllm.Messages{
		{Role: "user", Content: "In one sentence quickly, describe software that does not suck"},
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	if got.Usage == nil {
		t.Fatal("wanted usage got nil")
	}
	fmt.Printf("Usage: %s\nModel: %s", ncode.JSONString(got.Usage), got.Model)
}
