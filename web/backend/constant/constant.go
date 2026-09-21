package constant

// deploy targets
const (
	DEPLOY_TARGET_FREE5GC = "free5gc"
	DEPLOY_TARGET_GNB     = "gnb"
	DEPLOY_TARGET_UE      = "ue"
)

// free5gc deploy template variants - which of the DEPLOY_TARGET_FREE5GC
// compose templates to materialize on deploy. Every other deploy target has
// exactly one template, so only free5gc needs this.
const (
	FREE5GC_TEMPLATE_BASIC = "basic"
	FREE5GC_TEMPLATE_ULCL  = "ulcl"
)
