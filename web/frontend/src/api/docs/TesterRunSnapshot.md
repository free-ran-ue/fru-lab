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
**gnbs** | [**Array&lt;TesterGnbStatus&gt;**](TesterGnbStatus.md) |  | [default to undefined]
**ues** | [**TesterUeSummary**](TesterUeSummary.md) |  | [default to undefined]
**failedUes** | [**Array&lt;TesterUeFailure&gt;**](TesterUeFailure.md) | The first 200 UEs that failed; ues.failed has the total. | [default to undefined]

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
    gnbs,
    ues,
    failedUes,
};
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
