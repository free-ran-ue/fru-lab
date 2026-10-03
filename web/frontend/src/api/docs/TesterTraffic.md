# TesterTraffic

Fixed-rate traffic of every established UE. packetSize is the inner IP packet length.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**ulMbps** | **number** | Per UE; 0 disables uplink. | [default to undefined]
**dlMbps** | **number** | Per UE; 0 disables downlink. | [default to undefined]
**packetSize** | **number** | 64..9000, and at most the N3 interface\&#39;s MTU minus 44 (GTP-U and its headers) and the N6 interface\&#39;s MTU; 1456 on a 1500 MTU network. | [default to undefined]
**port** | **number** | UDP port at the N6 sink and at the UEs. | [default to undefined]
**maxDurationMin** | **number** | Stop the run this many minutes after it started, exactly like pressing Stop. 0 &#x3D; run until Stop. | [default to undefined]
**dlBatchMs** | **number** | Send each UE this many milliseconds of downlink in a row (0..10), so UDP GSO can send them in one piece. 0 &#x3D; one packet per UE in turn. Uplink needs no grouping. | [default to undefined]
**senders** | **number** | Sender threads for downlink, and as many for uplink spread over the gNBs, each with its own socket (0..1024). 0 &#x3D; one per CPU. | [default to undefined]
**sinkSockets** | **number** | Sockets that receive uplink at the sink, one reader each (0..1024). 0 &#x3D; one per CPU. | [default to undefined]

## Example

```typescript
import { TesterTraffic } from './api';

const instance: TesterTraffic = {
    ulMbps,
    dlMbps,
    packetSize,
    port,
    maxDurationMin,
    dlBatchMs,
    senders,
    sinkSockets,
};
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
