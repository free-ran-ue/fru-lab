# DeployGnbRequest


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**core** | **string** | The core this deploy attaches to - empty keeps the profile\&#39;s, lab is the lab free5GC, anything else an enabled external core network\&#39;s id (its N2 / N3 addresses become the AMF and UPF; the gNB listens on the profile\&#39;s typed-in hostIp on the core\&#39;s N2 interface), or a running free5gc core network\&#39;s id (the gNB joins its cn-ran network at an assigned address). | [optional] [default to undefined]

## Example

```typescript
import { DeployGnbRequest } from './api';

const instance: DeployGnbRequest = {
    core,
};
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
