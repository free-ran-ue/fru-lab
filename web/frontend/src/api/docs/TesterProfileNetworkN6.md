# TesterProfileNetworkN6

Data-network side. Uplink is addressed to the sink; downlink is sent from it to the UEs, whose pool is routed via upfIp for the run.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**_interface** | **string** |  | [default to undefined]
**sinkIp** | **string** | The sink\&#39;s address with its prefix length, as it goes on the interface (e.g. 172.26.6.1/16). If the host does not have the address, the tester adds it to the interface for the run with that prefix, which also gives the host a route to the subnet; it is removed afterwards. | [default to undefined]
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
