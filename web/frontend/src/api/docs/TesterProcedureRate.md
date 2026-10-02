# TesterProcedureRate

At most ratePerSec new attempts per second and at most maxInFlight at once; each attempt bounded by timeoutMs; a failed UE with retries left is requeued at the back.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**ratePerSec** | **number** |  | [default to undefined]
**maxInFlight** | **number** |  | [default to undefined]
**timeoutMs** | **number** |  | [default to undefined]
**retries** | **number** |  | [default to undefined]

## Example

```typescript
import { TesterProcedureRate } from './api';

const instance: TesterProcedureRate = {
    ratePerSec,
    maxInFlight,
    timeoutMs,
    retries,
};
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
