package secret

import (
	"bytes"
	"encoding/base64"
	"net/url"
	"testing"
)

func TestMasker(t *testing.T) {
	tests := []struct {
		name    string
		secrets []string
		input   string
		want    string
	}{
		{
			name:    "no secrets registered",
			secrets: nil,
			input:   "hello world\n",
			want:    "hello world\n",
		},
		{
			name:    "exact match",
			secrets: []string{"supersecret"},
			input:   "token is supersecret here\n",
			want:    "token is *** here\n",
		},
		{
			name:    "multiple occurrences",
			secrets: []string{"abc"},
			input:   "abc and abc again\n",
			want:    "*** and *** again\n",
		},
		{
			name:    "multiple secrets",
			secrets: []string{"secret1", "secret2"},
			input:   "secret1 and secret2\n",
			want:    "*** and ***\n",
		},
		{
			name:    "base64 encoded variant",
			secrets: []string{"mysecret"},
			input:   base64.StdEncoding.EncodeToString([]byte("mysecret")) + "\n",
			want:    "***\n",
		},
		{
			name:    "url encoded variant",
			secrets: []string{"my secret&value"},
			input:   url.QueryEscape("my secret&value") + "\n",
			want:    "***\n",
		},
		{
			name:    "secret in middle of line",
			secrets: []string{"password123"},
			input:   "connecting with password123 to server\n",
			want:    "connecting with *** to server\n",
		},
		{
			name:    "empty secret ignored",
			secrets: []string{""},
			input:   "no change\n",
			want:    "no change\n",
		},
		{
			name:    "multiline input",
			secrets: []string{"tok"},
			input:   "line1 tok\nline2 tok\n",
			want:    "line1 ***\nline2 ***\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			m := NewMasker(&buf)
			for _, s := range tt.secrets {
				m.AddSecret(s)
			}

			_, err := m.Write([]byte(tt.input))
			if err != nil {
				t.Fatalf("Write() error: %v", err)
			}
			if err := m.Flush(); err != nil {
				t.Fatalf("Flush() error: %v", err)
			}

			got := buf.String()
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMaskerConcurrent(t *testing.T) {
	var buf bytes.Buffer
	m := NewMasker(&buf)
	m.AddSecret("secret")

	done := make(chan struct{})
	// Concurrent adds.
	go func() {
		for i := 0; i < 100; i++ {
			m.AddSecret("another")
		}
		close(done)
	}()

	// Concurrent mask operations.
	for i := 0; i < 100; i++ {
		_ = m.MaskString("some secret data")
	}
	<-done
}

func TestMaskString(t *testing.T) {
	m := NewMasker(nil)
	m.AddSecret("token123")

	got := m.MaskString("my token123 is here")
	want := "my *** is here"
	if got != want {
		t.Errorf("MaskString() = %q, want %q", got, want)
	}
}

func BenchmarkMasker(b *testing.B) {
	var buf bytes.Buffer
	m := NewMasker(&buf)
	m.AddSecret("secret1")
	m.AddSecret("secret2")
	m.AddSecret("secret3")

	input := []byte("this line contains secret1 and maybe secret2 but not the third\n")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		_, _ = m.Write(input)
		_ = m.Flush()
	}
}
