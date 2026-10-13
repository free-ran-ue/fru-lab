# TesterNicLayout


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**name** | **string** |  | [default to undefined]
**driver** | **string** |  | [default to undefined]
**numaNode** | **number** | -1 when the kernel does not say (e.g. a VM). | [default to undefined]
**localCpus** | **string** | CPUs close to the NIC. | [default to undefined]
**queues** | **number** | Receive queues. | [default to undefined]
**irqCpus** | **string** | CPUs the NIC\&#39;s interrupts are delivered to; the kernel\&#39;s receive work runs there. | [default to undefined]

## Example

```typescript
import { TesterNicLayout } from './api';

const instance: TesterNicLayout = {
    name,
    driver,
    numaNode,
    localCpus,
    queues,
    irqCpus,
};
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
