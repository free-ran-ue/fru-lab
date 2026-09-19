package processor

import (
	flctx "backend/internal/context"
	"backend/model"
	"context"
	"net/http"
)

func (p *Processor) DeployUp(ctx context.Context, target string) (*model.ResponseDeployAction, *model.ErrorDetail) {
	p.ProcLog.Debugf("Processing deploy up for target: %s", target)

	if err := p.FlContext.Up(ctx, target); err != nil {
		p.ProcLog.Errorf("Failed to deploy up target %s: %v", target, err)
		return nil, &model.ErrorDetail{
			HttpStatus: http.StatusInternalServerError,
			Detail:     "Failed to deploy " + target,
		}
	}

	return &model.ResponseDeployAction{
		Message: "Deploy started for " + target,
	}, nil
}

func (p *Processor) DeployDown(ctx context.Context, target string) (*model.ResponseDeployAction, *model.ErrorDetail) {
	p.ProcLog.Debugf("Processing deploy down for target: %s", target)

	if err := p.FlContext.Down(ctx, target); err != nil {
		p.ProcLog.Errorf("Failed to deploy down target %s: %v", target, err)
		return nil, &model.ErrorDetail{
			HttpStatus: http.StatusInternalServerError,
			Detail:     "Failed to stop " + target,
		}
	}

	return &model.ResponseDeployAction{
		Message: "Stopped " + target,
	}, nil
}

func (p *Processor) DeployStatus(ctx context.Context, target string) (*model.ResponseDeployStatus, *model.ErrorDetail) {
	p.ProcLog.Debugf("Processing deploy status for target: %s", target)

	result, err := p.FlContext.Status(ctx, target)
	if err != nil {
		p.ProcLog.Errorf("Failed to get status for target %s: %v", target, err)
		return nil, &model.ErrorDetail{
			HttpStatus: http.StatusInternalServerError,
			Detail:     "Failed to get status for " + target,
		}
	}

	return &model.ResponseDeployStatus{
		Target:       target,
		Status:       aggregateServiceStatus(result.Services),
		Services:     result.Services,
		LastDeployed: result.LastDeployed,
	}, nil
}

func (p *Processor) DeployLogs(ctx context.Context, target string) (*model.ResponseDeployLogs, *model.ErrorDetail) {
	p.ProcLog.Debugf("Processing deploy logs for target: %s", target)

	lines, err := p.FlContext.Logs(ctx, target)
	if err != nil {
		p.ProcLog.Errorf("Failed to get logs for target %s: %v", target, err)
		return nil, &model.ErrorDetail{
			HttpStatus: http.StatusInternalServerError,
			Detail:     "Failed to get logs for " + target,
		}
	}

	return &model.ResponseDeployLogs{
		Target: target,
		Lines:  lines,
	}, nil
}

func (p *Processor) DeployUeUp(ctx context.Context, instance string, req *model.RequestDeployUe) (*model.ResponseDeployAction, *model.ErrorDetail) {
	p.ProcLog.Debugf("Processing deploy up for ue instance: %s", instance)

	cfg := &flctx.UeInstanceConfig{
		Mcc:          req.Mcc,
		Mnc:          req.Mnc,
		Msin:         req.Msin,
		PermanentKey: req.PermanentKey,
		OpValue:      req.OpValue,
		Amf:          req.Amf,
		Sqn:          req.Sqn,
		Dnn:          req.Dnn,
		Sst:          req.Sst,
		Sd:           req.Sd,
	}

	if err := p.FlContext.UpUe(ctx, instance, cfg); err != nil {
		p.ProcLog.Errorf("Failed to deploy up ue instance %s: %v", instance, err)
		return nil, &model.ErrorDetail{
			HttpStatus: http.StatusInternalServerError,
			Detail:     "Failed to deploy ue " + instance,
		}
	}

	return &model.ResponseDeployAction{
		Message: "Deploy started for ue " + instance,
	}, nil
}

func (p *Processor) DeployUeDown(ctx context.Context, instance string) (*model.ResponseDeployAction, *model.ErrorDetail) {
	p.ProcLog.Debugf("Processing deploy down for ue instance: %s", instance)

	if err := p.FlContext.DownUe(ctx, instance); err != nil {
		p.ProcLog.Errorf("Failed to deploy down ue instance %s: %v", instance, err)
		return nil, &model.ErrorDetail{
			HttpStatus: http.StatusInternalServerError,
			Detail:     "Failed to stop ue " + instance,
		}
	}

	return &model.ResponseDeployAction{
		Message: "Stopped ue " + instance,
	}, nil
}

func (p *Processor) DeployUeStatus(ctx context.Context, instance string) (*model.ResponseDeployUeStatus, *model.ErrorDetail) {
	p.ProcLog.Debugf("Processing deploy status for ue instance: %s", instance)

	result, err := p.FlContext.StatusUe(ctx, instance)
	if err != nil {
		p.ProcLog.Errorf("Failed to get status for ue instance %s: %v", instance, err)
		return nil, &model.ErrorDetail{
			HttpStatus: http.StatusInternalServerError,
			Detail:     "Failed to get status for ue " + instance,
		}
	}

	return &model.ResponseDeployUeStatus{
		Instance:     instance,
		Status:       aggregateServiceStatus(result.Services),
		Services:     result.Services,
		LastDeployed: result.LastDeployed,
	}, nil
}

func (p *Processor) DeployUeLogs(ctx context.Context, instance string) (*model.ResponseDeployUeLogs, *model.ErrorDetail) {
	p.ProcLog.Debugf("Processing deploy logs for ue instance: %s", instance)

	lines, err := p.FlContext.LogsUe(ctx, instance)
	if err != nil {
		p.ProcLog.Errorf("Failed to get logs for ue instance %s: %v", instance, err)
		return nil, &model.ErrorDetail{
			HttpStatus: http.StatusInternalServerError,
			Detail:     "Failed to get logs for ue " + instance,
		}
	}

	return &model.ResponseDeployUeLogs{
		Instance: instance,
		Lines:    lines,
	}, nil
}

func (p *Processor) DeployUeList(ctx context.Context) (*model.ResponseDeployUeList, *model.ErrorDetail) {
	p.ProcLog.Debugf("Processing deploy list for ue instances")

	instanceIDs, err := p.FlContext.ListUeInstances()
	if err != nil {
		p.ProcLog.Errorf("Failed to list ue instances: %v", err)
		return nil, &model.ErrorDetail{
			HttpStatus: http.StatusInternalServerError,
			Detail:     "Failed to list ue instances",
		}
	}

	instances := make([]model.ResponseDeployUeStatus, 0, len(instanceIDs))
	for _, instance := range instanceIDs {
		result, err := p.FlContext.StatusUe(ctx, instance)
		if err != nil {
			p.ProcLog.Warnf("Failed to get status for ue instance %s: %v", instance, err)
			continue
		}

		instances = append(instances, model.ResponseDeployUeStatus{
			Instance:     instance,
			Status:       aggregateServiceStatus(result.Services),
			Services:     result.Services,
			LastDeployed: result.LastDeployed,
		})
	}

	return &model.ResponseDeployUeList{
		Instances: instances,
	}, nil
}

func aggregateServiceStatus(services []model.ServiceStatus) string {
	if len(services) == 0 {
		return "stopped"
	}

	hasUnhealthy := false
	hasRunning := false
	hasDeploying := false
	for _, service := range services {
		switch service.Status {
		case "unhealthy":
			hasUnhealthy = true
		case "running":
			hasRunning = true
		case "deploying":
			hasDeploying = true
		}
	}

	switch {
	case hasUnhealthy:
		return "unhealthy"
	case hasDeploying:
		return "deploying"
	case hasRunning:
		return "running"
	default:
		return "stopped"
	}
}
