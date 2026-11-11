# TesterRunSnapshot


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**runId** | **string** |  | [default to undefined]
**profileName** | **string** |  | [default to undefined]
**state** | **string** |  | [default to undefined]
**error** | **string** |  | [default to undefined]
**startedAt** | **string** |  | [optional] [default to undefined]
**stoppedAt** | **string** |  | [optional] [default to undefined]
**n2** | [**TesterStageSnapshot**](TesterStageSnapshot.md) |  | [default to undefined]
**registration** | [**TesterStageSnapshot**](TesterStageSnapshot.md) |  | [default to undefined]
**pdu** | [**TesterStageSnapshot**](TesterStageSnapshot.md) |  | [default to undefined]
**deregistration** | [**TesterStageSnapshot**](TesterStageSnapshot.md) |  | [default to undefined]
**n2Release** | [**TesterStageSnapshot**](TesterStageSnapshot.md) |  | [default to undefined]
**stopReason** | **string** | Why the run stopped; empty while running. | [default to undefined]
**gnbs** | [**Array&lt;TesterGnbStatus&gt;**](TesterGnbStatus.md) | The gNBs asked for (gnbOffset, gnbLimit, gnbState); every gNB when no gnbLimit was given. | [default to undefined]
**gnbTotal** | **number** | How many gNBs match gnbState (every gNB without it). | [default to undefined]
**gnbOffset** | **number** | Where gnbs starts within them. | [default to undefined]
**gnbCounts** | **{ [key: string]: number; }** | Every gNB of the run counted by state. | [default to undefined]
**ues** | [**TesterUeSummary**](TesterUeSummary.md) |  | [default to undefined]
**failedUes** | [**Array&lt;TesterUeFailure&gt;**](TesterUeFailure.md) | The first 200 UEs that failed; ues.failed has the total. | [default to undefined]
**dataplane** | [**TesterDataplaneSnapshot**](TesterDataplaneSnapshot.md) |  | [default to undefined]
**cpus** | [**TesterCpuAllocation**](TesterCpuAllocation.md) |  | [default to undefined]
**cpLoop** | [**TesterCpLoopSnapshot**](TesterCpLoopSnapshot.md) |  | [default to undefined]
**setup** | [**Array&lt;TesterSetupPoint&gt;**](TesterSetupPoint.md) | Registrations and PDU sessions per second from the start of the run until both stages are done, in at most 600 points (like the data plane\&#39;s). Absent from testers older than this field. | [optional] [default to undefined]

## Example

```typescript
import { TesterRunSnapshot } from './api';

const instance: TesterRunSnapshot = {
    runId,
    profileName,
    state,
    error,
    startedAt,
    stoppedAt,
    n2,
    registration,
    pdu,
    deregistration,
    n2Release,
    stopReason,
    gnbs,
    gnbTotal,
    gnbOffset,
    gnbCounts,
    ues,
    failedUes,
    dataplane,
    cpus,
    cpLoop,
    setup,
};
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
