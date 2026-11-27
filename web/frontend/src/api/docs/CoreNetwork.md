# CoreNetwork


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**id** | **string** |  | [optional] [default to undefined]
**name** | **string** | Unique among the core networks (ignoring case) and not free5GC, the lab core\&#39;s name. 1-32 letters, digits, spaces, dots, dashes, underscores. | [default to undefined]
**kind** | **string** | external is a core outside fru-lab (n2Ip / n3Ip / interfaces given); free5gc is another free5GC fru-lab deploys on this host, whose addresses fru-lab assigns (n2Ip, n3Ip, n2Interface, n3Interface, slot, webconsolePort are then read-only, filled in on save, and dns is always [internet]). The kind cannot change after the first save. | [optional] [default to KindEnum_External]
**slot** | **number** | kind free5gc - 1 to 9, unique; picks its networks (cn-ran 10.0.(100+slot).0/24, SBI 10.0.(110+slot).0/24), its container name prefix and its webconsole port. | [optional] [readonly] [default to undefined]
**webconsolePort** | **number** | kind free5gc - the host port its webconsole is published on (deploy.webconsole.port + slot). The browser reaches it at /api/core-networks/{id}/webconsole/..., which proxies it like /api/webconsole does for the lab free5GC. | [optional] [readonly] [default to undefined]
**n2Ip** | **string** | The AMF\&#39;s N2 address. Required for kind external; assigned for kind free5gc (10.0.(100+slot).3). | [optional] [default to undefined]
**n2Interface** | **string** | The host interface that reaches n2Ip. | [optional] [default to undefined]
**n3Ip** | **string** | The UPF\&#39;s N3 address. Required for kind external; assigned for kind free5gc (10.0.(100+slot).5). | [optional] [default to undefined]
**n3Interface** | **string** | The host interface that reaches n3Ip. | [optional] [default to undefined]
**dns** | [**Array&lt;CoreDn&gt;**](CoreDn.md) |  | [default to undefined]
**enabled** | **boolean** | Switched on, the core counts as part of the lab and the Overview draws it. | [optional] [default to undefined]
**updatedAt** | **string** |  | [optional] [default to undefined]

## Example

```typescript
import { CoreNetwork } from './api';

const instance: CoreNetwork = {
    id,
    name,
    kind,
    slot,
    webconsolePort,
    n2Ip,
    n2Interface,
    n3Ip,
    n3Interface,
    dns,
    enabled,
    updatedAt,
};
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
