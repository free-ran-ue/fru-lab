# TesterProfileNetworkN6

Data-network side. Uplink is addressed to sinkIp (added to the interface if the host lacks it); downlink is sent from it to the UEs, whose pool is routed via upfIp for the run.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**_interface** | **string** |  | [default to undefined]
**sinkIp** | **string** |  | [default to undefined]
**upfIp** | **string** |  | [default to undefined]
**uePool** | **string** |  | [default to undefined]

## Example

```typescript
import { TesterProfileNetworkN6 } from './api';

const instance: TesterProfileNetworkN6 = {
    _interface,
    sinkIp,
    upfIp,
    uePool,
};
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
