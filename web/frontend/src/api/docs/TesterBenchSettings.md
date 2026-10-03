# TesterBenchSettings


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**packetSize** | **number** | Inner IP packet bytes, 64..1400. | [default to undefined]
**stepSeconds** | **number** | Measuring time per sender count, 1..30. | [default to undefined]
**gso** | **boolean** | Send with UDP GSO, as runs do; off measures plain sendmmsg, as on a kernel without GSO. | [default to undefined]

## Example

```typescript
import { TesterBenchSettings } from './api';

const instance: TesterBenchSettings = {
    packetSize,
    stepSeconds,
    gso,
};
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
