package context

import (
	"backend/model"
	"time"
)

const coreNetworkBucket = "core-networks"

// ListCoreNetworks returns the saved core networks.
func (d *dbContext) ListCoreNetworks() ([]model.CoreNetwork, error) {
	return listProfiles[model.CoreNetwork](d, coreNetworkBucket)
}

// GetCoreNetwork returns one core network, nil if there is none.
func (d *dbContext) GetCoreNetwork(id string) (*model.CoreNetwork, error) {
	return getProfile[model.CoreNetwork](d, coreNetworkBucket, id)
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
