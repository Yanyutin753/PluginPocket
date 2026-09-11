package marketplace

import (
	"bytes"
	"sync"
	"testing"
)

func TestGitRegistrySharesChangesAndHistoryAcrossReplicas(t *testing.T) {
	s := marketplaceDB(t)
	a, b := NewGitRegistry(s.Pool, Options{}), NewGitRegistry(s.Pool, Options{})
	first, err := a.Files(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = b.Files(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Pool.Exec(t.Context(), `UPDATE marketplace_items SET description='Changed on another replica' WHERE slug='deepwiki'`); err != nil {
		t.Fatal(err)
	}
	// The committed database change must invalidate every warm replica without
	// broadcasting to a process or waiting for its local TTL.
	changed, err := b.Files(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(first["info/refs"], changed["info/refs"]) {
		t.Fatal("warm replica kept stale refs after a committed market change")
	}
	restarted, err := NewGitRegistry(s.Pool, Options{}).Files(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(changed["info/refs"], restarted["info/refs"]) {
		t.Fatal("new replica invented a different commit history")
	}
	for path, content := range first {
		if len(path) > 8 && path[:8] == "objects/" && !bytes.Equal(content, restarted[path]) {
			t.Fatalf("restart lost an object already advertised to clients: %s", path)
		}
	}
}

func TestGitRegistryConcurrentPublishAndUnchangedRefresh(t *testing.T) {
	s := marketplaceDB(t)
	var workers sync.WaitGroup
	refs := make(chan string, 4)
	for range 4 {
		workers.Go(func() {
			files, err := NewGitRegistry(s.Pool, Options{}).Files(t.Context())
			if err != nil {
				t.Error(err)
				return
			}
			refs <- string(files["info/refs"])
		})
	}
	workers.Wait()
	close(refs)
	var expected string
	for value := range refs {
		if expected != "" && value != expected {
			t.Fatal("concurrent replicas published different histories")
		}
		expected = value
	}
	if _, err := s.Pool.Exec(t.Context(), "UPDATE marketplace_git_state SET built_at=NULL WHERE singleton"); err != nil {
		t.Fatal(err)
	}
	files, err := NewGitRegistry(s.Pool, Options{}).Files(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if string(files["info/refs"]) != expected {
		t.Fatal("unchanged refresh created an empty commit")
	}
}
