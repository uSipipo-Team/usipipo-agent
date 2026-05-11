# Design Doc: WireGuard-Only Agent Refactor

**Date:** 2026-05-10  
**Status:** Approved  
**Author:** Kilo AI (with uSipipo Team)  
**Version:** 1.0

---

## 1. Problem Statement

El agente uSipipo actualmente soporta tres protocolos VPN: Outline, WireGuard y TrustTunnel. Sin embargo, la estrategia de producto ha cambiado:

- **Outline** ya no se utiliza activamente en producción
- **TrustTunnel** (AdGuard) fue un experimento que no escaló
- **WireGuard** es el único protocolo con adopción real y mantenimiento activo

Mantener código muerto aumenta:
- Complejidad de mantenimiento
- Superficie de ataque (vulnerabilidades en código no usado)
- Tamaño binario (~3MB de código innecesario)
- Tiempo de revisión de código
- Confusión en documentación

**Objetivo:** Refactorizar el agente para que sea **WireGuard-only**, eliminando completamente Outline y TrustTunnel.

---

## 2. Scope

### In-Scope (Qué se elimina)

#### Outline VPN
- Paquete `internal/vpn/outline.go` (324 líneas)
- Tests: `outline_test.go`
- API endpoints:
  - `POST /outline/keys`
  - `DELETE /outline/keys/:id`
  - `POST /outline/keys/:id/regenerate`
- Configuración:
  - `OUTLINE_API_URL`
  - `OUTLINE_VERIFY_SSL`
  - `SUPPORTS_OUTLINE` (flag derivado)
- Métricas asociadas en `internal/metrics/collector.go` y `types.go`
- Referencias en README, DEPLOYMENT, CHANGELOG, scripts

#### TrustTunnel (AdGuard)
- Paquete `internal/vpn/trusttunnel.go` (387 líneas)
- Paquete `internal/vpn/trusttunnel_metrics.go` (94 líneas)
- Tests: `trusttunnel_test.go`, `trusttunnel_metrics_test.go`
- API endpoints (8):
  - `POST /trusttunnel/clients`
  - `DELETE /trusttunnel/clients/:username`
  - `GET /trusttunnel/clients`
  - `POST /trusttunnel/clients/:username/export`
  - `POST /trusttunnel/clients/:username/export-deeplink`
  - `GET /trusttunnel/metrics`
  - `POST /trusttunnel/rules`
  - `DELETE /trusttunnel/rules`
- Configuración (5 variables):
  - `TRUSTTUNNEL_BINARY`
  - `TRUSTTUNNEL_CONFIG_DIR`
  - `TRUSTTUNNEL_DOMAIN`
  - `TRUSTTUNNEL_PORT`
  - `TRUSTTUNNEL_PUBLIC_PORT`
- Métricas asociadas
- Dependencia: `github.com/pelletier/go-toml/v2` (exclusiva de TrustTunnel)
- Referencias en documentación

### Out-of-Scope (Qué se queda)

- **WireGuard completo:** incluyendo IP allocation DB-first, reconciliation, metrics
- **Infraestructura:** API server, middleware, security, rate limiting
- **Registro automático** con backend
- **Configuración general:** backend URL, API key, agent port, logging
- **Tests de WireGuard:** unit, integration, chaos tests
- **Documentación WireGuard:** WIREGUARD-SETUP.md, IP allocation guides

---

## 3. Architecture Decisions

### Decision 1: Eliminación Total vs Feature Flags

**Opción A: Feature Flags** (mantener código, deshabilitar en runtime)
- Pros: Rollback fácil, gradual migration
- Contras: Código muerto permanece, complexity penalty, no reduce binario

**Opción B: Eliminación Total** (escogida)
- Pros: Código limpio, binario más pequeño, cero mantención de código muerto
- Contras: Breaking change inmediato, requiere backend compatible

**Rationale:** El producto ya no usa Outline/TrustTunnel. No hay necesidad de gradualidad. Un breaking change controlado es más simple que mantener código inactivo.

---

### Decision 2: Backend Compatibility Strategy

**Opción A: Backward-compatible** (backend acepta ambos formatos)
- Pros: Despliegue independiente, sin coordinar
- Contras: Código backward-compatible persiste indefinidamente

**Opción B: Coordinated cutover** (backend actualizado primero)
- Pros: Código limpio en ambos lados
- Contras: Requiere sincronización de despliegues

**Decisión:** **B - Coordinated cutover**  
El backend debe ser actualizado ANTES del agente para que no espere campos `supports_outline` ni métricas de Outline/TrustTunnel.

**Acción requerida (fuera de este PR):** Actualizar backend en paralelo.

---

### Decision 3: Limpieza de Datos Huérfanos

TrustTunnel almacena datos en archivos:
- `/opt/trusttunnel/credentials.toml`
- `/opt/trusttunnel/rules.toml`

**Opción A: Auto-cleanup en primer arranque**
- Pros: Automático, no requiere intervención
- Contras: Puede borrar datos que el usuario quiera preservar

**Opción B: Script manual de limpieza**
- Pros: Control total del operador
- Contras: Olvidado, acumula basura

**Opción C: Ambas** — Agente intenta limpiar, pero proveer script manual `cleanup-trusttunnel.sh`

**Decisión:** **C - Ambas**  
El agente, al iniciar sin soporte TrustTunnel, detectará y limpiará `TrustTunnelConfigDir` si existe y está vacío (seguro). Además, se provee script para limpieza manual.

---

## 4. Data Model Changes

### Metadata de Registro (Registrar)

**Antes:**
```json
{
  "hostname": "...",
  "ip_address": "...",
  "supports_outline": true,    ← ELIMINAR
  "supports_wireguard": true
}
```

**Después:**
```json
{
  "hostname": "...",
  "ip_address": "...",
  "supports_wireguard": true
}
```

**Impacto backend:** Debe aceptar payload sin `supports_outline`.

---

### Métricas (MetricsHandler)

**Antes:**
```json
{
  "system": {...},
  "vpn": {
    "outline": {"active_keys": 42, ...},    ← ELIMINAR
    "wireguard": {"active_peers": 38, ...}
  },
  "detailed": {...}  // Outline time-series      ← ELIMINAR
}
```

**Después:**
```json
{
  "system": {...},
  "vpn": {
    "wireguard": {"active_peers": 38, ...}
  }
}
```

**Impacto backend:** Debe manejar `outline` y `detailed` como opcionales/null.

---

### Health Check

**Antes:**
```json
{
  "status": "healthy",
  "agent_status": "online",
  "outline": "online",       ← ELIMINAR
  "wireguard": "online",
  "timestamp": ...
}
```

**Después:**
```json
{
  "status": "healthy",
  "agent_status": "online",
  "wireguard": "online",
  "timestamp": ...
}
```

---

## 5. API Changes

### Endpoints Eliminados

| Método | Endpoint | Razón |
|--------|----------|-------|
| POST | `/outline/keys` | Outline eliminado |
| DELETE | `/outline/keys/:id` | Outline eliminado |
| POST | `/outline/keys/:id/regenerate` | Outline eliminado |
| POST | `/trusttunnel/clients` | TrustTunnel eliminado |
| DELETE | `/trusttunnel/clients/:username` | TrustTunnel eliminado |
| GET | `/trusttunnel/clients` | TrustTunnel eliminado |
| POST | `/trusttunnel/clients/:username/export` | TrustTunnel eliminado |
| POST | `/trusttunnel/clients/:username/export-deeplink` | TrustTunnel eliminado |
| GET | `/trusttunnel/metrics` | TrustTunnel eliminado |
| POST | `/trusttunnel/rules` | TrustTunnel eliminado |
| DELETE | `/trusttunnel/rules` | TrustTunnel eliminado |

**Respuesta:** Todos devuelven `404 Not Found` en nueva versión.

---

## 6. Configuration Changes

### Variables Eliminadas

**Outline:**
```
OUTLINE_API_URL
OUTLINE_VERIFY_SSL
SUPPORTS_OUTLINE  (derivado, ya no existe)
```

**TrustTunnel:**
```
TRUSTTUNNEL_BINARY
TRUSTTUNNEL_CONFIG_DIR
TRUSTTUNNEL_DOMAIN
TRUSTTUNNEL_PORT
TRUSTTUNNEL_PUBLIC_PORT
```

### Nuevas Variables (si se añaden)

Ningunas. Se mantieneWireGuard-only.

---

## 7. Dependencies

### Eliminar

```diff
- github.com/pelletier/go-toml/v2 v2.1.0
```

**Justificación:** Esta dependencia es usada exclusivamente por TrustTunnel para parsear `credentials.toml` y `rules.toml`. No hay otro uso en el códigobase.

### Mantener (sin cambios)

- `golang.zx2c4.com/wireguard/wgctrl` — WireGuard
- `github.com/gin-gonic/gin` — API
- `github.com/go-resty/resty/v2` — HTTP client (usado por IP allocation)
- `github.com/spf13/viper` — config
- `github.com/shirou/gopsutil/v3` — metrics
- etc.

---

## 8. File Removal List

### Eliminar completamente

```
internal/vpn/outline.go
internal/vpn/outline_test.go
internal/vpn/trusttunnel.go
internal/vpn/trusttunnel_metrics.go
internal/vpn/trusttunnel_test.go
internal/vpn/trusttunnel_metrics_test.go
```

### Modificar (para eliminar referencias)

```
internal/api/handlers.go         # Eliminar Outline/TrustTunnel handlers
internal/metrics/collector.go    # Eliminar métodos Outline/TrustTunnel
internal/metrics/types.go        # Eliminar OutlineMetrics, TrustTunnelMetrics structs
internal/config/config.go        # Eliminar 8 config vars
internal/registrar/registrar.go  # Eliminar SupportsOutline field
cmd/agent/main.go                # Eliminar inicialización Outline/TrustTunnel
```

### Actualizar (documentación)

```
README.md
DEPLOYMENT.md
CHANGELOG.md
.env.example
AUTO-REGISTRATION-GUIDE.md
scripts/install.sh
```

### Eliminar (documentación TrustTunnel-specific)

```
docs/WIREGUARD-SETUP.md              # MANTENER (WireGuard)
docs/ip-allocation-implementation-guide.md  # MANTENER
docs/ops/                            # ELIMINAR (contenido TrustTunnel)
docs/runbooks/                       # ELIMINAR (contenido TrustTunnel)
docs/api/                            # ELIMINAR (contenido TrustTunnel)
docs/alerting/wireguard-ip-allocation-alerts.yaml  # MANTENER
```

---

## 9. Security Considerations

### No hay nuevos riesgos de seguridad

- Se reduce la superficie de ataque al eliminar código
- No se introducen nuevas dependencias
- No se modifican permisos o autenticación

### Consideración: Limpieza de archivos TrustTunnel

Los archivos `credentials.toml` contienen passwords de clientes TrustTunnel. Deben ser eliminados de forma segura:

```bash
# Script de limpieza (shred + rm)
shred /opt/trusttunnel/credentials.toml
shred /opt/trusttunnel/rules.toml
rm -rf /opt/trusttunnel/
```

El agente puede detectar la existencia de TrustTunnel en primer arranque y limpiar automáticamente (con logging).

---

## 10. Testing Strategy

### Unit Tests

- Ejecutar: `go test ./...`
- Esperado: Todos los tests de WireGuard pasan
- Los tests de Outline/TrustTunnel deben eliminarse (no existir)

### Integration Tests

- WireGuard integration tests (`-tags=integration`) deben pasar
- No debe haber tests de Outline/TrustTunnel

### Manual Smoke Test

1. Construir binario: `go build -o agent ./cmd/agent`
2. Ejecutar: `./agent` con `.env` mínima (sin Outline/TrustTunnel vars)
3. Verificar health: `curl localhost:8080/health` → status + wireguard solo
4. Verificar metrics: `curl localhost:8080/metrics` → sin outline/detailed
5. Probar WireGuard endpoints (si wg0 existe): crear/eliminar peer
6. Verificar logs: sin errores de Unknown config vars

### CI/CD

- GitHub Actions `ci.yml` debe pasar sin cambios (solo elimina tests que ya no existen)
- No se requiere modificación a release workflow

---

## 11. Migration Path for Production

### Pre-requisitos (Backend)

1. Backend actualizado para manejar:
   - Payload de registro sin `supports_outline`
   - Métricas sin `outline` y `detailed`
   - Endpoints 404 para Outline/TrustTunnel
2. Dashboards de monitoreo actualizados
3. Documentación backend actualizada

### Pasos de Migración (Agente)

**Opción 1: Big Bang (recomendado)**
1. Construir nuevo binario (v0.12.0-wireguard-only)
2. Desplegar a todos los servidores simultáneamente (o rolling update)
3. Verificar health checks y metrics en backend
4. Ejecutar script de limpieza TrustTunnel (si existe)
5. Rollback a versión anterior si surgen errores críticos

**Opción 2: Canary**
1. Desplegar a 1-2 servidores de prueba
2. Validar métricas
3. Expandir gradualmente
4. Limpiar

### Rollback

Si surgen problemas, volver a última versión multi-protocolo (v0.11.4).
El backend debe soportar ambos formatos de métricas durante transición.

---

## 12. Success Criteria

✅ **Código**
- Todos los archivos Outline/TrustTunnel eliminados
- 0 referencias a `outline` o `trusttunnel` en códigogo
- go.mod sin go-toml/v2
- Build exitoso en todas las plataformas

✅ **Tests**
- `go test ./...` pasa
- No hay tests de Outline/TrustTunnel
- Tests de WireGuard intactos

✅ **Documentación**
- README refleja WireGuard-only
- .env.example sin Outline/TrustTunnel vars
- CHANGELOG con breaking change explícito

✅ **API**
- Endpoints eliminados devuelven 404
- Health check sinOutline field
- Metrics response sinOutline/Detailed

✅ **Binario**
- Tamaño reducido (al menos 10% más pequeño)
- Sin errores en logs al iniciar sin Outline/TrustTunnel configs

---

## 13. Risks and Mitigations

| Riesgo | Probabilidad | Impacto | Mitigación |
|--------|--------------|---------|------------|
| Backend no compatible | Alta | Alto | Actualizar backend PRIMERO, pruebas de integración |
| Despliegue rompe servidores existentes | Media | Alto | Canary deployment, rollback plan |
| Archivos TrustTunnel huérfanos | Alta | Bajo | Script de limpieza automático+manual |
| Errores en IP allocation después de cambios | Baja | Alta | Tests de WireGuard existentes son extensos |
| Documentación desactualizada | Media | Medio | Checklist de actualización de docs |

---

## 14. Open Questions

1. **¿El backend ya fue actualizado** para soportar agentes sin `supports_outline`? (CRÍTICO)
2. ¿Hay servidores en producción usando actualmente Outline o TrustTunnel? Si sí, ¿se requiere migración de datos?
3. ¿Debemos mantener el código por 1-2 releases más como "deprecated" antes de eliminar?
4. ¿Qué versión semántica usar? `v0.12.0` (breaking) o `v1.0.0`?

---

## 15. Recommended Next Steps

1. Obtener confirmación del backend team: backend acepta agentes WireGuard-only
2. Crear branch `refactor/wireguard-only`
3. Eliminar código según lista de archivos
4. Actualizar tests y documentación
5. Build + smoke test local
6. PR con changeset
7. Merge after approval
8. Tag release `v0.12.0-wireguard-only`
9. Deploy a staging
10. Deploy a producción (coordinado con backend)

---

**Design Approved By:** uSipipo Team  
**Approval Date:** 2026-05-10
