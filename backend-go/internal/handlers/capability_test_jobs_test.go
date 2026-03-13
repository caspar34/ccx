package handlers

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCapabilityJobStore_GetOrCreateByLookupKey_Concurrent(t *testing.T) {
	store := &capabilityTestJobStore{
		jobs:      make(map[string]*CapabilityTestJob),
		lookupKey: make(map[string]string),
	}

	var buildCount int32
	var reusedCount int32
	const total = 20
	lookupKey := "capability:messages:1"

	jobIDs := make(chan string, total)
	var wg sync.WaitGroup
	wg.Add(total)

	for i := 0; i < total; i++ {
		go func() {
			defer wg.Done()
			job, reused := store.getOrCreateByLookupKey(lookupKey, func() *CapabilityTestJob {
				atomic.AddInt32(&buildCount, 1)
				return newCapabilityTestJob(1, "channel", "messages", "claude", []string{"messages"}, 10*time.Second)
			})
			if reused {
				atomic.AddInt32(&reusedCount, 1)
			}
			jobIDs <- job.JobID
		}()
	}

	wg.Wait()
	close(jobIDs)

	var firstID string
	for id := range jobIDs {
		if firstID == "" {
			firstID = id
			continue
		}
		if id != firstID {
			t.Fatalf("jobID mismatch: got %s, want %s", id, firstID)
		}
	}

	if buildCount != 1 {
		t.Fatalf("builder called %d times, want 1", buildCount)
	}
	if reusedCount != total-1 {
		t.Fatalf("reusedCount = %d, want %d", reusedCount, total-1)
	}
}
