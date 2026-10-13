# TesterTopology


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**online** | **string** | CPUs the kernel runs. | [default to undefined]
**allowed** | **string** | Of those, the CPUs fru-tester may use (in a container, its cpuset). | [default to undefined]
**nodes** | [**Array&lt;TesterNumaNode&gt;**](TesterNumaNode.md) |  | [default to undefined]
**interfaces** | [**Array&lt;TesterNicLayout&gt;**](TesterNicLayout.md) | Physical NICs only. | [default to undefined]

## Example

```typescript
import { TesterTopology } from './api';

const instance: TesterTopology = {
    online,
    allowed,
    nodes,
    interfaces,
};
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
