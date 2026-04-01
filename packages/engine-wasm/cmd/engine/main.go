//go:build js && wasm

package main

import (
	"encoding/json"
	"syscall/js"

	"github.com/cwbudde/agogo-web/packages/engine-wasm/internal/buildinfo"
	"github.com/cwbudde/agogo-web/packages/engine-wasm/internal/engine"
)

func main() {
	js.Global().Set("EngineBuildInfo", js.ValueOf(map[string]any{
		"buildTime": buildinfo.BuildTime,
		"goVersion": buildinfo.GoVersion,
	}))

	js.Global().Set("EngineInit", js.FuncOf(func(_ js.Value, args []js.Value) any {
		configJSON := ""
		if len(args) > 0 {
			configJSON = args[0].String()
		}
		return engine.Init(configJSON)
	}))

	js.Global().Set("DispatchCommand", js.FuncOf(func(_ js.Value, args []js.Value) any {
		handle := int32(args[0].Int())
		commandID := int32(args[1].Int())
		payloadJSON := ""
		if len(args) > 2 {
			payloadJSON = args[2].String()
		}
		result, err := engine.DispatchCommand(handle, commandID, payloadJSON)
		if err != nil {
			return encodeResult(map[string]string{"error": err.Error()})
		}
		return encodeResult(result)
	}))

	js.Global().Set("RenderFrame", js.FuncOf(func(_ js.Value, args []js.Value) any {
		handle := int32(args[0].Int())
		result, err := engine.RenderFrame(handle)
		if err != nil {
			return encodeResult(map[string]string{"error": err.Error()})
		}
		return encodeResult(result)
	}))

	js.Global().Set("RenderFrameRaw", js.FuncOf(func(_ js.Value, args []js.Value) any {
		handle := int32(args[0].Int())
		result, err := engine.RenderFrameRaw(handle)
		if err != nil {
			return encodeResult(map[string]string{"error": err.Error()})
		}
		return encodeResult(result)
	}))

	js.Global().Set("ExportProject", js.FuncOf(func(_ js.Value, args []js.Value) any {
		handle := int32(args[0].Int())
		result, err := engine.ExportProject(handle)
		if err != nil {
			return encodeResult(map[string]string{"error": err.Error()})
		}
		return result
	}))

	js.Global().Set("ImportProject", js.FuncOf(func(_ js.Value, args []js.Value) any {
		handle := int32(args[0].Int())
		payload := ""
		if len(args) > 1 {
			payload = args[1].String()
		}
		result, err := engine.ImportProject(handle, payload)
		if err != nil {
			return encodeResult(map[string]string{"error": err.Error()})
		}
		return encodeResult(result)
	}))

	js.Global().Set("GetBufferPtr", js.FuncOf(func(_ js.Value, args []js.Value) any {
		return engine.GetBufferPtr(int32(args[0].Int()))
	}))

	js.Global().Set("GetBufferLen", js.FuncOf(func(_ js.Value, args []js.Value) any {
		return engine.GetBufferLen(int32(args[0].Int()))
	}))

	js.Global().Set("Free", js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) > 0 {
			engine.FreePointer(int32(args[0].Int()))
		}
		return nil
	}))

	done := make(chan struct{})
	<-done
}

func encodeResult(value any) string {
	bytes, err := json.Marshal(value)
	if err != nil {
		return `{"error":"failed to encode result"}`
	}
	return string(bytes)
}
