# Tareas de Implementación: `fix-backend-gaps-and-lean-arch`

Lista de tareas atómicas ordenadas por prioridad para ejecutar mediante `/opsx:apply`.

---

## Fase 1: Toolchain, Configuración y Migraciones Docker (Prioridad Crítica)

- [ ] **1. Alinear versión oficial de Go (1.25) y documentación de runtime**
  * Verificar que `go.mod`, `Dockerfile` y `.github/workflows/ci.yml` coincidan exactamente en `1.25` (o ajustar los tres de forma uniforme si el entorno de despliegue exige `1.22`).
  * Documentar la versión oficial de Go y el flujo de arquitectura lean sin Redis en `docs/README.md`.
- [ ] **2. Conectar `cfg.SessionTTL` en `AuthService`**
  * Modificar `internal/core/services/auth_service.go` (`AuthService` / `NewAuthService`) para aceptar un `sessionTTL time.Duration` opcional con fallback a `domain.SessionTTL` (`20m`).
  * Pasar `cfg.SessionTTL` desde `cmd/api/main.go` y agregar test unitario verificando el TTL configurado.
- [x] **3. Reemplazar montaje de `/docker-entrypoint-initdb.d` en Docker por runner explícito de migraciones**
  * Quitar `- ./migrations:/docker-entrypoint-initdb.d:ro` de `docker-compose.yml` para evitar que Postgres ejecute archivos `.down.sql`.
  * Agregar servicio `migrate` (`migrate/migrate:v4.17.1` o equivalente fijado) en `docker-compose.yml` (y revisar `docker-compose.prod.yml`) apuntando a `/migrations` con `depends_on` saludable sobre `postgres` y `condition: service_completed_successfully` en `api`.
  * Verificar levantando un volumen limpio que todas las migraciones `000001`..`000014` suban sin errores.

---

## Fase 2: Coherencia Temporal y Blindaje Transaccional en PostgreSQL (Prioridad Crítica)

- [ ] **4. Estandarizar el uso estricto de `NOW()` de PostgreSQL en consultas de TTL y expiración**
  * En `internal/adapters/repositories/postgres/booking_repository.go`:
    * Actualizar `LockExpiredPendingBookings` para usar `expires_at <= NOW()`.
    * Actualizar `CheckPassengerBookingConstraints` para usar `b.expires_at > NOW()`.
    * Actualizar `MarkBookingsExpiredBatch` para asignar `updated_at = NOW()`.
    * En `CreateWithHold`, calcular `expires_at` desde `NOW()` (o mantener consistencia retornando las marcas de Postgres con `RETURNING`).
  * En `internal/adapters/repositories/postgres/trip_repository.go`: usar `updated_at = NOW()` en `ApplySeatTransitionsBatch`.
  * En `internal/adapters/repositories/postgres/session_repository.go`: usar `NOW()` y `NOW() + ($ttl * INTERVAL '1 second')` en `Save`.
  * Ajustar los tests existentes de repositorios/servicios afectados.
- [ ] **5. Blindar `TripService.CompleteTrip` y `TripService.CancelTrip` con `ports.TransactionManager`**
  * Añadir `txManager ports.TransactionManager` a `TripService` (`internal/core/services/trip_service.go`) y conectarlo en `cmd/api/main.go`.
  * Envolver `CompleteTrip` en `WithinTransaction`: propagar todos los errores de `booking.MarkCompleted`, `bookingRepo.UpdateStatus` y `escrowRepo.UpdateEscrowStatus` (eliminar todos los `_ = ...`), y decidir explícitamente qué hacer con escrows en estado `DISPUTED` (no liberarlos automáticamente).
  * Envolver `CancelTrip` en `WithinTransaction` para cancelar reservas activas del viaje, restituir estado/asientos y coordinar liquidación de reembolsos sin dejar estados inconsistentes.
  * Añadir tests unitarios de rollback ante fallo parcial de repositorio en `trip_service_test.go`.
- [ ] **6. Unificar frontera transaccional en `BookingService.CancelBooking` y hardening de `CreateBooking`**
  * Refactorizar `BookingService.CancelBooking` y `SettlementService.SettleCancellationRefund` para que la actualización de estado de reserva, liberación de asientos en `trips`, actualización de `escrow_transactions` y registro en `refund_transactions` ocurran dentro de la **misma** transacción `WithinTransaction` (`txCtx`).
  * Garantizar idempotencia en llamadas previas a `paymentGateway.ProcessRefund` y manejo seguro si falla la BD tras el reembolso externo.
  * Añadir test de integración/unitario simulando error en `RecordRefund` para comprobar rollback atómico de la cancelación y asientos.

---

## Fase 3: Reconciliación Segura de Pagos y Webhooks Tardíos (Prioridad Alta)

- [ ] **7. Crear migración `000014` para auditoría e idempotencia de pagos tardíos (`payment_reconciliation_events`)**
  * Crear `migrations/000014_add_payment_reconciliation_events.up.sql` y `.down.sql` con columnas (`id UUID PK`, `booking_id UUID`, `payment_gateway_ref TEXT`, `event_type`, `status`, `amount NUMERIC(10,2)`, `currency CHAR(3)`, `gateway_refund_ref TEXT`, `error_message TEXT`, `created_at`, `updated_at`) y restricción `UNIQUE (booking_id, payment_gateway_ref)`.
- [ ] **8. Implementar reembolso compensatorio automático ante webhook tardío sobre reserva expirada**
  * Agregar puerto `PaymentReconciliationRepository` (y su implementación PostgreSQL) + método compensatorio idempotente en `SettlementService` (`RefundLateExpiredPayment`).
  * En `BookingService.ConfirmBookingPayment` (`IsWebhook == true`): si la reserva está `EXPIRED` (o vence justo antes del `UPDATE`), verificar la captura en pasarela, registrar/consultar evento de reconciliación idempotente, ejecutar `paymentGateway.ProcessRefund` por `booking.TotalPrice` y retornar error tipado o resultado compensado sin confirmar asientos.
  * En `BookingHandler.ProcessPaymentWebhook`: devolver `200 OK` con `status: "expired_auto_refunded"` si el reembolso compensatorio fue exitoso, o `503` si falló para permitir reintento seguro.
  * (Opcional/Best-effort) En `BookingExpirationService.ProcessExpiredBookings`, publicar/invocar cancelación asíncrona no bloqueante de `payment_intent_id` tras confirmar la transacción de expiración.
  * Escribir tests cubriendo la carrera `ExpirationWorker` vs `ProcessPaymentWebhook` (incluyendo reintento idempotente del webhook).

---

## Fase 4: Workers, Concurrencia y Graceful Shutdown (Prioridad Alta)

- [ ] **9. Añadir `sync.WaitGroup`, `recover()` y drenado ordenado de workers en `cmd/api/main.go`**
  * Envolver `ExpirationWorker.runOnce`, `PostgresSessionRepository.StartCleanup` y `StartCleanup` de rate limiters con `defer recover()` + log de stack trace para evitar que un panic derribe el servidor HTTP.
  * Registrar todos los workers en un `sync.WaitGroup` en `cmd/api/main.go`.
  * En el bloque de shutdown (`SIGINT`/`SIGTERM`), drenar primero `httpServer.Shutdown`, luego invocar `cancelWorker()`, esperar `wg.Wait()` con timeout acotado (`cfg.ShutdownTimeout`) y finalmente cerrar `dbPool.Close()`.
- [ ] **10. Mitigar contención de locks, deadlocks (ABBA) y sincronización de ticks en `ExpirationWorker`**
  * Reducir `ExpirationBatchSize` / `defaultExpireBookingsBatchSize` de `500` a `100` y ajustar `maxBatchesPerTick` (ej. `20`) en `internal/worker/expiration_worker.go` y `booking_expiration_service.go`.
  * Añadir jitter aleatorio inicial (`0` a `interval/4`) en `ExpirationWorker.Start`.
  * Implementar helper de reintento breve con backoff ante errores transitorios de PostgreSQL `40P01` (deadlock) o `40001` en `runInTx` / `BookingExpirationService`.
  * Añadir tests de concurrencia con `-race` validando cierre limpio tras cancelación de contexto y recuperación ante panic simulado.

---

## Fase 5: Caché Local Acotada (`ristretto` + `singleflight`) y Rate Limiting (Prioridad Media)

- [ ] **11. Incorporar `ristretto` + `singleflight` en `go.mod` e implementar adaptador genérico acotado**
  * Ejecutar `go get github.com/dgraph-io/ristretto golang.org/x/sync/singleflight`.
  * Crear `internal/adapters/cache/local_cache.go` con configuración acotada en memoria (ej. `MaxCost: 32 MiB`, `NumCounters: 100_000`), método `GetOrLoad` protegido con `singleflight.Group` y método `Invalidate` (`Del` + `Wait`).
- [ ] **12. Aplicar caché TTL 15–30s únicamente en lecturas tolerantes**
  * Envolver `PricingConfigRepository` con un decorador en caché (TTL `20s`) que invalide la clave local al ejecutar `UpdateActive`.
  * Cachear lectura de perfiles públicos / vehículo en `TripService.GetTripByID` o decorador de `UserRepository`/`VehicleRepository` de solo lectura no crítica (TTL `20s`), **sin cachear** `PostgresSessionRepository` ni checks de autorización crítica.
  * Añadir caché de rutas OSRM en `osrmProvider` (TTL `60s` por coordenadas redondeadas a 5 decimales) usando `singleflight`.
- [ ] **13. Acotar `MemoryRateLimiter` y reforzar guardrails de `RATE_LIMIT_STORE=postgres`**
  * Añadir cota `maxEntries` (por defecto `50000`) y evicción segura en `internal/adapters/external/ratelimit/memory_rate_limiter.go` para prevenir OOM ante ataques de múltiples claves.
  * Actualizar `configs/config.go` y `.env.example` documentando/exigiendo `RATE_LIMIT_STORE=postgres` fuera de entornos de instancia única (`ENV=production` o multirréplica).
  * Añadir tests unitarios verificando límite máximo de entradas en `MemoryRateLimiter` y comportamiento `singleflight` + TTL en `local_cache`.

---

## Fase 6: Verificación Integral

- [ ] **14. Ejecutar suite completa y validaciones de build**
  * Ejecutar `go test -race -v ./...` y `go vet ./...`.
  * Validar compilación del binario (`go build ./cmd/api/main.go`) y construcción Docker (`docker build` / validación de `docker-compose.yml`).
  * Confirmar que no existen fugas de goroutines ni advertencias del detector de carreras (`-race`).
