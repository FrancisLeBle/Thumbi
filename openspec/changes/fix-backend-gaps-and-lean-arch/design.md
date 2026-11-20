# Diseño Técnico: `fix-backend-gaps-and-lean-arch`

## 1. Blindaje Transaccional (`TripService` y `BookingService`)

### 1.1 `TripService` con `ports.TransactionManager`
Actualmente `TripService` (`internal/core/services/trip_service.go`) no recibe ni utiliza `ports.TransactionManager`.

* **Inyección**: Añadir `txManager ports.TransactionManager` como dependencia opcional en `NewTripService(...)` (siguiendo el patrón de `NewBookingService` y `NewDisputeService`) y actualizar el wiring en `cmd/api/main.go`.
* **`CompleteTrip(ctx, tripID, driverID)`**:
  1. Ejecutar dentro de `s.runInTx(ctx, func(txCtx context.Context) error { ... })`:
     * Leer el viaje en la transacción (o bloquearlo con guarda/estado).
     * Validar dominio (`trip.TransitionStatus(domain.TripStatusCompleted)`) y persistir `tripRepo.UpdateStatus(txCtx, tripID, domain.TripStatusCompleted)`.
     * Obtener reservas activas (`bookingRepo.GetActiveBookingsByTrip(txCtx, tripID)`).
     * Para cada reserva `CONFIRMED`:
       * Llamar `booking.MarkCompleted(now)` y persistir `bookingRepo.UpdateStatus(txCtx, booking.ID, domain.BookingStatusCompleted, nil)`. **Nunca ignorar el error** (`_ = ...` eliminado).
       * Si existe `escrowRepo`, buscar el escrow de la reserva (`HELD` o `DISPUTED` según política; nota: si el escrow está `DISPUTED`, mantener o rechazar transición explícitamente según regla de negocio — no liberar fondos disputados silenciosamente), ejecutar `escrow.Release(now)` y persistir `escrowRepo.UpdateEscrowStatus(txCtx, escrow.ID, domain.EscrowStatusReleased)` propagando cualquier error para hacer rollback completo si falla.
     * Rechazar o expirar cualquier reserva remanente en `PENDING_PAYMENT` asociada al viaje completado.
* **`CancelTrip(ctx, tripID, driverID)`**:
  1. Validar propiedad del conductor y transición `trip.Cancel()` dentro de `s.runInTx(...)`:
     * Persistir `tripRepo.UpdateStatus(txCtx, tripID, domain.TripStatusCancelled)`.
     * Obtener reservas activas del viaje (`PENDING_PAYMENT` y `CONFIRMED`).
     * Transicionar cada reserva activa a `CANCELLED_BY_DRIVER` dentro de la misma transacción.
     * Para reservas `CONFIRMED` con escrow en `HELD`, ejecutar liquidación/reembolso total del conductor mediante `SettlementService` (separando llamada externa a pasarela de la escritura en BD para no mantener locks largos, o registrando estado pendiente de reembolso reconciliable si la pasarela falla).

### 1.2 Corrección de Flujos en `BookingService`
* **`CancelBooking`** (`internal/core/services/booking_service.go`):
  * Problema actual: la Transacción 1 cambia el booking a `CANCELLED_*` y devuelve asientos al viaje; luego `SettleCancellationRefund` corre en una Transacción 2 independiente (tras llamar a `paymentGateway.ProcessRefund`). Si `SettleCancellationRefund` falla en BD, el dinero queda en `HELD` sin `refund_transactions`.
  * Diseño corregido:
    1. En la **Fase 1 (antes de llamar a pasarela o dentro de TX controlada)**: leer `booking`, `trip`, `escrow` y `PricingConfig` para calcular el monto exacto de reembolso (`CalculateCancellationRefundWithPolicy`).
    2. Si corresponde reembolso en pasarela (`passengerRefund > 0`), invocar `paymentGateway.ProcessRefund` con **clave de idempotencia** (`bookingID` / `escrowID`) antes de confirmar la transición terminal, o bien usar una **única transacción** para actualizar `booking.Status` + `releaseTripSeatsWithRetry` + `escrowRepo.UpdateEscrowStatus` + `escrowRepo.RecordRefund` (pasando `txCtx` a `SettleCancellationRefund` una vez obtenido `gatewayRefundRef`).
    3. De este modo nunca queda una reserva cancelada con `escrow` aún `HELD` por fallo de BD.
* **`CreateBooking`**:
  * Problema actual: `CreateWithHold` confirma el hold en Postgres y después llama a `CreatePaymentIntent` y `UpdatePaymentIntent` fuera de transacción.
  * Diseño corregido:
    * Verificar solapamiento y capacidad primero, crear la intención de pago o vincular `payment_intent_id` con manejo explícito de rollback si falla `UpdatePaymentIntent` (ya existe `rejectAndReleaseSeats`), y asegurar que `BookingExpirationService` pueda expirar también reservas en `PENDING_PAYMENT` con `payment_intent_id IS NULL`.

---

## 2. Coherencia Temporal (`NOW()` de PostgreSQL)

Para eliminar desajustes por *clock skew* (NTP drift) entre instancias Go y PostgreSQL:

1. **Fuente única de tiempo para expiraciones y TTLs**:
   * **`CreateWithHold`** (`booking_repository.go`): Persistir `expires_at = NOW() + ($ttl_seconds * INTERVAL '1 second')` (o recibir `TTL` en segundos y calcular `created_at`/`expires_at`/`updated_at` con `NOW()` en SQL), devolviendo vía `RETURNING created_at, expires_at, updated_at` las marcas reales de Postgres al dominio.
   * **`LockExpiredPendingBookings`** (`booking_repository.go`): Cambiar `WHERE status = $1 AND expires_at <= $2` por `WHERE status = $1 AND expires_at <= NOW()` (sin pasar el `time.Now()` de Go como parámetro de corte), retornando además `NOW()` si el dominio necesita el instante exacto de evaluación.
   * **`MarkBookingsExpiredBatch`** (`booking_repository.go`) y **`ApplySeatTransitionsBatch`** (`trip_repository.go`): Usar `updated_at = NOW()` directamente en SQL.
   * **`CheckPassengerBookingConstraints`** (`booking_repository.go`): Usar `b.expires_at > NOW()` en lugar de `$5`.
   * **`PostgresSessionRepository.Save`** (`session_repository.go`): Calcular `created_at = NOW()` y `expires_at = NOW() + ($ttl * INTERVAL '1 second')` en SQL para alinear el alta con `GetByToken` (`expires_at > NOW()`) y `PurgeExpired` (`expires_at <= NOW()`).
2. **Mantenibilidad de Tests Unitarios**:
   * Conservar reloj inyectable (`s.clock`) únicamente en reglas puras de dominio/servicio que no dependan de filtros SQL contra `expires_at`, o permitir sobreescritura en tests de repositorio mediante reloj mockeado solo en tests unitarios sin BD real.

---

## 3. Gestión de Workers, Concurrencia y Graceful Shutdown

### 3.1 Supervisión de Goroutines (`WaitGroup` + `recover()`)
En `cmd/api/main.go` e `internal/worker/`:

* Crear un helper/supervisor de workers o ampliar `ExpirationWorker` y los loops `StartCleanup`:
  * Envolver cada iteración del ticker (`runOnce`, limpieza de sesiones, limpieza de rate limits) en una función con:
    ```go
    defer func() {
        if r := recover(); r != nil {
            log.Printf("[ERROR] panic recuperado en worker (%s): %v\n%s", name, r, debug.Stack())
        }
    }()
    ```
  * Pasar un `*sync.WaitGroup` (o usar `wg.Add(1)` / `defer wg.Done()`) para los 3 workers en segundo plano:
    1. `expirationWorker.Start(workerCtx)`
    2. `sessionRepo.StartCleanup(workerCtx, 5*time.Minute)`
    3. `startRateLimitCleanup(workerCtx)`
* **Secuencia ordenada en `Graceful Shutdown`**:
  1. Al recibir `SIGINT`/`SIGTERM`, detener primero la recepción de nuevas peticiones HTTP (`httpServer.Shutdown(shutdownCtx)`).
  2. Ejecutar `cancelWorker()` para señalizar parada a los workers y esperar con `wg.Wait()` protegido por un canal + `time.After(cfg.ShutdownTimeout)` (ej. 10s–15s) para garantizar que una transacción en curso de `ExpirationWorker` no sea truncada abruptamente.
  3. Cerrar `dbPool.Close()` únicamente **después** de que HTTP y los workers hayan drenado.

### 3.2 Reducción de Contención, Prevención de Deadlocks (ABBA) y Jitter
* **Ordenamiento de Locks y Tamaño de Lote**:
  * Actualmente `ProcessExpiredBookings` bloquea hasta 500 `bookings` (`FOR UPDATE SKIP LOCKED`), actualiza `bookings` y luego bloquea `trips` (`FindByIDsForUpdate`), mientras que `CreateWithHold` bloquea `trips` primero e inserta en `bookings`.
  * Para minimizar contención y riesgo de `40P01` (deadlock):
    * Reducir el tamaño de lote por transacción de `500` a **`100`** (`ExpirationBatchSize = 100`) y aumentar el tope de iteraciones por tick (`maxBatchesPerTick = 20`) para mantener alto throughput manteniendo transacciones breves (<50ms de lock).
    * Agrupar/ordenar determinísticamente las actualizaciones por `trip_id ASC` y manejar reintento automático con backoff corto ante error SQLSTATE `40P01` (`deadlock_detected`) o `40001` (`serialization_failure`).
* **Jitter entre Réplicas**:
  * Antes de entrar al `time.NewTicker(w.interval)`, aplicar un retardo inicial aleatorio (`0` a `interval/4`) cuando `ctx` siga activo, evitando que múltiples réplicas iniciadas en paralelo disparen sus ticks exactamente en el mismo milisegundo.

---

## 4. Manejo Seguro de Pagos y Webhooks Tardíos

### Escenario Crítico (Race Condition minuto 15)
1. El pasajero inicia el pago en la pasarela justo antes del minuto 15.
2. `ExpirationWorker` marca la reserva como `EXPIRED` y restituye los asientos al viaje.
3. Segundos después llega `POST /v1/payments/webhook` (o `ConfirmPayment`) con el cargo ya capturado en la pasarela.
4. Hoy `BookingRepository.UpdateStatus` falla con `domain.ErrBookingExpired`, el handler devuelve `410 Gone`, y el dinero queda cobrado sin reserva ni reembolso.

### Diseño de Reconciliación / Reembolso Automático Seguro
1. **Detección Explícita en `BookingService.ConfirmBookingPayment`**:
   * Verificar el estado real de la reserva y `payment_intent_id`.
   * Si la reserva ya está `EXPIRED` (o `booking.IsExpired(...)` / `UpdateStatus` devuelve `ErrBookingExpired`) **y** `IsWebhook == true` (o el cobro en pasarela fue efectivamente capturado según `s.settlement.ConfirmPayment`):
     * **No confirmar la reserva** (los asientos ya pudieron haber sido vendidos a otro pasajero).
     * Disparar un flujo idempotente de **reembolso automático compensatorio**:
       * Invocar `s.settlement.RefundOrphanedPayment(ctx, booking, input.PaymentGatewayRef, "late_payment_after_booking_expiration")` -> llama a `paymentGateway.ProcessRefund(...)` por `booking.TotalPrice`.
       * Persistir un registro de auditoría/reconciliación en nueva tabla `payment_reconciliation_events` (con `UNIQUE (booking_id, payment_gateway_ref)` para garantizar idempotencia ante reintentos del webhook de la pasarela, guardando estado `AUTO_REFUNDED` o `REFUND_FAILED_NEEDS_MANUAL_ACTION`).
   * **Respuesta HTTP del Webhook (`BookingHandler.ProcessPaymentWebhook`)**:
     * Si el reembolso compensatorio tuvo éxito -> responder `200 OK` con `status: "expired_auto_refunded"` para detener reintentos infinitos de la pasarela.
     * Si la pasarela falló al reembolsar -> registrar `REFUND_FAILED_NEEDS_MANUAL_ACTION`, loguear `[CRITICAL]` y devolver `503 Service Unavailable` / `500` para que el webhook se reintente (o lo procese una tarea de reconciliación).
   * Para llamadas directas del cliente (`IsWebhook == false`) sobre reserva expirada sin captura confirmada: mantener `410 Gone` (`ErrBookingExpired`) evitando capturas tardías si la pasarela soporta cancelación del PaymentIntent en el worker (`CancelPaymentIntent` best-effort fuera de TX tras expirar).

---

## 5. Caché Local Acotada (`ristretto` + `singleflight`) y Rate Limiting

### 5.1 Adaptador Genérico de Caché Local
* Paquete sugerido: `internal/adapters/cache/ristretto_cache.go` usando `github.com/dgraph-io/ristretto` (v1/v2 compatible con Go 1.25) + `golang.org/x/sync/singleflight`.
* Configuración recomendada por defecto para proceso MVP/lean (~32–64 MB de techo):
  * `NumCounters`: `1e5` (10x elementos esperados ~10.000)
  * `MaxCost`: `32 << 20` (32 MiB)
  * `BufferItems`: `64`
* **Reglas de uso obligatorio**:
  * Envoltorio `GetOrLoad(ctx, key, ttl, cost, loaderFn)` protegido con `singleflight.Group` por `key` para eliminar *thundering herd* en arranque frío o expiración de TTL.
  * **TTL corto estricto**: `20s` (rango 15–30s) en `PricingConfig` y perfiles públicos; `60s` para rutas OSRM inmutables entre mismas coordenadas cuantizadas.
  * **No asumir lectura inmediata tras `Set`**: si `UpdateActive` modifica `PlatformPricingConfig`, llamar a `cache.Del(key)` + `cache.Wait()` (o invalidar la clave y dejar que la siguiente lectura regenere vía `singleflight`).

### 5.2 Guardrails para Multirréplica sin Redis
* `configs.Config`:
  * Mantener `RATE_LIMIT_STORE=postgres` documentado y validado: si `ENV=production` y `RATE_LIMIT_STORE=memory` (sin bandera explícita de instancia única `SINGLE_INSTANCE_MODE=true`), emitir error fail-fast o advertencia crítica según política de despliegue para evitar multiplicación `N×` de límites entre réplicas.
* `MemoryRateLimiter`:
  * Añadir límite duro de entradas en mapa (ej. `maxEntries = 50000`). Si se alcanza el tope al insertar una clave nueva, purgar expiradas primero y, si sigue lleno, evictar un muestreo o bloquear nuevas claves efímeras de forma segura sin permitir OOM.
