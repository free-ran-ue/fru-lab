# IpamLease


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**addr** | **string** |  | [default to undefined]
**owner** | **string** | Who holds it - a deploy target (gnb-&lt;profileId&gt;, ue-&lt;imsi&gt;, core-&lt;id&gt;) with an optional /&lt;role&gt;. | [default to undefined]
**role** | **string** |  | [optional] [default to undefined]
**since** | **string** |  | [optional] [default to undefined]
**live** | **boolean** | A running container holds it right now (false is a lease kept for a stopped owner so it gets the same address back). | [optional] [default to undefined]

## Example

```typescript
import { IpamLease } from './api';

const instance: IpamLease = {
    addr,
    owner,
    role,
    since,
    live,
};
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
