# TesterProfileNetworkN3


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**_interface** | **string** |  | [default to undefined]
**cidr** | **string** |  | [default to undefined]
**startIp** | **string** |  | [default to undefined]
**upfIp** | **string** |  | [default to undefined]
**upfPort** | **number** |  | [default to undefined]
**ipsPerGnb** | **number** | N3 IPs per gNB (1..64; 0 counts as 1). A gNB hands them out to its UEs\&#39; PDU sessions in turn as downlink tunnel addresses, so its downlink is several connections that NICs, the kernel and the tester can spread over CPUs. | [default to undefined]

## Example

```typescript
import { TesterProfileNetworkN3 } from './api';

const instance: TesterProfileNetworkN3 = {
    _interface,
    cidr,
    startIp,
    upfIp,
    upfPort,
    ipsPerGnb,
};
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
