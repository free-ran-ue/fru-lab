# GnbDeployment


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**profileId** | **string** |  | [optional] [default to undefined]
**profileName** | **string** |  | [default to undefined]
**core** | **string** |  | [default to undefined]
**amfIp** | **string** |  | [default to undefined]
**hostIp** | **string** |  | [optional] [default to undefined]
**hostInterface** | **string** |  | [optional] [default to undefined]
**gnbId** | **string** |  | [optional] [default to undefined]
**gnbName** | **string** |  | [optional] [default to undefined]
**sd** | **string** |  | [optional] [default to undefined]
**n2Ip** | **string** | Its N2/N3 address (cn-ran, or the host\&#39;s address on an external core). | [optional] [default to undefined]
**ranIp** | **string** | Where UEs dial it (ran-ue), picked by fru-lab; the host address on an external core. | [optional] [default to undefined]
**coreNetwork** | **string** | The core network it attached to (a core network id); empty for the lab free5GC. | [optional] [default to undefined]

## Example

```typescript
import { GnbDeployment } from './api';

const instance: GnbDeployment = {
    profileId,
    profileName,
    core,
    amfIp,
    hostIp,
    hostInterface,
    gnbId,
    gnbName,
    sd,
    n2Ip,
    ranIp,
    coreNetwork,
};
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
