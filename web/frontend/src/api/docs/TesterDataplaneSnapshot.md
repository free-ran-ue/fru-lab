# TesterDataplaneSnapshot


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**engine** | **string** | How packets move, e.g. \&quot;socket\&quot;, \&quot;socket (auto: docker-cn-ran is a bridge, not a NIC)\&quot; or \&quot;af_xdp (ens2f1 zero-copy, 8 queues)\&quot;. | [default to undefined]
**activeUes** | **number** |  | [default to undefined]
**ul** | [**TesterTrafficDirection**](TesterTrafficDirection.md) |  | [default to undefined]
**dl** | [**TesterTrafficDirection**](TesterTrafficDirection.md) |  | [default to undefined]
**gnbs** | [**Array&lt;TesterGnbTraffic&gt;**](TesterGnbTraffic.md) | Per gNB, same order as gnbs in the run snapshot. | [default to undefined]
**series** | [**Array&lt;TesterTrafficPoint&gt;**](TesterTrafficPoint.md) | The whole run from its first second, in at most 600 points. A point covers 1 second until the run outgrows that, then 2, 4, 8 ... seconds (averaged). | [default to undefined]

## Example

```typescript
import { TesterDataplaneSnapshot } from './api';

const instance: TesterDataplaneSnapshot = {
    engine,
    activeUes,
    ul,
    dl,
    gnbs,
    series,
};
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
