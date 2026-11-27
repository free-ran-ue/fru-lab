# CoreDeployStatus


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**coreId** | **string** |  | [default to undefined]
**status** | **string** |  | [default to undefined]
**services** | [**Array&lt;ServiceStatus&gt;**](ServiceStatus.md) |  | [optional] [default to undefined]
**lastDeployed** | **string** |  | [optional] [default to undefined]
**template** | **string** | The template it runs (basic / ulcl); empty while stopped. | [optional] [default to undefined]

## Example

```typescript
import { CoreDeployStatus } from './api';

const instance: CoreDeployStatus = {
    coreId,
    status,
    services,
    lastDeployed,
    template,
};
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
