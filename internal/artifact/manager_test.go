package artifact

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/org/github-runner/pkg/api"
)

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	dir := t.TempDir()
	m, err := NewManager(ManagerConfig{
		BaseDir:              dir,
		DefaultRetentionDays: 0,
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	return m
}

func TestManager(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name string
		fn   func(t *testing.T, m *Manager)
	}{
		{
			name: "upload and download",
			fn: func(t *testing.T, m *Manager) {
				data := []byte("artifact content here")
				err := m.Upload(ctx, "job1", "build.tar", bytes.NewReader(data), api.UploadOptions{})
				if err != nil {
					t.Fatalf("Upload: %v", err)
				}

				rc, err := m.Download(ctx, "job1", "build.tar")
				if err != nil {
					t.Fatalf("Download: %v", err)
				}
				defer rc.Close()

				got, err := io.ReadAll(rc)
				if err != nil {
					t.Fatalf("ReadAll: %v", err)
				}
				if !bytes.Equal(got, data) {
					t.Errorf("got %q, want %q", got, data)
				}
			},
		},
		{
			name: "download nonexistent returns error",
			fn: func(t *testing.T, m *Manager) {
				_, err := m.Download(ctx, "job1", "missing")
				if err == nil {
					t.Error("expected error for nonexistent artifact")
				}
			},
		},
		{
			name: "upload replaces existing artifact",
			fn: func(t *testing.T, m *Manager) {
				_ = m.Upload(ctx, "job1", "out", bytes.NewReader([]byte("v1")), api.UploadOptions{})
				_ = m.Upload(ctx, "job1", "out", bytes.NewReader([]byte("v2")), api.UploadOptions{})

				rc, err := m.Download(ctx, "job1", "out")
				if err != nil {
					t.Fatalf("Download: %v", err)
				}
				defer rc.Close()

				got, _ := io.ReadAll(rc)
				if string(got) != "v2" {
					t.Errorf("got %q, want %q", got, "v2")
				}
			},
		},
		{
			name: "list returns artifact metadata",
			fn: func(t *testing.T, m *Manager) {
				_ = m.Upload(ctx, "job1", "a.tar", bytes.NewReader([]byte("aaa")), api.UploadOptions{})
				_ = m.Upload(ctx, "job1", "b.tar", bytes.NewReader([]byte("bb")), api.UploadOptions{})

				items, err := m.List(ctx, "job1")
				if err != nil {
					t.Fatalf("List: %v", err)
				}

				if len(items) != 2 {
					t.Fatalf("expected 2 artifacts, got %d", len(items))
				}

				names := map[string]bool{}
				for _, item := range items {
					names[item.Name] = true
					if item.SHA256 == "" {
						t.Errorf("artifact %q has empty SHA256", item.Name)
					}
					if item.CreatedAt.IsZero() {
						t.Errorf("artifact %q has zero CreatedAt", item.Name)
					}
				}
				if !names["a.tar"] || !names["b.tar"] {
					t.Errorf("unexpected artifact names: %v", names)
				}
			},
		},
		{
			name: "list nonexistent job returns nil",
			fn: func(t *testing.T, m *Manager) {
				items, err := m.List(ctx, "nope")
				if err != nil {
					t.Fatalf("List: %v", err)
				}
				if items != nil {
					t.Errorf("expected nil, got %v", items)
				}
			},
		},
		{
			name: "delete removes artifact",
			fn: func(t *testing.T, m *Manager) {
				_ = m.Upload(ctx, "job1", "rm-me", bytes.NewReader([]byte("bye")), api.UploadOptions{})

				err := m.Delete(ctx, "job1", "rm-me")
				if err != nil {
					t.Fatalf("Delete: %v", err)
				}

				_, err = m.Download(ctx, "job1", "rm-me")
				if err == nil {
					t.Error("expected error after delete")
				}
			},
		},
		{
			name: "delete nonexistent is no-op",
			fn: func(t *testing.T, m *Manager) {
				err := m.Delete(ctx, "job1", "nope")
				if err != nil {
					t.Errorf("Delete nonexistent: %v", err)
				}
			},
		},
		{
			name: "upload with retention days sets expiration",
			fn: func(t *testing.T, m *Manager) {
				opts := api.UploadOptions{RetentionDays: 7}
				_ = m.Upload(ctx, "job1", "retained", bytes.NewReader([]byte("data")), opts)

				items, _ := m.List(ctx, "job1")
				if len(items) != 1 {
					t.Fatalf("expected 1 artifact, got %d", len(items))
				}
				if items[0].ExpiresAt.IsZero() {
					t.Error("expected non-zero ExpiresAt with retention days set")
				}

				expectedExpiry := items[0].CreatedAt.Add(7 * 24 * time.Hour)
				diff := items[0].ExpiresAt.Sub(expectedExpiry)
				if diff < -time.Second || diff > time.Second {
					t.Errorf("ExpiresAt %v not within 1s of expected %v", items[0].ExpiresAt, expectedExpiry)
				}
			},
		},
		{
			name: "upload computes correct sha256",
			fn: func(t *testing.T, m *Manager) {
				data := []byte("hash me")
				_ = m.Upload(ctx, "job1", "hashed", bytes.NewReader(data), api.UploadOptions{})

				items, _ := m.List(ctx, "job1")
				if len(items) != 1 {
					t.Fatalf("expected 1 artifact, got %d", len(items))
				}

				// Independently compute the expected hash.
				expected, _ := ComputeSHA256(bytes.NewReader(data))
				if items[0].SHA256 != expected {
					t.Errorf("SHA256 = %s, want %s", items[0].SHA256, expected)
				}
			},
		},
		{
			name: "concurrent uploads to different jobs",
			fn: func(t *testing.T, m *Manager) {
				const n = 10
				errs := make(chan error, n)

				for i := 0; i < n; i++ {
					go func(i int) {
						jobID := "job" + string(rune('0'+i))
						errs <- m.Upload(ctx, jobID, "artifact",
							bytes.NewReader([]byte("data")), api.UploadOptions{})
					}(i)
				}

				for i := 0; i < n; i++ {
					if err := <-errs; err != nil {
						t.Errorf("concurrent upload %d: %v", i, err)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestManager(t)
			tt.fn(t, m)
		})
	}
}

func TestEnforceRetention(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	m, err := NewManager(ManagerConfig{
		BaseDir:              dir,
		DefaultRetentionDays: 0,
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	// Upload some artifacts.
	_ = m.Upload(ctx, "job1", "keep", bytes.NewReader([]byte("keep")), api.UploadOptions{})
	_ = m.Upload(ctx, "job1", "expire", bytes.NewReader([]byte("expire")), api.UploadOptions{})

	// Manually set the expire artifact's metadata to be expired.
	metaPath := filepath.Join(dir, "job1", "expire.meta.json")
	data, _ := os.ReadFile(metaPath)
	var record artifactRecord
	_ = json.Unmarshal(data, &record)
	record.ExpiresAt = time.Now().Add(-time.Hour)
	updated, _ := json.Marshal(record)
	_ = os.WriteFile(metaPath, updated, 0o644)

	removed, err := EnforceRetention(ctx, dir, 0)
	if err != nil {
		t.Fatalf("EnforceRetention: %v", err)
	}
	if removed != 1 {
		t.Errorf("removed = %d, want 1", removed)
	}

	// Verify the "keep" artifact is still present.
	items, _ := m.List(ctx, "job1")
	if len(items) != 1 {
		t.Fatalf("expected 1 remaining artifact, got %d", len(items))
	}
	if items[0].Name != "keep" {
		t.Errorf("remaining artifact = %q, want %q", items[0].Name, "keep")
	}
}

func TestEnforceRetentionMaxAge(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	m, err := NewManager(ManagerConfig{BaseDir: dir})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	_ = m.Upload(ctx, "job1", "old", bytes.NewReader([]byte("old")), api.UploadOptions{})

	// Backdate the artifact creation.
	metaPath := filepath.Join(dir, "job1", "old.meta.json")
	data, _ := os.ReadFile(metaPath)
	var record artifactRecord
	_ = json.Unmarshal(data, &record)
	record.CreatedAt = time.Now().Add(-48 * time.Hour)
	updated, _ := json.Marshal(record)
	_ = os.WriteFile(metaPath, updated, 0o644)

	removed, err := EnforceRetention(ctx, dir, 24*time.Hour)
	if err != nil {
		t.Fatalf("EnforceRetention: %v", err)
	}
	if removed != 1 {
		t.Errorf("removed = %d, want 1", removed)
	}
}
