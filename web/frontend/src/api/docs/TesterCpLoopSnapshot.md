# TesterCpLoopSnapshot

The control-plane test. registration and deregistration count every attempt of every cycle (expected is 0, a loop has no end); latencies are of accepted procedures.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**ues** | **number** |  | [default to undefined]
**state** | **string** | off &#x3D; no control-plane UEs; waiting &#x3D; throughput UEs are still being set up. | [default to undefined]
**startedAt** | **string** |  | [optional] [default to undefined]
**durationSec** | **number** | How long the loop has run. | [default to undefined]
**registered** | **number** | Control-plane UEs registered right now. | [default to undefined]
**cycles** | **number** | Cycles whose registration and deregistration were both accepted. | [default to undefined]
**registration** | [**TesterStageSnapshot**](TesterStageSnapshot.md) |  | [default to undefined]
**deregistration** | [**TesterStageSnapshot**](TesterStageSnapshot.md) |  | [default to undefined]
**cleanup** | [**TesterStageSnapshot**](TesterStageSnapshot.md) | After Stop, the control-plane UEs still registered deregister (expected &#x3D; every control-plane UE; the others are skipped). The throughput UEs\&#39; cleanup is the run snapshot\&#39;s deregistration. | [default to undefined]
**series** | [**Array&lt;TesterCpPoint&gt;**](TesterCpPoint.md) | Per-second rates from the loop\&#39;s start, in at most 600 points (like the data plane\&#39;s). | [default to undefined]

## Example

```typescript
import { TesterCpLoopSnapshot } from './api';

const instance: TesterCpLoopSnapshot = {
    ues,
    state,
    startedAt,
    durationSec,
    registered,
    cycles,
    registration,
    deregistration,
    cleanup,
    series,
};
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
