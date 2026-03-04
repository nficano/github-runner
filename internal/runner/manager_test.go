package runner

import (
	"testing"
)

func TestParseGitHubURL(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		wantOwner string
		wantRepo  string
	}{
		{
			name:      "full repo URL",
			url:       "https://github.com/nficano/github-runner",
			wantOwner: "nficano",
			wantRepo:  "github-runner",
		},
		{
			name:      "org-level URL",
			url:       "https://github.com/myorg",
			wantOwner: "myorg",
			wantRepo:  "",
		},
		{
			name:      "URL with trailing slash",
			url:       "https://github.com/nficano/github-runner/",
			wantOwner: "nficano",
			wantRepo:  "github-runner",
		},
		{
			name:      "enterprise URL",
			url:       "https://github.example.com/team/project",
			wantOwner: "team",
			wantRepo:  "project",
		},
		{
			name:      "empty URL",
			url:       "",
			wantOwner: "",
			wantRepo:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, repo := parseGitHubURL(tt.url)
			if owner != tt.wantOwner {
				t.Errorf("owner = %q, want %q", owner, tt.wantOwner)
			}
			if repo != tt.wantRepo {
				t.Errorf("repo = %q, want %q", repo, tt.wantRepo)
			}
		})
	}
}
