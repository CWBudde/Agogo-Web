package descriptor

import (
	"runtime"
	"testing"
)

// allocatedBytes reports how many bytes fn caused to be allocated.
//
// Bytes, not allocation count: the failure this guards against is one
// make([]Item, 0, n) with an attacker-chosen n, which is a single allocation of
// arbitrary size. testing.AllocsPerRun counts it as 1 either way and would let
// a 5 MB reservation pass.
func allocatedBytes(t *testing.T, fn func()) uint64 {
	t.Helper()
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	fn()
	runtime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc
}
