package constant

// deploy targets. gnb is the singleton gNB used by the basic/ulcl free5gc
// templates; gnb-slice1/gnb-slice2 are independent, concurrently
// deployable targets only relevant under FREE5GC_TEMPLATE_ULCL_2SLICE,
// where each slice has its own dedicated SMF/UPF chain and needs its own
// gNB identity attached to it - unlike gnb, both can be up at once.
const (
	DEPLOY_TARGET_FREE5GC    = "free5gc"
	DEPLOY_TARGET_GNB        = "gnb"
	DEPLOY_TARGET_GNB_SLICE1 = "gnb-slice1"
	DEPLOY_TARGET_GNB_SLICE2 = "gnb-slice2"
	DEPLOY_TARGET_UE         = "ue"
)

// free5gc deploy template variants - which of the DEPLOY_TARGET_FREE5GC
// compose templates to materialize on deploy. Every other deploy target has
// exactly one template, so only free5gc needs this.
const (
	FREE5GC_TEMPLATE_BASIC       = "basic"
	FREE5GC_TEMPLATE_ULCL        = "ulcl"
	FREE5GC_TEMPLATE_ULCL_2SLICE = "ulcl-2slice"
)

// The S-NSSAI SDs the ulcl-2slice template's two SMF/UPF chains serve -
// used to route a UE deploy to the matching gNB slice target (see
// Processor.resolveUeGnbTarget).
const (
	SLICE1_SD = "010203"
	SLICE2_SD = "112233"
)
