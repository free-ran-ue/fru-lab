# TesterTrafficDirection

Tx is what the tester sent; Rx is what came back through the UPF. Bytes are inner IP packet bytes; rates are over the last second.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**txPackets** | **number** |  | [default to undefined]
**txBytes** | **number** |  | [default to undefined]
**rxPackets** | **number** |  | [default to undefined]
**rxBytes** | **number** |  | [default to undefined]
**txBps** | **number** |  | [default to undefined]
**rxBps** | **number** |  | [default to undefined]
**txPps** | **number** |  | [default to undefined]
**rxPps** | **number** |  | [default to undefined]
**lossRate** | **number** | 1 - rx/tx so far. | [default to undefined]
**outOfOrder** | **number** |  | [default to undefined]
**sendErrors** | **number** |  | [default to undefined]
**misrouted** | **number** | Downlink that arrived in another UE\&#39;s tunnel or at another gNB. Always 0 for uplink. | [default to undefined]
**latency** | [**TesterLatency**](TesterLatency.md) |  | [default to undefined]

## Example

```typescript
import { TesterTrafficDirection } from './api';

const instance: TesterTrafficDirection = {
    txPackets,
    txBytes,
    rxPackets,
    rxBytes,
    txBps,
    rxBps,
    txPps,
    rxPps,
    lossRate,
    outOfOrder,
    sendErrors,
    misrouted,
    latency,
};
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
