# RequestDeployGnb


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**slice** | **string** | Which slice\&#39;s gNB config to deploy. Only meaningful when free5gc is running the ulcl-2slice template - ignored otherwise. Defaults to slice1\&#39;s config when omitted. | [optional] [default to undefined]

## Example

```typescript
import { RequestDeployGnb } from './api';

const instance: RequestDeployGnb = {
    slice,
};
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
