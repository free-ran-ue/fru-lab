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
**n2Ip** | **string** | Lab core only - the gNB\&#39;s N2/N3 address on the lab\&#39;s cn-ran network (10.0.1.20 - 10.0.1.254; lower ones are the core\&#39;s). Unique among the gNB profiles. | [optional] [default to undefined]
**hostInterface** | **string** | External core only - the host interface the gNB listens on (see /api/host/interfaces). | [optional] [default to undefined]
**hostIp** | **string** | External core only - the IPv4 address of hostInterface the gNB listens on; UEs dial it too. Unique among the external gNB profiles. | [optional] [default to undefined]
**coreNetwork** | **string** | External core only - a saved core network\&#39;s id. Its N2 / N3 addresses become the AMF / UPF on deploy (amfIp / upfIp are then left empty), and hostInterface may be left empty for the core\&#39;s N2 interface. Deploys without a core of their own go there. | [optional] [default to undefined]
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
    coreNetwork,
    logLevel,
    updatedAt,
};
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
