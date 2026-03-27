//go:build js && wasm

package main

import (
	"encoding/json"
	"sync"
	"syscall/js"
	"testing"
	"time"

	"github.com/MeKo-Tech/agogo-web/packages/engine-wasm/internal/engine"
)

var registerBridgeOnce sync.Once

func TestMainRegistersBridgeFunctions(t *testing.T) {
	ensureBridgeRegistered(t)

	for _, name := range []string{"EngineInit", "DispatchCommand", "RenderFrame", "ExportProject", "ImportProject", "GetBufferPtr", "GetBufferLen", "Free"} {
		if value := waitForGlobalFunction(t, name); value.Type() != js.TypeFunction {
			t.Fatalf("%s type = %v, want function", name, value.Type())
		}
	}

	buildInfo := js.Global().Get("EngineBuildInfo")
	if buildInfo.Type() != js.TypeObject {
		t.Fatalf("EngineBuildInfo type = %v, want object", buildInfo.Type())
	}
	if buildInfo.Get("goVersion").Type() != js.TypeString {
		t.Fatalf("goVersion type = %v, want string", buildInfo.Get("goVersion").Type())
	}
}

func TestMainBridgeCanRenderAndRoundTripProject(t *testing.T) {
	ensureBridgeRegistered(t)

	handle := waitForGlobalFunction(t, "EngineInit").Invoke("").Int()
	if handle <= 0 {
		t.Fatalf("EngineInit returned handle %d, want > 0", handle)
	}

	rendered := decodeRenderResult(t, waitForGlobalFunction(t, "RenderFrame").Invoke(handle).String())
	if rendered.BufferLen == 0 {
		t.Fatal("RenderFrame returned an empty buffer")
	}

	bufferPtr := waitForGlobalFunction(t, "GetBufferPtr").Invoke(handle).Int()
	bufferLen := waitForGlobalFunction(t, "GetBufferLen").Invoke(handle).Int()
	if bufferPtr == 0 {
		t.Fatal("GetBufferPtr returned 0 after RenderFrame")
	}
	if int32(bufferLen) != rendered.BufferLen {
		t.Fatalf("GetBufferLen = %d, want %d", bufferLen, rendered.BufferLen)
	}

	exported := waitForGlobalFunction(t, "ExportProject").Invoke(handle).String()
	if exported == "" {
		t.Fatal("ExportProject returned an empty string")
	}

	imported := decodeRenderResult(t, waitForGlobalFunction(t, "ImportProject").Invoke(handle, exported).String())
	if imported.UIMeta.ActiveDocumentName == "" {
		t.Fatal("ImportProject returned no active document name")
	}
	if imported.BufferLen == 0 {
		t.Fatal("ImportProject returned an empty render buffer")
	}

	waitForGlobalFunction(t, "Free").Invoke(bufferPtr)
	if result := waitForGlobalFunction(t, "GetBufferLen").Invoke(handle).Int(); result == 0 {
		t.Fatal("Free should not release the engine-owned render buffer")
	}
}

func ensureBridgeRegistered(t *testing.T) {
	t.Helper()
	registerBridgeOnce.Do(func() {
		go main()
	})
	_ = waitForGlobalFunction(t, "EngineInit")
}

func waitForGlobalFunction(t *testing.T, name string) js.Value {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		value := js.Global().Get(name)
		if value.Type() == js.TypeFunction {
			return value
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for global function %s", name)
	return js.Undefined()
}

func decodeRenderResult(t *testing.T, raw string) engine.RenderResult {
	t.Helper()

	var bridgeErr struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(raw), &bridgeErr); err != nil {
		t.Fatalf("json.Unmarshal bridge error payload: %v", err)
	}
	if bridgeErr.Error != "" {
		t.Fatalf("bridge returned error: %s", bridgeErr.Error)
	}

	var result engine.RenderResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("json.Unmarshal render result: %v", err)
	}
	return result
}
