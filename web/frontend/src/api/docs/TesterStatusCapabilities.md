# TesterStatusCapabilities


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**netAdmin** | **boolean** | Needed to add the gNB IPs and the UE route. | [default to undefined]
**bpf** | **boolean** | Needed by the AF_XDP engine (CAP_BPF, or CAP_SYS_ADMIN before Linux 5.8). | [default to undefined]
**ipcLock** | **boolean** | Needed by the AF_XDP engine. | [default to undefined]
**sctp** | **boolean** | The sctp kernel module is loaded. | [default to undefined]

## Example

```typescript
import { TesterStatusCapabilities } from './api';

const instance: TesterStatusCapabilities = {
    netAdmin,
    bpf,
    ipcLock,
    sctp,
};
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
