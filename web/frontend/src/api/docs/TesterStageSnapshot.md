# TesterStageSnapshot

One stage card. Latencies are milliseconds over accepted items only; totalTimeMs is first start to last finish.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**name** | **string** |  | [default to undefined]
**expected** | **number** |  | [default to undefined]
**attempted** | **number** |  | [default to undefined]
**retries** | **number** |  | [default to undefined]
**inFlight** | **number** |  | [default to undefined]
**accepted** | **number** |  | [default to undefined]
**rejected** | **number** |  | [default to undefined]
**timedOut** | **number** |  | [default to undefined]
**failed** | **number** |  | [default to undefined]
**done** | **boolean** |  | [default to undefined]
**totalTimeMs** | **number** |  | [default to undefined]
**avgMs** | **number** |  | [default to undefined]
**p50Ms** | **number** |  | [default to undefined]
**p95Ms** | **number** |  | [default to undefined]
**p99Ms** | **number** |  | [default to undefined]
**maxMs** | **number** |  | [default to undefined]
**causes** | [**Array&lt;TesterCauseCount&gt;**](TesterCauseCount.md) |  | [default to undefined]

## Example

```typescript
import { TesterStageSnapshot } from './api';

const instance: TesterStageSnapshot = {
    name,
    expected,
    attempted,
    retries,
    inFlight,
    accepted,
    rejected,
    timedOut,
    failed,
    done,
    totalTimeMs,
    avgMs,
    p50Ms,
    p95Ms,
    p99Ms,
    maxMs,
    causes,
};
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
