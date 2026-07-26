# API 文档

本文档描述 SMS 接码平台当前后端接口。默认接口前缀为 `/api/v1`，前端 `VITE_API_BASE_URL` 未配置时也使用该前缀。

## 1. 通用约定

### 1.1 Base URL

```text
http://localhost:<HTTP_PORT>/api/v1
```

健康检查接口不带 `/api/v1` 前缀：

```http
GET /health
```

响应：

```json
{
  "status": "ok"
}
```

### 1.2 统一 JSON 响应

除 `GET /admin/card-batches/:id/export.txt` 外，业务接口统一返回：

```json
{
  "code": 0,
  "message": "ok",
  "data": {}
}
```

创建成功时 HTTP 状态码为 `201`，`message` 为 `created`。

错误响应：

```json
{
  "code": 422,
  "message": "card code has expired"
}
```

常见 HTTP 状态码：

| 状态码 | 含义 |
| --- | --- |
| `200` | 请求成功 |
| `201` | 创建成功 |
| `400` | 请求参数缺失或格式错误 |
| `401` | 管理端 Token 缺失、无效或过期 |
| `404` | 资源不存在 |
| `422` | 业务规则校验失败或供应商错误 |
| `429` | 触发限流 |
| `500` | 服务端异常 |

### 1.3 时间格式

接口中的时间字段使用 Go 标准 JSON 时间格式：

```text
2026-06-14T15:52:33+08:00
```

前端可展示为：

```text
2026-06-14 15:52:33
```

### 1.4 管理端鉴权

管理端除登录接口外均需要请求头：

```http
Authorization: Bearer <JWT_TOKEN>
```

Token 由 `POST /api/v1/admin/auth/login` 返回。Token 无效或过期时返回：

```json
{
  "code": 401,
  "message": "invalid authorization token"
}
```

### 1.5 分页约定

部分管理端列表支持：

| 参数 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `limit` | int | `50` | 每页数量，范围 `1-200`，超出后回退为 `50` |
| `offset` | int | `0` | 偏移量，小于 0 时回退为 `0` |

当前列表接口直接返回数组，未额外返回 `total` 字段。

### 1.6 状态枚举

通用启用状态：

| 值 | 说明 |
| --- | --- |
| `enabled` | 启用 |
| `disabled` | 禁用 |
| `voided` | 作废，主要用于卡密 |

订单状态：

| 值 | 说明 |
| --- | --- |
| `created` | 已创建，准备向供应商申请号码 |
| `active` | 已取号，等待验证码 |
| `completed` | 长效接码取号完成，等待用户手动查询短信 |
| `cancel_requested` | 正在取消 |
| `sms_received` | 已收到验证码 |
| `cancelled` | 已取消 |
| `expired` | 已过期 |
| `failed` | 失败 |

公告状态：

| 值 | 说明 |
| --- | --- |
| `draft` | 草稿 |
| `active` | 展示中 |
| `archived` | 已归档 |

公告通知方式：

| 值 | 说明 |
| --- | --- |
| `modal` | 前台访问时弹窗展示 |
| `silent` | 只在公告板展示 |

### 1.7 主要数据结构

#### ServiceConfig

```json
{
  "id": 1,
  "providerCode": "smspool",
  "targetPlatform": "smspool-US-OpenAI",
  "displayName": "OpenAI / ChatGPT",
  "countryCode": "US",
  "countryName": "United States",
  "providerCountryId": "1",
  "providerServiceId": "1012",
  "providerPoolId": "",
  "maxPrice": 0.8,
  "timeoutSeconds": 1200,
  "metadata": {},
  "status": "enabled",
  "createdAt": "2026-06-14T15:52:33+08:00",
  "updatedAt": "2026-06-14T15:52:33+08:00"
}
```

说明：

- `targetPlatform` 是本系统服务标识，建议格式为 `服务商-国家-服务`，不含空格。
- `displayName` 是用户端展示的真实服务名称。
- `providerCountryId`、`providerServiceId` 是供应商侧国家/服务 ID。
- `providerPoolId` 可用于供应商号码池、运营商或长效服务商的额外选择项。
- `timeoutSeconds=0` 表示系统不控制订单超时，常用于长效接码平台。
- `metadata` 用于保存供应商专属配置，例如 `68sms` 的号码有效期。

```json
{
  "validityType": "4",
  "validityLabel": "61 — 92 天",
  "validityMinDays": 61,
  "validityMaxDays": 92,
  "validityStock": 34103
}
```

#### ReceiveOrder

```json
{
  "id": 1001,
  "orderNo": "R202606141552330001",
  "cardCodeId": 10,
  "providerCode": "68sms",
  "serviceConfigId": 3,
  "supplierOrderId": "0123456789abcdef0123456789abcdef",
  "supplierToken": "0123456789abcdef0123456789abcdef",
  "phoneNumber": "+14155550123",
  "phoneCountryCode": "1",
  "phoneNationalNumber": "2722917613",
  "verificationCode": "78890",
  "smsContent": "Telegram code: 78890",
  "cost": 0.7,
  "maxPrice": 0.8,
  "status": "sms_received",
  "supplierStatus": "10000",
  "providerKind": "long_lived",
  "manualCheck": true,
  "messageUrl": "https://api.68sms.com/api/sms/get?key=0123456789abcdef0123456789abcdef",
  "failureReason": "",
  "startedAt": "2026-06-14T15:52:33+08:00",
  "receivedAt": "2026-06-14T15:55:10+08:00",
  "cancelledAt": null,
  "expiredAt": null,
  "createdAt": "2026-06-14T15:52:33+08:00",
  "updatedAt": "2026-06-14T15:55:10+08:00",
  "serviceConfig": {}
}
```

说明：

- 短效接码订单一般是 `active -> sms_received/cancelled/expired/failed`。
- 长效接码订单取号成功后状态为 `completed`，`manualCheck=true`，用户通过 `POST /public/orders/:orderNo/check` 手动查询短信。
- `messageUrl` 是长效接码地址，`68sms` 使用 `/api/sms/get?key=`。

#### Announcement

```json
{
  "id": 1,
  "title": "系统维护通知",
  "content": "今晚 23:00 维护。",
  "status": "active",
  "notifyMode": "modal",
  "readCount": 12,
  "startAt": null,
  "endAt": null,
  "createdBy": 1,
  "createdAt": "2026-06-14T15:52:33+08:00",
  "updatedAt": "2026-06-14T15:52:33+08:00",
  "publishedAt": "2026-06-14T15:52:33+08:00",
  "unread": true
}
```

`unread` 只在公共端公告列表返回。

## 2. 公共端接口

公共端接口不需要登录。公共接口应用基于 IP 的限流：每分钟 120 次。

### 2.1 校验卡密

```http
POST /api/v1/public/cards/verify
Content-Type: application/json
```

请求体：

```json
{
  "cardCode": "QM-EXAMPLE-CODE"
}
```

字段说明：

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `cardCode` | string | 是 | 用户购买的卡密明文 |

成功响应：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "codeMask": "QM************1234",
    "remainingUses": 1,
    "expiresAt": null,
    "serviceConfig": {
      "id": 3,
      "providerCode": "68sms",
      "targetPlatform": "68sms-US-Telegram",
      "displayName": "Telegram",
      "countryCode": "US",
      "countryName": "United States",
      "providerCountryId": "188",
      "providerServiceId": "2",
      "maxPrice": 0.7,
      "timeoutSeconds": 0,
      "metadata": {
        "validityType": "4"
      },
      "status": "enabled"
    }
  }
}
```

可能错误：

| HTTP | message 示例 | 说明 |
| --- | --- | --- |
| `400` | `cardCode is required` | 未传卡密 |
| `422` | `card code not found` | 卡密不存在 |
| `422` | `card code is not enabled` | 卡密被禁用或作废 |
| `422` | `card code has expired` | 卡密过期 |
| `422` | `card code has no remaining uses` | 剩余次数不足 |
| `422` | `service is disabled` | 服务配置被禁用 |

### 2.2 创建接码订单/获取手机号

```http
POST /api/v1/public/orders
Content-Type: application/json
```

请求体：

```json
{
  "cardCode": "QM-EXAMPLE-CODE"
}
```

成功响应：

```json
{
  "code": 0,
  "message": "created",
  "data": {
    "id": 1001,
    "orderNo": "R202606141552330001",
    "providerCode": "smspool",
    "serviceConfigId": 1,
    "supplierOrderId": "987654321",
    "phoneNumber": "+14155550123",
    "phoneCountryCode": "1",
    "phoneNationalNumber": "4155550123",
    "verificationCode": "",
    "cost": 0.25,
    "maxPrice": 0.8,
    "status": "active",
    "supplierStatus": "active",
    "manualCheck": false,
    "createdAt": "2026-06-14T15:52:33+08:00",
    "updatedAt": "2026-06-14T15:52:33+08:00",
    "serviceConfig": {}
  }
}
```

长效接码供应商成功响应示例：

```json
{
  "code": 0,
  "message": "created",
  "data": {
    "orderNo": "R202606141552330002",
    "providerCode": "68sms",
    "supplierOrderId": "0123456789abcdef0123456789abcdef",
    "supplierToken": "0123456789abcdef0123456789abcdef",
    "phoneNumber": "+14155550123",
    "phoneCountryCode": "1",
    "phoneNationalNumber": "2722917613",
    "status": "completed",
    "manualCheck": true,
    "providerKind": "long_lived",
    "messageUrl": "https://api.68sms.com/api/sms/get?key=0123456789abcdef0123456789abcdef"
  }
}
```

业务规则：

- 同一卡密同一时间允许并发订单数不超过卡密剩余次数。
- 成功拿到手机号后才扣减卡密次数。
- 短效订单由后台轮询供应商获取验证码。
- 长效订单取号成功即视为订单完成，用户需要手动调用查询接口。

可能错误：

| HTTP | message 示例 | 说明 |
| --- | --- | --- |
| `422` | `card code has no remaining uses` | 卡密次数不足 |
| `422` | `AUTH_ERROR: ...` | 供应商鉴权失败或配置错误 |
| `422` | `OUT_OF_STOCK: ...` | 供应商无库存 |
| `422` | `BALANCE_NOT_ENOUGH: ...` | 供应商余额不足 |
| `422` | `PROVIDER_REJECTED: ...` | 供应商拒绝请求 |

### 2.3 查询订单

```http
GET /api/v1/public/orders/:orderNo?cardCode=QM-EXAMPLE-CODE
```

也兼容旧参数名：

```http
GET /api/v1/public/orders/:orderNo?card_code=QM-EXAMPLE-CODE
```

Query 参数：

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `cardCode` | string | 是 | 卡密明文，用于鉴权该订单 |

成功响应：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "orderNo": "R202606141552330001",
    "phoneNumber": "+14155550123",
    "verificationCode": "123456",
    "smsContent": "Your code is 123456",
    "status": "sms_received"
  }
}
```

错误：

| HTTP | message 示例 | 说明 |
| --- | --- | --- |
| `400` | `cardCode is required` | 未传卡密 |
| `404` | `order not found` | 订单不存在或卡密不匹配 |

### 2.4 手动查询验证码

```http
POST /api/v1/public/orders/:orderNo/check
Content-Type: application/json
```

请求体：

```json
{
  "cardCode": "QM-EXAMPLE-CODE"
}
```

说明：

- 主要用于长效接码平台，例如 `68sms`、`62-us`。
- 短效接码平台调用该接口通常只返回当前订单状态，不主动查询外部短信。

成功响应，未收到短信：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "orderNo": "R202606141552330002",
    "status": "completed",
    "manualCheck": true,
    "supplierStatus": "10018",
    "verificationCode": "",
    "smsContent": ""
  }
}
```

成功响应，已收到短信：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "orderNo": "R202606141552330002",
    "status": "sms_received",
    "verificationCode": "78890",
    "smsContent": "Telegram code: 78890",
    "receivedAt": "2026-06-14T15:55:10+08:00"
  }
}
```

### 2.5 取消订单/取消号码

```http
POST /api/v1/public/orders/:orderNo/cancel
Content-Type: application/json
```

请求体：

```json
{
  "cardCode": "QM-EXAMPLE-CODE"
}
```

业务规则：

- 仅短效接码订单支持取消。
- 订单必须处于 `active` 状态。
- 取号后需等待至少 2 分钟，且未收到验证码，才允许取消。
- 必须供应商取消成功后才恢复卡密次数。
- 长效订单返回不可取消。

成功响应：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "orderNo": "R202606141552330001",
    "status": "cancelled",
    "cancelledAt": "2026-06-14T15:55:00+08:00"
  }
}
```

常见错误：

| HTTP | message 示例 | 说明 |
| --- | --- | --- |
| `422` | `cancel is allowed after two minutes if no sms has been received` | 未达到可取消时间 |
| `422` | `manual check orders cannot be cancelled` | 长效接码订单不支持取消 |
| `422` | `order already received sms` | 已收到验证码 |
| `422` | `order cannot be cancelled in current status` | 当前状态不允许取消 |

### 2.6 卡密历史订单

```http
GET /api/v1/public/cards/history?cardCode=QM-EXAMPLE-CODE
```

也兼容：

```http
GET /api/v1/public/cards/history?card_code=QM-EXAMPLE-CODE
```

说明：

- 返回该卡密下最近历史订单。
- 只返回已经成功获取到手机号的订单。
- 当前固定返回最多 50 条。

成功响应：

```json
{
  "code": 0,
  "message": "ok",
  "data": [
    {
      "orderNo": "R202606141552330001",
      "phoneNumber": "+14155550123",
      "phoneCountryCode": "1",
      "phoneNationalNumber": "4155550123",
      "verificationCode": "123456",
      "status": "sms_received",
      "createdAt": "2026-06-14T15:52:33+08:00"
    }
  ]
}
```

### 2.7 记录网站访问

```http
POST /api/v1/public/visits
Content-Type: application/json
```

请求体：

```json
{
  "path": "/"
}
```

成功响应：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "recorded": true
  }
}
```

### 2.8 公共公告列表

```http
GET /api/v1/public/announcements?readerId=<reader-id>
```

只返回 `active`、已开始且未结束的公告。

成功响应：

```json
{
  "code": 0,
  "message": "ok",
  "data": [
    {
      "id": 1,
      "title": "系统维护通知",
      "content": "今晚 23:00 维护。",
      "status": "active",
      "notifyMode": "modal",
      "readCount": 12,
      "publishedAt": "2026-06-14T15:52:33+08:00",
      "createdAt": "2026-06-14T15:52:33+08:00",
      "updatedAt": "2026-06-14T15:52:33+08:00",
      "unread": true
    }
  ]
}
```

### 2.9 标记公告已读

```http
POST /api/v1/public/announcements/:id/read
Content-Type: application/json
```

请求体：

```json
{
  "readerId": "reader-8f3a1c"
}
```

成功响应：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "read": true
  }
}
```

说明：同一个 `readerId` 对同一公告重复标记不会重复增加阅读数。

## 3. 管理端基础接口

管理端除登录外均需要 `Authorization: Bearer <JWT_TOKEN>`。

### 3.1 管理员登录

```http
POST /api/v1/admin/auth/login
Content-Type: application/json
```

限流：每分钟 20 次/IP。

请求体：

```json
{
  "username": "admin",
  "password": "admin123456"
}
```

成功响应：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIs...",
    "admin": {
      "id": 1,
      "username": "admin",
      "role": "admin",
      "status": "enabled",
      "lastLoginAt": "2026-06-14T15:52:33+08:00",
      "createdAt": "2026-06-14T12:00:00+08:00",
      "updatedAt": "2026-06-14T15:52:33+08:00"
    }
  }
}
```

错误：

| HTTP | message 示例 |
| --- | --- |
| `400` | `username and password are required` |
| `401` | `invalid username or password` |
| `401` | `admin account is disabled` |

### 3.2 修改密码

```http
POST /api/v1/admin/auth/password
Authorization: Bearer <JWT_TOKEN>
Content-Type: application/json
```

请求体：

```json
{
  "oldPassword": "admin123456",
  "newPassword": "newPassword123"
}
```

成功响应：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "changed": true
  }
}
```

规则：

- `newPassword` 最少 8 个字符。
- 旧密码必须正确。

### 3.3 工作台统计

```http
GET /api/v1/admin/dashboard
Authorization: Bearer <JWT_TOKEN>
```

成功响应：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "totalCompletedOrders": 1200,
    "todayCompletedOrders": 36,
    "activeOrders": 8,
    "todayOrders": 45,
    "todayVisits": 180,
    "totalVisits": 20000,
    "availableCards": 300,
    "providerRanking": [
      { "key": "smspool", "name": "smspool", "count": 520 }
    ],
    "serviceRanking": [
      { "key": "smspool-US-OpenAI", "name": "smspool-US-OpenAI", "count": 200 }
    ],
    "statusSummary": [
      { "key": "sms_received", "name": "sms_received", "count": 900 }
    ]
  }
}
```

统计口径：

- 完成订单包含 `sms_received` 和 `completed`。
- 今日统计以服务端本地时区当天 00:00 起算。
- 服务配置排行按完成订单数排序。

## 4. 管理端供应商接口

### 4.1 供应商列表

```http
GET /api/v1/admin/providers
Authorization: Bearer <JWT_TOKEN>
```

成功响应：

```json
{
  "code": 0,
  "message": "ok",
  "data": [
    {
      "id": 1,
      "code": "smspool",
      "name": "SMSPool",
      "baseUrl": "https://api.smspool.net",
      "currencyCode": "USD",
      "status": "enabled",
      "capabilities": {},
      "lastBalance": "123.45",
      "lastBalanceCheckedAt": "2026-06-14T15:52:33+08:00",
      "apiKeySet": true,
      "metadataTokenSet": false,
      "loginCredentialSet": false,
      "requiresLoginCredential": false,
      "providerKind": "",
      "manualCheck": false,
      "authMode": "",
      "createdAt": "2026-06-14T12:00:00+08:00",
      "updatedAt": "2026-06-14T15:52:33+08:00"
    },
    {
      "id": 6,
      "code": "68sms",
      "name": "68SMS",
      "baseUrl": "https://api.68sms.com",
      "currencyCode": "USD",
      "status": "enabled",
      "apiKeySet": true,
      "metadataTokenSet": true,
      "loginCredentialSet": true,
      "requiresLoginCredential": true,
      "providerKind": "long_lived",
      "manualCheck": true
    }
  ]
}
```

说明：

- `apiKeySet`、`metadataTokenSet`、`loginCredentialSet` 只表示是否已配置，不返回密钥明文。
- `providerKind=long_lived` 表示长效接码平台。

### 4.2 更新供应商配置

```http
PATCH /api/v1/admin/providers/:provider
Authorization: Bearer <JWT_TOKEN>
Content-Type: application/json
```

路径参数：

| 参数 | 说明 |
| --- | --- |
| `provider` | 供应商代码，例如 `smspool`、`firefox`、`herosms`、`smsbower`、`lubansms`、`68sms`、`62-us` |

请求体：

```json
{
  "name": "68SMS",
  "baseUrl": "https://api.68sms.com",
  "currencyCode": "USD",
  "apiKey": "YOUR_API_KEY",
  "loginCredential": "Token: xxx\nCookie: user=xxx; pass=xxx\nCommunication: <base64-communication-value>",
  "status": "enabled"
}
```

字段说明：

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `name` | string | 是 | 后台显示名称 |
| `baseUrl` | string | 否 | 供应商 API 地址 |
| `currencyCode` | string | 否 | 法币单位，例如 `USD`、`CNY`，空值默认 `USD` |
| `apiKey` | string | 否 | 供应商 API 密钥；空值表示保持原配置不变 |
| `loginCredential` | string | 否 | 登录凭证；空值表示保持原配置不变 |
| `metadataToken` | string | 否 | 兼容字段，与 `loginCredential` 等效 |
| `authMode` | string | 否 | `62-us` 专用：`openapi_token` 或 `account_password` |
| `account` | string | 否 | `62-us` 账号密码模式账号 |
| `password` | string | 否 | `62-us` 账号密码模式密码 |
| `status` | string | 否 | `enabled` 或 `disabled` |

`68sms` 登录凭证格式建议：

```text
Token: eyJ0eXAiOiJKV1Qi...
Cookie: user=xxx; pass=xxx
Communication: <base64-communication-value>
```

`62-us` 账号密码模式请求示例：

```json
{
  "name": "62-US",
  "baseUrl": "https://api.62-us.com",
  "currencyCode": "USD",
  "authMode": "account_password",
  "account": "your-account",
  "password": "your-password",
  "status": "enabled"
}
```

成功响应返回更新后的 `SMSProvider`，不会返回密钥明文。

### 4.3 供应商国家列表

```http
GET /api/v1/admin/providers/:provider/countries
Authorization: Bearer <JWT_TOKEN>
```

成功响应：

```json
{
  "code": 0,
  "message": "ok",
  "data": [
    {
      "id": 1,
      "providerCode": "68sms",
      "providerCountryId": "188",
      "name": "United States",
      "shortName": "US",
      "region": "",
      "dialCode": "1",
      "status": "enabled",
      "syncedAt": "2026-06-14T06:00:00+08:00",
      "servicesSyncedAt": "2026-06-14T06:00:00+08:00",
      "createdAt": "2026-06-14T06:00:00+08:00",
      "updatedAt": "2026-06-14T06:00:00+08:00"
    }
  ]
}
```

说明：国家列表优先从本地 `sys_provider_countries` 返回；当本地没有数据时，后端会调用供应商元数据接口同步。

### 4.4 供应商服务列表

```http
GET /api/v1/admin/providers/:provider/services?countryId=188
Authorization: Bearer <JWT_TOKEN>
```

成功响应：

```json
{
  "code": 0,
  "message": "ok",
  "data": [
    {
      "id": 1,
      "providerCode": "68sms",
      "providerCountryId": "188",
      "providerServiceId": "2",
      "name": "Telegram",
      "countryName": "United States",
      "status": "enabled",
      "syncedAt": "2026-06-14T06:00:00+08:00",
      "createdAt": "2026-06-14T06:00:00+08:00",
      "updatedAt": "2026-06-14T06:00:00+08:00"
    }
  ]
}
```

说明：

- 服务列表用于后台下拉搜索，不缓存价格和库存。
- 价格、库存、有效期等按国家和服务动态查询供应商接口。

### 4.5 获取价格

```http
GET /api/v1/admin/providers/:provider/price?countryId=188&serviceId=2&poolId=
Authorization: Bearer <JWT_TOKEN>
```

成功响应：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "pool": 0,
    "lowPrice": "0.0400",
    "highPrice": "0.7000",
    "price": "0.0400",
    "successRate": 92.5
  }
}
```

### 4.6 获取库存

```http
GET /api/v1/admin/providers/:provider/stock?countryId=188&serviceId=2&poolId=
Authorization: Bearer <JWT_TOKEN>
```

成功响应：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "amount": 34103
  }
}
```

### 4.7 获取价格和库存组合信息

```http
GET /api/v1/admin/providers/:provider/quote?countryId=188&serviceId=2&poolId=
Authorization: Bearer <JWT_TOKEN>
```

成功响应：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "price": {
      "pool": 0,
      "lowPrice": "0.0400",
      "highPrice": "0.7000",
      "price": "0.0400",
      "successRate": 92.5
    },
    "stock": {
      "amount": 3396
    }
  }
}
```

### 4.8 获取长效号码有效期选项

```http
GET /api/v1/admin/providers/:provider/validity-options?countryId=188&serviceId=2&poolId=
Authorization: Bearer <JWT_TOKEN>
```

说明：

- 用于长效接码供应商的服务配置，例如 `68sms`、`62-us`。
- `68sms` 从 `/admin/api/activity` 读取活动规则。
- 返回包含库存为 0 的选项，前端应禁用 `stock <= 0` 的选项。

成功响应：

```json
{
  "code": 0,
  "message": "ok",
  "data": [
    {
      "value": "1",
      "label": "1-10",
      "minDays": 1,
      "maxDays": 10,
      "stock": 0
    },
    {
      "value": "4",
      "label": "61-92",
      "minDays": 61,
      "maxDays": 92,
      "stock": 34103
    }
  ]
}
```

### 4.9 检测供应商余额

```http
GET /api/v1/admin/providers/:provider/balance
Authorization: Bearer <JWT_TOKEN>
```

成功响应：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "balance": "123.45",
    "checkedAt": "2026-06-14T15:52:33+08:00"
  }
}
```

说明：

- 检测成功后会更新供应商的 `lastBalance` 和 `lastBalanceCheckedAt`。
- 失败时通常返回 `422`，`message` 中包含供应商错误原因。

## 5. 管理端服务配置接口

### 5.1 服务配置列表

```http
GET /api/v1/admin/service-configs
Authorization: Bearer <JWT_TOKEN>
```

成功响应：

```json
{
  "code": 0,
  "message": "ok",
  "data": [
    {
      "id": 1,
      "providerCode": "smspool",
      "targetPlatform": "smspool-US-OpenAI",
      "displayName": "OpenAI / ChatGPT",
      "countryCode": "US",
      "countryName": "United States",
      "providerCountryId": "1",
      "providerServiceId": "1012",
      "providerPoolId": "",
      "maxPrice": 0.8,
      "timeoutSeconds": 1200,
      "metadata": {},
      "status": "enabled",
      "createdAt": "2026-06-14T15:52:33+08:00",
      "updatedAt": "2026-06-14T15:52:33+08:00"
    }
  ]
}
```

### 5.2 创建服务配置

```http
POST /api/v1/admin/service-configs
Authorization: Bearer <JWT_TOKEN>
Content-Type: application/json
```

请求体，短效供应商示例：

```json
{
  "providerCode": "smspool",
  "targetPlatform": "smspool-US-OpenAI",
  "displayName": "OpenAI / ChatGPT",
  "countryCode": "US",
  "countryName": "United States",
  "providerCountryId": "1",
  "providerServiceId": "1012",
  "providerPoolId": "",
  "maxPrice": 0.8,
  "timeoutSeconds": 1200,
  "metadata": {},
  "status": "enabled"
}
```

请求体，`68sms` 长效供应商示例：

```json
{
  "providerCode": "68sms",
  "targetPlatform": "68sms-US-Telegram",
  "displayName": "Telegram",
  "countryCode": "US",
  "countryName": "United States",
  "providerCountryId": "188",
  "providerServiceId": "2",
  "providerPoolId": "",
  "maxPrice": 0.7,
  "timeoutSeconds": 0,
  "metadata": {
    "validityType": "4",
    "validityLabel": "61 — 92 天",
    "validityMinDays": 61,
    "validityMaxDays": 92,
    "validityStock": 34103
  },
  "status": "enabled"
}
```

字段说明：

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `providerCode` | string | 是 | 供应商代码 |
| `targetPlatform` | string | 是 | 本系统服务标识，必须无空格 |
| `displayName` | string | 是 | 用户端展示服务名 |
| `countryCode` | string | 是 | 国家短码，如 `US` |
| `countryName` | string | 否 | 国家名称 |
| `providerCountryId` | string | 是 | 供应商国家 ID |
| `providerServiceId` | string | 是 | 供应商服务 ID |
| `providerPoolId` | string | 否 | 供应商号码池、运营商或额外参数 |
| `maxPrice` | number | 否 | 取号最高价格限制 |
| `timeoutSeconds` | int | 否 | 等待验证码秒数；长效供应商为 `0` |
| `metadata` | object | 否 | 供应商专属配置 |
| `status` | string | 否 | 空值默认 `enabled` |

成功响应：`201 created`，返回创建后的 `ServiceConfig`。

规则：

- `timeoutSeconds < 0` 会被修正为 `0`。
- 非长效供应商 `timeoutSeconds=0` 时会回退为 `1200`。
- 长效供应商 `timeoutSeconds=0` 表示系统不自动超时。

### 5.3 更新服务配置

```http
PATCH /api/v1/admin/service-configs/:id
Authorization: Bearer <JWT_TOKEN>
Content-Type: application/json
```

请求体同创建接口。成功响应返回更新后的 `ServiceConfig`。

### 5.4 删除服务配置

```http
DELETE /api/v1/admin/service-configs/:id
Authorization: Bearer <JWT_TOKEN>
```

成功响应：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "deleted": true
  }
}
```

删除规则：

- 只检查是否存在对应卡密批次。
- 只有该配置下所有卡密批次都删除后，服务配置才允许删除。
- 订单历史不阻止服务配置删除。

常见错误：

```json
{
  "code": 422,
  "message": "SERVICE_CONFIG_HAS_CARD_BATCHES: current config has card batches, please delete the card batches before deleting this config"
}
```

## 6. 管理端卡密批次与卡密接口

### 6.1 创建卡密批次

```http
POST /api/v1/admin/card-batches
Authorization: Bearer <JWT_TOKEN>
Content-Type: application/json
```

请求体：

```json
{
  "name": "OpenAI US 2026-06 批次",
  "serviceConfigId": 1,
  "quantity": 100,
  "usesPerCode": 1,
  "expiresAt": "2026-12-31T23:59:59+08:00"
}
```

字段说明：

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `name` | string | 是 | 批次名称 |
| `serviceConfigId` | uint | 是 | 服务配置 ID |
| `quantity` | int | 是 | 生成数量，范围 `1-10000` |
| `usesPerCode` | int | 是 | 每张卡密可用次数，必须大于 0 |
| `expiresAt` | string/null | 否 | 有效期，空值表示永久有效 |

成功响应：

```json
{
  "code": 0,
  "message": "created",
  "data": {
    "id": 1,
    "name": "OpenAI US 2026-06 批次",
    "providerCode": "smspool",
    "serviceConfigId": 1,
    "quantity": 100,
    "usesPerCode": 1,
    "expiresAt": "2026-12-31T23:59:59+08:00",
    "exportedAt": null,
    "createdBy": 1,
    "createdAt": "2026-06-14T15:52:33+08:00",
    "updatedAt": "2026-06-14T15:52:33+08:00"
  }
}
```

说明：

- 卡密明文不会直接保存在普通字段中，会保存 hash、mask 和加密密文。
- 创建批次时会生成导出 TXT 文件，每行一个卡密。

### 6.2 卡密批次列表

```http
GET /api/v1/admin/card-batches?limit=50&offset=0
Authorization: Bearer <JWT_TOKEN>
```

成功响应返回 `CardBatch[]`，按 `id desc` 排序。

### 6.3 导出卡密 TXT

```http
GET /api/v1/admin/card-batches/:id/export.txt
Authorization: Bearer <JWT_TOKEN>
```

响应头：

```http
Content-Type: text/plain; charset=utf-8
Content-Disposition: attachment; filename=card-batch-1.txt
```

响应体示例：

```text
QM-AAAAAAAA-BBBBBBBB
QM-CCCCCCCC-DDDDDDDD
QM-EEEEEEEE-FFFFFFFF
```

说明：

- 该接口不使用统一 JSON 响应。
- 首次导出会记录 `exportedAt`。

### 6.4 删除卡密批次

```http
DELETE /api/v1/admin/card-batches/:id
Authorization: Bearer <JWT_TOKEN>
```

成功响应：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "deleted": true
  }
}
```

规则：

- 删除批次会硬删除该批次下所有卡密。
- 如果批次下任一卡密存在进行中的订单，删除会失败。

错误示例：

```json
{
  "code": 422,
  "message": "CARD_BATCH_HAS_ACTIVE_ORDER: this batch has active orders and cannot be deleted"
}
```

### 6.5 卡密列表

```http
GET /api/v1/admin/card-codes?limit=50&offset=0
Authorization: Bearer <JWT_TOKEN>
```

成功响应：

```json
{
  "code": 0,
  "message": "ok",
  "data": [
    {
      "id": 10,
      "codeMask": "QM************1234",
      "providerCode": "smspool",
      "serviceConfigId": 1,
      "batchId": 1,
      "totalUses": 1,
      "remainingUses": 1,
      "expiresAt": null,
      "status": "enabled",
      "createdAt": "2026-06-14T15:52:33+08:00",
      "updatedAt": "2026-06-14T15:52:33+08:00",
      "serviceConfig": {}
    }
  ]
}
```

说明：

- 默认只返回 `codeMask`，不返回明文。
- 复制明文需调用 reveal 接口。

### 6.6 查看卡密明文

```http
GET /api/v1/admin/card-codes/:id/reveal
Authorization: Bearer <JWT_TOKEN>
```

成功响应：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "code": "QM-AAAAAAAA-BBBBBBBB"
  }
}
```

说明：

- 该接口会写入审计日志。
- 仅当该卡密记录保存了加密密文时可揭示。

### 6.7 修改卡密状态

```http
PATCH /api/v1/admin/card-codes/:id/status
Authorization: Bearer <JWT_TOKEN>
Content-Type: application/json
```

请求体：

```json
{
  "status": "disabled"
}
```

可选状态：`enabled`、`disabled`、`voided`。

成功响应：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "id": 10,
    "status": "disabled"
  }
}
```

### 6.8 删除单张卡密

```http
DELETE /api/v1/admin/card-codes/:id
Authorization: Bearer <JWT_TOKEN>
```

成功响应：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "deleted": true
  }
}
```

规则：

- 删除为硬删除。
- 如果该卡密存在 `created`、`active`、`cancel_requested` 状态订单，则不允许删除。

错误示例：

```json
{
  "code": 422,
  "message": "CARD_HAS_ACTIVE_ORDER: this card has an active order and cannot be deleted"
}
```

## 7. 管理端订单接口

### 7.1 订单列表

```http
GET /api/v1/admin/orders?limit=50&offset=0
Authorization: Bearer <JWT_TOKEN>
```

成功响应：

```json
{
  "code": 0,
  "message": "ok",
  "data": [
    {
      "id": 1001,
      "orderNo": "R202606141552330001",
      "cardCodeId": 10,
      "providerCode": "smspool",
      "serviceConfigId": 1,
      "supplierOrderId": "987654321",
      "phoneNumber": "+14155550123",
      "phoneCountryCode": "1",
      "phoneNationalNumber": "4155550123",
      "verificationCode": "123456",
      "smsContent": "Your code is 123456",
      "cost": 0.25,
      "maxPrice": 0.8,
      "status": "sms_received",
      "supplierStatus": "received",
      "failureReason": "",
      "startedAt": "2026-06-14T15:52:33+08:00",
      "receivedAt": "2026-06-14T15:55:10+08:00",
      "cancelledAt": null,
      "expiredAt": null,
      "lastPolledAt": "2026-06-14T15:55:10+08:00",
      "createdAt": "2026-06-14T15:52:33+08:00",
      "updatedAt": "2026-06-14T15:55:10+08:00",
      "serviceConfig": {}
    }
  ]
}
```

说明：

- 返回订单按 `id desc` 排序。
- `firefox` 未实际收到验证码或已取消订单展示成本为 0；收到验证码后成本使用订单价格或配置最高价兜底。

### 7.2 管理员取消订单

```http
POST /api/v1/admin/orders/:id/cancel
Authorization: Bearer <JWT_TOKEN>
```

路径参数：

| 参数 | 类型 | 说明 |
| --- | --- | --- |
| `id` | uint | 订单数据库 ID，不是 `orderNo` |

成功响应：返回取消后的 `ReceiveOrder`。

规则同公共端取消接口：短效订单、`active` 状态、取号后至少 2 分钟、未收到验证码，并且供应商取消成功后才恢复次数。

## 8. 管理端审计日志接口

```http
GET /api/v1/admin/audit-logs?limit=50&offset=0
Authorization: Bearer <JWT_TOKEN>
```

成功响应：

```json
{
  "code": 0,
  "message": "ok",
  "data": [
    {
      "id": 1,
      "actorType": "admin",
      "actorId": 1,
      "action": "service_config.create",
      "resourceType": "service_config",
      "resourceId": "3",
      "ip": "127.0.0.1",
      "userAgent": "Mozilla/5.0 ...",
      "metadata": {
        "providerCode": "68sms",
        "targetPlatform": "68sms-US-Telegram",
        "status": "enabled"
      },
      "createdAt": "2026-06-14T15:52:33+08:00"
    }
  ]
}
```

常见 `action`：

| action | 说明 |
| --- | --- |
| `admin.login` | 管理员登录 |
| `admin.change_password` | 修改密码 |
| `provider.update` | 修改供应商配置 |
| `provider.balance_check` | 检测供应商余额 |
| `service_config.create` | 创建服务配置 |
| `service_config.update` | 更新服务配置 |
| `service_config.delete` | 删除服务配置 |
| `card_batch.create` | 创建卡密批次 |
| `card_batch.export` | 导出卡密 TXT |
| `card_batch.delete` | 删除卡密批次 |
| `card.update_status` | 修改卡密状态 |
| `card.reveal_code` | 查看卡密明文 |
| `card.delete` | 删除卡密 |
| `order.cancel` | 管理员取消订单 |
| `announcement.create` | 创建公告 |
| `announcement.update` | 更新公告 |
| `announcement.delete` | 删除公告 |

## 9. 管理端公告接口

### 9.1 公告列表

```http
GET /api/v1/admin/announcements?keyword=维护&status=active&notifyMode=modal&limit=50&offset=0
Authorization: Bearer <JWT_TOKEN>
```

Query 参数：

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `keyword` | string | 否 | 按标题模糊搜索 |
| `status` | string | 否 | `draft`、`active`、`archived` |
| `notifyMode` | string | 否 | `modal`、`silent` |
| `limit` | int | 否 | 分页数量 |
| `offset` | int | 否 | 分页偏移 |

成功响应：返回 `Announcement[]`。

### 9.2 创建公告

```http
POST /api/v1/admin/announcements
Authorization: Bearer <JWT_TOKEN>
Content-Type: application/json
```

请求体：

```json
{
  "title": "系统维护通知",
  "content": "今晚 23:00 维护。支持 Markdown。",
  "status": "active",
  "notifyMode": "modal",
  "startAt": "2026-06-14T20:00:00+08:00",
  "endAt": "2026-06-15T08:00:00+08:00"
}
```

字段说明：

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `title` | string | 是 | 公告标题 |
| `content` | string | 是 | 公告内容，前端支持 Markdown 展示 |
| `status` | string | 否 | 空值默认 `draft` |
| `notifyMode` | string | 否 | 空值默认 `silent` |
| `startAt` | string/null | 否 | 展示开始时间，空值表示立即 |
| `endAt` | string/null | 否 | 展示结束时间，空值表示永久 |

成功响应：`201 created`，返回 `Announcement`。

规则：

- `title`、`content` 会 trim，不能为空。
- `status=active` 时会设置 `publishedAt`。
- `endAt` 不能早于 `startAt`。

### 9.3 更新公告

```http
PATCH /api/v1/admin/announcements/:id
Authorization: Bearer <JWT_TOKEN>
Content-Type: application/json
```

请求体同创建公告。成功响应返回更新后的 `Announcement`。

说明：如果公告从非 active 改为 active，且之前没有 `publishedAt`，会设置发布时间。

### 9.4 删除公告

```http
DELETE /api/v1/admin/announcements/:id
Authorization: Bearer <JWT_TOKEN>
```

成功响应：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "deleted": true
  }
}
```

说明：删除公告会同时删除对应已读记录。

## 10. 供应商错误码与前端提示建议

供应商适配器会尽量将外部错误转换为统一业务错误，常见前缀：

| 前缀 | 说明 |
| --- | --- |
| `AUTH_ERROR` | API Key、登录凭证、Token、Cookie 等配置错误或过期 |
| `OUT_OF_STOCK` | 没有可用号码或库存不足 |
| `BALANCE_NOT_ENOUGH` | 供应商余额不足 |
| `RATE_LIMITED` | 供应商限流或请求过快 |
| `CANNOT_CANCEL` | 当前号码不能取消，或取消时间未到 |
| `ORDER_NOT_FOUND` | 供应商订单或短信记录不存在 |
| `PRICE_NOT_FOUND` | 无法获取价格或服务配置不存在 |
| `PROVIDER_REJECTED` | 供应商返回失败但无法归类 |

前端建议：

- 保留原始 `message` 用于后台排查。
- 用户端显示时使用国际化映射，将常见错误转为中文/英文友好提示。
- 对 `AUTH_ERROR`，管理端应提示检查供应商密钥或登录凭证。

## 11. cURL 示例

### 11.1 登录并查询服务配置

```bash
curl -X POST http://localhost:8080/api/v1/admin/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123456"}'
```

```bash
curl http://localhost:8080/api/v1/admin/service-configs \
  -H "Authorization: Bearer <JWT_TOKEN>"
```

### 11.2 校验卡密并取号

```bash
curl -X POST http://localhost:8080/api/v1/public/cards/verify \
  -H "Content-Type: application/json" \
  -d '{"cardCode":"QM-EXAMPLE-CODE"}'
```

```bash
curl -X POST http://localhost:8080/api/v1/public/orders \
  -H "Content-Type: application/json" \
  -d '{"cardCode":"QM-EXAMPLE-CODE"}'
```

### 11.3 查询订单验证码

```bash
curl "http://localhost:8080/api/v1/public/orders/R202606141552330001?cardCode=QM-EXAMPLE-CODE"
```

长效订单手动查询：

```bash
curl -X POST http://localhost:8080/api/v1/public/orders/R202606141552330002/check \
  -H "Content-Type: application/json" \
  -d '{"cardCode":"QM-EXAMPLE-CODE"}'
```
