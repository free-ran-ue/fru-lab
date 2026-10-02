# TesterBenchResult


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**state** | **string** |  | [default to undefined]
**error** | **string** |  | [default to undefined]
**cpus** | **number** |  | [default to undefined]
**kernel** | **string** |  | [default to undefined]
**settings** | [**TesterBenchSettings**](TesterBenchSettings.md) |  | [default to undefined]
**plannedSenders** | **Array&lt;number&gt;** |  | [default to undefined]
**steps** | [**Array&lt;TesterBenchStep&gt;**](TesterBenchStep.md) |  | [default to undefined]
**startedAt** | **string** |  | [optional] [default to undefined]
**finishedAt** | **string** |  | [optional] [default to undefined]

## Example

```typescript
import { TesterBenchResult } from './api';

const instance: TesterBenchResult = {
    state,
    error,
    cpus,
    kernel,
    settings,
    plannedSenders,
    steps,
    startedAt,
    finishedAt,
};
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
