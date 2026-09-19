# DeployUeStatusResponse


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**instance** | **string** |  | [default to undefined]
**status** | **string** |  | [default to undefined]
**services** | [**Array&lt;ServiceStatus&gt;**](ServiceStatus.md) |  | [optional] [default to undefined]
**lastDeployed** | **string** |  | [optional] [default to undefined]

## Example

```typescript
import { DeployUeStatusResponse } from './api';

const instance: DeployUeStatusResponse = {
    instance,
    status,
    services,
    lastDeployed,
};
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
