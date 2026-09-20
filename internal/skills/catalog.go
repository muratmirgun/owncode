package skills

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Result is an installable catalog entry. Installation still fetches its Git source.
type Result struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Slug       string `json:"slug"`
	Source     string `json:"source"`
	InstallURL string `json:"installUrl"`
	Installs   int    `json:"installs"`
}

// Search queries the documented skills.sh API. A token is optional but the server may require it.
func Search(ctx context.Context, endpoint, token, query string) ([]Result, error) {
	if len(strings.TrimSpace(query)) < 2 {
		return nil, fmt.Errorf("enter at least two characters")
	}
	if endpoint == "" {
		endpoint = "https://skills.sh"
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/api/v1/skills/search"
	u.RawQuery = url.Values{"q": {query}, "limit": {"30"}}.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	client := &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = response.Body.Close() }() // Best-effort cleanup; preserve the primary result.
	if response.StatusCode == 401 {
		return nil, fmt.Errorf("skills.sh requires catalog authentication; set OWNCODE_SKILLS_TOKEN or install a Git source")
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("catalog returned HTTP %d; Git installation remains available", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	var result struct {
		Data []Result `json:"data"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("invalid catalog response: %w", err)
	}
	return result.Data, nil
}

type cachedCatalog struct {
	when    time.Time
	results []Result
}

var catalogCache = struct {
	sync.Mutex
	entries map[[32]byte]cachedCatalog
}{entries: map[[32]byte]cachedCatalog{}}

// SearchCached returns metadata for fifteen minutes and reports stale fallback data.
func SearchCached(ctx context.Context, endpoint, token, query string) ([]Result, bool, error) {
	key := sha256.Sum256([]byte(endpoint + "\x00" + token + "\x00" + query))
	catalogCache.Lock()
	cached, ok := catalogCache.entries[key]
	catalogCache.Unlock()
	if ok && time.Since(cached.when) < 15*time.Minute {
		return append([]Result(nil), cached.results...), false, nil
	}
	results, err := Search(ctx, endpoint, token, query)
	if err != nil {
		if ok && ctx.Err() == nil {
			return append([]Result(nil), cached.results...), true, nil
		}
		return nil, false, err
	}
	catalogCache.Lock()
	if len(catalogCache.entries) >= 32 {
		clear(catalogCache.entries)
	}
	catalogCache.entries[key] = cachedCatalog{when: time.Now(), results: append([]Result(nil), results...)}
	catalogCache.Unlock()
	return results, false, nil
}
