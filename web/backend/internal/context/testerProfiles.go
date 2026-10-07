package context

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

// Throughput Tester profiles live in testerProfilesBucket, keyed by a
// random ID so a profile can be renamed. fru-lab stores each profile
// opaquely (fru-tester owns its schema and validation) and only reads its
// "name", which must be unique, ignoring case and surrounding spaces.
const (
	testerProfilesBucket = "tester-profiles"

	// the single profile older versions kept; moved into the bucket on
	// first use
	legacyTesterBucket     = "tester"
	legacyTesterProfileKey = "profile"
)

var (
	ErrProfileNotFound    = errors.New("profile not found")
	ErrProfileNameMissing = errors.New("profile name is empty")
	ErrProfileNameTaken   = errors.New("profile name is already used")
)

// TesterProfile is one stored profile.
type TesterProfile struct {
	ID        string          `json:"id"`
	UpdatedAt time.Time       `json:"updatedAt"`
	Profile   json.RawMessage `json:"profile"`
}

// ProfileName returns the trimmed "name" of a profile JSON object.
func ProfileName(profile []byte) string {
	var p struct {
		Name string `json:"name"`
	}
	_ = json.Unmarshal(profile, &p)
	return strings.TrimSpace(p.Name)
}

func sameName(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

// ListTesterProfiles returns every stored profile, by name.
func (d *dbContext) ListTesterProfiles() ([]TesterProfile, error) {
	d.profileMu.Lock()
	defer d.profileMu.Unlock()
	if err := d.migrateLegacyProfile(); err != nil {
		return nil, err
	}
	return d.listProfiles()
}

func (d *dbContext) listProfiles() ([]TesterProfile, error) {
	kvs, err := d.db.List(testerProfilesBucket)
	if err != nil {
		return nil, err
	}
	out := make([]TesterProfile, 0, len(kvs))
	for _, kv := range kvs {
		var p TesterProfile
		if err := json.Unmarshal(kv.Value, &p); err != nil {
			return nil, fmt.Errorf("profile %s: %w", kv.Key, err)
		}
		out = append(out, p)
	}
	slices.SortFunc(out, func(a, b TesterProfile) int {
		return strings.Compare(strings.ToLower(ProfileName(a.Profile)), strings.ToLower(ProfileName(b.Profile)))
	})
	return out, nil
}

// GetTesterProfile returns one profile, or ErrProfileNotFound.
func (d *dbContext) GetTesterProfile(id string) (*TesterProfile, error) {
	raw, err := d.db.Get(testerProfilesBucket, id)
	if err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, ErrProfileNotFound
	}
	var p TesterProfile
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// CreateTesterProfile stores profile under a new ID.
func (d *dbContext) CreateTesterProfile(profile []byte) (*TesterProfile, error) {
	d.profileMu.Lock()
	defer d.profileMu.Unlock()
	if err := d.migrateLegacyProfile(); err != nil {
		return nil, err
	}
	id, err := newProfileID()
	if err != nil {
		return nil, err
	}
	return d.putProfile(id, profile)
}

// UpdateTesterProfile replaces the profile stored under id.
func (d *dbContext) UpdateTesterProfile(id string, profile []byte) (*TesterProfile, error) {
	d.profileMu.Lock()
	defer d.profileMu.Unlock()
	if raw, err := d.db.Get(testerProfilesBucket, id); err != nil {
		return nil, err
	} else if raw == nil {
		return nil, ErrProfileNotFound
	}
	return d.putProfile(id, profile)
}

// putProfile writes profile under id if its name is set and no other
// profile has it. The caller holds profileMu.
func (d *dbContext) putProfile(id string, profile []byte) (*TesterProfile, error) {
	name := ProfileName(profile)
	if name == "" {
		return nil, ErrProfileNameMissing
	}
	all, err := d.listProfiles()
	if err != nil {
		return nil, err
	}
	for _, p := range all {
		if p.ID != id && sameName(ProfileName(p.Profile), name) {
			return nil, ErrProfileNameTaken
		}
	}
	p := TesterProfile{ID: id, UpdatedAt: time.Now().UTC(), Profile: profile}
	raw, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	if err := d.db.Put(testerProfilesBucket, id, raw); err != nil {
		return nil, err
	}
	return &p, nil
}

// DeleteTesterProfile removes one profile, or returns ErrProfileNotFound.
func (d *dbContext) DeleteTesterProfile(id string) error {
	d.profileMu.Lock()
	defer d.profileMu.Unlock()
	if raw, err := d.db.Get(testerProfilesBucket, id); err != nil {
		return err
	} else if raw == nil {
		return ErrProfileNotFound
	}
	return d.db.Delete(testerProfilesBucket, id)
}

// migrateLegacyProfile moves the single profile older versions saved
// into the bucket, renaming it if its name is empty or taken. The caller
// holds profileMu.
func (d *dbContext) migrateLegacyProfile() error {
	raw, err := d.db.Get(legacyTesterBucket, legacyTesterProfileKey)
	if err != nil || raw == nil {
		return err
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		// not a profile; drop it rather than fail every request
		return d.db.Delete(legacyTesterBucket, legacyTesterProfileKey)
	}
	all, err := d.listProfiles()
	if err != nil {
		return err
	}
	base := ProfileName(raw)
	if base == "" {
		base = "basic"
	}
	name := base
	for i := 2; slices.ContainsFunc(all, func(p TesterProfile) bool { return sameName(ProfileName(p.Profile), name) }); i++ {
		name = fmt.Sprintf("%s (%d)", base, i)
	}
	obj["name"] = name
	profile, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	id, err := newProfileID()
	if err != nil {
		return err
	}
	if _, err := d.putProfile(id, profile); err != nil {
		return err
	}
	return d.db.Delete(legacyTesterBucket, legacyTesterProfileKey)
}

func newProfileID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
