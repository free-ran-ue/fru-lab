# DefaultApi

All URIs are relative to *http://127.0.0.1:5000*

|Method | HTTP request | Description|
|------------- | ------------- | -------------|
|[**deployFree5gcDown**](#deployfree5gcdown) | **DELETE** /api/deploy/free5gc | Stop free5GC|
|[**deployFree5gcLogs**](#deployfree5gclogs) | **GET** /api/deploy/free5gc/logs | Get free5GC container logs|
|[**deployFree5gcStatus**](#deployfree5gcstatus) | **GET** /api/deploy/free5gc/status | Get free5GC deploy status|
|[**deployFree5gcUp**](#deployfree5gcup) | **POST** /api/deploy/free5gc | Deploy free5GC|
|[**deployGnbDown**](#deploygnbdown) | **DELETE** /api/deploy/gnb | Stop gNB|
|[**deployGnbLogs**](#deploygnblogs) | **GET** /api/deploy/gnb/logs | Get gNB container logs|
|[**deployGnbSliceDown**](#deploygnbslicedown) | **DELETE** /api/deploy/gnb-slice/{slice} | Stop a gNB slice (ulcl-2slice only)|
|[**deployGnbSliceLogs**](#deploygnbslicelogs) | **GET** /api/deploy/gnb-slice/{slice}/logs | Get a gNB slice\&#39;s container logs|
|[**deployGnbSliceStatus**](#deploygnbslicestatus) | **GET** /api/deploy/gnb-slice/{slice}/status | Get a gNB slice\&#39;s deploy status|
|[**deployGnbSliceUp**](#deploygnbsliceup) | **POST** /api/deploy/gnb-slice/{slice} | Deploy a gNB slice (ulcl-2slice only)|
|[**deployGnbStatus**](#deploygnbstatus) | **GET** /api/deploy/gnb/status | Get gNB deploy status|
|[**deployGnbUp**](#deploygnbup) | **POST** /api/deploy/gnb | Deploy gNB|
|[**deployUeDown**](#deployuedown) | **DELETE** /api/deploy/ue/{instance} | Stop a UE instance|
|[**deployUeList**](#deployuelist) | **GET** /api/deploy/ue | List all deployed UE instances|
|[**deployUeLogs**](#deployuelogs) | **GET** /api/deploy/ue/{instance}/logs | Get a UE instance\&#39;s container logs|
|[**deployUeStatus**](#deployuestatus) | **GET** /api/deploy/ue/{instance}/status | Get a UE instance\&#39;s deploy status|
|[**deployUeUp**](#deployueup) | **POST** /api/deploy/ue/{instance} | Deploy a UE instance for one subscriber|
|[**imageList**](#imagelist) | **GET** /api/images | List the images this app deploys|
|[**imagePull**](#imagepull) | **POST** /api/images/{key}/pull | Pull an image|
|[**imageRemove**](#imageremove) | **DELETE** /api/images/{key} | Clear a locally cached image|
|[**login**](#login) | **POST** /api/login | Login|
|[**logout**](#logout) | **POST** /api/logout | Logout|

# **deployFree5gcDown**
> MessageResponse deployFree5gcDown()

Fails with 409 if gNB is still running - gNB must be stopped first.

### Example

```typescript
import {
    DefaultApi,
    Configuration
} from './api';

const configuration = new Configuration();
const apiInstance = new DefaultApi(configuration);

const { status, data } = await apiInstance.deployFree5gcDown();
```

### Parameters
This endpoint does not have any parameters.


### Return type

**MessageResponse**

### Authorization

[bearerAuth](../README.md#bearerAuth)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
|**200** | OK |  -  |
|**401** | Unauthorized |  -  |
|**409** | Conflict - gNB is still running |  -  |
|**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **deployFree5gcLogs**
> DeployLogsResponse deployFree5gcLogs()


### Example

```typescript
import {
    DefaultApi,
    Configuration
} from './api';

const configuration = new Configuration();
const apiInstance = new DefaultApi(configuration);

let service: string; //Filter logs down to one compose service (NF), e.g. \"amf\". Omit for every service in the project. (optional) (default to undefined)

const { status, data } = await apiInstance.deployFree5gcLogs(
    service
);
```

### Parameters

|Name | Type | Description  | Notes|
|------------- | ------------- | ------------- | -------------|
| **service** | [**string**] | Filter logs down to one compose service (NF), e.g. \&quot;amf\&quot;. Omit for every service in the project. | (optional) defaults to undefined|


### Return type

**DeployLogsResponse**

### Authorization

[bearerAuth](../README.md#bearerAuth)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
|**200** | OK |  -  |
|**401** | Unauthorized |  -  |
|**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **deployFree5gcStatus**
> DeployStatusResponse deployFree5gcStatus()


### Example

```typescript
import {
    DefaultApi,
    Configuration
} from './api';

const configuration = new Configuration();
const apiInstance = new DefaultApi(configuration);

const { status, data } = await apiInstance.deployFree5gcStatus();
```

### Parameters
This endpoint does not have any parameters.


### Return type

**DeployStatusResponse**

### Authorization

[bearerAuth](../README.md#bearerAuth)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
|**200** | OK |  -  |
|**401** | Unauthorized |  -  |
|**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **deployFree5gcUp**
> MessageResponse deployFree5gcUp()

Deploys one of free5gc\'s compose templates (see RequestDeployFree5gc.template). Omitting the request body deploys \"basic\".

### Example

```typescript
import {
    DefaultApi,
    Configuration,
    RequestDeployFree5gc
} from './api';

const configuration = new Configuration();
const apiInstance = new DefaultApi(configuration);

let requestDeployFree5gc: RequestDeployFree5gc; // (optional)

const { status, data } = await apiInstance.deployFree5gcUp(
    requestDeployFree5gc
);
```

### Parameters

|Name | Type | Description  | Notes|
|------------- | ------------- | ------------- | -------------|
| **requestDeployFree5gc** | **RequestDeployFree5gc**|  | |


### Return type

**MessageResponse**

### Authorization

[bearerAuth](../README.md#bearerAuth)

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
|**200** | OK |  -  |
|**400** | Bad Request - unknown template |  -  |
|**401** | Unauthorized |  -  |
|**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **deployGnbDown**
> MessageResponse deployGnbDown()

Fails with 409 if any UE instance is still running - all UE instances must be stopped first.

### Example

```typescript
import {
    DefaultApi,
    Configuration
} from './api';

const configuration = new Configuration();
const apiInstance = new DefaultApi(configuration);

const { status, data } = await apiInstance.deployGnbDown();
```

### Parameters
This endpoint does not have any parameters.


### Return type

**MessageResponse**

### Authorization

[bearerAuth](../README.md#bearerAuth)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
|**200** | OK |  -  |
|**401** | Unauthorized |  -  |
|**409** | Conflict - a UE instance is still running |  -  |
|**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **deployGnbLogs**
> DeployLogsResponse deployGnbLogs()


### Example

```typescript
import {
    DefaultApi,
    Configuration
} from './api';

const configuration = new Configuration();
const apiInstance = new DefaultApi(configuration);

let service: string; //Filter logs down to one compose service (NF), e.g. \"amf\". Omit for every service in the project. (optional) (default to undefined)

const { status, data } = await apiInstance.deployGnbLogs(
    service
);
```

### Parameters

|Name | Type | Description  | Notes|
|------------- | ------------- | ------------- | -------------|
| **service** | [**string**] | Filter logs down to one compose service (NF), e.g. \&quot;amf\&quot;. Omit for every service in the project. | (optional) defaults to undefined|


### Return type

**DeployLogsResponse**

### Authorization

[bearerAuth](../README.md#bearerAuth)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
|**200** | OK |  -  |
|**401** | Unauthorized |  -  |
|**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **deployGnbSliceDown**
> MessageResponse deployGnbSliceDown()

Fails with 409 if any UE instance is still running - all UE instances must be stopped first.

### Example

```typescript
import {
    DefaultApi,
    Configuration
} from './api';

const configuration = new Configuration();
const apiInstance = new DefaultApi(configuration);

let slice: 'slice1' | 'slice2'; //Which ulcl-2slice slice\'s gNB to act on. (default to undefined)

const { status, data } = await apiInstance.deployGnbSliceDown(
    slice
);
```

### Parameters

|Name | Type | Description  | Notes|
|------------- | ------------- | ------------- | -------------|
| **slice** | [**&#39;slice1&#39; | &#39;slice2&#39;**]**Array<&#39;slice1&#39; &#124; &#39;slice2&#39;>** | Which ulcl-2slice slice\&#39;s gNB to act on. | defaults to undefined|


### Return type

**MessageResponse**

### Authorization

[bearerAuth](../README.md#bearerAuth)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
|**200** | OK |  -  |
|**401** | Unauthorized |  -  |
|**404** | Not Found - unknown slice |  -  |
|**409** | Conflict - a UE instance is still running |  -  |
|**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **deployGnbSliceLogs**
> DeployLogsResponse deployGnbSliceLogs()


### Example

```typescript
import {
    DefaultApi,
    Configuration
} from './api';

const configuration = new Configuration();
const apiInstance = new DefaultApi(configuration);

let slice: 'slice1' | 'slice2'; //Which ulcl-2slice slice\'s gNB to act on. (default to undefined)

const { status, data } = await apiInstance.deployGnbSliceLogs(
    slice
);
```

### Parameters

|Name | Type | Description  | Notes|
|------------- | ------------- | ------------- | -------------|
| **slice** | [**&#39;slice1&#39; | &#39;slice2&#39;**]**Array<&#39;slice1&#39; &#124; &#39;slice2&#39;>** | Which ulcl-2slice slice\&#39;s gNB to act on. | defaults to undefined|


### Return type

**DeployLogsResponse**

### Authorization

[bearerAuth](../README.md#bearerAuth)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
|**200** | OK |  -  |
|**401** | Unauthorized |  -  |
|**404** | Not Found - unknown slice |  -  |
|**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **deployGnbSliceStatus**
> DeployStatusResponse deployGnbSliceStatus()


### Example

```typescript
import {
    DefaultApi,
    Configuration
} from './api';

const configuration = new Configuration();
const apiInstance = new DefaultApi(configuration);

let slice: 'slice1' | 'slice2'; //Which ulcl-2slice slice\'s gNB to act on. (default to undefined)

const { status, data } = await apiInstance.deployGnbSliceStatus(
    slice
);
```

### Parameters

|Name | Type | Description  | Notes|
|------------- | ------------- | ------------- | -------------|
| **slice** | [**&#39;slice1&#39; | &#39;slice2&#39;**]**Array<&#39;slice1&#39; &#124; &#39;slice2&#39;>** | Which ulcl-2slice slice\&#39;s gNB to act on. | defaults to undefined|


### Return type

**DeployStatusResponse**

### Authorization

[bearerAuth](../README.md#bearerAuth)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
|**200** | OK |  -  |
|**401** | Unauthorized |  -  |
|**404** | Not Found - unknown slice |  -  |
|**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **deployGnbSliceUp**
> MessageResponse deployGnbSliceUp()

Deploys the gNB dedicated to one slice of the ulcl-2slice free5gc template. Unlike /api/deploy/gnb, gnb-slice1 and gnb-slice2 are independent and can both be deployed at the same time. Fails with 409 if the core network isn\'t running yet.

### Example

```typescript
import {
    DefaultApi,
    Configuration
} from './api';

const configuration = new Configuration();
const apiInstance = new DefaultApi(configuration);

let slice: 'slice1' | 'slice2'; //Which ulcl-2slice slice\'s gNB to act on. (default to undefined)

const { status, data } = await apiInstance.deployGnbSliceUp(
    slice
);
```

### Parameters

|Name | Type | Description  | Notes|
|------------- | ------------- | ------------- | -------------|
| **slice** | [**&#39;slice1&#39; | &#39;slice2&#39;**]**Array<&#39;slice1&#39; &#124; &#39;slice2&#39;>** | Which ulcl-2slice slice\&#39;s gNB to act on. | defaults to undefined|


### Return type

**MessageResponse**

### Authorization

[bearerAuth](../README.md#bearerAuth)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
|**200** | OK |  -  |
|**401** | Unauthorized |  -  |
|**404** | Not Found - unknown slice |  -  |
|**409** | Conflict - the core network isn\&#39;t running yet |  -  |
|**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **deployGnbStatus**
> DeployStatusResponse deployGnbStatus()


### Example

```typescript
import {
    DefaultApi,
    Configuration
} from './api';

const configuration = new Configuration();
const apiInstance = new DefaultApi(configuration);

const { status, data } = await apiInstance.deployGnbStatus();
```

### Parameters
This endpoint does not have any parameters.


### Return type

**DeployStatusResponse**

### Authorization

[bearerAuth](../README.md#bearerAuth)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
|**200** | OK |  -  |
|**401** | Unauthorized |  -  |
|**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **deployGnbUp**
> MessageResponse deployGnbUp()

Fails with 409 if the core network isn\'t running yet - deploy free5gc first. This is the singleton gNB used by the basic/ulcl free5gc templates - see /api/deploy/gnb-slice/{slice} for ulcl-2slice.

### Example

```typescript
import {
    DefaultApi,
    Configuration
} from './api';

const configuration = new Configuration();
const apiInstance = new DefaultApi(configuration);

const { status, data } = await apiInstance.deployGnbUp();
```

### Parameters
This endpoint does not have any parameters.


### Return type

**MessageResponse**

### Authorization

[bearerAuth](../README.md#bearerAuth)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
|**200** | OK |  -  |
|**401** | Unauthorized |  -  |
|**409** | Conflict - the core network isn\&#39;t running yet |  -  |
|**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **deployUeDown**
> MessageResponse deployUeDown()


### Example

```typescript
import {
    DefaultApi,
    Configuration
} from './api';

const configuration = new Configuration();
const apiInstance = new DefaultApi(configuration);

let instance: string; //The subscriber\'s UE ID (IMSI), used as the UE instance identifier. (default to undefined)

const { status, data } = await apiInstance.deployUeDown(
    instance
);
```

### Parameters

|Name | Type | Description  | Notes|
|------------- | ------------- | ------------- | -------------|
| **instance** | [**string**] | The subscriber\&#39;s UE ID (IMSI), used as the UE instance identifier. | defaults to undefined|


### Return type

**MessageResponse**

### Authorization

[bearerAuth](../README.md#bearerAuth)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
|**200** | OK |  -  |
|**401** | Unauthorized |  -  |
|**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **deployUeList**
> DeployUeListResponse deployUeList()


### Example

```typescript
import {
    DefaultApi,
    Configuration
} from './api';

const configuration = new Configuration();
const apiInstance = new DefaultApi(configuration);

const { status, data } = await apiInstance.deployUeList();
```

### Parameters
This endpoint does not have any parameters.


### Return type

**DeployUeListResponse**

### Authorization

[bearerAuth](../README.md#bearerAuth)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
|**200** | OK |  -  |
|**401** | Unauthorized |  -  |
|**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **deployUeLogs**
> DeployUeLogsResponse deployUeLogs()


### Example

```typescript
import {
    DefaultApi,
    Configuration
} from './api';

const configuration = new Configuration();
const apiInstance = new DefaultApi(configuration);

let instance: string; //The subscriber\'s UE ID (IMSI), used as the UE instance identifier. (default to undefined)

const { status, data } = await apiInstance.deployUeLogs(
    instance
);
```

### Parameters

|Name | Type | Description  | Notes|
|------------- | ------------- | ------------- | -------------|
| **instance** | [**string**] | The subscriber\&#39;s UE ID (IMSI), used as the UE instance identifier. | defaults to undefined|


### Return type

**DeployUeLogsResponse**

### Authorization

[bearerAuth](../README.md#bearerAuth)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
|**200** | OK |  -  |
|**401** | Unauthorized |  -  |
|**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **deployUeStatus**
> DeployUeStatusResponse deployUeStatus()


### Example

```typescript
import {
    DefaultApi,
    Configuration
} from './api';

const configuration = new Configuration();
const apiInstance = new DefaultApi(configuration);

let instance: string; //The subscriber\'s UE ID (IMSI), used as the UE instance identifier. (default to undefined)

const { status, data } = await apiInstance.deployUeStatus(
    instance
);
```

### Parameters

|Name | Type | Description  | Notes|
|------------- | ------------- | ------------- | -------------|
| **instance** | [**string**] | The subscriber\&#39;s UE ID (IMSI), used as the UE instance identifier. | defaults to undefined|


### Return type

**DeployUeStatusResponse**

### Authorization

[bearerAuth](../README.md#bearerAuth)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
|**200** | OK |  -  |
|**401** | Unauthorized |  -  |
|**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **deployUeUp**
> MessageResponse deployUeUp(deployUeRequest)

Fails with 409 if gNB isn\'t running yet - deploy gNB first.

### Example

```typescript
import {
    DefaultApi,
    Configuration,
    DeployUeRequest
} from './api';

const configuration = new Configuration();
const apiInstance = new DefaultApi(configuration);

let instance: string; //The subscriber\'s UE ID (IMSI), used as the UE instance identifier. (default to undefined)
let deployUeRequest: DeployUeRequest; //

const { status, data } = await apiInstance.deployUeUp(
    instance,
    deployUeRequest
);
```

### Parameters

|Name | Type | Description  | Notes|
|------------- | ------------- | ------------- | -------------|
| **deployUeRequest** | **DeployUeRequest**|  | |
| **instance** | [**string**] | The subscriber\&#39;s UE ID (IMSI), used as the UE instance identifier. | defaults to undefined|


### Return type

**MessageResponse**

### Authorization

[bearerAuth](../README.md#bearerAuth)

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
|**200** | OK |  -  |
|**400** | Bad Request |  -  |
|**401** | Unauthorized |  -  |
|**409** | Conflict - gNB isn\&#39;t running yet |  -  |
|**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **imageList**
> ImageListResponse imageList()

Reports every image pinned by the compose templates (free5gc\'s NFs, mongo, and free-ran-ue), and whether each is currently present in the local docker image cache.

### Example

```typescript
import {
    DefaultApi,
    Configuration
} from './api';

const configuration = new Configuration();
const apiInstance = new DefaultApi(configuration);

const { status, data } = await apiInstance.imageList();
```

### Parameters
This endpoint does not have any parameters.


### Return type

**ImageListResponse**

### Authorization

[bearerAuth](../README.md#bearerAuth)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
|**200** | OK |  -  |
|**401** | Unauthorized |  -  |
|**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **imagePull**
> MessageResponse imagePull()

Re-pulls the image\'s pinned tag from its registry.

### Example

```typescript
import {
    DefaultApi,
    Configuration
} from './api';

const configuration = new Configuration();
const apiInstance = new DefaultApi(configuration);

let key: string; //The image\'s stable route key, as returned by GET /api/images (e.g. \"amf\", \"mongo\", \"free-ran-ue\"). (default to undefined)

const { status, data } = await apiInstance.imagePull(
    key
);
```

### Parameters

|Name | Type | Description  | Notes|
|------------- | ------------- | ------------- | -------------|
| **key** | [**string**] | The image\&#39;s stable route key, as returned by GET /api/images (e.g. \&quot;amf\&quot;, \&quot;mongo\&quot;, \&quot;free-ran-ue\&quot;). | defaults to undefined|


### Return type

**MessageResponse**

### Authorization

[bearerAuth](../README.md#bearerAuth)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
|**200** | OK |  -  |
|**401** | Unauthorized |  -  |
|**404** | Not Found - unknown image key |  -  |
|**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **imageRemove**
> MessageResponse imageRemove()

Removes the image from the local docker image cache, forcing the next deploy to pull it fresh. Fails if a container is currently using it.

### Example

```typescript
import {
    DefaultApi,
    Configuration
} from './api';

const configuration = new Configuration();
const apiInstance = new DefaultApi(configuration);

let key: string; //The image\'s stable route key, as returned by GET /api/images (e.g. \"amf\", \"mongo\", \"free-ran-ue\"). (default to undefined)

const { status, data } = await apiInstance.imageRemove(
    key
);
```

### Parameters

|Name | Type | Description  | Notes|
|------------- | ------------- | ------------- | -------------|
| **key** | [**string**] | The image\&#39;s stable route key, as returned by GET /api/images (e.g. \&quot;amf\&quot;, \&quot;mongo\&quot;, \&quot;free-ran-ue\&quot;). | defaults to undefined|


### Return type

**MessageResponse**

### Authorization

[bearerAuth](../README.md#bearerAuth)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
|**200** | OK |  -  |
|**401** | Unauthorized |  -  |
|**404** | Not Found - unknown image key |  -  |
|**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **login**
> LoginResponse login(loginRequest)


### Example

```typescript
import {
    DefaultApi,
    Configuration,
    LoginRequest
} from './api';

const configuration = new Configuration();
const apiInstance = new DefaultApi(configuration);

let loginRequest: LoginRequest; //

const { status, data } = await apiInstance.login(
    loginRequest
);
```

### Parameters

|Name | Type | Description  | Notes|
|------------- | ------------- | ------------- | -------------|
| **loginRequest** | **LoginRequest**|  | |


### Return type

**LoginResponse**

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
|**200** | OK |  -  |
|**400** | Bad Request |  -  |
|**401** | Unauthorized |  -  |
|**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **logout**
> logout()


### Example

```typescript
import {
    DefaultApi,
    Configuration
} from './api';

const configuration = new Configuration();
const apiInstance = new DefaultApi(configuration);

const { status, data } = await apiInstance.logout();
```

### Parameters
This endpoint does not have any parameters.


### Return type

void (empty response body)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: Not defined


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
|**204** | No Content |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

