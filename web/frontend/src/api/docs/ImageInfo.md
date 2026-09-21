# ImageInfo


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**key** | **string** |  | [default to undefined]
**name** | **string** |  | [default to undefined]
**image** | **string** |  | [default to undefined]
**group** | **string** |  | [default to undefined]
**present** | **boolean** | Whether the image is currently present in the local docker image cache. | [default to undefined]
**size** | **number** | Image size in bytes. Omitted when not present locally. | [optional] [default to undefined]
**createdAt** | **string** | When the locally cached image was built. Omitted when not present locally. | [optional] [default to undefined]

## Example

```typescript
import { ImageInfo } from './api';

const instance: ImageInfo = {
    key,
    name,
    image,
    group,
    present,
    size,
    createdAt,
};
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
