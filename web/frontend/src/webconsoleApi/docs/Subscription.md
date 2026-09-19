# Subscription


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**userNumber** | **number** |  | [optional] [default to undefined]
**plmnID** | **string** |  | [default to undefined]
**ueId** | **string** |  | [default to undefined]
**AuthenticationSubscription** | [**AuthenticationSubscription**](AuthenticationSubscription.md) |  | [default to undefined]
**AccessAndMobilitySubscriptionData** | [**AccessAndMobilitySubscriptionData**](AccessAndMobilitySubscriptionData.md) |  | [default to undefined]
**SessionManagementSubscriptionData** | [**Array&lt;SessionManagementSubscriptionData&gt;**](SessionManagementSubscriptionData.md) |  | [default to undefined]
**SmfSelectionSubscriptionData** | [**SmfSelectionSubscriptionData**](SmfSelectionSubscriptionData.md) |  | [default to undefined]
**AmPolicyData** | [**AmPolicyData**](AmPolicyData.md) |  | [default to undefined]
**SmPolicyData** | [**SmPolicyData**](SmPolicyData.md) |  | [default to undefined]
**FlowRules** | [**Array&lt;FlowRules&gt;**](FlowRules.md) |  | [default to undefined]
**QosFlows** | [**Array&lt;QosFlows&gt;**](QosFlows.md) |  | [default to undefined]
**ChargingDatas** | [**Array&lt;ChargingData&gt;**](ChargingData.md) |  | [default to undefined]

## Example

```typescript
import { Subscription } from './api';

const instance: Subscription = {
    userNumber,
    plmnID,
    ueId,
    AuthenticationSubscription,
    AccessAndMobilitySubscriptionData,
    SessionManagementSubscriptionData,
    SmfSelectionSubscriptionData,
    AmPolicyData,
    SmPolicyData,
    FlowRules,
    QosFlows,
    ChargingDatas,
};
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
