# TesterGnbSpec


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**cpUeCount** | **number** | Of ueCount, how many are in the control-plane loop. | [default to undefined]
**index** | **number** |  | [default to undefined]
**name** | **string** |  | [default to undefined]
**gnbId** | **string** |  | [default to undefined]
**n2Ip** | **string** |  | [default to undefined]
**n3Ip** | **string** |  | [default to undefined]
**ueCount** | **number** |  | [default to undefined]
**ueFirst** | **number** | 1-based index of the first UE on this gNB; 0 when it has none. | [default to undefined]
**ueLast** | **number** |  | [default to undefined]
**firstSupi** | **string** | SUPI of this gNB\&#39;s first UE; empty when it has none. | [default to undefined]
**lastSupi** | **string** |  | [default to undefined]

## Example

```typescript
import { TesterGnbSpec } from './api';

const instance: TesterGnbSpec = {
    cpUeCount,
    index,
    name,
    gnbId,
    n2Ip,
    n3Ip,
    ueCount,
    ueFirst,
    ueLast,
    firstSupi,
    lastSupi,
};
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
