# WireGuard IP Allocation API Specification

> **Version:** 1.0 | **Base Path:** `/api/v1` | **Authentication:** X-API-Key (Bearer Token)

This document provides the OpenAPI-style specification for the WireGuard IP Allocation system endpoints. All endpoints require authentication unless otherwise noted.

---

## Table of Contents

1. [Overview](#overview)
2. [Authentication](#authentication)
3. [Endpoints](#endpoints)
   - [POST /wireguard/reserve-ip](#post-wireguardreserve-ip)
   - [POST /wireguard/confirm-allocation](#post-wireguardconfirm-allocation)
   - [POST /wireguard/release-ip](#post-wireguardrelease-ip)
   - [GET /wireguard/ip-pool-status/{server_id}](#get-wireguardip-pool-statusserver_id)
   - [GET /health/v2](#get-healthv2)
4. [Error Codes](#error-codes)
5. [Change Log](#change-log)

---

## Overview

The WireGuard IP Allocation API provides a stateful mechanism for managing IP address assignments from a centralized pool. It follows a reservation-confirmation-release lifecycle:

```
┌─────────────┐     ┌──────────────┐     ┌─────────────┐
│   RESERVE   │ ──▶ │  CONFIRM    │ ──▶ │ ALLOCATED  │
│  (reserved) │     │ (allocated)  │     │   (in use) │
└─────────────┘     └──────────────┘     └─────────────┘
        │                                        │
        ▼                                        ▼
┌─────────────────────────────────────────────────┐
│                   RELEASE                        │
│  (returns to free pool)                         │
└─────────────────────────────────────────────────┘
```

### IP Pool Concepts

| Concept | Description |
|---------|-------------|
| **Pool** | Set of IP addresses allocated to a specific server |
| **Reservation** | Temporary hold on an IP (default 300 seconds) |
| **Allocation** | Permanent assignment after confirmation |
| **Lease ID** | UUID identifying a reservation/allocation |

---

## Authentication

All WireGuard endpoints require agent authentication via `X-API-Key` header:

```http
X-API-Key: agent_xxxxxxxxxxxxxxxxxxxx
```

Admin endpoints (e.g., `/ip-pool-status`) require JWT authentication via `Authorization: Bearer <token>`.

### Authentication Errors

| Status | Description |
|--------|-------------|
| 401 | Invalid or missing API key |
| 403 | API key revoked or expired |
| 404 | Agent not registered |

---

## Endpoints

### POST /wireguard/reserve-ip

Reserve the next available IP address from the pool for a WireGuard peer.

```http
POST /api/v1/wireguard/reserve-ip
Content-Type: application/json
X-API-Key: agent_xxxxxxxxxxxxxxxxxxxx
```

#### Request Body Schema

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "required": ["vpn_key_name"],
  "properties": {
    "vpn_key_name": {
      "type": "string",
      "minLength": 1,
      "maxLength": 100,
      "description": "Human-readable name for the VPN key (peer identifier)"
    },
    "vpn_type": {
      "type": "string",
      "default": "wireguard",
      "pattern": "^wireguard$",
      "description": "VPN type - only 'wireguard' supported"
    },
    "request_id": {
      "type": "string",
      "nullable": true,
      "description": "Optional correlation ID for request tracing"
    }
  }
}
```

#### Response Schema (200 OK)

```json
{
  "ip_address": "10.88.88.2",
  "ip_int": 2,
  "lease_id": "550e8400-e29b-41d4-a716-446655440000",
  "cidr": "10.88.88.0/24",
  "lease_expires_at": "2026-04-23T10:34:52Z"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `ip_address` | string | Assigned IP address (dot notation) |
| `ip_int` | integer | Last octet for fast queries (2-254) |
| `lease_id` | string (UUID) | Unique identifier for this reservation |
| `cidr` | string | Network CIDR for the pool |
| `lease_expires_at` | string (ISO 8601) | Expiration timestamp |

#### Example Request

```bash
curl -X POST https://backend.usipipo.com/api/v1/wireguard/reserve-ip \
  -H "Content-Type: application/json" \
  -H "X-API-Key: agent_abc123def456" \
  -d '{"vpn_key_name": "user-juan-perez", "vpn_type": "wireguard"}'
```

#### Example Response

```json
{
  "ip_address": "10.88.88.2",
  "ip_int": 2,
  "lease_id": "550e8400-e29b-41d4-a716-446655440000",
  "cidr": "10.88.88.0/24",
  "lease_expires_at": "2026-04-23T10:34:52Z"
}
```

#### Error Responses

| Status | Code | Description |
|--------|------|-------------|
| 400 | `INVALID_VPN_TYPE` | VPN type must be "wireguard" |
| 401 | `UNAUTHORIZED` | Invalid or missing API key |
| 404 | `VPN_KEY_NOT_FOUND` | VPN key not found on server |
| 429 | `NO_AVAILABLE_IPS` | IP pool exhausted |

---

### POST /wireguard/confirm-allocation

Confirm a previously reserved IP allocation, marking it as permanently allocated.

```http
POST /api/v1/wireguard/confirm-allocation
Content-Type: application/json
X-API-Key: agent_xxxxxxxxxxxxxxxxxxxx
```

#### Request Body Schema

```json
{
  "type": "object",
  "required": ["lease_id", "ip_address", "public_key"],
  "properties": {
    "lease_id": {
      "type": "string",
      "format": "uuid",
      "description": "UUID of the reservation to confirm"
    },
    "ip_address": {
      "type": "string",
      "pattern": "^\\d{1,3}\\.\\d{1,3}\\.\\d{1,3}\\.\\d{1,3}$",
      "description": "IP address (must match reservation)"
    },
    "public_key": {
      "type": "string",
      "minLength": 40,
      "maxLength": 100,
      "description": "WireGuard public key of the peer"
    },
    "request_id": {
      "type": "string",
      "nullable": true,
      "description": "Optional correlation ID"
    }
  }
}
```

#### Response Schema (200 OK)

```json
{
  "confirmed": true
}
```

| Field | Type | Description |
|-------|------|-------------|
| `confirmed` | boolean | Always `true` on success |

#### Example Request

```bash
curl -X POST https://backend.usipipo.com/api/v1/wireguard/confirm-allocation \
  -H "Content-Type: application/json" \
  -H "X-API-Key: agent_abc123def456" \
  -d '{
    "lease_id": "550e8400-e29b-41d4-a716-446655440000",
    "ip_address": "10.88.88.2",
    "public_key": "aBcDeFgHiJkLmNoPqRsTuVwXyZ0123456789+/="
  }'
```

#### Example Response

```json
{
  "confirmed": true
}
```

#### Idempotency

If the allocation is already confirmed (status=allocated), this endpoint returns `200 OK` with `{"confirmed": true}` - it is idempotent.

#### Error Responses

| Status | Code | Description |
|--------|------|-------------|
| 400 | `IP_MISMATCH` | Provided IP doesn't match reservation |
| 400 | `INVALID_STATUS` | Cannot confirm in current status |
| 404 | `LEASE_NOT_FOUND` | Reservation not found |

---

### POST /wireguard/release-ip

Release a reserved or allocated IP address back to the free pool.

```http
POST /api/v1/wireguard/release-ip
Content-Type: application/json
X-API-Key: agent_xxxxxxxxxxxxxxxxxxxx
```

#### Request Body Schema

```json
{
  "type": "object",
  "required": ["lease_id", "reason"],
  "properties": {
    "lease_id": {
      "type": "string",
      "format": "uuid",
      "description": "UUID of the allocation to release"
    },
    "reason": {
      "type": "string",
      "minLength": 1,
      "maxLength": 200,
      "description": "Reason for release"
    },
    "request_id": {
      "type": "string",
      "nullable": true,
      "description": "Optional correlation ID"
    }
  }
}
```

#### Valid Reason Codes

| Reason | Description |
|--------|-------------|
| `user_request` | User requested key deletion |
| `expired` | Lease expired |
| `key_revoked` | Key was revoked |
| `agent_failure` | Agent failed during setup |
| `admin` | Administrative release |
| `reconciliation` | Released via reconciliation |

#### Response Schema (200 OK)

```json
{
  "released": true
}
```

#### Example Request

```bash
curl -X POST https://backend.usipipo.com/api/v1/wireguard/release-ip \
  -H "Content-Type: application/json" \
  -H "X-API-Key: agent_abc123def456" \
  -d '{
    "lease_id": "550e8400-e29b-41d4-a716-446655440000",
    "reason": "user_request"
  }'
```

#### Example Response

```json
{
  "released": true
}
```

#### Idempotency

If the IP is already free, returns `200 OK` with `{"released": true}`.

#### Error Responses

| Status | Code | Description |
|--------|------|-------------|
| 400 | `INVALID_STATUS` | Cannot release in current status |
| 404 | `LEASE_NOT_FOUND` | Allocation not found |

---

### GET /wireguard/ip-pool-status/{server_id}

Get IP pool statistics for a specific server. **Requires admin privileges.**

```http
GET /api/v1/wireguard/ip-pool-status/{server_id}
Authorization: Bearer <admin_jwt_token>
```

#### Path Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `server_id` | UUID | Server identifier |

#### Response Schema (200 OK)

```json
{
  "total": 253,
  "free": 198,
  "reserved": 12,
  "allocated": 43,
  "exhausted": false
}
```

| Field | Type | Description |
|-------|------|-------------|
| `total` | integer | Total IPs in pool |
| `free` | integer | Available IPs |
| `reserved` | integer | Pending confirmation |
| `allocated` | integer | In use |
| `exhausted` | boolean | Pool exhausted (`free == 0`) |

#### Example Request

```bash
curl -X GET "https://backend.usipipo.com/api/v1/wireguard/ip-pool-status/123e4567-e89b-12d3-a456-426614174000" \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
```

#### Example Response

```json
{
  "total": 253,
  "free": 198,
  "reserved": 12,
  "allocated": 43,
  "exhausted": false
}
```

#### Error Responses

| Status | Code | Description |
|--------|------|-------------|
| 401 | `UNAUTHORIZED` | Missing or invalid JWT |
| 403 | `FORBIDDEN` | User not admin |
| 404 | `SERVER_NOT_FOUND` | Server not found |

---

### GET /health/v2

Enhanced health check endpoint with IP pool statistics.

```http
GET /api/v1/health/v2
Authorization: Bearer <admin_jwt_token>
```

#### Response Schema (200 OK)

```json
{
  "status": "healthy",
  "version": "1.0.0",
  "timestamp": "2026-04-23T13:39:52Z",
  "components": {
    "database": {
      "status": "healthy",
      "latency_ms": 12
    },
    "redis": {
      "status": "healthy",
      "latency_ms": 2
    },
    "ip_pool": {
      "status": "healthy",
      "total": 253,
      "free": 198,
      "reserved": 12,
      "allocated": 43,
      "exhausted": false,
      "server_id": "123e4567-e89b-12d3-a456-426614174000"
    }
  }
}
```

#### Response Fields

| Field | Type | Description |
|-------|------|-------------|
| `status` | string | Overall status: "healthy", "degraded", "unhealthy" |
| `version` | string | API version |
| `timestamp` | string | ISO 8601 timestamp |
| `components` | object | Component statuses |
| `components.database` | object | Database health |
| `components.database.status` | string | "healthy" or "unhealthy" |
| `components.database.latency_ms` | integer | Query latency |
| `components.redis` | object | Redis health |
| `components.ip_pool` | object | IP pool for requesting server |
| `components.ip_pool.server_id` | string | Server ID (from JWT) |

#### Health Status Logic

| Status | Condition |
|--------|-----------|
| `healthy` | All components operational, pool > 10% free |
| `degraded` | Some latency, pool 5-10% free |
| `unhealthy` | Component down, pool < 5% free |

#### Example Request

```bash
curl -X GET "https://backend.usipipo.com/api/v1/health/v2" \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
```

#### Example Response (Degraded)

```json
{
  "status": "degraded",
  "version": "1.0.0",
  "timestamp": "2026-04-23T13:39:52Z",
  "components": {
    "database": {
      "status": "healthy",
      "latency_ms": 12
    },
    "redis": {
      "status": "unhealthy",
      "latency_ms": null
    },
    "ip_pool": {
      "status": "healthy",
      "total": 253,
      "free": 25,
      "reserved": 45,
      "allocated": 183,
      "exhausted": false,
      "server_id": "123e4567-e89b-12d3-a456-426614174000"
    }
  }
}
```

#### Error Responses

| Status | Code | Description |
|--------|------|-------------|
| 503 | `UNHEALTHY` | Service degraded/unhealthy |

---

## Error Codes

### HTTP Status Code Summary

| Status | Meaning |
|--------|---------|
| 200 | Success |
| 400 | Bad Request -Invalid parameters |
| 401 | Unauthorized - Authentication required |
| 403 | Forbidden - Insufficient permissions |
| 404 | Not Found - Resource missing |
| 429 | Too Many Requests - Rate limited |
| 500 | Internal Server Error |
| 503 | Service Unavailable |

### Application Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `INVALID_VPN_TYPE` | 400 | VPN type must be "wireguard" |
| `INVALID_REQUEST` | 400 | Malformed request body |
| `IP_MISMATCH` | 400 | IP address doesn't match |
| `INVALID_STATUS` | 400 | Invalid state transition |
| `UNAUTHORIZED` | 401 | Invalid/missing API key |
| `API_KEY_REVOKED` | 403 | API key revoked |
| `VPN_KEY_NOT_FOUND` | 404 | VPN key not found |
| `LEASE_NOT_FOUND` | 404 | Lease not found |
| `SERVER_NOT_FOUND` | 404 | Server not found |
| `NO_AVAILABLE_IPS` | 429 | IP pool exhausted |
| `INTERNAL_ERROR` | 500 | Server error |

---

## Change Log

| Version | Date | Changes |
|--------|------|--------|
| 1.0.0 | 2026-04-23 | Initial specification |

---

## See Also

- [Operator Guide](./ops/operator-guide-ip-allocation.md) - Deployment and operations
- [Implementation Guide](./ip-allocation-implementation-guide.md) - Agent integration
- [Runbook](./runbooks/wireguard-ip-allocation-runbook.md) - Troubleshooting