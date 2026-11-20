# Propuesta: `fix-backend-gaps-and-lean-arch`

## Resumen

Esta propuesta formaliza las correcciones arquitectónicas, transaccionales y operativas identificadas en el análisis exploratorio del backend de **Thumbi**, consolidando una arquitectura **lean sin Redis** basada en **Go + Gin (Hexagonal)**, **PostgreSQL 16 + PostGIS** como única fuente de verdad transaccional y **caché en memoria RAM acotada** para lecturas tolerantes a consistencia eventual corta.

---

## Motivación y Problemas Detectados

1. **Alineación de Toolchain (`go.mod` / `Dockerfile` / CI)**
   * El proyecto declara `go 1.25` en `go.mod`, `golang:1.25-alpine` en `Dockerfile` y `GO_VERSION: '1.25'` en `.github/workflows/ci.yml`, mientras que en documentación previa del diseño se asumía Go 1.22. Mantener ambigüedad en la versión objetivo introduce riesgo al compilar o introducir APIs de stdlib.
2. **Riesgo de Bootstrap en Migraciones Docker (`/docker-entrypoint-initdb.d`)**
   * `docker-compose.yml` monta `./migrations:/docker-entrypoint-initdb.d:ro`, pero el directorio contiene pares `.up.sql` y `.down.sql`. Un arranque desde cero de PostgreSQL ejecuta todos los `.sql` en orden alfabético, aplicando `000001_init_schema.down.sql` antes que `.up.sql` y rompiendo la inicialización.
3. **Ausencia de Caché Acotada y Límites en Memoria ("Cero Redis")**
   * Hoy no existe caché en RAM para lecturas frecuentes no críticas (por ejemplo `PlatformPricingConfig`, perfiles públicos de conductor o rutas OSRM), generando lecturas repetidas a Postgres.
   * Al mismo tiempo, `MemoryRateLimiter` utiliza un `map[string]*rateLimitEntry` sin cota máxima de entradas bajo un mutex global. En despliegues multirréplica con `RATE_LIMIT_STORE=memory` los límites se multiplican por `N` réplicas, y un pico de IPs distintas puede provocar crecimiento descontrolado de RAM.
4. **Brechas Transaccionales, Reloj Dual, Workers y Webhooks Tardíos**
   * `TripService.CompleteTrip` y `TripService.CancelTrip` operan sin `ports.TransactionManager` (y `CompleteTrip` descarta errores de actualización de reservas/escrow).
   * `BookingService.CancelBooking` ejecuta la restitución de asientos y `SettleCancellationRefund` en transacciones separadas.
   * El TTL de reservas mezcla reloj de Go (`time.Now()`) y reloj de Postgres (`NOW()`), los workers en segundo plano carecen de `sync.WaitGroup` y `recover()` en shutdown, y un webhook de pago tardío sobre una reserva recién expirada devuelve `410 Gone` sin reembolso automático ni conciliación.

---

## Alcance de la Propuesta

### 1. Estandarización del Runtime Go (Go 1.25)
* **Decisión**: Estandarizar oficialmente el backend en **Go 1.25** (alineado con `go.mod`, `Dockerfile` y `.github/workflows/ci.yml`), documentando en `docs/README.md` y `.env.example`/toolchain que Go 1.25 es la versión soportada. *(Nota: si el equipo de infraestructura exigiera bajar estrictamente a Go 1.22 por compatibilidad de plataforma, el cambio de versión se aplicará de manera uniforme en `go.mod`, `Dockerfile` y `ci.yml` en la Tarea 1).*
* Conectar `cfg.SessionTTL` desde `configs.Config` hacia `services.NewAuthService` en `cmd/api/main.go` para que el TTL de sesión sea realmente configurable vía entorno.

### 2. Consolidación de Estrategia "Cero Redis"
* **Caché local con `github.com/dgraph-io/ristretto` + `golang.org/x/sync/singleflight`**:
  * Incorporar un decorador/adaptador de caché en RAM estrictamente acotado (por coste en bytes, `NumCounters` y TTL corto de **15 a 30 segundos**) únicamente para **lecturas tolerantes a consistencia eventual corta**:
    1. `PlatformPricingConfig` (`PricingConfigRepository.GetActive`): TTL 15–30s con invalidación local inmediata en `UpdateActive` en la réplica que procesa el cambio.
    2. Perfiles públicos de usuario / resumen de conductor-vehículo usados en vistas de búsqueda y detalle de viaje (`GetTripByID` / `buildTripSummary`): TTL 15–30s.
    3. Resultados determinísticos de ruteo OSRM por coordenadas redondeadas (evita saturar la API pública en búsquedas/cotizaciones repetidas).
  * **Exclusiones estrictas de caché en RAM**:
    * **No cachear** tokens de sesión (`PostgresSessionRepository.GetByToken`) ni el estado de bloqueo/rol crítico cuando se requiera revocación inmediata multirréplica (`Logout` o suspensión), salvo eventual opt-in explícito con TTL <= 10s documentado.
    * **No usar `ristretto`** para contadores de rate limiting ni estado transaccional (debido a sus buffers de escritura asíncronos y admisión TinyLFU).
* **Rate Limiting sin Redis**:
  * Mantener `RATE_LIMIT_STORE=postgres` (`PostgresRateLimiter` sobre tabla `UNLOGGED rate_limits`) como **opción obligatoria recomendada en producción / multirréplica** (validada en `configs.Config.Validate()` cuando `ENV=production` o `RATE_LIMIT_REQUIRED_DISTRIBUTED=true`).
  * Endurecer `MemoryRateLimiter` (para entorno local / instancia única) añadiendo límite máximo de claves (`maxEntries`, p. ej. 50.000) con política de evicción para impedir OOM.

### 3. Runner Explícito de Migraciones en Docker
* Eliminar el montaje `./migrations:/docker-entrypoint-initdb.d:ro` del servicio `postgres` en `docker-compose.yml` (y revisar `docker-compose.prod.yml` si aplica).
* Incorporar un servicio efímero `migrate` (imagen oficial `migrate/migrate` o binario `golang-migrate` en `depends_on: service_completed_successfully`) que ejecute explícitamente `migrate -path /migrations -database ... up` antes de levantar `api`, garantizando que los `.down.sql` nunca corran automáticamente durante la inicialización.

---

## Fuera de Alcance (No incluido en este cambio)
* Incorporación de Redis, NATS, Kafka u otros brokers externos.
* Reemplazo de los adaptadores mock externos (`MockPaymentGateway`, `MockBiometricsProvider`, `PushNotifier`) por SDKs productivos reales (Stripe/MercadoPago/FCM), salvo definir los hooks limpios de compensación/reembolso sobre el puerto `ports.PaymentGateway`.
* Implementación de suscripciones/pases mensuales (reservado para fases posteriores de monetización).
