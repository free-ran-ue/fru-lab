# TesterUeSummary

UEs per pipeline state; the fields add up to the UE count.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**pending** | **number** |  | [default to undefined]
**registering** | **number** |  | [default to undefined]
**registered** | **number** |  | [default to undefined]
**establishing** | **number** |  | [default to undefined]
**established** | **number** |  | [default to undefined]
**failed** | **number** |  | [default to undefined]
**skipped** | **number** |  | [default to undefined]
**cancelled** | **number** |  | [default to undefined]

## Example

```typescript
import { TesterUeSummary } from './api';

const instance: TesterUeSummary = {
    pending,
    registering,
    registered,
    establishing,
    established,
    failed,
    skipped,
    cancelled,
};
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
