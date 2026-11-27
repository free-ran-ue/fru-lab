package context

import (
	"backend/model"
	"time"
)

const coreNetworkBucket = "core-networks"

// withKind fills the kind of a core network saved before kinds existed:
// those are all external.
func withKind(n *model.CoreNetwork) {
	if n.Kind == "" {
		n.Kind = model.CoreKindExternal
	}
}

// ListCoreNetworks returns the saved core networks.
func (d *dbContext) ListCoreNetworks() ([]model.CoreNetwork, error) {
	list, err := listProfiles[model.CoreNetwork](d, coreNetworkBucket)
	for i := range list {
		withKind(&list[i])
	}
	return list, err
}

// GetCoreNetwork returns one core network, nil if there is none.
func (d *dbContext) GetCoreNetwork(id string) (*model.CoreNetwork, error) {
	n, err := getProfile[model.CoreNetwork](d, coreNetworkBucket, id)
	if n != nil {
		withKind(n)
	}
	return n, err
}

// SaveCoreNetwork stores n, giving it an ID when it has none.
func (d *dbContext) SaveCoreNetwork(n *model.CoreNetwork) error {
	if n.ID == "" {
		n.ID = newProfileID()
	}
	n.UpdatedAt = time.Now()
	return putProfile(d, coreNetworkBucket, n.ID, n)
}

// DeleteCoreNetwork removes a core network.
func (d *dbContext) DeleteCoreNetwork(id string) error {
	return d.db.Delete(coreNetworkBucket, id)
}
