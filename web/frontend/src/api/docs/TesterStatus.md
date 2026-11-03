# TesterStatus


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**name** | **string** | The tester\&#39;s testerName (tester.yaml); its host name when that is empty. | [default to undefined]
**hostname** | **string** |  | [default to undefined]
**version** | **string** | VCS revision fru-tester was built from; empty when the build does not say. | [default to undefined]
**goVersion** | **string** |  | [default to undefined]
**kernel** | **string** | Kernel release of the tester host. | [default to undefined]
**pid** | **number** |  | [default to undefined]
**startedAt** | **string** |  | [default to undefined]
**uptimeSec** | **number** |  | [default to undefined]
**memory** | [**TesterStatusMemory**](TesterStatusMemory.md) |  | [default to undefined]
**cpu** | [**TesterStatusCpu**](TesterStatusCpu.md) |  | [default to undefined]
**capabilities** | [**TesterStatusCapabilities**](TesterStatusCapabilities.md) |  | [default to undefined]

## Example

```typescript
import { TesterStatus } from './api';

const instance: TesterStatus = {
    name,
    hostname,
    version,
    goVersion,
    kernel,
    pid,
    startedAt,
    uptimeSec,
    memory,
    cpu,
    capabilities,
};
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
