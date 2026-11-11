# TesterHistorySummary


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**id** | **string** | Where the run is stored, for /api/tester/history/{runId}. The run ID for the default tester\&#39;s runs; the run ID, \&quot;@\&quot; and the tester\&#39;s id for another tester\&#39;s. | [default to undefined]
**testerId** | **string** | The tester that ran it (\&quot;default\&quot; for the one in config.yaml, and for runs stored before testers could be added). | [default to undefined]
**testerName** | **string** | That tester\&#39;s name when the run was stored; empty for runs stored before testers could be added. | [default to undefined]
**name** | **string** | The name the user gave the run in the History page; empty shows the run ID. | [default to undefined]
**runId** | **string** |  | [default to undefined]
**profileName** | **string** |  | [default to undefined]
**state** | **string** |  | [default to undefined]
**error** | **string** |  | [default to undefined]
**stopReason** | **string** |  | [default to undefined]
**startedAt** | **string** |  | [optional] [default to undefined]
**stoppedAt** | **string** |  | [optional] [default to undefined]
**gnbCount** | **number** |  | [default to undefined]
**ueCount** | **number** |  | [default to undefined]
**registered** | **number** |  | [default to undefined]
**registrationP95Ms** | **number** |  | [default to undefined]
**established** | **number** |  | [default to undefined]
**deregistered** | **number** |  | [default to undefined]
**ulTxBytes** | **number** |  | [default to undefined]
**ulRxBytes** | **number** |  | [default to undefined]
**ulLossRate** | **number** |  | [default to undefined]
**dlTxBytes** | **number** |  | [default to undefined]
**dlRxBytes** | **number** |  | [default to undefined]
**dlLossRate** | **number** |  | [default to undefined]

## Example

```typescript
import { TesterHistorySummary } from './api';

const instance: TesterHistorySummary = {
    id,
    testerId,
    testerName,
    name,
    runId,
    profileName,
    state,
    error,
    stopReason,
    startedAt,
    stoppedAt,
    gnbCount,
    ueCount,
    registered,
    registrationP95Ms,
    established,
    deregistered,
    ulTxBytes,
    ulRxBytes,
    ulLossRate,
    dlTxBytes,
    dlRxBytes,
    dlLossRate,
};
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
