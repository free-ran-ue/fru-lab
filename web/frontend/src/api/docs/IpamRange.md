# IpamRange


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**name** | **string** | gnb-n2 (gNB N2/N3 addresses), gnb-ran (where UEs dial a gNB), docker (what Docker hands out itself, e.g. to UEs) | [default to undefined]
**first** | **string** |  | [default to undefined]
**last** | **string** |  | [default to undefined]

## Example

```typescript
import { IpamRange } from './api';

const instance: IpamRange = {
    name,
    first,
    last,
};
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
