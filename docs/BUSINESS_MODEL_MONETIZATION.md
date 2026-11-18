# BUSINESS_MODEL_MONETIZATION.md — Modelo de Negocio, Monetización y Parametrización de Tarifas

## 1. Propósito y Contexto Estratégico

**Thumbi** es una plataforma de *carpooling* de mediana y larga distancia diseñada bajo el principio de **economía colaborativa no lucrativa** para los conductores (*Cap Pricing* definido en [SPEC_MODULE_2.md](../SPEC_MODULE_2.md)).

En su versión inicial, el sistema operaba como un intermediario puro con **comisión 0%** (*pass-through*): el pasajero pagaba únicamente `UnitPrice × SeatsBooked` y el 100% se liberaba al conductor o se reembolsaba ante cancelaciones. Ese esquema generaba margen de contribución negativo al incorporar costos reales de pasarela de pagos, verificación biométrica (KYC), infraestructura y soporte.

Este documento define el **modelo de negocio rentable (Fase 1)** y sus **implicancias funcionales y técnicas** en todo el sistema, preservando el escudo legal no lucrativo del conductor.

---

## 2. Arquitectura del Modelo de Negocio

### 2.1 Principio Rector: Protección del *Cap Pricing* (Oferta)
* **El conductor NO paga comisión descontada de su *Cap Price*.**
* El precio por asiento publicado por el conductor (`trip.PricePerSeat <= trip.CapPricePerSeat`) representa únicamente la prorrata de combustible, peajes y hasta un 10% de imprevistos.
* Al completar el viaje (`COMPLETED`), el conductor recibe el **100% del monto base de los asientos reservados** (`BaseAmount = UnitPrice × SeatsBooked`).
* **Beneficio legal y de liquidez:** El conductor no lucra (mantiene el encuadre de transporte colaborativo no comercial) y no pierde incentivo para publicar viajes.

### 2.2 Palanca Principal (Core Revenue): *Passenger Service Fee* Parametrizable
La plataforma monetiza cobrando una **tarifa de servicio al pasajero** (*Service Fee*) sumada por encima del aporte al viaje:

$$\text{BaseAmount} = \text{UnitPrice} \times \text{SeatsBooked}$$
$$\text{PlatformFee} = \text{RoundHalfUp}\left(\frac{\text{BaseAmount} \times \text{PassengerServiceFeeBps}}{10000}\right) + \text{PassengerFixedFee}$$
$$\text{TotalPrice} = \text{BaseAmount} + \text{PlatformFee}$$

* **Regla de viaje gratuito:** Si `BaseAmount == 0`, `PlatformFee = 0` (no se cobra tarifa de gestión sobre viajes cedidos sin costo).
* **Regla de moneda para tarifa fija:** `PassengerFixedFee` se suma únicamente si su moneda coincide con la moneda del viaje (por defecto `ARS`), mientras que el porcentaje (`PassengerServiceFeeBps`) aplica independientemente de la moneda.
* **Valor por defecto inicial:** `15.00%` (`1500` bps) + `$0.00 ARS` fijo, **100% parametrizable por el Administrador**.

### 2.3 Palanca Secundaria: Retención Operativa en Cancelaciones y Reembolsos
Cuando se procesan cancelaciones sobre reservas ya pagadas (`EscrowStatus == HELD`), las pasarelas de pago ya devengaron costos por la captura (*pay-in*) y/o reembolso (*refund*). Thumbi protege su margen mediante reglas parametrizables sobre dos componentes separados: el `PlatformFee` y el `BaseAmount`.

| Escenario de Cancelación / Reembolso | Quién inicia | Qué ocurre con el `PlatformFee` | Qué ocurre con el `BaseAmount` | Estado de `Escrow` resultante |
| :--- | :--- | :--- | :--- | :--- |
| **`DRIVER_CANCELLATION`** | Conductor | **100% reembolsado al pasajero** (0% retención) | **100% reembolsado al pasajero** | `REFUNDED_FULL` |
| **`PASSENGER_EARLY`** ($\Delta t \ge \text{LateCancelWindowHours}$) | Pasajero (temprana) | Se retiene `EarlyCancelFeeRetentionBps` % (default **100%** retenido por Thumbi) y se devuelve el resto al pasajero | **100% reembolsado al pasajero**, `0%` conductor | `REFUNDED_PARTIAL` (si hay retención > 0) o `REFUNDED_FULL` |
| **`PASSENGER_LATE`** ($\Delta t < \text{LateCancelWindowHours}$) | Pasajero (tardía) | Se retiene `LateCancelFeeRetentionBps` % (default **100%** retenido por Thumbi) y se devuelve el resto al pasajero | Se distribuye exactamente en 3 vías parametrizables que **suman 100% (`10000` bps)**: Pasajero (default **50%**) + Conductor (default **50%**) + Retención Operativa Plataforma (default **0%**) | `REFUNDED_PARTIAL` |
| **`ADMIN_DISPUTE`** (`RESOLVED_PASSENGER_REFUND`) | Mesa de Control | Reembolso total del `Escrow.Amount` salvo resolución explícita | Reembolso total al pasajero | `REFUNDED_FULL` |

#### Invariante Contable de Conservación de Fondos
En toda liquidación de reembolso se cumple estrictamente al centavo (aritmética entera sin `float64`):
$$\text{PassengerRefundAmount} + \text{DriverCompensationAmount} + \text{PlatformRetainedAmount} \equiv \text{Escrow.Amount}$$

### 2.4 Unit Economics Objetivo
* **Ingreso bruto por reserva completada:** $\text{PlatformFee}$
* **Costo variable directo por reserva:** $\text{Costo Pasarela (Pay-in + Payout)} + \text{Amortización KYC} + \text{Ruteo/Notificaciones}$
* **Margen de contribución por reserva:** $\text{PlatformFee} - \text{Costos Variables}$ (el *fee* parametrizable permite al Administrador ajustar el *take rate* por encima del costo de pasarela local según el corredor y ticket promedio).
* **Protección de cancelaciones:** Al retener el *Service Fee* (y opcionalmente un porcentaje de retención operativa en cancelaciones tardías), las cancelaciones del pasajero dejan de generar pérdidas netas por costos financieros irrecuperables.

### 2.5 Hoja de Ruta Futura (Fuera de Alcance de esta Fase)
Las siguientes palancas quedan documentadas para iteraciones posteriores y **no forman parte de esta entrega**:
* Pase mensual / suscripción de pasajero frecuente (*Commuter Pass* con 0% *Service Fee*).
* Convenios B2B / campus universitarios / parques industriales.
* *Add-on* de micro-seguros de viaje o asistencia en ruta.

---

## 3. Requisitos Funcionales de Parametrización (EARS)

* **RF-MON-01 (Configuración Dinámica):** EL sistema DEBE persistir una configuración activa de tarifas (`PlatformPricingConfig`) con todos los porcentajes expresados en puntos básicos enteros (`bps`, donde `100 bps = 1.00%`) y montos fijos en centavos exactos (`Money`).
* **RF-MON-02 (Control de Acceso Administrativo):** SOLO usuarios autenticados con rol `ADMIN` (`domain.RoleAdmin`) PUEDEN consultar el historial/auditoría administrativa y modificar los parámetros de precios mediante `PUT /v1/admin/pricing-config`.
* **RF-MON-03 (Validación de Invariantes de Negocio):** CUANDO un administrador actualice la configuración de tarifas, EL sistema DEBE validar que:
  1. `passenger_service_fee_bps` esté en `[0, 5000]` (0% a 50%).
  2. `passenger_fixed_fee >= 0`.
  3. `late_cancel_window_hours` esté en `[1, 168]` (1 hora a 7 días).
  4. `early_cancel_fee_retention_bps` y `late_cancel_fee_retention_bps` estén en `[0, 10000]`.
  5. `late_cancel_passenger_refund_bps + late_cancel_driver_comp_bps + late_cancel_platform_retention_bps == 10000` (cada componente en `[0, 10000]`).
* **RF-MON-04 (Inmutabilidad por Reserva / *Snapshot*):** CUANDO un pasajero cree una reserva (`POST /v1/bookings`), EL sistema DEBE calcular y congelar `base_amount`, `platform_fee` y `total_price` en la entidad `Booking` y posteriormente en `EscrowTransaction`. Cambios posteriores realizados por el administrador NO deben alterar retroactivamente reservas ya creadas o pagadas.
* **RF-MON-05 (Transparencia en Checkout):** EL sistema DEBE exponer endpoints de lectura pública (`GET /v1/pricing/config` y `GET /v1/pricing/quote`) para que los clientes Web y Móvil muestren explícitamente al pasajero el desglose entre aporte al conductor y gastos de servicio antes de confirmar el pago.
* **RF-MON-06 (Liquidación de Viaje Completado):** CUANDO un viaje finalice (`COMPLETED`), EL sistema DEBE liberar los fondos en custodia (`RELEASED`) acreditando al conductor exclusivamente `Escrow.DriverPayoutAmount()` (`BaseAmount`) y reconociendo `Escrow.PlatformFee` como ingreso retenido por la plataforma.

---

## 4. Implicancias Técnicas en el Proyecto

### 4.1 Base de Datos y Migraciones (`migrations/000013_add_platform_monetization_and_pricing_config`)
1. **Nueva tabla `platform_pricing_config`:**
   * Almacena la configuración parametrizable vigente con restricción de unicidad para una única fila activa (`uq_platform_pricing_active`), `CHECK` constraints estrictos (incluyendo suma exacta `= 10000` bps en cancelación tardía) y trazabilidad (`updated_by`, `updated_at`).
   * Incluye semilla inicial con valores seguros (`passenger_service_fee_bps = 1500`, `passenger_fixed_fee = 0.00`, `early_cancel_fee_retention_bps = 10000`, `late_cancel_window_hours = 24`, `late_cancel_fee_retention_bps = 10000`, `late_cancel_passenger_refund_bps = 5000`, `late_cancel_driver_comp_bps = 5000`, `late_cancel_platform_retention_bps = 0`).
2. **Evolución de `bookings`:**
   * Incorpora `base_amount NUMERIC(10, 2)` y `platform_fee NUMERIC(10, 2)`.
   * Actualiza restricciones: `chk_booking_base_amount (base_amount = ROUND(unit_price * seats_booked, 2))` y `chk_booking_total_price (total_price = base_amount + platform_fee)`.
3. **Evolución de `escrow_transactions`:**
   * Incorpora `base_amount NUMERIC(10, 2)` y `platform_fee NUMERIC(10, 2)` con `chk_escrow_amount_breakdown (amount = base_amount + platform_fee)`.
4. **Evolución de `refund_transactions`:**
   * Incorpora `platform_retained_amount NUMERIC(10, 2)` y actualiza `chk_refund_amounts_positive (passenger_refund_amount + driver_compensation_amount + platform_retained_amount > 0)`.

### 4.2 Capa de Dominio (`internal/core/domain`)
* **`platform_pricing.go`:** Entidad `PlatformPricingConfig`, value object `BookingPriceBreakdown`, `CalculateBookingBreakdown()` y validaciones de invariantes en centavos/bps exactos.
* **`booking.go`:** `Booking` incorpora `BaseAmount` y `PlatformFee` manteniendo `TotalPrice = BaseAmount + PlatformFee`.
* **`escrow.go`:** `EscrowTransaction` incorpora `BaseAmount`, `PlatformFee` y `DriverPayoutAmount()`; `RefundTransaction` incorpora `PlatformRetainedAmount`; `CalculateCancellationRefundWithPolicy()` computa la distribución exacta en 3 vías sin deriva de centavos.

### 4.3 Puertos, Servicios y Repositorios (`internal/core/ports`, `internal/core/services`, `internal/adapters/repositories/postgres`)
* **`PricingConfigRepository` (`pricing_config_repository.go`):** Lectura y actualización atómica de `platform_pricing_config`.
* **`PlatformPricingService` (`platform_pricing_service.go`):** Gestión administrativa (`GetPricingConfig`, `UpdatePricingConfig`) y cotizador de checkout (`QuoteBookingPrice`).
* **`BookingService` (`booking_service.go`):** Consulta la configuración activa al crear la reserva, genera el `PaymentIntent` por `TotalPrice` y constituye el `Escrow` con el desglose `BaseAmount` + `PlatformFee`.
* **`SettlementService` (`settlement_service.go`):** Aplica la configuración vigente de cancelación/retención en `SettleCancellationRefund` y registra `PlatformRetainedAmount` en `refund_transactions`.

### 4.4 Capa HTTP y Frontend (`internal/adapters/handlers/http`, `src/`)
* **Endpoints Públicos:**
  * `GET /v1/pricing/config` — Retorna las reglas de tarifa vigentes para transparencia del usuario.
  * `GET /v1/pricing/quote?trip_id=<uuid>&seats=<n>` — Calcula y devuelve la cotización exacta del checkout.
* **Endpoints Administrativos (`RequireRole(ADMIN)`):**
  * `GET /v1/admin/pricing-config` — Consulta la configuración vigente con metadatos de auditoría.
  * `PUT /v1/admin/pricing-config` — Actualiza porcentajes (`*_bps` o `*_percent`), tarifa fija, ventana horaria y reglas de retención/reparto de cancelaciones.
* **Checkout Web/Móvil (`src/screens/BookingScreen.tsx`):**
  * Reemplaza el valor estático `$0` en *"Gastos de servicio"* por el cálculo dinámico basado en la configuración vigente obtenida desde `/v1/pricing/config` y muestra claramente el porcentaje aplicado y el total a pagar.
