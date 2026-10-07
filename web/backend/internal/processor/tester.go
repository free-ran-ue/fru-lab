package processor

import (
	"backend/internal/context"
	"backend/model"
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
)

// maxTesterProfileBytes caps what a client can make us store.
const maxTesterProfileBytes = 64 << 10

// TesterProfileList returns every saved profile, by name.
func (p *Processor) TesterProfileList() ([]context.TesterProfile, *model.ErrorDetail) {
	list, err := p.FlContext.ListTesterProfiles()
	if err != nil {
		p.ProcLog.Errorf("Failed to list tester profiles: %v", err)
		return nil, &model.ErrorDetail{HttpStatus: http.StatusInternalServerError, Detail: "Failed to read the saved profiles"}
	}
	return list, nil
}

// TesterProfileGet returns one saved profile.
func (p *Processor) TesterProfileGet(id string) (*context.TesterProfile, *model.ErrorDetail) {
	profile, err := p.FlContext.GetTesterProfile(id)
	if err != nil {
		return nil, p.profileError("read", err)
	}
	return profile, nil
}

// TesterProfileCreate stores raw as a new profile.
func (p *Processor) TesterProfileCreate(raw []byte) (*context.TesterProfile, *model.ErrorDetail) {
	compact, errDetail := compactProfile(raw)
	if errDetail != nil {
		return nil, errDetail
	}
	profile, err := p.FlContext.CreateTesterProfile(compact)
	if err != nil {
		return nil, p.profileError("save", err)
	}
	return profile, nil
}

// TesterProfileUpdate replaces the profile stored under id with raw.
func (p *Processor) TesterProfileUpdate(id string, raw []byte) (*context.TesterProfile, *model.ErrorDetail) {
	compact, errDetail := compactProfile(raw)
	if errDetail != nil {
		return nil, errDetail
	}
	profile, err := p.FlContext.UpdateTesterProfile(id, compact)
	if err != nil {
		return nil, p.profileError("save", err)
	}
	return profile, nil
}

// TesterProfileDelete removes one saved profile.
func (p *Processor) TesterProfileDelete(id string) (*model.ResponseTesterAction, *model.ErrorDetail) {
	if err := p.FlContext.DeleteTesterProfile(id); err != nil {
		return nil, p.profileError("delete", err)
	}
	return &model.ResponseTesterAction{Message: "Profile deleted"}, nil
}

// compactProfile checks raw is one JSON object of at most 64 KiB.
// Field-level validation belongs to fru-tester (/api/tester/profile/validate).
func compactProfile(raw []byte) ([]byte, *model.ErrorDetail) {
	if len(raw) > maxTesterProfileBytes {
		return nil, &model.ErrorDetail{HttpStatus: http.StatusRequestEntityTooLarge, Detail: "Profile is larger than 64 KiB"}
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, &model.ErrorDetail{HttpStatus: http.StatusBadRequest, Detail: "Profile must be a JSON object: " + err.Error()}
	}
	var compact bytes.Buffer
	_ = json.Compact(&compact, raw)
	return compact.Bytes(), nil
}

func (p *Processor) profileError(action string, err error) *model.ErrorDetail {
	switch {
	case errors.Is(err, context.ErrProfileNotFound):
		return &model.ErrorDetail{HttpStatus: http.StatusNotFound, Detail: "No such profile"}
	case errors.Is(err, context.ErrProfileNameMissing):
		return &model.ErrorDetail{HttpStatus: http.StatusBadRequest, Detail: "Profile name is required"}
	case errors.Is(err, context.ErrProfileNameTaken):
		return &model.ErrorDetail{HttpStatus: http.StatusConflict, Detail: "Another profile already has this name"}
	}
	p.ProcLog.Errorf("Failed to %s tester profile: %v", action, err)
	return &model.ErrorDetail{HttpStatus: http.StatusInternalServerError, Detail: "Failed to " + action + " the profile"}
}
