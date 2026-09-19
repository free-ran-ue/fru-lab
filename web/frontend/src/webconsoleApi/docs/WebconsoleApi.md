# WebconsoleApi

All URIs are relative to *http://localhost:5000*

|Method | HTTP request | Description|
|------------- | ------------- | -------------|
|[**apiProfileGet**](#apiprofileget) | **GET** /api/profile | Get all profiles|
|[**deleteMultipleSubscribers**](#deletemultiplesubscribers) | **DELETE** /api/subscriber | Delete multiple subscribers|
|[**deleteSubscriberByID**](#deletesubscriberbyid) | **DELETE** /api/subscriber/{ueId}/{servingPlmnId} | Delete a subscriber|
|[**getSubscriberByID**](#getsubscriberbyid) | **GET** /api/subscriber/{ueId}/{servingPlmnId} | Get one subscriber|
|[**getSubscribers**](#getsubscribers) | **GET** /api/subscriber | Get all subscribers|
|[**login**](#login) | **POST** /api/login | Login|
|[**patchSubscriberByID**](#patchsubscriberbyid) | **PATCH** /api/subscriber/{ueId}/{servingPlmnId} | Partially update a subscriber|
|[**postSubscriberByID**](#postsubscriberbyid) | **POST** /api/subscriber/{ueId}/{servingPlmnId} | Create a subscriber|
|[**putSubscriberByID**](#putsubscriberbyid) | **PUT** /api/subscriber/{ueId}/{servingPlmnId} | Replace a subscriber|

# **apiProfileGet**
> Array<ProfileListIE> apiProfileGet()

Returns an array of profile.

### Example

```typescript
import {
    WebconsoleApi,
    Configuration
} from './api';

const configuration = new Configuration();
const apiInstance = new WebconsoleApi(configuration);

let limit: number; // (optional) (default to undefined)
let page: number; // (optional) (default to undefined)

const { status, data } = await apiInstance.apiProfileGet(
    limit,
    page
);
```

### Parameters

|Name | Type | Description  | Notes|
|------------- | ------------- | ------------- | -------------|
| **limit** | [**number**] |  | (optional) defaults to undefined|
| **page** | [**number**] |  | (optional) defaults to undefined|


### Return type

**Array<ProfileListIE>**

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
|**200** | Returns an array of profile. |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **deleteMultipleSubscribers**
> deleteMultipleSubscribers(subscriber)


### Example

```typescript
import {
    WebconsoleApi,
    Configuration
} from './api';

const configuration = new Configuration();
const apiInstance = new WebconsoleApi(configuration);

let subscriber: Array<Subscriber>; //

const { status, data } = await apiInstance.deleteMultipleSubscribers(
    subscriber
);
```

### Parameters

|Name | Type | Description  | Notes|
|------------- | ------------- | ------------- | -------------|
| **subscriber** | **Array<Subscriber>**|  | |


### Return type

void (empty response body)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: Not defined


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
|**204** | No Content |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **deleteSubscriberByID**
> deleteSubscriberByID()


### Example

```typescript
import {
    WebconsoleApi,
    Configuration
} from './api';

const configuration = new Configuration();
const apiInstance = new WebconsoleApi(configuration);

let ueId: string; // (default to undefined)
let servingPlmnId: string; // (default to undefined)

const { status, data } = await apiInstance.deleteSubscriberByID(
    ueId,
    servingPlmnId
);
```

### Parameters

|Name | Type | Description  | Notes|
|------------- | ------------- | ------------- | -------------|
| **ueId** | [**string**] |  | defaults to undefined|
| **servingPlmnId** | [**string**] |  | defaults to undefined|


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

# **getSubscriberByID**
> Subscription getSubscriberByID()


### Example

```typescript
import {
    WebconsoleApi,
    Configuration
} from './api';

const configuration = new Configuration();
const apiInstance = new WebconsoleApi(configuration);

let ueId: string; // (default to undefined)
let servingPlmnId: string; // (default to undefined)

const { status, data } = await apiInstance.getSubscriberByID(
    ueId,
    servingPlmnId
);
```

### Parameters

|Name | Type | Description  | Notes|
|------------- | ------------- | ------------- | -------------|
| **ueId** | [**string**] |  | defaults to undefined|
| **servingPlmnId** | [**string**] |  | defaults to undefined|


### Return type

**Subscription**

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
|**200** | OK |  -  |
|**404** | Not Found |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **getSubscribers**
> Array<Subscriber> getSubscribers()

Returns an array of subscriber.

### Example

```typescript
import {
    WebconsoleApi,
    Configuration
} from './api';

const configuration = new Configuration();
const apiInstance = new WebconsoleApi(configuration);

let limit: number; // (optional) (default to undefined)
let page: number; // (optional) (default to undefined)

const { status, data } = await apiInstance.getSubscribers(
    limit,
    page
);
```

### Parameters

|Name | Type | Description  | Notes|
|------------- | ------------- | ------------- | -------------|
| **limit** | [**number**] |  | (optional) defaults to undefined|
| **page** | [**number**] |  | (optional) defaults to undefined|


### Return type

**Array<Subscriber>**

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
|**200** | Returns an array of subscriber. |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **login**
> LoginResponse login(loginRequest)


### Example

```typescript
import {
    WebconsoleApi,
    Configuration,
    LoginRequest
} from './api';

const configuration = new Configuration();
const apiInstance = new WebconsoleApi(configuration);

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
|**403** | Forbidden |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **patchSubscriberByID**
> patchSubscriberByID(subscription)


### Example

```typescript
import {
    WebconsoleApi,
    Configuration,
    Subscription
} from './api';

const configuration = new Configuration();
const apiInstance = new WebconsoleApi(configuration);

let ueId: string; // (default to undefined)
let servingPlmnId: string; // (default to undefined)
let subscription: Subscription; //

const { status, data } = await apiInstance.patchSubscriberByID(
    ueId,
    servingPlmnId,
    subscription
);
```

### Parameters

|Name | Type | Description  | Notes|
|------------- | ------------- | ------------- | -------------|
| **subscription** | **Subscription**|  | |
| **ueId** | [**string**] |  | defaults to undefined|
| **servingPlmnId** | [**string**] |  | defaults to undefined|


### Return type

void (empty response body)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: Not defined


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
|**204** | No Content |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **postSubscriberByID**
> postSubscriberByID(subscription)


### Example

```typescript
import {
    WebconsoleApi,
    Configuration,
    Subscription
} from './api';

const configuration = new Configuration();
const apiInstance = new WebconsoleApi(configuration);

let ueId: string; // (default to undefined)
let servingPlmnId: string; // (default to undefined)
let subscription: Subscription; //

const { status, data } = await apiInstance.postSubscriberByID(
    ueId,
    servingPlmnId,
    subscription
);
```

### Parameters

|Name | Type | Description  | Notes|
|------------- | ------------- | ------------- | -------------|
| **subscription** | **Subscription**|  | |
| **ueId** | [**string**] |  | defaults to undefined|
| **servingPlmnId** | [**string**] |  | defaults to undefined|


### Return type

void (empty response body)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: Not defined


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
|**200** | OK |  -  |
|**400** | Bad Request |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **putSubscriberByID**
> putSubscriberByID(subscription)


### Example

```typescript
import {
    WebconsoleApi,
    Configuration,
    Subscription
} from './api';

const configuration = new Configuration();
const apiInstance = new WebconsoleApi(configuration);

let ueId: string; // (default to undefined)
let servingPlmnId: string; // (default to undefined)
let subscription: Subscription; //

const { status, data } = await apiInstance.putSubscriberByID(
    ueId,
    servingPlmnId,
    subscription
);
```

### Parameters

|Name | Type | Description  | Notes|
|------------- | ------------- | ------------- | -------------|
| **subscription** | **Subscription**|  | |
| **ueId** | [**string**] |  | defaults to undefined|
| **servingPlmnId** | [**string**] |  | defaults to undefined|


### Return type

void (empty response body)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: Not defined


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
|**204** | No Content |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

