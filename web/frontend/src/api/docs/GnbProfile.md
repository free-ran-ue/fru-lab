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
**hostIp** | **string** | The host address the gNB listens on (external core); UEs dial it too. | [optional] [default to undefined]
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
    hostIp,
    logLevel,
    updatedAt,
};
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
