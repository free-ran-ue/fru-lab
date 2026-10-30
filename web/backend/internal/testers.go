package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	flctx "backend/internal/context"
)

// defaultTesterID is the tester in config.yaml (backend.tester): it runs
// next to fru-lab, the other tester pages use it, and it cannot be
// removed from the Status page.
const defaultTesterID = "default"

// testerStore is the slice of fru-lab's DB that keeps the added testers.
type testerStore interface {
	ListTesters() ([]flctx.TesterRecord, error)
	AddTester(url, apiToken string) (*flctx.TesterRecord, error)
	DeleteTester(id string) error
}

// testerTarget is one tester fru-lab can reach.
type testerTarget struct {
	id      string
	url     string
	builtIn bool
	proxy   *httputil.ReverseProxy
}

// TesterEntry is a tester as the Status page lists it. Its name comes
// from the tester's own /status.
type TesterEntry struct {
	ID      string `json:"id"`
	URL     string `json:"url"`
	BuiltIn bool   `json:"builtIn"`
}

// TesterAddRequest adds a tester by where fru-lab reaches it.
type TesterAddRequest struct {
	URL      string `json:"url"`
	ApiToken string `json:"apiToken"`
}

// TesterAdded is a tester just added, with the name it reported.
type TesterAdded struct {
	TesterEntry
	Name string `json:"name"`
}

var (
	errTesterNotFound = errors.New("no such tester")
	errTesterBuiltIn  = errors.New("the tester in config.yaml (backend.tester) cannot be removed here")
)

// testerRegistry is every tester: the one in config.yaml, then the ones
// added on the Status page, in the order they were added.
type testerRegistry struct {
	mu      sync.RWMutex
	targets []*testerTarget
	store   testerStore
	client  *http.Client
}

// newTesterRegistry loads the testers. defaultURL may be empty: no tester
// in config.yaml.
func newTesterRegistry(defaultURL, defaultToken string, store testerStore) (*testerRegistry, error) {
	r := &testerRegistry{store: store, client: &http.Client{Timeout: 5 * time.Second}}
	if defaultURL != "" {
		t, err := newTesterTarget(defaultTesterID, defaultURL, defaultToken, true)
		if err != nil {
			return nil, err
		}
		r.targets = append(r.targets, t)
	}
	records, err := store.ListTesters()
	if err != nil {
		return nil, fmt.Errorf("read the added testers: %w", err)
	}
	for _, rec := range records {
		t, err := newTesterTarget(rec.ID, rec.URL, rec.ApiToken, false)
		if err != nil {
			return nil, fmt.Errorf("added tester %s: %w", rec.ID, err)
		}
		r.targets = append(r.targets, t)
	}
	return r, nil
}

func newTesterTarget(id, testerURL, apiToken string, builtIn bool) (*testerTarget, error) {
	proxy, err := newTesterProxyAt(testerURL, apiToken, "/api/testers/"+id)
	if err != nil {
		return nil, err
	}
	return &testerTarget{id: id, url: testerURL, builtIn: builtIn, proxy: proxy}, nil
}

func (r *testerRegistry) list() []TesterEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]TesterEntry, 0, len(r.targets))
	for _, t := range r.targets {
		out = append(out, TesterEntry{ID: t.id, URL: t.url, BuiltIn: t.builtIn})
	}
	return out
}

func (r *testerRegistry) get(id string) *testerTarget {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, t := range r.targets {
		if t.id == id {
			return t
		}
	}
	return nil
}

// add checks that a fru-tester answers at testerURL with apiToken, then
// keeps it. The error says what to fix.
func (r *testerRegistry) add(ctx context.Context, testerURL, apiToken string) (*TesterAdded, error) {
	testerURL = strings.TrimRight(strings.TrimSpace(testerURL), "/")
	apiToken = strings.TrimSpace(apiToken)
	u, err := url.Parse(testerURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("URL %q must be like http://10.0.0.2:9100", testerURL)
	}
	if apiToken == "" {
		return nil, errors.New("the API token is empty: use the apiToken in that tester's tester.yaml")
	}
	r.mu.RLock()
	for _, t := range r.targets {
		if strings.EqualFold(t.url, testerURL) {
			r.mu.RUnlock()
			return nil, fmt.Errorf("%s is already on the list", testerURL)
		}
	}
	r.mu.RUnlock()

	name, err := r.probe(ctx, testerURL, apiToken)
	if err != nil {
		return nil, err
	}
	rec, err := r.store.AddTester(testerURL, apiToken)
	if err != nil {
		return nil, fmt.Errorf("save the tester: %w", err)
	}
	t, err := newTesterTarget(rec.ID, rec.URL, rec.ApiToken, false)
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	r.targets = append(r.targets, t)
	r.mu.Unlock()
	return &TesterAdded{TesterEntry: TesterEntry{ID: t.id, URL: t.url}, Name: name}, nil
}

// probe asks the tester for its status and returns its name.
func (r *testerRegistry) probe(ctx context.Context, testerURL, apiToken string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, testerURL+"/api/status", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+apiToken)
	resp, err := r.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("no fru-tester answered at %s: %v", testerURL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return "", errors.New("that tester rejected the API token: use the apiToken in its tester.yaml")
	case resp.StatusCode == http.StatusNotFound:
		return "", fmt.Errorf("%s answered but has no /api/status: is it a fru-tester, and a recent one?", testerURL)
	case resp.StatusCode != http.StatusOK:
		return "", fmt.Errorf("%s answered %s", testerURL, resp.Status)
	}
	var s struct {
		Name     string `json:"name"`
		Hostname string `json:"hostname"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&s); err != nil {
		return "", fmt.Errorf("%s did not answer like a fru-tester: %v", testerURL, err)
	}
	if s.Name == "" {
		return s.Hostname, nil
	}
	return s.Name, nil
}

func (r *testerRegistry) remove(id string) error {
	t := r.get(id)
	if t == nil {
		return errTesterNotFound
	}
	if t.builtIn {
		return errTesterBuiltIn
	}
	if err := r.store.DeleteTester(id); err != nil && !errors.Is(err, flctx.ErrTesterNotFound) {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, x := range r.targets {
		if x.id == id {
			r.targets = append(r.targets[:i], r.targets[i+1:]...)
			break
		}
	}
	return nil
}
