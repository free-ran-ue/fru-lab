# TesterBenchSettings


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**packetSize** | **number** | Inner IP packet bytes, 64..1400. | [default to undefined]
**stepSeconds** | **number** | Measuring time per sender count, 1..30. | [default to undefined]

## Example

```typescript
import { TesterBenchSettings } from './api';

const instance: TesterBenchSettings = {
    packetSize,
    stepSeconds,
};
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
