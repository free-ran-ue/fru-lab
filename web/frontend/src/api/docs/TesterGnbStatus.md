# TesterGnbStatus


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**index** | **number** |  | [default to undefined]
**name** | **string** |  | [default to undefined]
**gnbId** | **string** |  | [default to undefined]
**n2Ip** | **string** |  | [default to undefined]
**n3Ip** | **string** |  | [default to undefined]
**ueCount** | **number** |  | [default to undefined]
**ueFirst** | **number** | 1-based index of the first UE on this gNB; 0 when it has none. | [default to undefined]
**ueLast** | **number** |  | [default to undefined]
**state** | **string** | lost &#x3D; was up, then the AMF side dropped the association. | [default to undefined]
**attempts** | **number** |  | [default to undefined]
**latencyMs** | **number** |  | [default to undefined]
**cause** | **string** |  | [default to undefined]

## Example

```typescript
import { TesterGnbStatus } from './api';

const instance: TesterGnbStatus = {
    index,
    name,
    gnbId,
    n2Ip,
    n3Ip,
    ueCount,
    ueFirst,
    ueLast,
    state,
    attempts,
    latencyMs,
    cause,
};
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
