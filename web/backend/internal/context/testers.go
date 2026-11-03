package context

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"
)

// testersBucket holds the testers added on the Status page, keyed by a
// random ID. The tester in config.yaml (backend.tester) is not stored.
const testersBucket = "testers"

var ErrTesterNotFound = errors.New("tester not found")

// TesterRecord is one added tester: where fru-lab reaches it and the API
// token it sends.
type TesterRecord struct {
	ID       string    `json:"id"`
	URL      string    `json:"url"`
	ApiToken string    `json:"apiToken"`
	AddedAt  time.Time `json:"addedAt"`
}

// ListTesters returns the added testers, oldest first.
func (d *dbContext) ListTesters() ([]TesterRecord, error) {
	kvs, err := d.db.List(testersBucket)
	if err != nil {
		return nil, err
	}
	out := make([]TesterRecord, 0, len(kvs))
	for _, kv := range kvs {
		var r TesterRecord
		if err := json.Unmarshal(kv.Value, &r); err != nil {
			return nil, fmt.Errorf("tester %s: %w", kv.Key, err)
		}
		out = append(out, r)
	}
	slices.SortFunc(out, func(a, b TesterRecord) int { return a.AddedAt.Compare(b.AddedAt) })
	return out, nil
}

// AddTester stores a tester under a new ID.
func (d *dbContext) AddTester(url, apiToken string) (*TesterRecord, error) {
	id, err := newProfileID()
	if err != nil {
		return nil, err
	}
	r := TesterRecord{ID: id, URL: url, ApiToken: apiToken, AddedAt: time.Now().UTC()}
	raw, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	if err := d.db.Put(testersBucket, id, raw); err != nil {
		return nil, err
	}
	return &r, nil
}

// DeleteTester removes one added tester, or returns ErrTesterNotFound.
func (d *dbContext) DeleteTester(id string) error {
	if raw, err := d.db.Get(testersBucket, id); err != nil {
		return err
	} else if raw == nil {
		return ErrTesterNotFound
	}
	return d.db.Delete(testersBucket, id)
}
