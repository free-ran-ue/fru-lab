# GnbInstance


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**instance** | **string** | The gNB target name, gnb-&lt;profileId&gt;; the log stream and UE deploys use it. | [default to undefined]
**profileId** | **string** |  | [default to undefined]
**status** | **string** |  | [default to undefined]
**services** | [**Array&lt;ServiceStatus&gt;**](ServiceStatus.md) |  | [optional] [default to undefined]
**lastDeployed** | **string** |  | [optional] [default to undefined]
**gnb** | [**GnbDeployment**](GnbDeployment.md) |  | [default to undefined]

## Example

```typescript
import { GnbInstance } from './api';

const instance: GnbInstance = {
    instance,
    profileId,
    status,
    services,
    lastDeployed,
    gnb,
};
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
