# DeployStatusResponse


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**target** | **string** |  | [default to undefined]
**status** | **string** |  | [default to undefined]
**services** | [**Array&lt;ServiceStatus&gt;**](ServiceStatus.md) |  | [optional] [default to undefined]
**lastDeployed** | **string** |  | [optional] [default to undefined]

## Example

```typescript
import { DeployStatusResponse } from './api';

const instance: DeployStatusResponse = {
    target,
    status,
    services,
    lastDeployed,
};
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
