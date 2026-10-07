# TesterCpLoopRate

Paces the control-plane loop - at most ratePerSec registrations start per second and at most maxInFlight UEs are in a cycle at once. A cycle registers, stays registered holdMs, then deregisters; each procedure is bounded by timeoutMs. No retries - a failed cycle is counted and the UE tries again later. Only checked when there are control-plane UEs.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**ratePerSec** | **number** |  | [default to undefined]
**maxInFlight** | **number** |  | [default to undefined]
**timeoutMs** | **number** |  | [default to undefined]
**holdMs** | **number** | 0..3600000 | [default to undefined]

## Example

```typescript
import { TesterCpLoopRate } from './api';

const instance: TesterCpLoopRate = {
    ratePerSec,
    maxInFlight,
    timeoutMs,
    holdMs,
};
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
