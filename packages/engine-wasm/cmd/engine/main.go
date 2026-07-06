//go:build js && wasm

package main

import (
	"encoding/json"
	"runtime/debug"
	"syscall/js"

	"github.com/cwbudde/agogo-web/packages/engine-wasm/internal/buildinfo"
	"github.com/cwbudde/agogo-web/packages/engine-wasm/internal/engine"
)

func main() {
	// Interim GC-hitch mitigation for the render path: let the heap grow to
	// 300% between collections (default 100%) so bursty per-frame compositing
	// allocations do not trigger a stop-the-world GC mid stroke-batch. This is
	// a stopgap until buffer reuse fully lands across the composite path
	// (PLAN.md S.4); it is a percentage bump only — GOMEMLIMIT is left unset.
	debug.SetGCPercent(300)

	js.Global().Set("EngineBuildInfo", js.ValueOf(map[string]any{
		"buildTime": buildinfo.BuildTime,
		"goVersion": buildinfo.GoVersion,
	}))

	js.Global().Set("EngineInit", js.FuncOf(func(_ js.Value, args []js.Value) any {
		configJSON, ok := optionalString(args, 0, "")
		if !ok {
			// No error channel available on this ABI (it returns a bare
			// handle), so signal failure with a handle no instance will
			// ever have. Downstream calls with this handle safely fail
			// with "invalid engine handle" rather than panicking.
			return int32(-1)
		}
		return engine.Init(configJSON)
	}))

	js.Global().Set("DispatchCommand", js.FuncOf(func(_ js.Value, args []js.Value) any {
		handle, ok := requireInt32(args, 0)
		if !ok {
			return errorResult("DispatchCommand: missing or invalid handle argument")
		}
		commandID, ok := requireInt32(args, 1)
		if !ok {
			return errorResult("DispatchCommand: missing or invalid commandId argument")
		}
		payloadJSON, ok := optionalString(args, 2, "")
		if !ok {
			return errorResult("DispatchCommand: payload argument must be a string")
		}
		result, err := engine.DispatchCommand(handle, commandID, payloadJSON)
		if err != nil {
			return errorResult(err.Error())
		}
		return encodeResult(result)
	}))

	js.Global().Set("RenderFrame", js.FuncOf(func(_ js.Value, args []js.Value) any {
		handle, ok := requireInt32(args, 0)
		if !ok {
			return errorResult("RenderFrame: missing or invalid handle argument")
		}
		result, err := engine.RenderFrame(handle)
		if err != nil {
			return errorResult(err.Error())
		}
		return encodeResult(result)
	}))

	js.Global().Set("RenderFrameRaw", js.FuncOf(func(_ js.Value, args []js.Value) any {
		handle, ok := requireInt32(args, 0)
		if !ok {
			return errorResult("RenderFrameRaw: missing or invalid handle argument")
		}
		result, err := engine.RenderFrameRaw(handle)
		if err != nil {
			return errorResult(err.Error())
		}
		return encodeResult(result)
	}))

	js.Global().Set("ExportProject", js.FuncOf(func(_ js.Value, args []js.Value) any {
		handle, ok := requireInt32(args, 0)
		if !ok {
			return errorResult("ExportProject: missing or invalid handle argument")
		}
		result, err := engine.ExportProject(handle)
		if err != nil {
			return errorResult(err.Error())
		}
		return result
	}))

	js.Global().Set("ExportDocument", js.FuncOf(func(_ js.Value, args []js.Value) any {
		handle, ok := requireInt32(args, 0)
		if !ok {
			return errorResult("ExportDocument: missing or invalid handle argument")
		}
		format, ok := optionalString(args, 1, "")
		if !ok {
			return errorResult("ExportDocument: format argument must be a string")
		}
		result, err := engine.ExportDocument(handle, format)
		if err != nil {
			return errorResult(err.Error())
		}
		return result
	}))

	js.Global().Set("ImportProject", js.FuncOf(func(_ js.Value, args []js.Value) any {
		handle, ok := requireInt32(args, 0)
		if !ok {
			return errorResult("ImportProject: missing or invalid handle argument")
		}
		payload, ok := optionalString(args, 1, "")
		if !ok {
			return errorResult("ImportProject: payload argument must be a string")
		}
		result, err := engine.ImportProject(handle, payload)
		if err != nil {
			return errorResult(err.Error())
		}
		return encodeResult(result)
	}))

	js.Global().Set("GetBufferPtr", js.FuncOf(func(_ js.Value, args []js.Value) any {
		handle, ok := requireInt32(args, 0)
		if !ok {
			return int32(0)
		}
		return engine.GetBufferPtr(handle)
	}))

	js.Global().Set("GetBufferLen", js.FuncOf(func(_ js.Value, args []js.Value) any {
		handle, ok := requireInt32(args, 0)
		if !ok {
			return int32(0)
		}
		return engine.GetBufferLen(handle)
	}))

	// Free releases the pixel buffer pointer returned by a previous render.
	// It is a no-op today because the render buffer lives inside the engine
	// instance's own memory (see engine.FreePointer) rather than being a
	// separately-owned allocation, but the export is kept for ABI
	// compatibility with existing callers.
	js.Global().Set("Free", js.FuncOf(func(_ js.Value, args []js.Value) any {
		if pointer, ok := requireInt32(args, 0); ok {
			engine.FreePointer(pointer)
		}
		return nil
	}))

	// EngineFree releases the engine instance identified by handle (as
	// returned by EngineInit), allowing Go's GC to reclaim the document and
	// its buffers. Freeing an unknown or already-freed handle is a safe
	// no-op. Callers should invoke this exactly once per EngineInit, when
	// the engine is no longer needed (e.g. on document close or teardown).
	js.Global().Set("EngineFree", js.FuncOf(func(_ js.Value, args []js.Value) any {
		if handle, ok := requireInt32(args, 0); ok {
			engine.Free(handle)
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

// errorResult wraps a message in the same JSON error shape used across the
// command ABI: {"error": "..."}.
func errorResult(msg string) string {
	return encodeResult(map[string]string{"error": msg})
}

// requireInt32 returns args[i] as an int32 if it exists and is a JS number,
// reporting false otherwise so callers never index out of range or type-assert
// a mismatched js.Value (both of which panic and would otherwise crash the
// whole Wasm runtime on malformed input from JS).
func requireInt32(args []js.Value, i int) (int32, bool) {
	if i >= len(args) || args[i].Type() != js.TypeNumber {
		return 0, false
	}
	return int32(args[i].Int()), true
}

// optionalString returns args[i] as a string if present, or def if the
// argument was omitted. It reports false only when the argument is present
// but is not a JS string, so callers can distinguish "omitted" (fine) from
// "wrong type" (reject).
func optionalString(args []js.Value, i int, def string) (string, bool) {
	if i >= len(args) {
		return def, true
	}
	if args[i].Type() != js.TypeString {
		return "", false
	}
	return args[i].String(), true
}
