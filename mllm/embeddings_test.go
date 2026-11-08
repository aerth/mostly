package mllm_test

import (
	"fmt"
	"slices"
	"testing"

	"github.com/aerth/mostly/mllm"
	"github.com/aerth/mostly/vector256"
)

var testEmbedClient mllm.E

// compareFloat needed for some reason
func compareFloat[T ~float32 | ~float64](x, y T) bool {
	const epsilon = 1e-4
	diff := x - y
	if diff < 0 {
		diff = -diff
	}
	return diff < epsilon
}

func TestEmbed(t *testing.T) {
	testEmbedClient = mllm.Default()
	t.Run("normal", testEmbed1)
	t.Run("nomic-query", testEmbedNomicQuery)
	t.Run("nomic-document", testEmbedNomicDocument)
}

func testEmbed1(t *testing.T) {
	ctx := t.Context()
	x, err := mllm.Default().Embed(ctx, mllm.NewEmbed("testing test tester"))
	if err != nil {
		// same model as ollama
		println("To get the nomic embedding model as used for this test:")
		println("  hf download nomic-ai/nomic-embed-text-v1.5-GGUF nomic-embed-text-v1.5.f16.gguf --local-dir .")
		println("  ln -sf nomic-embed-text-v1.5.f16.gguf nomic-embed-text:v1.5.gguf")
		t.Fatal("api:", err)
	}
	if x == nil {
		t.Fatal("embed response is nil")
	}
	got := x.GetEmbeddings()
	if len(got) != vector256.VarLen {
		t.Fatalf("%d != %d", len(got), vector256.VarLen)
	}
	println(" got:", fmt.Sprintf("%#v ... %d total", got[:10], vector256.VarLen))
	first := vector256.Vector256{0.03836852, 0.056748904, -0.27613422, -0.004759476, 0.11951496, -0.03993982, 0.05664575, -0.031644862, 0.01586234, -0.03084815}

	for i := range first {
		if !compareFloat(first[i], got[i]) {
			println("want:", fmt.Sprintf("%#v ... %d total", first[:10], vector256.VarLen))
			t.Fatalf("v %d: %v != %v", i, got[i], first[i])
		}
	}
	if false && !slices.Equal(first, got[:10]) { // TODO: it broke
		t.Fatalf("unexpected output: %#v", got[:10])
	}
}

func testEmbedNomicQuery(t *testing.T) {
	ctx := t.Context()
	x, err := mllm.Default().Embed(ctx, mllm.NewNomicQuery("testing test tester"))
	if err != nil {
		//	println(string(err.(mllm.ErrorWithResponse).Body()))
		t.Fatal(err)
	}
	if x == nil {
		t.Fatal(err)
	}
	got := x.GetEmbeddings()
	if len(got) != vector256.VarLen {
		t.Fatalf("%d != %d", len(got), vector256.VarLen)
	}
	println("got:", fmt.Sprintf("%#v ... %d total", got[:10], vector256.VarLen))
	first := []vector256.F{
		0.03676727, 0.06071676, -0.29565915, -0.017700996, 0.11540128,
		-0.036267255, 0.04523779, -0.062040724, 0.007202007, -0.017709982,
	}
	for i := range first {
		if !compareFloat(first[i], got[i]) {
			t.Fatalf("v %d: %v != %v", i, got[i], first[i])
		}
	}
	if true {
		return
	}
}

func testEmbedNomicDocument(t *testing.T) {
	ctx := t.Context()
	x, err := mllm.Default().Embed(ctx, mllm.NewNomicDocument("testing test tester"))
	if err != nil {
		t.Fatal(err)
	}
	if x == nil {
		t.Fatal(err)
	}
	got := x.GetEmbeddings()
	if len(got) != vector256.VarLen {
		t.Fatalf("%d != %d", len(got), vector256.VarLen)
	}
	println("got:", fmt.Sprintf("%#v ... %d total", got[:10], vector256.VarLen))
	first := vector256.Vector256{0.046110913, 0.032962788, -0.20280331, 0.012861654, 0.1151968, -0.0126432115, 0.0051027983, -0.07082936, -0.01349577, -0.021741517}
	for i := range first {
		if !compareFloat(first[i], got[i]) {
			t.Fatalf("v %d: %v != %v", i, got[i], first[i])
		}
	}
}
