# IpamPool


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**id** | **string** | cn-ran (the lab free5GC\&#39;s), cn-ran-&lt;coreId&gt;, sbi, sbi-&lt;coreId&gt;, ran-ue | [default to undefined]
**network** | **string** | The Docker network it is. | [default to undefined]
**subnet** | **string** |  | [default to undefined]
**purpose** | **string** |  | [default to undefined]
**coreId** | **string** | The core network it belongs to; empty for the lab free5GC\&#39;s and the shared ones. | [optional] [default to undefined]
**reserved** | [**Array&lt;IpamReserved&gt;**](IpamReserved.md) |  | [default to undefined]
**ranges** | [**Array&lt;IpamRange&gt;**](IpamRange.md) |  | [default to undefined]
**leases** | [**Array&lt;IpamLease&gt;**](IpamLease.md) |  | [default to undefined]

## Example

```typescript
import { IpamPool } from './api';

const instance: IpamPool = {
    id,
    network,
    subnet,
    purpose,
    coreId,
    reserved,
    ranges,
    leases,
};
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
