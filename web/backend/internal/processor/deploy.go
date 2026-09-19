package processor

import (
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
