package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Helper functions for testing

// createTestTags generates a slice of test tag names
func createTestTags(count int) []string {
	tags := make([]string, count)
	for i := 0; i < count; i++ {
		tags[i] = fmt.Sprintf("tag%d", i)
	}
	return tags
}

// Test parseImageReference function
func TestParseImageReference(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantURL  string
		wantRepo string
		wantErr  bool
	}{
		{
			name:     "simple image (no registry)",
			input:    "nginx",
			wantURL:  "https://registry-1.docker.io",
			wantRepo: "library/nginx",
			wantErr:  false,
		},
		{
			name:     "docker.io with single name",
			input:    "docker.io/nginx",
			wantURL:  "https://registry-1.docker.io",
			wantRepo: "library/nginx",
			wantErr:  false,
		},
		{
			name:     "docker.io with org/repo",
			input:    "docker.io/myorg/myrepo",
			wantURL:  "https://registry-1.docker.io",
			wantRepo: "myorg/myrepo",
			wantErr:  false,
		},
		{
			name:     "ghcr.io registry",
			input:    "ghcr.io/owner/repo",
			wantURL:  "https://ghcr.io",
			wantRepo: "owner/repo",
			wantErr:  false,
		},
		{
			name:     "quay.io registry",
			input:    "quay.io/org/repo",
			wantURL:  "https://quay.io",
			wantRepo: "org/repo",
			wantErr:  false,
		},
		{
			name:     "custom registry",
			input:    "registry.example.com/project/image",
			wantURL:  "https://registry.example.com",
			wantRepo: "project/image",
			wantErr:  false,
		},
		{
			name:     "custom registry with port",
			input:    "localhost:5000/myimage",
			wantURL:  "https://localhost:5000",
			wantRepo: "myimage",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotURL, gotRepo, err := parseImageReference(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseImageReference() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotURL != tt.wantURL {
				t.Errorf("parseImageReference() gotURL = %v, want %v", gotURL, tt.wantURL)
			}
			if gotRepo != tt.wantRepo {
				t.Errorf("parseImageReference() gotRepo = %v, want %v", gotRepo, tt.wantRepo)
			}
		})
	}
}

// Test parseLinkHeader function
func TestParseLinkHeader(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "valid next link",
			input: `</v2/repo/tags/list?n=100&last=tag99>; rel="next"`,
			want:  "/v2/repo/tags/list?n=100&last=tag99",
		},
		{
			name:  "no next rel",
			input: `</v2/repo/tags/list?n=100&last=tag99>; rel="prev"`,
			want:  "",
		},
		{
			name:  "malformed - no brackets",
			input: `/v2/repo/tags/list; rel="next"`,
			want:  "",
		},
		{
			name:  "empty header",
			input: "",
			want:  "",
		},
		{
			name:  "no semicolon",
			input: `</v2/repo/tags/list>`,
			want:  "",
		},
		{
			name:  "single link without next rel",
			input: `</v2/repo/tags/list?page=2>; rel="prev"`,
			want:  "",
		},
		{
			name:  "malformed brackets",
			input: `<incomplete; rel="next"`,
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseLinkHeader(tt.input)
			if got != tt.want {
				t.Errorf("parseLinkHeader() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Test getBearerToken function
func TestGetBearerToken(t *testing.T) {
	// Setup mock token server
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify service and scope parameters
		if r.URL.Query().Get("service") != "registry.docker.io" {
			t.Errorf("expected service=registry.docker.io, got %s", r.URL.Query().Get("service"))
		}
		scope := r.URL.Query().Get("scope")
		if scope != "repository:library/nginx:pull" {
			t.Errorf("expected scope=repository:library/nginx:pull, got %s", scope)
		}

		json.NewEncoder(w).Encode(map[string]string{
			"token": "test-token-123",
		})
	}))
	defer tokenServer.Close()

	client := NewRegistryClient(1)
	authHeader := fmt.Sprintf(`Bearer realm="%s",service="registry.docker.io",scope="repository:library/nginx:pull"`, tokenServer.URL)

	token, err := client.getBearerToken(authHeader, "library/nginx")
	if err != nil {
		t.Fatalf("getBearerToken() error = %v", err)
	}
	if token != "test-token-123" {
		t.Errorf("getBearerToken() = %v, want test-token-123", token)
	}

	// Test token caching - second call should return cached token
	token2, err := client.getBearerToken(authHeader, "library/nginx")
	if err != nil {
		t.Fatalf("getBearerToken() cached error = %v", err)
	}
	if token2 != token {
		t.Errorf("Token not cached properly")
	}
}

// Test getBearerToken with access_token response field
func TestGetBearerToken_AccessToken(t *testing.T) {
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return access_token instead of token
		json.NewEncoder(w).Encode(map[string]string{
			"access_token": "access-token-456",
		})
	}))
	defer tokenServer.Close()

	client := NewRegistryClient(1)
	authHeader := fmt.Sprintf(`Bearer realm="%s",service="test",scope="repository:test:pull"`, tokenServer.URL)

	token, err := client.getBearerToken(authHeader, "test")
	if err != nil {
		t.Fatalf("getBearerToken() error = %v", err)
	}
	if token != "access-token-456" {
		t.Errorf("getBearerToken() = %v, want access-token-456", token)
	}
}

// Test fetchTagsPage function
func TestFetchTagsPage(t *testing.T) {
	tests := []struct {
		name        string
		setupServer func() *httptest.Server
		wantTags    []string
		wantNextURL string
		wantErr     bool
	}{
		{
			name: "successful fetch without pagination",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					json.NewEncoder(w).Encode(map[string][]string{
						"tags": {"latest", "v1.0", "v2.0"},
					})
				}))
			},
			wantTags:    []string{"latest", "v1.0", "v2.0"},
			wantNextURL: "",
			wantErr:     false,
		},
		{
			name: "fetch with Link header for next page",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Link", `</v2/repo/tags/list?n=100&last=v2.0>; rel="next"`)
					json.NewEncoder(w).Encode(map[string][]string{
						"tags": {"latest", "v1.0", "v2.0"},
					})
				}))
			},
			wantTags:    []string{"latest", "v1.0", "v2.0"},
			wantNextURL: "/v2/repo/tags/list?n=100&last=v2.0",
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.setupServer()
			defer server.Close()

			client := NewRegistryClient(1)
			tags, nextURL, err := client.fetchTagsPage(server.URL+"/v2/repo/tags/list", "repo")

			if (err != nil) != tt.wantErr {
				t.Errorf("fetchTagsPage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(tags) != len(tt.wantTags) {
				t.Errorf("fetchTagsPage() got %d tags, want %d", len(tags), len(tt.wantTags))
			}
			for i, tag := range tags {
				if tag != tt.wantTags[i] {
					t.Errorf("fetchTagsPage() tag[%d] = %v, want %v", i, tag, tt.wantTags[i])
				}
			}

			// Verify next URL path matches (ignoring server base URL)
			if tt.wantNextURL != "" {
				if !strings.Contains(nextURL, tt.wantNextURL) {
					t.Errorf("fetchTagsPage() nextURL = %v, should contain %v", nextURL, tt.wantNextURL)
				}
			} else if nextURL != "" {
				t.Errorf("fetchTagsPage() nextURL = %v, want empty", nextURL)
			}
		})
	}
}

// Test fetchTagsPage with 401 authentication retry
func TestFetchTagsPage_AuthRetry(t *testing.T) {
	// Setup token server
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{
			"token": "auth-token-xyz",
		})
	}))
	defer tokenServer.Close()

	// Setup registry server that requires auth
	callCount := 0
	registryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++

		// First call: return 401
		if callCount == 1 {
			w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Bearer realm="%s",service="registry",scope="repository:test:pull"`, tokenServer.URL))
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Second call: verify token and return tags
		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer auth-token-xyz" {
			t.Errorf("Expected Bearer auth-token-xyz, got %s", authHeader)
		}
		json.NewEncoder(w).Encode(map[string][]string{
			"tags": {"tag1", "tag2"},
		})
	}))
	defer registryServer.Close()

	client := NewRegistryClient(1)
	tags, _, err := client.fetchTagsPage(registryServer.URL+"/v2/test/tags/list", "test")
	if err != nil {
		t.Fatalf("fetchTagsPage() error = %v", err)
	}

	if len(tags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(tags))
	}
	if callCount != 2 {
		t.Errorf("Expected 2 calls (401 + retry), got %d", callCount)
	}
}

// Test fetchTagsList with pagination
func TestFetchTagsList(t *testing.T) {
	callCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++

		switch callCount {
		case 1:
			// Use relative path like real Docker Registry API
			w.Header().Set("Link", `</v2/repo/tags/list?n=100&last=tag100>; rel="next"`)
			json.NewEncoder(w).Encode(map[string][]string{
				"tags": createTestTags(100),
			})
		case 2:
			w.Header().Set("Link", `</v2/repo/tags/list?n=100&last=tag200>; rel="next"`)
			json.NewEncoder(w).Encode(map[string][]string{
				"tags": createTestTags(100),
			})
		case 3:
			// Last page - no Link header
			json.NewEncoder(w).Encode(map[string][]string{
				"tags": createTestTags(50),
			})
		}
	}))
	defer server.Close()

	client := NewRegistryClient(1)
	tags, err := client.fetchTagsList(server.URL, "repo")
	if err != nil {
		t.Fatalf("fetchTagsList() error = %v", err)
	}

	if len(tags) != 250 {
		t.Errorf("fetchTagsList() returned %d tags, want 250", len(tags))
	}
	if callCount != 3 {
		t.Errorf("Expected 3 HTTP calls for pagination, got %d", callCount)
	}
}

// Test fetchManifestDigest function
func TestFetchManifestDigest(t *testing.T) {
	expectedDigest := "sha256:abcd1234"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify Accept headers
		acceptHeader := r.Header.Get("Accept")
		if !strings.Contains(acceptHeader, "application/vnd.docker.distribution.manifest.v2+json") {
			t.Errorf("Missing expected Accept header")
		}

		w.Header().Set("Docker-Content-Digest", expectedDigest)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewRegistryClient(1)
	digest, err := client.fetchManifestDigest(server.URL, "repo", "latest")
	if err != nil {
		t.Fatalf("fetchManifestDigest() error = %v", err)
	}
	if digest != expectedDigest {
		t.Errorf("fetchManifestDigest() = %v, want %v", digest, expectedDigest)
	}
}

// Test fetchManifestDigest with 401 authentication retry
func TestFetchManifestDigest_AuthRetry(t *testing.T) {
	// Setup token server
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{
			"token": "manifest-token",
		})
	}))
	defer tokenServer.Close()

	expectedDigest := "sha256:manifestdigest123"

	// Setup registry server that requires auth
	callCount := 0
	registryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++

		// First call: return 401
		if callCount == 1 {
			w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Bearer realm="%s",service="registry",scope="repository:test:pull"`, tokenServer.URL))
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Second call: verify token and return manifest digest
		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer manifest-token" {
			t.Errorf("Expected Bearer manifest-token, got %s", authHeader)
		}

		// Verify Accept headers
		acceptHeader := r.Header.Get("Accept")
		if !strings.Contains(acceptHeader, "application/vnd.docker.distribution.manifest.v2+json") {
			t.Errorf("Missing expected Accept header")
		}

		w.Header().Set("Docker-Content-Digest", expectedDigest)
		w.WriteHeader(http.StatusOK)
	}))
	defer registryServer.Close()

	client := NewRegistryClient(1)
	digest, err := client.fetchManifestDigest(registryServer.URL, "test", "v1.0")
	if err != nil {
		t.Fatalf("fetchManifestDigest() error = %v", err)
	}

	if digest != expectedDigest {
		t.Errorf("fetchManifestDigest() = %v, want %v", digest, expectedDigest)
	}
	if callCount != 2 {
		t.Errorf("Expected 2 calls (401 + retry), got %d", callCount)
	}
}

// Test FetchDigests worker pool
func TestFetchDigests(t *testing.T) {
	// Setup mock server for manifest endpoints
	digestMap := map[string]string{
		"tag1": "sha256:aaaa",
		"tag2": "sha256:bbbb",
		"tag3": "sha256:cccc",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract tag from URL path
		parts := strings.Split(r.URL.Path, "/")
		tag := parts[len(parts)-1]

		if digest, ok := digestMap[tag]; ok {
			w.Header().Set("Docker-Content-Digest", digest)
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewRegistryClient(3) // 3 workers
	ctx := context.Background()
	resultsChan := make(chan TagInfo, 10)

	tags := []string{"tag1", "tag2", "tag3"}
	go client.FetchDigests(ctx, server.URL, "repo", tags, resultsChan)

	// Collect results
	results := make(map[string]string)
	for i := 0; i < len(tags); i++ {
		info := <-resultsChan
		if info.Err != nil {
			t.Errorf("Unexpected error for tag %s: %v", info.Tag, info.Err)
		}
		results[info.Tag] = info.Digest
	}

	// Verify all tags processed
	for tag, expectedDigest := range digestMap {
		if got, ok := results[tag]; !ok {
			t.Errorf("Tag %s not processed", tag)
		} else if got != expectedDigest {
			t.Errorf("Tag %s digest = %v, want %v", tag, got, expectedDigest)
		}
	}

	// Verify channel is closed
	select {
	case _, ok := <-resultsChan:
		if ok {
			t.Error("Channel should be closed")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Channel was not closed in time")
	}
}

// Test FetchDigests with context cancellation
func TestFetchDigests_Cancellation(t *testing.T) {
	// Mock slow server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(1 * time.Second) // Simulate slow response
		w.Header().Set("Docker-Content-Digest", "sha256:test")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewRegistryClient(2)
	ctx, cancel := context.WithCancel(context.Background())
	resultsChan := make(chan TagInfo, 10)

	tags := make([]string, 100) // Many tags
	for i := range tags {
		tags[i] = fmt.Sprintf("tag%d", i)
	}

	go client.FetchDigests(ctx, server.URL, "repo", tags, resultsChan)

	// Cancel after brief period
	time.Sleep(50 * time.Millisecond)
	cancel()

	// Collect what was processed
	processed := 0
	timeout := time.After(2 * time.Second)
	for {
		select {
		case _, ok := <-resultsChan:
			if !ok {
				// Channel closed as expected
				if processed >= 100 {
					t.Error("All tags were processed despite cancellation")
				}
				return
			}
			processed++
		case <-timeout:
			t.Error("Channel was not closed after cancellation")
			return
		}
	}
}

// Test model Update with tags fetched
func TestModelUpdate_TagsFetched(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	m := model{
		workers: 10,
		ctx:     ctx,
		cancel:  cancel,
	}

	// Simulate tags fetched message
	msg := tagsMsg{
		tags: []string{"tag1", "tag2", "tag3"},
	}

	newModel, cmd := m.Update(msg)
	updatedModel := newModel.(model)

	if updatedModel.total != 3 {
		t.Errorf("Expected total=3, got %d", updatedModel.total)
	}
	if cmd == nil {
		t.Error("Expected command to start worker pool")
	}
}

// Test model Update with error in tags
func TestModelUpdate_TagsError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	m := model{
		workers: 10,
		ctx:     ctx,
		cancel:  cancel,
	}

	// Simulate error in fetching tags
	msg := tagsMsg{
		err: fmt.Errorf("failed to fetch tags"),
	}

	newModel, _ := m.Update(msg)
	updatedModel := newModel.(model)

	if !updatedModel.done {
		t.Error("Expected model.done to be true after error")
	}
	if updatedModel.err == nil {
		t.Error("Expected error to be set")
	}
}
