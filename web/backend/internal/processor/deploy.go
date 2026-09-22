package processor

import (
	"backend/constant"
	flctx "backend/internal/context"
	"backend/model"
	"context"
	"net/http"
)

// knownFree5gcTemplates are the free5gc compose templates DeployUp accepts.
var knownFree5gcTemplates = map[string]bool{
	constant.FREE5GC_TEMPLATE_BASIC:       true,
	constant.FREE5GC_TEMPLATE_ULCL:        true,
	constant.FREE5GC_TEMPLATE_ULCL_2SLICE: true,
}

// DeployUp deploys target. templateVariant only matters for
// constant.DEPLOY_TARGET_FREE5GC - it picks which of free5gc's compose
// templates (basic, ulcl, ulcl-2slice) to materialize; every other target
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
			Detail:     "Failed to deploy " + target,
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

func (p *Processor) DeployUeUp(ctx context.Context, instance string, req *model.RequestDeployUe) (*model.ResponseDeployAction, *model.ErrorDetail) {
	p.ProcLog.Debugf("Processing deploy up for ue instance: %s", instance)

	gnbTarget, err := p.resolveUeGnbTarget(ctx, req.Sd)
	if err != nil {
		p.ProcLog.Warnf("Failed to resolve gnb target before deploying ue instance %s: %v", instance, err)
		gnbTarget = constant.DEPLOY_TARGET_GNB // fail open, best-effort fallback
	} else if gnbTarget == "" {
		return nil, &model.ErrorDetail{
			HttpStatus: http.StatusConflict,
			Detail:     "Deploy gNB before deploying a UE instance",
		}
	}

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
		RanIp:        gnbRanIp(gnbTarget),
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

// checkStopDependents enforces the network's actual dependency order: a UE
// dials a specific gNB, and a gNB registers against the core's AMF/UPF, so
// stopping a lower layer out from under a still-running upper layer doesn't
// just leave it stuck - it breaks the upper layer's live connection and
// crashes it. Blocking the stop up front is much clearer than letting that
// happen and leaving the operator to wonder why a UE container "disappeared"
// on its own.
// gnbTargets are every target that materializes a gNB container - the
// legacy singleton (basic/ulcl) plus the two independent, concurrently
// deployable slice targets (ulcl-2slice).
var gnbTargets = []string{
	constant.DEPLOY_TARGET_GNB,
	constant.DEPLOY_TARGET_GNB_SLICE1,
	constant.DEPLOY_TARGET_GNB_SLICE2,
}

func isGnbTarget(target string) bool {
	for _, t := range gnbTargets {
		if t == target {
			return true
		}
	}
	return false
}

func (p *Processor) checkStopDependents(ctx context.Context, target string) *model.ErrorDetail {
	switch {
	case target == constant.DEPLOY_TARGET_FREE5GC:
		running, err := p.anyGnbRunning(ctx)
		if err != nil {
			p.ProcLog.Warnf("Failed to check gnb status before stopping core: %v", err)
			return nil
		}
		if running {
			return &model.ErrorDetail{
				HttpStatus: http.StatusConflict,
				Detail:     "Stop gNB before stopping the core network",
			}
		}
	case isGnbTarget(target):
		running, err := p.anyUeRunning(ctx)
		if err != nil {
			p.ProcLog.Warnf("Failed to check ue instances before stopping gnb: %v", err)
			return nil
		}
		if running {
			return &model.ErrorDetail{
				HttpStatus: http.StatusConflict,
				Detail:     "Stop all UE instances before stopping gNB",
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

func (p *Processor) anyGnbRunning(ctx context.Context) (bool, error) {
	for _, target := range gnbTargets {
		running, err := p.isTargetRunning(ctx, target)
		if err != nil {
			return false, err
		}
		if running {
			return true, nil
		}
	}
	return false, nil
}

// resolveUeGnbTarget picks which gNB target a UE with the given slice
// differentiator should dial. Under ulcl-2slice, gnb-slice1/gnb-slice2 can
// both be running at once, so sd disambiguates which is actually this UE's;
// otherwise (basic/ulcl, or ulcl-2slice with neither slice deployed yet)
// falls back to the legacy singleton gnb. Returns ("", nil) - not an error -
// when nothing matching is running, so the caller can turn that into a 409
// rather than a fail-open pass.
func (p *Processor) resolveUeGnbTarget(ctx context.Context, sd string) (string, error) {
	slice1Running, err := p.isTargetRunning(ctx, constant.DEPLOY_TARGET_GNB_SLICE1)
	if err != nil {
		return "", err
	}
	slice2Running, err := p.isTargetRunning(ctx, constant.DEPLOY_TARGET_GNB_SLICE2)
	if err != nil {
		return "", err
	}

	if slice1Running || slice2Running {
		if sd == constant.SLICE1_SD && slice1Running {
			return constant.DEPLOY_TARGET_GNB_SLICE1, nil
		}
		if sd == constant.SLICE2_SD && slice2Running {
			return constant.DEPLOY_TARGET_GNB_SLICE2, nil
		}
		return "", nil
	}

	running, err := p.isTargetRunning(ctx, constant.DEPLOY_TARGET_GNB)
	if err != nil {
		return "", err
	}
	if !running {
		return "", nil
	}
	return constant.DEPLOY_TARGET_GNB, nil
}

// gnbRanIp is the ran-ue static IP a UE dials for a given gNB target - see
// the gnb/gnb-slice1/gnb-slice2 compose templates' own ran-ue ipv4_address.
func gnbRanIp(target string) string {
	if target == constant.DEPLOY_TARGET_GNB_SLICE2 {
		return "10.0.2.5"
	}
	return "10.0.2.3"
}

func (p *Processor) anyUeRunning(ctx context.Context) (bool, error) {
	instanceIDs, err := p.FlContext.ListUeInstances()
	if err != nil {
		return false, err
	}

	for _, instance := range instanceIDs {
		result, err := p.FlContext.StatusUe(ctx, instance)
		if err != nil {
			return false, err
		}
		if aggregateServiceStatus(result.Services) != "stopped" {
			return true, nil
		}
	}

	return false, nil
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
