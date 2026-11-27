# UeProfile


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**id** | **string** |  | [optional] [default to undefined]
**name** | **string** |  | [default to undefined]
**mcc** | **string** |  | [default to undefined]
**mnc** | **string** |  | [default to undefined]
**msin** | **string** | 9 or 10 digits. The IMSI (mcc + mnc + msin) is unique among the UE profiles. | [default to undefined]
**permanentKey** | **string** |  | [default to undefined]
**opValue** | **string** | OPc | [default to undefined]
**amf** | **string** |  | [default to undefined]
**sqn** | **string** |  | [default to undefined]
**dnn** | **string** |  | [optional] [default to undefined]
**sst** | **string** |  | [default to undefined]
**sd** | **string** |  | [optional] [default to undefined]
**gnbProfileId** | **string** | The gNB profile this UE attaches to when a deploy names no gNB; empty picks a running gNB by SD. | [optional] [default to undefined]
**updatedAt** | **string** |  | [optional] [default to undefined]

## Example

```typescript
import { UeProfile } from './api';

const instance: UeProfile = {
    id,
    name,
    mcc,
    mnc,
    msin,
    permanentKey,
    opValue,
    amf,
    sqn,
    dnn,
    sst,
    sd,
    gnbProfileId,
    updatedAt,
};
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
