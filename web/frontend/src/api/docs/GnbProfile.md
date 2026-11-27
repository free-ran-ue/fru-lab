# GnbProfile


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**id** | **string** |  | [optional] [default to undefined]
**name** | **string** |  | [default to undefined]
**gnbId** | **string** |  | [default to undefined]
**gnbName** | **string** |  | [optional] [default to undefined]
**mcc** | **string** |  | [default to undefined]
**mnc** | **string** |  | [default to undefined]
**tac** | **string** |  | [default to undefined]
**sst** | **string** |  | [default to undefined]
**sd** | **string** |  | [optional] [default to undefined]
**core** | **string** | lab dials the free5GC fru-lab deploys; external runs the gNB on the host network and dials amfIp / upfIp. | [default to undefined]
**amfIp** | **string** |  | [optional] [default to undefined]
**amfPort** | **number** |  | [optional] [default to undefined]
**upfIp** | **string** |  | [optional] [default to undefined]
**upfPort** | **number** |  | [optional] [default to undefined]
**n2Ip** | **string** | Ignored - on the lab free5GC or a free5gc core network the gNB\&#39;s N2/N3 address is assigned on deploy (the lowest free one from .20 up on that core\&#39;s cn-ran network) and shows in GnbDeployment.n2Ip. | [optional] [default to undefined]
**hostInterface** | **string** | External core only - the host interface the gNB listens on (see /api/host/interfaces). | [optional] [default to undefined]
**hostIp** | **string** | External core only - the IPv4 address the gNB listens on, typed in; UEs dial it too. On a saved external core network it must be an address of that core\&#39;s n2Interface (which becomes hostInterface). Unique among the external gNB profiles. | [optional] [default to undefined]
**radioRange** | **number** | The gNB\&#39;s simulated radio reach in metres - where its signal falls to 1% (100% at the gNB, S(d) &#x3D; 1 / (1 + 99 (d / radioRange)^2), inverse-square far out, never 0). Left out or 0 is unlimited - 100% everywhere. Only the UI\&#39;s map uses it; the gNB itself is unaffected. | [optional] [default to undefined]
**coreNetwork** | **string** | A saved core network\&#39;s id - the core this gNB deploys onto when a deploy names none. Kind free5gc - the gNB joins that core\&#39;s cn-ran network (core stays lab, addresses assigned). Kind external - core is external, its N2 / N3 addresses become the AMF / UPF and its n2Interface the host interface; hostIp is typed in. | [optional] [default to undefined]
**logLevel** | **string** |  | [optional] [default to undefined]
**updatedAt** | **string** |  | [optional] [default to undefined]

## Example

```typescript
import { GnbProfile } from './api';

const instance: GnbProfile = {
    id,
    name,
    gnbId,
    gnbName,
    mcc,
    mnc,
    tac,
    sst,
    sd,
    core,
    amfIp,
    amfPort,
    upfIp,
    upfPort,
    n2Ip,
    hostInterface,
    hostIp,
    radioRange,
    coreNetwork,
    logLevel,
    updatedAt,
};
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
