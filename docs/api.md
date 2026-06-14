# API Overview

## Public

- `POST /api/v1/public/cards/verify`
- `POST /api/v1/public/orders`
- `GET /api/v1/public/orders/:orderNo?cardCode=...`
- `POST /api/v1/public/orders/:orderNo/cancel`
- `GET /api/v1/public/cards/history?cardCode=...`

## Admin

- `POST /api/v1/admin/auth/login`
- `GET /api/v1/admin/providers`
- `GET /api/v1/admin/service-configs`
- `POST /api/v1/admin/service-configs`
- `PATCH /api/v1/admin/service-configs/:id`
- `GET /api/v1/admin/card-batches`
- `POST /api/v1/admin/card-batches`
- `GET /api/v1/admin/card-batches/:id/export.txt`
- `GET /api/v1/admin/card-codes`
- `PATCH /api/v1/admin/card-codes/:id/status`
- `GET /api/v1/admin/orders`
- `POST /api/v1/admin/orders/:id/cancel`
- `GET /api/v1/admin/audit-logs`
