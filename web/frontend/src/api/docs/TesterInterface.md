# TesterInterface


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**name** | **string** |  | [default to undefined]
**kind** | **string** | Link type, e.g. device (a NIC), bridge, veth, macvlan, vlan, bond. | [default to undefined]
**state** | **string** | Operational state, e.g. up, down, unknown. | [default to undefined]
**mtu** | **number** |  | [default to undefined]
**addresses** | **Array&lt;string&gt;** | IPv4 addresses with prefix length. | [default to undefined]

## Example

```typescript
import { TesterInterface } from './api';

const instance: TesterInterface = {
    name,
    kind,
    state,
    mtu,
    addresses,
};
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
