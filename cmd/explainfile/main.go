package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aerth/mostly/mllm"
)

var readfile = mllm.Tool{
	Spec: json.RawMessage(`{
  "name": "readfile",
  "description": "reads the content of a file from the filesystem",
  "parameters": {
    "type": "object",
    "properties": {
      "name": {
        "type": "string",
        "description": "the name or path of the file to read"
      }
    },
    "required": [
      "name"
    ]
  }
}`),
	Exec: func(ctx context.Context, args map[string]any) (any, error) {
		name, ok := args["name"].(string)
		if !ok || name == "" {
			return nil, fmt.Errorf("tool call forgot 'name' to readfile (got %v)", args)
		}
		got, err := os.ReadFile(name)
		if err != nil {
			return nil, fmt.Errorf("ReadFile: %w", err)
		}
		// TODO: check if text...
		return fmt.Sprintf("Contents of %q (%d bytes):\n%s", filepath.Base(name), len(got), string(got)), nil
	},
}

func main() {
	flag.BoolVar(&mllm.HideTools, "q", mllm.HideTools, "hide tool calls")
	flag.Parse()

	filename := flag.Arg(0)
	if filename == "" {
		println("filename argument missing")
		os.Exit(1)
	}
	if _, err := os.Stat(filename); err != nil {
		println("fatal", err.Error())
		os.Exit(1)

	}
	ctx := context.TODO()
	client := mllm.Default()

	toolbox := mllm.Tools{readfile}
	client.Tools = toolbox.ToJSON()
	if false {
		for _, clientTool := range client.Tools {
			js, _ := clientTool.MarshalJSON()
			println("TOOL:", string(js))
		}
	}

	convo := mllm.Messages{
		{Role: "system", Content: "Use the tools to help the user"},
		{Role: "user", Content: "Read and explain the file. No markdown, No fluff. Filename: " + filename},
	}
	caller, err := client.Get(ctx, convo, false)
	if err != nil {
		println("fatal", err.Error())
		os.Exit(1)
	}
	convo = append(convo, caller.Message)

	for _, js := range caller.Message.ToolCalls {
		var call mllm.Call[struct {
			Name string `json:"name"`
		}]
		err = json.Unmarshal(js, &call)
		if err != nil {
			println("fatal", err.Error())
			os.Exit(1)
		}
		toolout, err := toolbox.Run(ctx, call.Function.Name, call.Function.ArgsToMap())
		if err != nil {
			println("fatal toolbox", call.Function.Name, err.Error())
			os.Exit(1)
		}
		convo = append(convo, call.ReplyTool(toolout))
	}
	_, err = client.Get(ctx, convo, false)
	if err != nil {
		println("fatal", err.Error())
		os.Exit(1)
	}
}
