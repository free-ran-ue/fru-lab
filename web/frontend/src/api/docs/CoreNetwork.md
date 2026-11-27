# CoreNetwork


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**id** | **string** |  | [optional] [default to undefined]
**name** | **string** | Unique among the core networks (ignoring case) and not free5GC, the lab core\&#39;s name. 1-32 letters, digits, spaces, dots, dashes, underscores. | [default to undefined]
**n2Ip** | **string** | The AMF\&#39;s N2 address. | [default to undefined]
**n2Interface** | **string** | The host interface that reaches n2Ip. | [optional] [default to undefined]
**n3Ip** | **string** | The UPF\&#39;s N3 address. | [default to undefined]
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
