package processor

import (
	"backend/model"
	"context"
)

// IpamView is the address plan with who holds what, reconciled with
// Docker, and the warnings the reconcile found.
func (p *Processor) IpamView(ctx context.Context) (*model.IpamView, *model.ErrorDetail) {
	view, err := p.FlContext.IpamView(ctx)
	if err != nil {
		return nil, internal(err)
	}
	return view, nil
}
