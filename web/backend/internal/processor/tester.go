package processor

import (
	"backend/model"
	"bytes"
	"encoding/json"
	"net/http"
)

// maxTesterProfileBytes caps what a client can make us store.
const maxTesterProfileBytes = 64 << 10

// TesterProfileGet returns the saved profile JSON, or nil if none exists.
func (p *Processor) TesterProfileGet() ([]byte, *model.ErrorDetail) {
	raw, err := p.FlContext.GetTesterProfile()
	if err != nil {
		p.ProcLog.Errorf("Failed to read tester profile: %v", err)
		return nil, &model.ErrorDetail{HttpStatus: http.StatusInternalServerError, Detail: "Failed to read the saved profile"}
	}
	return raw, nil
}

// TesterProfilePut stores raw as-is after checking it is one JSON object.
// Field-level validation belongs to fru-tester (/api/tester/profile/validate).
func (p *Processor) TesterProfilePut(raw []byte) (*model.ResponseTesterAction, *model.ErrorDetail) {
	if len(raw) > maxTesterProfileBytes {
		return nil, &model.ErrorDetail{HttpStatus: http.StatusRequestEntityTooLarge, Detail: "Profile is larger than 64 KiB"}
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, &model.ErrorDetail{HttpStatus: http.StatusBadRequest, Detail: "Profile must be a JSON object: " + err.Error()}
	}
	var compact bytes.Buffer
	_ = json.Compact(&compact, raw)
	if err := p.FlContext.PutTesterProfile(compact.Bytes()); err != nil {
		p.ProcLog.Errorf("Failed to save tester profile: %v", err)
		return nil, &model.ErrorDetail{HttpStatus: http.StatusInternalServerError, Detail: "Failed to save the profile"}
	}
	return &model.ResponseTesterAction{Message: "Profile saved"}, nil
}
