# TesterTraffic

Fixed-rate traffic of every established UE. packetSize is the inner IP packet length.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**ulMbps** | **number** | Per UE; 0 disables uplink. | [default to undefined]
**dlMbps** | **number** | Per UE; 0 disables downlink. | [default to undefined]
**packetSize** | **number** |  | [default to undefined]
**port** | **number** | UDP port at the N6 sink and at the UEs. | [default to undefined]
**maxDurationMin** | **number** | Stop the run this many minutes after it started, exactly like pressing Stop. 0 &#x3D; run until Stop. | [default to undefined]

## Example

```typescript
import { TesterTraffic } from './api';

const instance: TesterTraffic = {
    ulMbps,
    dlMbps,
    packetSize,
    port,
    maxDurationMin,
};
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
