# TesterCpuAllocation

Linux CPU lists such as \"0-3,8\". The control plane (registration, PDU, the control-plane loop, the API) runs on cpCpus for a run; each data plane sender and reader keeps a thread on dpCpus, one of each per CPU. Empty = that side is not pinned.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**cpCpus** | **string** |  | [default to undefined]
**dpCpus** | **string** |  | [default to undefined]

## Example

```typescript
import { TesterCpuAllocation } from './api';

const instance: TesterCpuAllocation = {
    cpCpus,
    dpCpus,
};
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
