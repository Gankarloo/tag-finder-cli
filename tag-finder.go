package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var version = "dev" // Overridden by -ldflags during build

// RegistryClient handles HTTP requests to Docker Registry API v2
type RegistryClient struct {
	httpClient *http.Client
	workers    int
	token      string
	tokenMutex sync.Mutex
}

// TagInfo represents the result of checking a tag
type TagInfo struct {
	Tag    string
	Digest string
	Err    error
}

type model struct {
	spinner      spinner.Model
	progress     progress.Model
	image        string
	targetDigest string
	tags         []string
	matchingTags []string
	current      int
	total        int
	done         bool
	err          error
	resultsChan  <-chan TagInfo
	workers      int
	ctx          context.Context
	cancel       context.CancelFunc
}

type tickMsg struct{}
type tagsMsg struct {
	tags []string
	err  error
}
type checkMsg struct {
	tag    string
	digest string
	err    error
}

var (
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	infoStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))
)

// parseImageReference parses an image reference and returns the registry URL and repository path
func parseImageReference(image string) (registryURL, repository string, err error) {
	parts := strings.SplitN(image, "/", 2)

	if len(parts) == 1 {
		// No registry specified, default to docker.io
		return "https://registry-1.docker.io", "library/" + parts[0], nil
	}

	registry := parts[0]
	repo := parts[1]

	switch registry {
	case "docker.io":
		// Special handling for Docker Hub
		if !strings.Contains(repo, "/") {
			repo = "library/" + repo
		}
		return "https://registry-1.docker.io", repo, nil
	case "ghcr.io":
		return "https://ghcr.io", repo, nil
	case "quay.io":
		return "https://quay.io", repo, nil
	default:
		// Generic registry
		return "https://" + registry, repo, nil
	}
}

// NewRegistryClient creates a new registry client with the specified number of workers
func NewRegistryClient(workers int) *RegistryClient {
	return &RegistryClient{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		workers: workers,
	}
}

// getBearerToken attempts to get an anonymous bearer token from the registry
func (rc *RegistryClient) getBearerToken(authHeader, repository string) (string, error) {
	// Check if we already have a token cached
	rc.tokenMutex.Lock()
	if rc.token != "" {
		token := rc.token
		rc.tokenMutex.Unlock()
		return token, nil
	}
	rc.tokenMutex.Unlock()

	// Parse WWW-Authenticate header
	// Example: Bearer realm="https://auth.docker.io/token",service="registry.docker.io",scope="repository:library/nginx:pull"
	parts := strings.Split(authHeader, " ")
	if len(parts) < 2 || parts[0] != "Bearer" {
		return "", fmt.Errorf("unsupported auth type: %s", parts[0])
	}

	params := make(map[string]string)
	for _, part := range strings.Split(parts[1], ",") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			params[kv[0]] = strings.Trim(kv[1], "\"")
		}
	}

	realm, ok := params["realm"]
	if !ok {
		return "", fmt.Errorf("no realm in auth header")
	}

	// Build token request URL
	tokenURL := realm + "?"
	if service, ok := params["service"]; ok {
		tokenURL += "service=" + service + "&"
	}
	if scope, ok := params["scope"]; ok {
		tokenURL += "scope=" + scope
	} else {
		// If no scope in header, construct it
		tokenURL += "scope=repository:" + repository + ":pull"
	}

	// Request token
	resp, err := rc.httpClient.Get(tokenURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token request failed with status %d", resp.StatusCode)
	}

	var tokenResp struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", err
	}

	token := tokenResp.Token
	if token == "" {
		token = tokenResp.AccessToken
	}

	// Cache the token
	rc.tokenMutex.Lock()
	rc.token = token
	rc.tokenMutex.Unlock()

	return token, nil
}

// parseLinkHeader parses the Link header to extract the next page URL
func parseLinkHeader(linkHeader string) string {
	// Link header format: </v2/repo/tags/list?n=100&last=tag99>; rel="next"
	parts := strings.Split(linkHeader, ";")
	if len(parts) < 2 {
		return ""
	}

	// Extract URL from angle brackets
	urlPart := strings.TrimSpace(parts[0])
	if !strings.HasPrefix(urlPart, "<") || !strings.HasSuffix(urlPart, ">") {
		return ""
	}

	// Check if this is a "next" link
	for _, part := range parts[1:] {
		if strings.Contains(part, `rel="next"`) {
			return strings.Trim(urlPart, "<>")
		}
	}

	return ""
}

// fetchTagsPage fetches a single page of tags and returns the next URL if available
func (rc *RegistryClient) fetchTagsPage(url, repository string) ([]string, string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, "", err
	}

	// Use cached token if available
	rc.tokenMutex.Lock()
	if rc.token != "" {
		req.Header.Set("Authorization", "Bearer "+rc.token)
	}
	rc.tokenMutex.Unlock()

	resp, err := rc.httpClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	// Handle 401 by getting bearer token
	if resp.StatusCode == http.StatusUnauthorized {
		authHeader := resp.Header.Get("WWW-Authenticate")
		if authHeader == "" {
			return nil, "", fmt.Errorf("registry returned 401 without WWW-Authenticate header")
		}

		token, err := rc.getBearerToken(authHeader, repository)
		if err != nil {
			return nil, "", fmt.Errorf("failed to get auth token: %v", err)
		}

		// Retry with token
		req, err = http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, "", err
		}
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err = rc.httpClient.Do(req)
		if err != nil {
			return nil, "", err
		}
		defer resp.Body.Close()
	}

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("registry returned %d", resp.StatusCode)
	}

	var result struct {
		Tags []string `json:"tags"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, "", err
	}

	// Parse Link header for next page
	linkHeader := resp.Header.Get("Link")
	nextURL := ""
	if linkHeader != "" {
		nextPath := parseLinkHeader(linkHeader)
		if nextPath != "" {
			// nextPath is relative, need to construct full URL
			// Extract base URL from current URL
			baseURL := url
			if idx := strings.Index(url, "/v2/"); idx >= 0 {
				baseURL = url[:idx]
			}
			nextURL = baseURL + nextPath
		}
	}

	return result.Tags, nextURL, nil
}

// fetchTagsList fetches all tags from the registry with pagination support
func (rc *RegistryClient) fetchTagsList(registryURL, repository string) ([]string, error) {
	var allTags []string
	url := fmt.Sprintf("%s/v2/%s/tags/list?n=1000", registryURL, repository)

	for url != "" {
		tags, nextURL, err := rc.fetchTagsPage(url, repository)
		if err != nil {
			return nil, err
		}
		allTags = append(allTags, tags...)
		url = nextURL
	}

	return allTags, nil
}

// fetchManifestDigest fetches the digest for a specific tag
func (rc *RegistryClient) fetchManifestDigest(registryURL, repository, tag string) (string, error) {
	url := fmt.Sprintf("%s/v2/%s/manifests/%s", registryURL, repository, tag)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	// Accept headers for different manifest types
	req.Header.Set("Accept", strings.Join([]string{
		"application/vnd.docker.distribution.manifest.v2+json",
		"application/vnd.docker.distribution.manifest.list.v2+json",
		"application/vnd.oci.image.manifest.v1+json",
		"application/vnd.oci.image.index.v1+json",
	}, ", "))

	// Use cached token if available
	rc.tokenMutex.Lock()
	if rc.token != "" {
		req.Header.Set("Authorization", "Bearer "+rc.token)
	}
	rc.tokenMutex.Unlock()

	resp, err := rc.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Handle 401 by getting bearer token
	if resp.StatusCode == http.StatusUnauthorized {
		authHeader := resp.Header.Get("WWW-Authenticate")
		if authHeader == "" {
			return "", fmt.Errorf("registry returned 401 without WWW-Authenticate header")
		}

		token, err := rc.getBearerToken(authHeader, repository)
		if err != nil {
			return "", fmt.Errorf("failed to get auth token: %v", err)
		}

		// Retry with token
		req, err = http.NewRequest("GET", url, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("Accept", strings.Join([]string{
			"application/vnd.docker.distribution.manifest.v2+json",
			"application/vnd.docker.distribution.manifest.list.v2+json",
			"application/vnd.oci.image.manifest.v1+json",
			"application/vnd.oci.image.index.v1+json",
		}, ", "))
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err = rc.httpClient.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("registry returned %d for tag %s", resp.StatusCode, tag)
	}

	// Digest is in the Docker-Content-Digest header
	digest := resp.Header.Get("Docker-Content-Digest")
	if digest == "" {
		return "", fmt.Errorf("no digest header for tag %s", tag)
	}

	return digest, nil
}

func initialModel(image, digest string, workers int) model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	ctx, cancel := context.WithCancel(context.Background())

	return model{
		spinner:      s,
		progress:     progress.New(progress.WithDefaultGradient()),
		image:        image,
		targetDigest: digest,
		workers:      workers,
		ctx:          ctx,
		cancel:       cancel,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, fetchTags(m.image, m.workers))
}

// FetchDigests spawns worker pool to fetch digests for all tags concurrently
func (rc *RegistryClient) FetchDigests(ctx context.Context, registryURL, repository string, tags []string, resultsChan chan<- TagInfo) {
	jobs := make(chan string, len(tags))
	var wg sync.WaitGroup

	// Start workers
	for i := 0; i < rc.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for tag := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
					digest, err := rc.fetchManifestDigest(registryURL, repository, tag)
					resultsChan <- TagInfo{Tag: tag, Digest: digest, Err: err}
				}
			}
		}()
	}

	// Send jobs
	for _, tag := range tags {
		jobs <- tag
	}
	close(jobs)

	// Wait and close results
	go func() {
		wg.Wait()
		close(resultsChan)
	}()
}

func fetchTags(image string, workers int) tea.Cmd {
	return func() tea.Msg {
		registryURL, repository, err := parseImageReference(image)
		if err != nil {
			return tagsMsg{err: err}
		}

		client := NewRegistryClient(workers)
		tags, err := client.fetchTagsList(registryURL, repository)
		if err != nil {
			return tagsMsg{err: err}
		}

		return tagsMsg{tags: tags}
	}
}

func startWorkerPool(image string, tags []string, workers int, ctx context.Context, resultsChan chan TagInfo) tea.Cmd {
	return func() tea.Msg {
		registryURL, repository, err := parseImageReference(image)
		if err != nil {
			return checkMsg{err: err}
		}

		client := NewRegistryClient(workers)

		go client.FetchDigests(ctx, registryURL, repository, tags, resultsChan)

		return waitForNextResult(resultsChan)()
	}
}

func waitForNextResult(ch <-chan TagInfo) tea.Cmd {
	return func() tea.Msg {
		info, ok := <-ch
		if !ok {
			return nil
		}
		return checkMsg{tag: info.Tag, digest: info.Digest, err: info.Err}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || msg.String() == "q" {
			if m.cancel != nil {
				m.cancel() // Cancel all workers
			}
			return m, tea.Quit
		}

	case tagsMsg:
		if msg.err != nil {
			m.err = msg.err
			m.done = true
			return m, tea.Quit
		}
		m.tags = msg.tags
		m.total = len(msg.tags)
		if m.total > 0 {
			// Start worker pool - create channel and pass to worker pool
			resultsChan := make(chan TagInfo, m.workers*2)
			m.resultsChan = resultsChan
			return m, tea.Batch(
				startWorkerPool(m.image, m.tags, m.workers, m.ctx, resultsChan),
			)
		}
		m.done = true
		return m, tea.Quit

	case checkMsg:
		if msg.err == nil && msg.digest == m.targetDigest {
			m.matchingTags = append(m.matchingTags, msg.tag)
		}
		m.current++

		if m.current >= m.total {
			m.done = true
			return m, tea.Quit
		}

		// Wait for next result from worker pool
		return m, waitForNextResult(m.resultsChan)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	default:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m model) View() string {
	if m.err != nil {
		return errorStyle.Render(fmt.Sprintf("Error: %v\n", m.err))
	}

	if m.done {
		var result strings.Builder
		result.WriteString(successStyle.Render("✓ Scan complete!"))
		result.WriteString("\n\n")

		if len(m.matchingTags) == 0 {
			result.WriteString(infoStyle.Render("No tags found matching the digest."))
			result.WriteString("\n")
		} else {
			result.WriteString(successStyle.Render(fmt.Sprintf("Found %d matching tag(s):", len(m.matchingTags))))
			result.WriteString("\n")
			for _, tag := range m.matchingTags {
				result.WriteString(fmt.Sprintf("  • %s\n", tag))
			}
		}
		return result.String()
	}

	if m.total == 0 {
		return fmt.Sprintf("%s Fetching tags...\n", m.spinner.View())
	}

	percent := float64(m.current) / float64(m.total)
	
	var s strings.Builder
	s.WriteString(fmt.Sprintf("%s Checking tags for digest match...\n\n", m.spinner.View()))
	s.WriteString(fmt.Sprintf("Progress: %d/%d tags\n", m.current, m.total))
	s.WriteString(m.progress.ViewAs(percent))
	s.WriteString("\n\n")
	
	if len(m.matchingTags) > 0 {
		s.WriteString(successStyle.Render(fmt.Sprintf("Matches found so far: %d\n", len(m.matchingTags))))
	}
	
	s.WriteString(infoStyle.Render("\nPress q or ctrl+c to quit"))
	
	return s.String()
}

func main() {
	workers := flag.Int("workers", 10, "number of concurrent HTTP requests")
	versionFlag := flag.Bool("version", false, "print version information")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("tag-finder %s\n", version)
		os.Exit(0)
	}

	args := flag.Args()
	if len(args) < 2 {
		fmt.Println("Usage: tag-finder [flags] <image> <digest>")
		fmt.Println("Example: tag-finder docker.io/library/nginx sha256:abc123...")
		fmt.Println("\nFlags:")
		flag.PrintDefaults()
		os.Exit(1)
	}

	if *workers < 1 {
		fmt.Println("Error: workers must be at least 1")
		os.Exit(1)
	}

	image := args[0]
	digest := args[1]

	// Strip docker:// prefix if provided
	image = strings.TrimPrefix(image, "docker://")

	// Ensure digest has sha256: prefix
	if !strings.HasPrefix(digest, "sha256:") {
		digest = "sha256:" + digest
	}

	p := tea.NewProgram(initialModel(image, digest, *workers))
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}