# TesterDataplaneSnapshot


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**activeUes** | **number** |  | [default to undefined]
**ul** | [**TesterTrafficDirection**](TesterTrafficDirection.md) |  | [default to undefined]
**dl** | [**TesterTrafficDirection**](TesterTrafficDirection.md) |  | [default to undefined]
**gnbs** | [**Array&lt;TesterGnbTraffic&gt;**](TesterGnbTraffic.md) | Per gNB, same order as gnbs in the run snapshot. | [default to undefined]
**series** | [**Array&lt;TesterTrafficPoint&gt;**](TesterTrafficPoint.md) | One point per second, the last 300 seconds. | [default to undefined]

## Example

```typescript
import { TesterDataplaneSnapshot } from './api';

const instance: TesterDataplaneSnapshot = {
    activeUes,
    ul,
    dl,
    gnbs,
    series,
};
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
