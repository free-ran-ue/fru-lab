# TesterStatusMemory

Bytes.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**hostTotal** | **number** |  | [default to undefined]
**hostAvailable** | **number** |  | [default to undefined]
**processRss** | **number** | fru-tester\&#39;s resident memory. | [default to undefined]
**goHeap** | **number** |  | [default to undefined]
**cgroupLimit** | **number** | Memory limit of fru-tester\&#39;s cgroup (its container); null when there is none. | [default to undefined]
**cgroupUsage** | **number** | What the cgroup uses, page cache included; null when it cannot be read. | [default to undefined]

## Example

```typescript
import { TesterStatusMemory } from './api';

const instance: TesterStatusMemory = {
    hostTotal,
    hostAvailable,
    processRss,
    goHeap,
    cgroupLimit,
    cgroupUsage,
};
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
