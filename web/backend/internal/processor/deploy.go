package processor

import (
	"backend/constant"
	flctx "backend/internal/context"
	"backend/model"
	"context"
	"net/http"
	"strings"
)

// knownFree5gcTemplates are the free5gc compose templates DeployUp accepts.
var knownFree5gcTemplates = map[string]bool{
	constant.FREE5GC_TEMPLATE_BASIC: true,
	constant.FREE5GC_TEMPLATE_ULCL:  true,
}

// DeployUp deploys target. templateVariant only matters for
// constant.DEPLOY_TARGET_FREE5GC - it picks which of free5gc's compose
// templates (basic, ulcl) to materialize; every other target
// ignores it entirely.
func (p *Processor) DeployUp(ctx context.Context, target string, templateVariant string) (*model.ResponseDeployAction, *model.ErrorDetail) {
	p.ProcLog.Debugf("Processing deploy up for target: %s (template: %s)", target, templateVariant)

	if target == constant.DEPLOY_TARGET_FREE5GC {
		if templateVariant == "" {
			templateVariant = constant.FREE5GC_TEMPLATE_BASIC
		}
		if !knownFree5gcTemplates[templateVariant] {
			return nil, &model.ErrorDetail{
				HttpStatus: http.StatusBadRequest,
				Detail:     "Unknown free5gc template " + templateVariant,
			}
		}
	}

	if detail := p.checkDeployPrerequisite(ctx, target); detail != nil {
		return nil, detail
	}

	if err := p.FlContext.Up(ctx, target, templateVariant); err != nil {
		p.ProcLog.Errorf("Failed to deploy up target %s: %v", target, err)
		return nil, &model.ErrorDetail{
			HttpStatus: http.StatusInternalServerError,
			Detail:     "Failed to deploy " + target + ", so it was taken down again: " + err.Error(),
		}
	}

	return &model.ResponseDeployAction{
		Message: "Deploy started for " + target,
	}, nil
}

func (p *Processor) DeployDown(ctx context.Context, target string) (*model.ResponseDeployAction, *model.ErrorDetail) {
	p.ProcLog.Debugf("Processing deploy down for target: %s", target)

	if detail := p.checkStopDependents(ctx, target); detail != nil {
		return nil, detail
	}

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

func (p *Processor) DeployLogs(ctx context.Context, target string, services []string) (*model.ResponseDeployLogs, *model.ErrorDetail) {
	p.ProcLog.Debugf("Processing deploy logs for target: %s (services: %v)", target, services)

	lines, err := p.FlContext.Logs(ctx, target, services)
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

// DeployUeUp deploys a UE instance from subscriber fields (the
// subscriber pages); DeployUeProfileUp deploys a UE profile's UE.
func (p *Processor) DeployUeUp(ctx context.Context, instance string, req *model.RequestDeployUe) (*model.ResponseDeployAction, *model.ErrorDetail) {
	p.ProcLog.Debugf("Processing deploy up for ue instance: %s", instance)

	return p.deployUe(ctx, instance, req.Gnb, &flctx.UeInstanceConfig{
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
	})
}

// deployUe attaches cfg to the gNB requested (or picked by its SD) and
// brings it up as instance.
func (p *Processor) deployUe(ctx context.Context, instance, requestedGnb string, cfg *flctx.UeInstanceConfig) (*model.ResponseDeployAction, *model.ErrorDetail) {
	gnbs, err := p.runningGnbs(ctx)
	if err != nil {
		p.ProcLog.Errorf("Failed to list the running gNBs before deploying ue instance %s: %v", instance, err)
		return nil, internal(err)
	}
	gnb, problem := pickUeGnb(cfg.Sd, requestedGnb, gnbs)
	if problem != "" {
		return nil, &model.ErrorDetail{HttpStatus: http.StatusConflict, Detail: problem}
	}
	cfg.Gnb, cfg.RanIp = gnb.Target, gnb.RanIp

	if err := p.FlContext.UpUe(ctx, instance, cfg); err != nil {
		p.ProcLog.Errorf("Failed to deploy up ue instance %s: %v", instance, err)
		return nil, &model.ErrorDetail{
			HttpStatus: http.StatusInternalServerError,
			Detail:     "Failed to deploy ue " + instance + ", so it was taken down again: " + err.Error(),
		}
	}

	return &model.ResponseDeployAction{
		Message: "Deploy started for ue " + instance + " on " + gnb.Target,
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
		Gnb:          p.FlContext.UeGnb(instance),
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
			Gnb:          p.FlContext.UeGnb(instance),
			Status:       aggregateServiceStatus(result.Services),
			Services:     result.Services,
			LastDeployed: result.LastDeployed,
		})
	}

	return &model.ResponseDeployUeList{
		Instances: instances,
	}, nil
}

// checkStopDependents enforces the network's actual dependency order: a UE
// dials a specific gNB, and a gNB registers against the core's AMF/UPF, so
// stopping a lower layer out from under a still-running upper layer doesn't
// just leave it stuck - it breaks the upper layer's live connection and
// crashes it. Blocking the stop up front is much clearer than letting that
// happen and leaving the operator to wonder why a UE container "disappeared"
// on its own.
func (p *Processor) checkStopDependents(ctx context.Context, target string) *model.ErrorDetail {
	switch {
	case target == constant.DEPLOY_TARGET_FREE5GC:
		running, err := p.anyLabGnbRunning(ctx)
		if err != nil {
			p.ProcLog.Warnf("Failed to check gnb status before stopping core: %v", err)
			return nil
		}
		if running {
			return &model.ErrorDetail{
				HttpStatus: http.StatusConflict,
				Detail:     "Stop every gNB on the lab core before stopping the core network",
			}
		}
	case isGnbTarget(target):
		ues, err := p.uesOnGnb(ctx, target)
		if err != nil {
			p.ProcLog.Warnf("Failed to check ue instances before stopping gnb: %v", err)
			return nil
		}
		if len(ues) > 0 {
			return &model.ErrorDetail{
				HttpStatus: http.StatusConflict,
				Detail:     "Stop the UEs on " + target + " first: " + strings.Join(ues, ", "),
			}
		}
	}

	return nil
}

// checkDeployPrerequisite enforces the same dependency chain in the opposite
// direction: deploying an upper layer before its lower layer is actually up
// just means it immediately fails to connect (a gNB with no core to
// register against, a UE with no gNB to dial) - blocking it up front avoids
// a confusing failed-deploy state that looks like a real bug.
func (p *Processor) checkDeployPrerequisite(ctx context.Context, target string) *model.ErrorDetail {
	if !isGnbTarget(target) {
		return nil
	}

	status, err := p.targetStatus(ctx, constant.DEPLOY_TARGET_FREE5GC)
	if err != nil {
		p.ProcLog.Warnf("Failed to check core status before deploying gnb: %v", err)
		return nil
	}
	if status != "running" {
		return &model.ErrorDetail{
			HttpStatus: http.StatusConflict,
			Detail:     "Deploy the core network before deploying gNB",
		}
	}

	return nil
}

func (p *Processor) targetStatus(ctx context.Context, target string) (string, error) {
	result, err := p.FlContext.Status(ctx, target)
	if err != nil {
		return "", err
	}
	return aggregateServiceStatus(result.Services), nil
}

func (p *Processor) isTargetRunning(ctx context.Context, target string) (bool, error) {
	status, err := p.targetStatus(ctx, target)
	if err != nil {
		return false, err
	}
	return status != "stopped", nil
}

// anyLabGnbRunning tells whether a gNB that registers with the lab core
// runs: a lab-core profile gNB.
func (p *Processor) anyLabGnbRunning(ctx context.Context) (bool, error) {
	gnbs, err := p.runningGnbs(ctx)
	if err != nil {
		return false, err
	}
	for _, g := range gnbs {
		if g.Core != model.GnbCoreExternal {
			return true, nil
		}
	}
	return false, nil
}

// uesOnGnb are the running UE instances that dial gNB target; a UE that
// has no recorded gNB counts for every gNB, to be safe.
func (p *Processor) uesOnGnb(ctx context.Context, target string) ([]string, error) {
	instanceIDs, err := p.FlContext.ListUeInstances()
	if err != nil {
		return nil, err
	}
	var out []string
	for _, instance := range instanceIDs {
		if g := p.FlContext.UeGnb(instance); g != "" && g != target {
			continue
		}
		result, err := p.FlContext.StatusUe(ctx, instance)
		if err != nil {
			return nil, err
		}
		if aggregateServiceStatus(result.Services) != "stopped" {
			out = append(out, instance)
		}
	}
	return out, nil
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
