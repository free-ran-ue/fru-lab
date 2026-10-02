# TesterUeTemplate

Expanded once per UE. MCC/MNC come from the gNB template; MCC+MNC+MSIN must be 15 digits. Key/OPc/AMF/SQN must match the subscribers created in the core.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**msinStart** | **string** | Decimal; incremented per UE, keeping its width. | [default to undefined]
**key** | **string** |  | [default to undefined]
**opc** | **string** |  | [default to undefined]
**amf** | **string** |  | [default to undefined]
**sqn** | **string** |  | [default to undefined]
**integrity** | **string** |  | [default to undefined]
**ciphering** | **string** |  | [default to undefined]
**dnn** | **string** |  | [default to undefined]
**sst** | **number** |  | [default to undefined]
**sd** | **string** |  | [default to undefined]

## Example

```typescript
import { TesterUeTemplate } from './api';

const instance: TesterUeTemplate = {
    msinStart,
    key,
    opc,
    amf,
    sqn,
    integrity,
    ciphering,
    dnn,
    sst,
    sd,
};
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
