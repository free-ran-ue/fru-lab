# ProfileConflict


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**field** | **string** | The JSON field name, e.g. gnbId, n2Ip, hostIp, name, msin. | [default to undefined]
**message** | **string** |  | [default to undefined]
**suggestion** | **string** | The next free value, when there is one. | [optional] [default to undefined]

## Example

```typescript
import { ProfileConflict } from './api';

const instance: ProfileConflict = {
    field,
    message,
    suggestion,
};
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
