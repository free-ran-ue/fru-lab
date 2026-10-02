# TesterProfileGnb


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**gnbIdStart** | **string** | 6 or 8 hex digits; incremented per gNB, keeping its width. | [default to undefined]
**namePattern** | **string** | Must contain {i}, replaced by the 1-based gNB index. | [default to undefined]
**mcc** | **string** |  | [default to undefined]
**mnc** | **string** |  | [default to undefined]
**tac** | **string** |  | [default to undefined]
**sst** | **number** |  | [default to undefined]
**sd** | **string** | Empty or 6 hex digits. | [default to undefined]

## Example

```typescript
import { TesterProfileGnb } from './api';

const instance: TesterProfileGnb = {
    gnbIdStart,
    namePattern,
    mcc,
    mnc,
    tac,
    sst,
    sd,
};
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
