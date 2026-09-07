# SPEC_MODULE_3.md — Módulo 3: Reservas, Gestión Atómica de Asientos y Pagos en Custodia (Escrow)

## Proyecto
Plataforma de carpooling de mediana y larga distancia (**Thumbi**) basada en economía colaborativa no lucrativa con modelo de gastos compartidos. Arquitectura de microservicios con backend en Go, base de datos PostgreSQL con extensión PostGIS, frontend Web en React y aplicaciones móviles iOS/Android en React Native. Infraestructura orientada a eventos en AWS (API Gateway, Lambda, EKS) gestionada 100% mediante IaC con Terraform.

---

## Contexto de Dominio & Principios Rectores

1. **Economía Colaborativa y Fondos en Custodia (Escrow):**
   - Para proteger tanto al pasajero como al conductor y garantizar que la transacción respete la naturaleza no lucrativa de gastos compartidos, los pagos de las reservas no se transfieren de forma inmediata ni directa a la cuenta del conductor.
   - El sistema actúa como custodio (*Escrow Agent*). Los fondos se retienen en una cuenta de depósito en garantía (*Escrow Account*) desde el momento en que el pasajero realiza el pago de la reserva (`CONFIRMED`) hasta que el viaje se completa satisfactoriamente (`COMPLETED`).
   - La liberación del pago (*Payout / Settlement*) al conductor únicamente se ejecuta una vez finalizado el viaje o verificada la confirmación de llegada de los pasajeros.

2. **Garantía Atómica de Concurrencia y Prevención de Sobreventa (Zero Overbooking):**
   - Múltiples pasajeros pueden competir simultáneamente por los últimos asientos disponibles de un viaje de alta demanda.
   - El sistema debe garantizar consistencia estricta mediante transacciones atómicas en PostgreSQL con bloqueo pesimista a nivel de fila (`SELECT ... FOR UPDATE`) sobre la entidad `trips`, o mecanismos de control de concurrencia serializable/aislada.
   - Nunca debe ser posible que la suma de asientos ocupados más los asientos reservados supere la capacidad total ofrecida (`available_seats >= requested_seats`).

3. **Retención Temporal de Asientos (Seat Hold & TTL Expiration):**
   - Cuando un pasajero inicia una reserva, los asientos solicitados se descuentan inmediatamente del inventario disponible y pasan a un estado temporal de retención (`PENDING_PAYMENT`).
   - Se asigna un tiempo de expiración estricto (TTL de 15 minutos). Si el pago no se completa dentro de esa ventana temporal, la reserva pasa automáticamente a `EXPIRED` y los asientos son devueltos de forma atómica al inventario disponible del viaje.

4. **Transparencia en Gastos Compartidos y Política de Reembolsos:**
   - Si el conductor cancela el viaje, el 100% de los fondos retenidos en Escrow son reembolsados inmediatamente al pasajero sin costo administrativo ni penalización.
   - Si el pasajero cancela con anticipación mayor a 24 horas antes de la partida, recibe un reembolso total (100%).
   - Si el pasajero cancela con menos de 24 horas de antelación o no se presenta al punto de encuentro (*no-show*), se aplica una política de compensación de gastos fijos al conductor prorrateando los fondos retenidos según las reglas de la plataforma.

---

## Requisitos Funcionales (Sintaxis EARS)

### 1. Solicitud de Reserva y Validación de Disponibilidad
* **RF-01 (Ubiquitous):** El sistema DEBE permitir a cualquier usuario autenticado en rol de Pasajero solicitar la reserva de 1 o más asientos en viajes que se encuentren en estado `PUBLISHED`.
* **RF-02 (Event-Driven):** CUANDO un pasajero inicie una solicitud de reserva indicando el `trip_id`, la cantidad de asientos requeridos y el punto de abordaje/descenso (origen/destino o waypoints del trayecto), EL sistema DEBE verificar que el solicitante no sea el mismo conductor del viaje (`passenger_id != driver_id`).
* **RF-03 (State-Driven):** MIENTRAS un pasajero posea una reserva activa (`PENDING_PAYMENT` o `CONFIRMED`) en un viaje que se superponga temporalmente con otro viaje solicitado, EL sistema DEBE rechazar la nueva reserva indicando conflicto de horario.
* **RF-04 (Event-Driven):** CUANDO la cantidad de asientos solicitada sea superior a los asientos disponibles actuales del viaje (`requested_seats > available_seats`), EL sistema DEBE rechazar la solicitud de forma inmediata con un error de disponibilidad insuficiente (`409 Conflict`).

### 2. Control Atómico de Concurrencia y Retención Temporal de Asientos
* **RF-05 (Event-Driven):** CUANDO se reciba una solicitud de reserva válida, EL sistema DEBE iniciar una transacción atómica de base de datos adquiriendo un bloqueo exclusivo de fila (`SELECT available_seats, status FROM trips WHERE id = $1 FOR UPDATE`).
* **RF-06 (Event-Driven):** CUANDO se confirme la disponibilidad dentro de la transacción atómica, EL sistema DEBE decrementar `available_seats = available_seats - requested_seats`, registrar la reserva en estado `PENDING_PAYMENT` con una marca temporal de expiración (`expires_at = NOW() + INTERVAL '15 minutes'`), y comprometer (*commit*) la transacción.
* **RF-07 (Event-Driven):** CUANDO el decremento atómico de asientos reduzca `available_seats` a exactamente 0, EL sistema DEBE actualizar de forma síncrona el estado del viaje a `FULL`, retirándolo temporalmente de los resultados de búsqueda de nuevos pasajeros.
* **RF-08 (Unwanted Behavior):** SI dos o más transacciones concurrentes intentan reservar los mismos asientos restantes y una de ellas agota la disponibilidad, EL sistema DEBE abortar y hacer rollback de las transacciones excedentes, retornando un error de asientos insuficientes sin inconsistencias de saldo ni sobreventa.

### 3. Procesamiento de Pago y Retención en Custodia (Escrow)
* **RF-09 (Event-Driven):** CUANDO una reserva sea creada en estado `PENDING_PAYMENT`, EL sistema DEBE generar una intención de pago vinculada (`PaymentIntent`) con el importe exacto total calculado (`total_amount = price_per_seat * requested_seats`).
* **RF-10 (Event-Driven):** CUANDO la pasarela de pagos notifique la captura exitosa de los fondos antes del vencimiento del TTL (`NOW() <= expires_at`), EL sistema DEBE transicionar el estado de la reserva a `CONFIRMED`.
* **RF-11 (Event-Driven):** CUANDO una reserva pase a estado `CONFIRMED`, EL sistema DEBE crear un registro en la tabla de transacciones de custodia (`escrow_transactions`) en estado `HELD` vinculando la reserva, el viaje, el pasajero, el conductor y el importe retenido.
* **RF-12 (State-Driven):** MIENTRAS los fondos permanezcan en estado `HELD`, EL sistema DEBE impedir cualquier retiro o transferencia hacia la cuenta del conductor hasta la finalización del viaje.

### 4. Expiración de Reservas y Liberación de Inventario
* **RF-13 (Event-Driven):** CUANDO se cumpla el plazo de 15 minutos (`NOW() > expires_at`) sin confirmación de pago para una reserva en estado `PENDING_PAYMENT`, EL sistema DEBE actualizar automáticamente el estado de la reserva a `EXPIRED`.
* **RF-14 (Event-Driven):** CUANDO una reserva transicione a `EXPIRED`, EL sistema DEBE reintegrar de forma atómica los asientos liberados (`available_seats = available_seats + requested_seats`) en la entidad `trips`.
* **RF-15 (State-Driven):** MIENTRAS un viaje se encuentre en estado `FULL` y ocurra una expiración o cancelación de reserva, EL sistema DEBE reactivar el estado del viaje a `PUBLISHED` para permitir nuevas reservas en las búsquedas activas.

### 5. Finalización del Viaje y Liberación de Fondos (Payout Settlement)
* **RF-16 (Event-Driven):** CUANDO el conductor marque el viaje como concluido y transicione a estado `COMPLETED`, EL sistema DEBE programar la liquidación automática de los fondos en custodia asociados a todas las reservas confirmadas del viaje.
* **RF-17 (Event-Driven):** CUANDO se procese la liquidación de una reserva completada, EL sistema DEBE actualizar el estado de la transacción de custodia de `HELD` a `RELEASED`, registrando la fecha/hora de acreditación y habilitando el saldo a favor del conductor.
* **RF-18 (State-Driven):** MIENTRAS un pasajero reporte una disputa o incidencia justificada dentro de la ventana de reclamos (máximo 2 horas tras la llegada estimada), EL sistema DEBE congelar la transacción de custodia en estado `DISPUTED` para revisión administrativa manual antes de liberar los fondos.

### 6. Políticas de Cancelación y Reembolsos (Refunds)
* **RF-19 (Event-Driven):** CUANDO el conductor cancele un viaje que posea reservas en estado `CONFIRMED`, EL sistema DEBE cancelar todas las reservas asociadas (`CANCELLED_BY_DRIVER`), cambiar el estado del viaje a `CANCELLED`, e iniciar inmediatamente el reembolso del 100% de los fondos en custodia (`escrow_transactions.status = REFUNDED_FULL`).
* **RF-20 (Event-Driven):** CUANDO un pasajero cancele su reserva confirmada con más de 24 horas de antelación respecto a la hora de partida (`departure_time - NOW() > 24 hours`), EL sistema DEBE actualizar la reserva a `CANCELLED_BY_PASSENGER`, reintegrar los asientos al viaje y emitir un reembolso del 100% al pasajero.
* **RF-21 (Event-Driven):** CUANDO un pasajero cancele su reserva confirmada con menos de 24 horas de antelación respecto a la hora de partida, EL sistema DEBE aplicar la política de penalización por cancelación tardía (50% de reembolso al pasajero y 50% de compensación de gastos al conductor por plaza retenida).
* **RF-22 (Event-Driven):** CUANDO ocurra una cancelación o reembolso, EL sistema DEBE registrar la trazabilidad completa en la tabla de reembolsos (`refund_transactions`) con el motivo, montos debitados/acreditados y el identificador de la pasarela de pagos.

### 7. Notificaciones y Auditoría de Eventos
* **RF-23 (Event-Driven):** CUANDO el estado de una reserva cambie (`CONFIRMED`, `EXPIRED`, `CANCELLED`, `COMPLETED`), EL sistema DEBE emitir notificaciones asíncronas vía Push/Email tanto al pasajero como al conductor informando el detalle de la operación.

---

## Modelo de Datos y Entidades Principales

### Entidad `Booking` (Reserva de Asiento)
* `id`: UUID (Primary Key)
* `trip_id`: UUID (Foreign Key -> `trips.id`, Indexed)
* `passenger_id`: UUID (Foreign Key -> `users.id`, Indexed)
* `seats_booked`: SMALLINT (Check: `seats_booked > 0`)
* `unit_price`: NUMERIC(10, 2) (Precio pactado por asiento al reservar)
* `total_price`: NUMERIC(10, 2) (Check: `total_price = unit_price * seats_booked`)
* `status`: ENUM (
  `PENDING_PAYMENT`,
  `CONFIRMED`,
  `CANCELLED_BY_PASSENGER`,
  `CANCELLED_BY_DRIVER`,
  `REJECTED`,
  `EXPIRED`,
  `COMPLETED`
)
* `pickup_stop_id`: UUID (Opcional, Foreign Key -> `trip_waypoints.id`)
* `dropoff_stop_id`: UUID (Opcional, Foreign Key -> `trip_waypoints.id`)
* `expires_at`: TIMESTAMP WITH TIME ZONE (Plazo para completar el pago, p. ej. `created_at + 15 min`)
* `confirmed_at`: TIMESTAMP WITH TIME ZONE (Null hasta completar pago)
* `cancelled_at`: TIMESTAMP WITH TIME ZONE
* `cancellation_reason`: TEXT
* `created_at` / `updated_at`: TIMESTAMP WITH TIME ZONE

### Entidad `EscrowTransaction` (Transacción de Fondos en Custodia)
* `id`: UUID (Primary Key)
* `booking_id`: UUID (Foreign Key -> `bookings.id`, Unique)
* `trip_id`: UUID (Foreign Key -> `trips.id`, Indexed)
* `payer_id`: UUID (Foreign Key -> `users.id` - Pasajero)
* `payee_id`: UUID (Foreign Key -> `users.id` - Conductor)
* `amount`: NUMERIC(10, 2) (Monto en custodia)
* `currency`: VARCHAR(3) (Por defecto 'USD' o 'ARS')
* `payment_gateway_ref`: VARCHAR(255) (ID de transacción o PaymentIntent de la pasarela)
* `status`: ENUM (
  `HELD`,            -- Fondos cobrados y retenidos en custodia
  `RELEASED`,        -- Fondos liberados al conductor al finalizar viaje
  `REFUNDED_FULL`,   -- Reembolsado 100% al pasajero
  `REFUNDED_PARTIAL`,-- Reembolso parcial según política de cancelación
  `DISPUTED`         -- Fondos retenidos por disputa abierta
)
* `held_at`: TIMESTAMP WITH TIME ZONE
* `released_at`: TIMESTAMP WITH TIME ZONE
* `refunded_at`: TIMESTAMP WITH TIME ZONE
* `created_at` / `updated_at`: TIMESTAMP WITH TIME ZONE

### Entidad `RefundTransaction` (Historial y Auditoría de Reembolsos)
* `id`: UUID (Primary Key)
* `escrow_transaction_id`: UUID (Foreign Key -> `escrow_transactions.id`)
* `booking_id`: UUID (Foreign Key -> `bookings.id`)
* `passenger_refund_amount`: NUMERIC(10, 2)
* `driver_compensation_amount`: NUMERIC(10, 2)
* `refund_type`: ENUM (`DRIVER_CANCELLATION`, `PASSENGER_EARLY`, `PASSENGER_LATE`, `ADMIN_DISPUTE`)
* `gateway_refund_ref`: VARCHAR(255)
* `processed_at`: TIMESTAMP WITH TIME ZONE
* `created_at`: TIMESTAMP WITH TIME ZONE

---

## Máquina de Estados (State Machine)

### Ciclo de Vida de la Reserva (`BookingStatus`)
```
                     [Crear Reserva]
                            │
                            ▼
                    PENDING_PAYMENT ──────── (TTL 15 min vence) ──────► EXPIRED
                            │                                       (Asientos reintegrados)
                 (Pago exitoso en Escrow)
                            │
                            ▼
                        CONFIRMED
                       /    │    \
      (Cancela Conductor)   │   (Cancela Pasajero)
             /              │              \
            ▼               │               ▼
  CANCELLED_BY_DRIVER       │     CANCELLED_BY_PASSENGER
 (100% Reembolso Escrow)    │    (Reembolso según política)
                            │
                  (Viaje Concluido)
                            │
                            ▼
                        COMPLETED
                 (Escrow pasa a RELEASED)
```

### Ciclo de Vida de Fondos en Custodia (`EscrowStatus`)
```
[Pago Exitoso Pasajero] ──► HELD ──┬──► (Viaje COMPLETED) ──────────► RELEASED (Pago al conductor)
                                   ├──► (Cancelación total) ────────► REFUNDED_FULL (100% pasajero)
                                   ├──► (Cancelación tardía) ───────► REFUNDED_PARTIAL (50% pasajero / 50% conductor)
                                   └──► (Reclamo/Incidencia) ───────► DISPUTED (Revisión manual)
```

---

## Contrato de API REST (Endpoints HTTP)

### Reservas (`/api/v1/bookings`)
* `POST /api/v1/bookings`: Crear una solicitud de reserva de asientos (decremento atómico en `trips`, estado `PENDING_PAYMENT`, retorno de ID de reserva y tiempo restante de TTL).
* `GET /api/v1/bookings/{id}`: Obtener detalle de una reserva específica con estado de pago y viaje.
* `GET /api/v1/bookings/my-bookings`: Listar historial de reservas del pasajero autenticado.
* `GET /api/v1/trips/{trip_id}/bookings`: Listar reservas de un viaje (autorizado únicamente para el conductor del viaje).
* `POST /api/v1/bookings/{id}/confirm-payment`: Confirmación de pago (simulación o webhook de pasarela), transicionando a `CONFIRMED` y fondeando el Escrow en estado `HELD`.
* `POST /api/v1/bookings/{id}/cancel`: Cancelación de reserva por parte del pasajero con cálculo y ejecución del reembolso correspondiente.

### Gestión de Escrow y Conclusión de Viaje
* `POST /api/v1/trips/{id}/complete`: Endpoint ejecutado por el conductor para marcar el viaje como finalizado y disparar la liquidación automática de fondos en custodia (`RELEASED`).
* `GET /api/v1/escrow/transactions/{id}`: Consulta de auditoría de fondos en custodia para la reserva.

---

## Fuera de Alcance (Out of Scope - MVP Módulo 3)

* Conexión con proveedores bancarios en tiempo real fuera de sandbox (se proveerá adaptador mock / sandbox de pasarela de pagos con confirmación webhook).
* Transferencias bancarias CBU/CVU automatizadas inmediatas hacia cuentas bancarias externas del conductor (el saldo acreditado queda asentado en balance contable de la plataforma para retiro programado).
* División de pagos con múltiples tarjetas de crédito en una misma reserva.
* Sistema de propinas voluntarias adicionales al conductor.
* Chat en tiempo real durante la reserva (corresponde a Módulo 4).

---

## Criterios de Finalización (Definition of Done)

1. **Especificación Aprobada:** Documento `SPEC_MODULE_3.md` exhaustivo en sintaxis EARS reflejando el modelo de reservas, concurrencia pesimista, ciclo de vida y custodia Escrow.
2. **Migración de Base de Datos:** Scripts de migración SQL (`000003_add_bookings_and_escrow.up.sql` y `.down.sql`) con constraints, índices y tablas `bookings`, `escrow_transactions` y `refund_transactions`.
3. **Control de Concurrencia y Atomicidad:**
   - Pruebas unitarias y de concurrencia verificando que solicitudes simultáneas con `SELECT ... FOR UPDATE` no permitan sobreventa de asientos bajo ninguna circunstancia (cero overbooking).
   - Mecanismo de expiración automática de asientos en `PENDING_PAYMENT` y restitución inmediata al inventario del viaje.
4. **Dominio y Servicios de Negocio:**
   - Entidades de dominio `Booking`, `BookingStatus`, `EscrowTransaction`, `EscrowStatus`, `RefundTransaction`.
   - Reglas de negocio de Cap Pricing y políticas de reembolso (100% anticipado, parcial tardío, 100% por cancelación de conductor).
   - Servicio de liquidación (Settlement/Release) al marcar viaje `COMPLETED`.
5. **Endpoints REST y Handlers HTTP:**
   - Implementación y registro de rutas en el router de Gin/Chi/Mux de Go con validación de autenticación JWT y roles.
6. **Calidad de Código y Cobertura:** Cobertura de pruebas unitarias y de integración $\ge 80\%$, y linter sin advertencias ni fallas.
