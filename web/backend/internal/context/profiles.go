package context

import (
	"backend/model"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

const (
	gnbProfileBucket = "gnb-profiles"
	ueProfileBucket  = "ue-profiles"
)

func newProfileID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func listProfiles[T any](d *dbContext, bucket string) ([]T, error) {
	kvs, err := d.db.List(bucket)
	if err != nil {
		return nil, err
	}
	out := make([]T, 0, len(kvs))
	for _, kv := range kvs {
		var p T
		if err := json.Unmarshal(kv.Value, &p); err != nil {
			return nil, fmt.Errorf("profile %s: %v", kv.Key, err)
		}
		out = append(out, p)
	}
	return out, nil
}

func getProfile[T any](d *dbContext, bucket, id string) (*T, error) {
	raw, err := d.db.Get(bucket, id)
	if err != nil || raw == nil {
		return nil, err
	}
	var p T
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

func putProfile(d *dbContext, bucket, id string, p any) error {
	raw, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return d.db.Put(bucket, id, raw)
}

// ListGnbProfiles returns the saved gNB profiles.
func (d *dbContext) ListGnbProfiles() ([]model.GnbProfile, error) {
	return listProfiles[model.GnbProfile](d, gnbProfileBucket)
}

// GetGnbProfile returns one gNB profile, nil if there is none.
func (d *dbContext) GetGnbProfile(id string) (*model.GnbProfile, error) {
	return getProfile[model.GnbProfile](d, gnbProfileBucket, id)
}

// SaveGnbProfile stores p, giving it an ID when it has none.
func (d *dbContext) SaveGnbProfile(p *model.GnbProfile) error {
	if p.ID == "" {
		p.ID = newProfileID()
	}
	p.UpdatedAt = time.Now()
	return putProfile(d, gnbProfileBucket, p.ID, p)
}

// DeleteGnbProfile removes a gNB profile.
func (d *dbContext) DeleteGnbProfile(id string) error {
	return d.db.Delete(gnbProfileBucket, id)
}

// ListUeProfiles returns the saved UE profiles.
func (d *dbContext) ListUeProfiles() ([]model.UeProfile, error) {
	return listProfiles[model.UeProfile](d, ueProfileBucket)
}

// GetUeProfile returns one UE profile, nil if there is none.
func (d *dbContext) GetUeProfile(id string) (*model.UeProfile, error) {
	return getProfile[model.UeProfile](d, ueProfileBucket, id)
}

// SaveUeProfile stores p, giving it an ID when it has none.
func (d *dbContext) SaveUeProfile(p *model.UeProfile) error {
	if p.ID == "" {
		p.ID = newProfileID()
	}
	p.UpdatedAt = time.Now()
	return putProfile(d, ueProfileBucket, p.ID, p)
}

// DeleteUeProfile removes a UE profile.
func (d *dbContext) DeleteUeProfile(id string) error {
	return d.db.Delete(ueProfileBucket, id)
}
