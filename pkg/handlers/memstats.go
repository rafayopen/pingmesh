package handlers

import (
	"runtime"
	"time"
)

type MemStatSummary struct {
	When     time.Time
	ActiveMB uint64 // The number of live objects is Mallocs - Frees.
	AllocMB  uint64 // HeapAlloc is bytes of allocated heap objects.
	SystemMB uint64 // Sys is the total bytes of memory obtained from the OS.
	NumGC    uint32 // NumGC is the number of completed GC cycles.

}

func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}

func GetMemStatSummary() *MemStatSummary {
	// with thanks from https://golangcode.com/print-the-current-memory-usage/
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	active := m.Mallocs - m.Frees
	alloc := bToMb(m.Alloc)

	return &MemStatSummary{
		When:     time.Now(),
		ActiveMB: active,
		AllocMB:  alloc,
		SystemMB: bToMb(m.Sys),
		NumGC:    m.NumGC,
	}
}
