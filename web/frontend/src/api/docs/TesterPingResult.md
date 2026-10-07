# TesterPingResult


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**plane** | **string** |  | [default to undefined]
**_interface** | **string** |  | [default to undefined]
**source** | **string** | The address the test used, with its prefix length. | [default to undefined]
**target** | **string** | The AMF (N2), the UPF\&#39;s N3 or the UPF\&#39;s N6 address. | [default to undefined]
**added** | **boolean** | The source was added to the interface for the test and removed again. | [default to undefined]
**sent** | **number** |  | [default to undefined]
**received** | **number** |  | [default to undefined]
**rttMs** | **Array&lt;number&gt;** |  | [default to undefined]
**error** | **string** | Why the test could not run (e.g. the address could not be added) or a send error such as no route to host; empty otherwise. | [default to undefined]

## Example

```typescript
import { TesterPingResult } from './api';

const instance: TesterPingResult = {
    plane,
    _interface,
    source,
    target,
    added,
    sent,
    received,
    rttMs,
    error,
};
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
