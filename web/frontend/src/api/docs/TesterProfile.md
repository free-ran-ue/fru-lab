# TesterProfile


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**name** | **string** |  | [default to undefined]
**testerId** | **string** | Which tester (GET /api/testers) runs this profile; absent means \&quot;default\&quot;, the one in config.yaml. Only fru-lab reads it; leave it out of what is sent to a tester, which rejects fields it does not know. | [optional] [default to undefined]
**scale** | [**TesterProfileScale**](TesterProfileScale.md) |  | [default to undefined]
**gnb** | [**TesterProfileGnb**](TesterProfileGnb.md) |  | [default to undefined]
**ue** | [**TesterUeTemplate**](TesterUeTemplate.md) |  | [default to undefined]
**traffic** | [**TesterTraffic**](TesterTraffic.md) |  | [default to undefined]
**network** | [**TesterProfileNetwork**](TesterProfileNetwork.md) |  | [default to undefined]
**rates** | [**TesterProfileRates**](TesterProfileRates.md) |  | [default to undefined]

## Example

```typescript
import { TesterProfile } from './api';

const instance: TesterProfile = {
    name,
    testerId,
    scale,
    gnb,
    ue,
    traffic,
    network,
    rates,
};
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
