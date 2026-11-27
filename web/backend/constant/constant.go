package constant

// deploy targets. Every gNB is deployed from a gNB profile, as target
// GNB_INSTANCE_PREFIX + <profile id>.
const (
	DEPLOY_TARGET_FREE5GC = "free5gc"
	DEPLOY_TARGET_UE      = "ue"

	GNB_INSTANCE_PREFIX = "gnb-"
)

// free5gc deploy template variants - which of the DEPLOY_TARGET_FREE5GC
// compose templates to materialize on deploy. Every other deploy target has
// exactly one template, so only free5gc needs this.
const (
	FREE5GC_TEMPLATE_BASIC = "basic"
	FREE5GC_TEMPLATE_ULCL  = "ulcl"
)
