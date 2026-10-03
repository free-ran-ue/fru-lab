# TesterBenchStep


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**senders** | **number** |  | [default to undefined]
**pps** | **number** |  | [default to undefined]
**bps** | **number** | Inner IP bits per second. | [default to undefined]
**sendErrors** | **number** |  | [default to undefined]
**gso** | **boolean** | Whether the senders did use UDP GSO (false if the kernel refused it). | [default to undefined]

## Example

```typescript
import { TesterBenchStep } from './api';

const instance: TesterBenchStep = {
    senders,
    pps,
    bps,
    sendErrors,
    gso,
};
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
