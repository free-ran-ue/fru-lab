# TesterLatency

One-way latency in milliseconds (send and receive share the host clock).

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**count** | **number** |  | [default to undefined]
**avgMs** | **number** |  | [default to undefined]
**p50Ms** | **number** |  | [default to undefined]
**p99Ms** | **number** |  | [default to undefined]
**maxMs** | **number** |  | [default to undefined]

## Example

```typescript
import { TesterLatency } from './api';

const instance: TesterLatency = {
    count,
    avgMs,
    p50Ms,
    p99Ms,
    maxMs,
};
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
