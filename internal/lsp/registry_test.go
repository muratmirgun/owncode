package lsp

import (
	"sync"
	"testing"
)

func TestRegistrySnapshotsDuringStartup(t *testing.T) {
	t.Parallel()
	r := &Registry{}
	var group sync.WaitGroup
	for range 4 {
		group.Go(func() {
			for range 100 {
				r.Store("server", &Client{})
				copy := r.Snapshot()
				delete(copy, "server")
				r.Remove("server")
			}
		})
	}
	group.Wait()
	r.Store("server", &Client{})
	snapshot := r.Snapshot()
	delete(snapshot, "server")
	if len(r.Snapshot()) != 1 {
		t.Fatal("snapshot changed registry")
	}
}
