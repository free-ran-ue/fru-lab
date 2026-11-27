# DefaultApi

All URIs are relative to *http://127.0.0.1:5000*

|Method | HTTP request | Description|
|------------- | ------------- | -------------|
|[**coreNetworkCreate**](#corenetworkcreate) | **POST** /api/core-networks | Save a new core network|
|[**coreNetworkDelete**](#corenetworkdelete) | **DELETE** /api/core-networks/{id} | Delete a core network|
|[**coreNetworkList**](#corenetworklist) | **GET** /api/core-networks | List the saved core networks|
|[**coreNetworkUpdate**](#corenetworkupdate) | **PUT** /api/core-networks/{id} | Update a core network|
|[**deployCoreDown**](#deploycoredown) | **DELETE** /api/deploy/cores/{id} | Stop a free5GC core network|
|[**deployCoreList**](#deploycorelist) | **GET** /api/deploy/cores | List the status of every free5GC core network|
|[**deployCoreLogs**](#deploycorelogs) | **GET** /api/deploy/cores/{id}/logs | Get the logs of a free5GC core network|
|[**deployCoreUp**](#deploycoreup) | **POST** /api/deploy/cores/{id} | Deploy a free5GC core network|
|[**deployFree5gcDown**](#deployfree5gcdown) | **DELETE** /api/deploy/free5gc | Stop free5GC|
|[**deployFree5gcLogs**](#deployfree5gclogs) | **GET** /api/deploy/free5gc/logs | Get free5GC container logs|
|[**deployFree5gcStatus**](#deployfree5gcstatus) | **GET** /api/deploy/free5gc/status | Get free5GC deploy status|
|[**deployFree5gcUp**](#deployfree5gcup) | **POST** /api/deploy/free5gc | Deploy free5GC|
|[**deployGnbDown**](#deploygnbdown) | **DELETE** /api/deploy/gnbs/{profileId} | Stop the gNB of one gNB profile|
|[**deployGnbList**](#deploygnblist) | **GET** /api/deploy/gnbs | List the gNBs deployed from gNB profiles|
|[**deployGnbLogs**](#deploygnblogs) | **GET** /api/deploy/gnbs/{profileId}/logs | Get the logs of one profile gNB|
|[**deployGnbUp**](#deploygnbup) | **POST** /api/deploy/gnbs/{profileId} | Deploy the gNB of one gNB profile|
|[**deployUeDown**](#deployuedown) | **DELETE** /api/deploy/ue/{instance} | Stop a UE instance|
|[**deployUeList**](#deployuelist) | **GET** /api/deploy/ue | List all deployed UE instances|
|[**deployUeLogs**](#deployuelogs) | **GET** /api/deploy/ue/{instance}/logs | Get a UE instance\&#39;s container logs|
|[**deployUeProfileUp**](#deployueprofileup) | **POST** /api/deploy/ue-profiles/{profileId} | Deploy the UE of one UE profile|
|[**deployUeStatus**](#deployuestatus) | **GET** /api/deploy/ue/{instance}/status | Get a UE instance\&#39;s deploy status|
|[**deployUeUp**](#deployueup) | **POST** /api/deploy/ue/{instance} | Deploy a UE instance for one subscriber|
|[**gnbProfileBulkCreate**](#gnbprofilebulkcreate) | **POST** /api/profiles/gnb/bulk | Create several gnb profiles from one|
|[**gnbProfileCheck**](#gnbprofilecheck) | **POST** /api/profiles/gnb/check | Check a gnb profile for conflicts with the saved ones|
|[**gnbProfileCreate**](#gnbprofilecreate) | **POST** /api/profiles/gnb | Save a new gNB profile|
|[**gnbProfileDelete**](#gnbprofiledelete) | **DELETE** /api/profiles/gnb/{id} | Delete a gNB profile|
|[**gnbProfileList**](#gnbprofilelist) | **GET** /api/profiles/gnb | List the saved gNB profiles|
|[**gnbProfileRender**](#gnbprofilerender) | **POST** /api/profiles/gnb/render | Preview the config file a gNB profile deploys|
|[**gnbProfileUpdate**](#gnbprofileupdate) | **PUT** /api/profiles/gnb/{id} | Update a gNB profile|
|[**hostInterfaceList**](#hostinterfacelist) | **GET** /api/host/interfaces | List the host\&#39;s network interfaces|
|[**imageList**](#imagelist) | **GET** /api/images | List the images this app deploys|
|[**imagePull**](#imagepull) | **POST** /api/images/{key}/pull | Pull an image|
|[**imageRemove**](#imageremove) | **DELETE** /api/images/{key} | Clear a locally cached image|
|[**ipamView**](#ipamview) | **GET** /api/ipam | The lab\&#39;s address plan and who holds what|
|[**login**](#login) | **POST** /api/login | Login|
|[**logout**](#logout) | **POST** /api/logout | Logout|
|[**ueProfileBulkCreate**](#ueprofilebulkcreate) | **POST** /api/profiles/ue/bulk | Create several ue profiles from one|
|[**ueProfileCheck**](#ueprofilecheck) | **POST** /api/profiles/ue/check | Check a ue profile for conflicts with the saved ones|
|[**ueProfileCreate**](#ueprofilecreate) | **POST** /api/profiles/ue | Save a new UE profile|
|[**ueProfileDelete**](#ueprofiledelete) | **DELETE** /api/profiles/ue/{id} | Delete a UE profile|
|[**ueProfileList**](#ueprofilelist) | **GET** /api/profiles/ue | List the saved UE profiles|
|[**ueProfileRender**](#ueprofilerender) | **POST** /api/profiles/ue/render | Preview the config file a UE profile deploys|
|[**ueProfileUpdate**](#ueprofileupdate) | **PUT** /api/profiles/ue/{id} | Update a UE profile|

# **coreNetworkCreate**
> CoreNetwork coreNetworkCreate(coreNetwork)

Fails with 409 when the name (compared ignoring case) is the lab core\'s (free5GC) or another core network\'s, or (kind free5gc) when all 9 slots are taken.

### Example

```typescript
import {
    DefaultApi,
    Configuration,
    CoreNetwork
} from './api';

const configuration = new Configuration();
const apiInstance = new DefaultApi(configuration);

let coreNetwork: CoreNetwork; //

const { status, data } = await apiInstance.coreNetworkCreate(
    coreNetwork
);
```

### Parameters

|Name | Type | Description  | Notes|
|------------- | ------------- | ------------- | -------------|
| **coreNetwork** | **CoreNetwork**|  | |


### Return type

**CoreNetwork**

### Authorization

[bearerAuth](../README.md#bearerAuth)

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
|**200** | The saved core network, with its id |  -  |
|**400** | Bad Request - a field is malformed |  -  |
|**409** | Conflict - the name is taken, or every free5gc slot is |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **coreNetworkDelete**
> MessageResponse coreNetworkDelete()

A free5gc core network cannot be deleted while it is deployed (409).

### Example

```typescript
import {
    DefaultApi,
    Configuration
} from './api';

const configuration = new Configuration();
const apiInstance = new DefaultApi(configuration);

let id: string; // (default to undefined)

const { status, data } = await apiInstance.coreNetworkDelete(
    id
);
```

### Parameters

|Name | Type | Description  | Notes|
|------------- | ------------- | ------------- | -------------|
| **id** | [**string**] |  | defaults to undefined|


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
|**409** | Conflict - the free5gc core network is deployed; stop it first |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **coreNetworkList**
> Array<CoreNetwork> coreNetworkList()

5G cores besides the lab free5GC that gNBs can attach to - external ones and free5gc ones fru-lab deploys on this host; the lab free5GC is not among them.

### Example

```typescript
import {
    DefaultApi,
    Configuration
} from './api';

const configuration = new Configuration();
const apiInstance = new DefaultApi(configuration);

const { status, data } = await apiInstance.coreNetworkList();
```

### Parameters
This endpoint does not have any parameters.


### Return type

**Array<CoreNetwork>**

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

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **coreNetworkUpdate**
> CoreNetwork coreNetworkUpdate(coreNetwork)

The kind cannot change (400); a kind left out keeps the saved one. A free5gc core keeps its slot.

### Example

```typescript
import {
    DefaultApi,
    Configuration,
    CoreNetwork
} from './api';

const configuration = new Configuration();
const apiInstance = new DefaultApi(configuration);

let id: string; // (default to undefined)
let coreNetwork: CoreNetwork; //

const { status, data } = await apiInstance.coreNetworkUpdate(
    id,
    coreNetwork
);
```

### Parameters

|Name | Type | Description  | Notes|
|------------- | ------------- | ------------- | -------------|
| **coreNetwork** | **CoreNetwork**|  | |
| **id** | [**string**] |  | defaults to undefined|


### Return type

**CoreNetwork**

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
|**404** | No such core network |  -  |
|**409** | Conflict - the name is taken |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **deployCoreDown**
> MessageResponse deployCoreDown()

409 while a gNB attached to it runs.

### Example

```typescript
import {
    DefaultApi,
    Configuration
} from './api';

const configuration = new Configuration();
const apiInstance = new DefaultApi(configuration);

let id: string; //A core network of kind free5gc. (default to undefined)

const { status, data } = await apiInstance.deployCoreDown(
    id
);
```

### Parameters

|Name | Type | Description  | Notes|
|------------- | ------------- | ------------- | -------------|
| **id** | [**string**] | A core network of kind free5gc. | defaults to undefined|


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
|**404** | No such core network |  -  |
|**409** | Conflict - a gNB on it still runs, or the core network is not of kind free5gc |  -  |
|**401** | Unauthorized |  -  |
|**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **deployCoreList**
> DeployCoreListResponse deployCoreList()

One entry per core network of kind free5gc (the extra free5GC instances on this host). The lab\'s own free5GC stays under /api/deploy/free5gc.

### Example

```typescript
import {
    DefaultApi,
    Configuration
} from './api';

const configuration = new Configuration();
const apiInstance = new DefaultApi(configuration);

const { status, data } = await apiInstance.deployCoreList();
```

### Parameters
This endpoint does not have any parameters.


### Return type

**DeployCoreListResponse**

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

# **deployCoreLogs**
> DeployLogsResponse deployCoreLogs()

The live log stream is /api/stream/logs?target=core-<id> (service= narrows it to one NF).

### Example

```typescript
import {
    DefaultApi,
    Configuration
} from './api';

const configuration = new Configuration();
const apiInstance = new DefaultApi(configuration);

let id: string; //A core network of kind free5gc. (default to undefined)

const { status, data } = await apiInstance.deployCoreLogs(
    id
);
```

### Parameters

|Name | Type | Description  | Notes|
|------------- | ------------- | ------------- | -------------|
| **id** | [**string**] | A core network of kind free5gc. | defaults to undefined|


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
|**404** | No such core network |  -  |
|**409** | Conflict - the core network is not of kind free5gc |  -  |
|**401** | Unauthorized |  -  |
|**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **deployCoreUp**
> MessageResponse deployCoreUp()

Brings up the free5GC of core network id (kind free5gc) from a template on its own networks - a cn-ran network 10.0.(100+slot).0/24 with the AMF at .3 and the UPF at .5 (as the lab free5GC on 10.0.1.0/24), an internal SBI network, and its webconsole on host port deploy.webconsole.port + slot. 409 if it already runs or the core is not of kind free5gc.

### Example

```typescript
import {
    DefaultApi,
    Configuration,
    RequestDeployFree5gc
} from './api';

const configuration = new Configuration();
const apiInstance = new DefaultApi(configuration);

let id: string; //A core network of kind free5gc. (default to undefined)
let requestDeployFree5gc: RequestDeployFree5gc; // (optional)

const { status, data } = await apiInstance.deployCoreUp(
    id,
    requestDeployFree5gc
);
```

### Parameters

|Name | Type | Description  | Notes|
|------------- | ------------- | ------------- | -------------|
| **requestDeployFree5gc** | **RequestDeployFree5gc**|  | |
| **id** | [**string**] | A core network of kind free5gc. | defaults to undefined|


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
|**404** | No such core network |  -  |
|**409** | Conflict |  -  |
|**401** | Unauthorized |  -  |
|**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

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

Fails with 409 while a UE deployed onto this gNB still runs.

### Example

```typescript
import {
    DefaultApi,
    Configuration
} from './api';

const configuration = new Configuration();
const apiInstance = new DefaultApi(configuration);

let profileId: string; // (default to undefined)

const { status, data } = await apiInstance.deployGnbDown(
    profileId
);
```

### Parameters

|Name | Type | Description  | Notes|
|------------- | ------------- | ------------- | -------------|
| **profileId** | [**string**] |  | defaults to undefined|


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
|**409** | Conflict - a UE on this gNB still runs |  -  |
|**401** | Unauthorized |  -  |
|**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **deployGnbList**
> DeployGnbListResponse deployGnbList()

One entry per gNB profile that has been deployed and not stopped since (a stopped one is removed).

### Example

```typescript
import {
    DefaultApi,
    Configuration
} from './api';

const configuration = new Configuration();
const apiInstance = new DefaultApi(configuration);

const { status, data } = await apiInstance.deployGnbList();
```

### Parameters
This endpoint does not have any parameters.


### Return type

**DeployGnbListResponse**

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

let profileId: string; // (default to undefined)

const { status, data } = await apiInstance.deployGnbLogs(
    profileId
);
```

### Parameters

|Name | Type | Description  | Notes|
|------------- | ------------- | ------------- | -------------|
| **profileId** | [**string**] |  | defaults to undefined|


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

# **deployGnbUp**
> MessageResponse deployGnbUp()

Every gNB profile is one gNB; several run at once. Its ran-ue address (where UEs dial it) is picked by fru-lab. On the lab free5GC or a free5gc core network its N2/N3 address is assigned on deploy (the lowest free one from .20 up on that core\'s cn-ran network, shown in GnbDeployment.n2Ip); on an external core it is the profile\'s typed-in hostIp, which must be an address of the core\'s n2Interface (or of hostInterface for an external core given by address). Fails with 409 if it already runs, if its free5GC core (lab or free5gc core network) is not running, if its external core network is disabled or has no n2Interface, if the hostIp is missing or not on that interface, or if a running gNB already uses its gNB ID or host address.

### Example

```typescript
import {
    DefaultApi,
    Configuration,
    DeployGnbRequest
} from './api';

const configuration = new Configuration();
const apiInstance = new DefaultApi(configuration);

let profileId: string; // (default to undefined)
let deployGnbRequest: DeployGnbRequest; // (optional)

const { status, data } = await apiInstance.deployGnbUp(
    profileId,
    deployGnbRequest
);
```

### Parameters

|Name | Type | Description  | Notes|
|------------- | ------------- | ------------- | -------------|
| **deployGnbRequest** | **DeployGnbRequest**|  | |
| **profileId** | [**string**] |  | defaults to undefined|


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
|**404** | No such gNB profile |  -  |
|**409** | Conflict |  -  |
|**401** | Unauthorized |  -  |
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

# **deployUeProfileUp**
> MessageResponse deployUeProfileUp()

Every UE profile is one UE (instance imsi-<mcc><mnc><msin>). gnb picks the gNB it attaches to; without it fru-lab picks a running gNB whose SD matches the profile\'s, else any running gNB. Fails with 409 if no gNB runs, the picked gNB does not run, or the UE already runs.

### Example

```typescript
import {
    DefaultApi,
    Configuration,
    DeployUeProfileRequest
} from './api';

const configuration = new Configuration();
const apiInstance = new DefaultApi(configuration);

let profileId: string; // (default to undefined)
let deployUeProfileRequest: DeployUeProfileRequest; // (optional)

const { status, data } = await apiInstance.deployUeProfileUp(
    profileId,
    deployUeProfileRequest
);
```

### Parameters

|Name | Type | Description  | Notes|
|------------- | ------------- | ------------- | -------------|
| **deployUeProfileRequest** | **DeployUeProfileRequest**|  | |
| **profileId** | [**string**] |  | defaults to undefined|


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
|**404** | No such UE profile |  -  |
|**409** | Conflict |  -  |
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

# **gnbProfileBulkCreate**
> Array<GnbProfile> gnbProfileBulkCreate(profileBulkRequest)

Copies the source profile count times; the fields that must be unique count up to the next free value (gNB ID and N2 IP; lab core only). Names get a -2, -3, ... suffix.

### Example

```typescript
import {
    DefaultApi,
    Configuration,
    ProfileBulkRequest
} from './api';

const configuration = new Configuration();
const apiInstance = new DefaultApi(configuration);

let profileBulkRequest: ProfileBulkRequest; //

const { status, data } = await apiInstance.gnbProfileBulkCreate(
    profileBulkRequest
);
```

### Parameters

|Name | Type | Description  | Notes|
|------------- | ------------- | ------------- | -------------|
| **profileBulkRequest** | **ProfileBulkRequest**|  | |


### Return type

**Array<GnbProfile>**

### Authorization

[bearerAuth](../README.md#bearerAuth)

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
|**200** | The created profiles |  -  |
|**400** | Bad Request |  -  |
|**404** | No such source profile |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **gnbProfileCheck**
> ProfileCheckResult gnbProfileCheck(gnbProfile)

The fields that must be unique are compared with every other saved profile (the one with the same id is skipped). Saving runs the same check and fails with 409 when it finds any.

### Example

```typescript
import {
    DefaultApi,
    Configuration,
    GnbProfile
} from './api';

const configuration = new Configuration();
const apiInstance = new DefaultApi(configuration);

let gnbProfile: GnbProfile; //

const { status, data } = await apiInstance.gnbProfileCheck(
    gnbProfile
);
```

### Parameters

|Name | Type | Description  | Notes|
|------------- | ------------- | ------------- | -------------|
| **gnbProfile** | **GnbProfile**|  | |


### Return type

**ProfileCheckResult**

### Authorization

[bearerAuth](../README.md#bearerAuth)

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
|**200** | OK |  -  |
|**400** | Bad Request - a field is malformed |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **gnbProfileCreate**
> GnbProfile gnbProfileCreate(gnbProfile)


### Example

```typescript
import {
    DefaultApi,
    Configuration,
    GnbProfile
} from './api';

const configuration = new Configuration();
const apiInstance = new DefaultApi(configuration);

let gnbProfile: GnbProfile; //

const { status, data } = await apiInstance.gnbProfileCreate(
    gnbProfile
);
```

### Parameters

|Name | Type | Description  | Notes|
|------------- | ------------- | ------------- | -------------|
| **gnbProfile** | **GnbProfile**|  | |


### Return type

**GnbProfile**

### Authorization

[bearerAuth](../README.md#bearerAuth)

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
|**200** | The saved profile, with its id |  -  |
|**400** | Bad Request |  -  |
|**409** | Conflict - a field that must be unique is used by another saved profile (see the check endpoint) |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **gnbProfileDelete**
> MessageResponse gnbProfileDelete()


### Example

```typescript
import {
    DefaultApi,
    Configuration
} from './api';

const configuration = new Configuration();
const apiInstance = new DefaultApi(configuration);

let id: string; // (default to undefined)

const { status, data } = await apiInstance.gnbProfileDelete(
    id
);
```

### Parameters

|Name | Type | Description  | Notes|
|------------- | ------------- | ------------- | -------------|
| **id** | [**string**] |  | defaults to undefined|


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

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **gnbProfileList**
> Array<GnbProfile> gnbProfileList()


### Example

```typescript
import {
    DefaultApi,
    Configuration
} from './api';

const configuration = new Configuration();
const apiInstance = new DefaultApi(configuration);

const { status, data } = await apiInstance.gnbProfileList();
```

### Parameters
This endpoint does not have any parameters.


### Return type

**Array<GnbProfile>**

### Authorization

[bearerAuth](../README.md#bearerAuth)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
|**200** | OK |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **gnbProfileRender**
> RenderedConfig gnbProfileRender(gnbProfile)


### Example

```typescript
import {
    DefaultApi,
    Configuration,
    GnbProfile
} from './api';

const configuration = new Configuration();
const apiInstance = new DefaultApi(configuration);

let gnbProfile: GnbProfile; //

const { status, data } = await apiInstance.gnbProfileRender(
    gnbProfile
);
```

### Parameters

|Name | Type | Description  | Notes|
|------------- | ------------- | ------------- | -------------|
| **gnbProfile** | **GnbProfile**|  | |


### Return type

**RenderedConfig**

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

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **gnbProfileUpdate**
> GnbProfile gnbProfileUpdate(gnbProfile)


### Example

```typescript
import {
    DefaultApi,
    Configuration,
    GnbProfile
} from './api';

const configuration = new Configuration();
const apiInstance = new DefaultApi(configuration);

let id: string; // (default to undefined)
let gnbProfile: GnbProfile; //

const { status, data } = await apiInstance.gnbProfileUpdate(
    id,
    gnbProfile
);
```

### Parameters

|Name | Type | Description  | Notes|
|------------- | ------------- | ------------- | -------------|
| **gnbProfile** | **GnbProfile**|  | |
| **id** | [**string**] |  | defaults to undefined|


### Return type

**GnbProfile**

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
|**409** | Conflict - a field that must be unique is used by another saved profile (see the check endpoint) |  -  |
|**404** | Not Found |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **hostInterfaceList**
> Array<HostInterface> hostInterfaceList()

The interfaces with an IPv4 address an external-core gNB can listen on (loopback and Docker\'s own bridges and veths are left out).

### Example

```typescript
import {
    DefaultApi,
    Configuration
} from './api';

const configuration = new Configuration();
const apiInstance = new DefaultApi(configuration);

const { status, data } = await apiInstance.hostInterfaceList();
```

### Parameters
This endpoint does not have any parameters.


### Return type

**Array<HostInterface>**

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

# **ipamView**
> IpamView ipamView()

Every address pool fru-lab manages (each core\'s cn-ran and SBI networks, the shared ran-ue network) with its reserved addresses, its dynamic ranges and its current leases, reconciled with Docker; plus any warning found (an address held outside fru-lab\'s leases, a host network overlapping a pool).

### Example

```typescript
import {
    DefaultApi,
    Configuration
} from './api';

const configuration = new Configuration();
const apiInstance = new DefaultApi(configuration);

const { status, data } = await apiInstance.ipamView();
```

### Parameters
This endpoint does not have any parameters.


### Return type

**IpamView**

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
|**500** | The leases or the core networks could not be read from the database (Docker being unreachable is a warning, not an error) |  -  |

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

# **ueProfileBulkCreate**
> Array<UeProfile> ueProfileBulkCreate(profileBulkRequest)

Copies the source profile count times; the fields that must be unique count up to the next free value (MSIN). Names get a -2, -3, ... suffix.

### Example

```typescript
import {
    DefaultApi,
    Configuration,
    ProfileBulkRequest
} from './api';

const configuration = new Configuration();
const apiInstance = new DefaultApi(configuration);

let profileBulkRequest: ProfileBulkRequest; //

const { status, data } = await apiInstance.ueProfileBulkCreate(
    profileBulkRequest
);
```

### Parameters

|Name | Type | Description  | Notes|
|------------- | ------------- | ------------- | -------------|
| **profileBulkRequest** | **ProfileBulkRequest**|  | |


### Return type

**Array<UeProfile>**

### Authorization

[bearerAuth](../README.md#bearerAuth)

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
|**200** | The created profiles |  -  |
|**400** | Bad Request |  -  |
|**404** | No such source profile |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **ueProfileCheck**
> ProfileCheckResult ueProfileCheck(ueProfile)

The fields that must be unique are compared with every other saved profile (the one with the same id is skipped). Saving runs the same check and fails with 409 when it finds any.

### Example

```typescript
import {
    DefaultApi,
    Configuration,
    UeProfile
} from './api';

const configuration = new Configuration();
const apiInstance = new DefaultApi(configuration);

let ueProfile: UeProfile; //

const { status, data } = await apiInstance.ueProfileCheck(
    ueProfile
);
```

### Parameters

|Name | Type | Description  | Notes|
|------------- | ------------- | ------------- | -------------|
| **ueProfile** | **UeProfile**|  | |


### Return type

**ProfileCheckResult**

### Authorization

[bearerAuth](../README.md#bearerAuth)

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
|**200** | OK |  -  |
|**400** | Bad Request - a field is malformed |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **ueProfileCreate**
> UeProfile ueProfileCreate(ueProfile)


### Example

```typescript
import {
    DefaultApi,
    Configuration,
    UeProfile
} from './api';

const configuration = new Configuration();
const apiInstance = new DefaultApi(configuration);

let ueProfile: UeProfile; //

const { status, data } = await apiInstance.ueProfileCreate(
    ueProfile
);
```

### Parameters

|Name | Type | Description  | Notes|
|------------- | ------------- | ------------- | -------------|
| **ueProfile** | **UeProfile**|  | |


### Return type

**UeProfile**

### Authorization

[bearerAuth](../README.md#bearerAuth)

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
|**200** | The saved profile, with its id |  -  |
|**400** | Bad Request |  -  |
|**409** | Conflict - a field that must be unique is used by another saved profile (see the check endpoint) |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **ueProfileDelete**
> MessageResponse ueProfileDelete()


### Example

```typescript
import {
    DefaultApi,
    Configuration
} from './api';

const configuration = new Configuration();
const apiInstance = new DefaultApi(configuration);

let id: string; // (default to undefined)

const { status, data } = await apiInstance.ueProfileDelete(
    id
);
```

### Parameters

|Name | Type | Description  | Notes|
|------------- | ------------- | ------------- | -------------|
| **id** | [**string**] |  | defaults to undefined|


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

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **ueProfileList**
> Array<UeProfile> ueProfileList()


### Example

```typescript
import {
    DefaultApi,
    Configuration
} from './api';

const configuration = new Configuration();
const apiInstance = new DefaultApi(configuration);

const { status, data } = await apiInstance.ueProfileList();
```

### Parameters
This endpoint does not have any parameters.


### Return type

**Array<UeProfile>**

### Authorization

[bearerAuth](../README.md#bearerAuth)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
|**200** | OK |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **ueProfileRender**
> RenderedConfig ueProfileRender(ueProfile)


### Example

```typescript
import {
    DefaultApi,
    Configuration,
    UeProfile
} from './api';

const configuration = new Configuration();
const apiInstance = new DefaultApi(configuration);

let ueProfile: UeProfile; //

const { status, data } = await apiInstance.ueProfileRender(
    ueProfile
);
```

### Parameters

|Name | Type | Description  | Notes|
|------------- | ------------- | ------------- | -------------|
| **ueProfile** | **UeProfile**|  | |


### Return type

**RenderedConfig**

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

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **ueProfileUpdate**
> UeProfile ueProfileUpdate(ueProfile)


### Example

```typescript
import {
    DefaultApi,
    Configuration,
    UeProfile
} from './api';

const configuration = new Configuration();
const apiInstance = new DefaultApi(configuration);

let id: string; // (default to undefined)
let ueProfile: UeProfile; //

const { status, data } = await apiInstance.ueProfileUpdate(
    id,
    ueProfile
);
```

### Parameters

|Name | Type | Description  | Notes|
|------------- | ------------- | ------------- | -------------|
| **ueProfile** | **UeProfile**|  | |
| **id** | [**string**] |  | defaults to undefined|


### Return type

**UeProfile**

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
|**409** | Conflict - a field that must be unique is used by another saved profile (see the check endpoint) |  -  |
|**404** | Not Found |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

