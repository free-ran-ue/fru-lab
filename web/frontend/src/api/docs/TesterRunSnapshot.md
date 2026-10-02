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
**gnbs** | [**Array&lt;TesterGnbStatus&gt;**](TesterGnbStatus.md) |  | [default to undefined]

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
    gnbs,
};
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
