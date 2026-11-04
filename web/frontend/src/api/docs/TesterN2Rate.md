# TesterN2Rate

Paces the gNBs\' N2 setup (SCTP association and NG Setup). A failed attempt with retries left goes to the back of the queue.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**ratePerSec** | **number** | At most this many gNBs start their N2 setup per second. 0 or absent is no limit. | [optional] [default to undefined]
**maxInFlight** | **number** | At most this many gNBs are setting up at once, up to 1000. 0 or absent is 1000. | [optional] [default to undefined]
**timeoutMs** | **number** |  | [default to undefined]
**retries** | **number** |  | [default to undefined]

## Example

```typescript
import { TesterN2Rate } from './api';

const instance: TesterN2Rate = {
    ratePerSec,
    maxInFlight,
    timeoutMs,
    retries,
};
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
