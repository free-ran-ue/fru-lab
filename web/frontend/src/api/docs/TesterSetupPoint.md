# TesterSetupPoint


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**t** | **number** | Seconds since the run started. | [default to undefined]
**regPerSec** | **number** | Registrations accepted. | [default to undefined]
**pduPerSec** | **number** | PDU sessions established. | [default to undefined]
**failPerSec** | **number** | Registrations and PDU sessions that finally failed. | [default to undefined]

## Example

```typescript
import { TesterSetupPoint } from './api';

const instance: TesterSetupPoint = {
    t,
    regPerSec,
    pduPerSec,
    failPerSec,
};
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
