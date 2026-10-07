# TesterProfileScale


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**gnbCount** | **number** |  | [default to undefined]
**ueCount** | **number** |  | [default to undefined]
**throughputUeCount** | **number** | How many UEs, the first ones, establish a PDU session and carry traffic (0..ueCount). The rest are the control-plane test - they register and deregister in a loop (rates.cpLoop) once every throughput UE is done. fru-tester treats an absent value as ueCount. | [default to undefined]

## Example

```typescript
import { TesterProfileScale } from './api';

const instance: TesterProfileScale = {
    gnbCount,
    ueCount,
    throughputUeCount,
};
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
