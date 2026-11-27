# DeployUeProfileRequest


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**gnb** | **string** | The gNB target to attach to (gnb-&lt;profileId&gt;); empty uses the UE profile\&#39;s gnbProfileId, else lets fru-lab pick by SD; auto always lets fru-lab pick. | [optional] [default to undefined]

## Example

```typescript
import { DeployUeProfileRequest } from './api';

const instance: DeployUeProfileRequest = {
    gnb,
};
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
