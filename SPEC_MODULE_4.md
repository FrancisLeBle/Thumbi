# SPEC_MODULE_4.md — Módulo 4: Finalización de Viajes, Liquidación de Escrow (Payouts), Sistema Reputacional (Reviews) y Disputas

## Proyecto
Plataforma de carpooling de mediana y larga distancia (**Thumbi**) basada en economía colaborativa no lucrativa con modelo de gastos compartidos. Arquitectura de microservicios con backend en Go, base de datos PostgreSQL con extensión PostGIS, frontend Web en React y aplicaciones móviles iOS/Android en React Native. Infraestructura orientada a eventos en AWS (API Gateway, Lambda, EKS) gestionada 100% mediante IaC con Terraform.

---

## Contexto de Dominio & Principios Rectores

1. **Ciclo de Cierre de Viaje y Liquidación de Pagos (Escrow Payout Settlement):**
   - En el modelo no lucrativo de Thumbi, el conductor comparte los costos reales de combustible y peajes del viaje. Los fondos abonados por cada pasajero en estado `CONFIRMED` permanecen retenidos en custodia (*Escrow*) bajo estado `HELD`.
   - Cuando el conductor arriba al destino final y concluye el trayecto marcando el viaje como `COMPLETED`, el sistema inicia el proceso de cierre y liquidación automática (*Payout Settlement*).
   - Se abre una ventana de seguridad operativa (2 horas post-llegada) durante la cual los pasajeros pueden confirmar el arribo conforme o reportar una incidencia. Vencida esta ventana sin observaciones o mediando confirmación de los pasajeros, los fondos en custodia transicionan automáticamente a `RELEASED`, acreditando el saldo compensatorio a favor del conductor.

2. **Sistema Reputacional Bidireccional (Mutual Reviews & Trust Score):**
   - La confianza comunitaria es el pilar fundamental del carpooling. El sistema habilita la calificación bidireccional entre las partes:
     * El **Pasajero** califica al **Conductor** (puntualidad, conducción segura, estado del vehículo, respeto a paradas acordadas).
     * El **Conductor** califica al **Pasajero** (puntualidad en punto de encuentro, respeto a normas del vehículo, comunicación).
   - **Regla de Bloqueo Estricto:** Ninguna evaluación puede ser emitida si el viaje no ha alcanzado de manera comprobada el estado `COMPLETED` y la reserva asociada no se encuentra en estado `COMPLETED`.
   - **Regla de Evaluación Única:** Solo se admite una única reseña por reserva y calificador (`booking_id` + `reviewer_id`), impidiendo valoraciones duplicadas o manipulaciones de reputación.
   - **Cálculo Atómico de Reputación:** La calificación promedio (`rating_avg`) y el volumen total de reseñas (`rating_count`) del usuario calificado (`reviewee`) se actualizan atómicamente a nivel de motor de base de datos mediante disparadores (*triggers*), evitando condiciones de carrera (*race conditions*).

3. **Protocolo de Reclamos y Disputas de Escrow (Dispute Resolution Workflow):**
   - Si un pasajero o un conductor experimenta una contingencia grave (p. ej., inasistencia sin aviso / *no-show*, desvío arbitrario de ruta, conducción peligrosa, cobro indebido adicional en efectivo, o vehículo no coincidente con el registrado), cualquiera de las partes involucradas puede iniciar un reclamo formal dentro de la ventana de protección.
   - Al registrarse la disputa, la transacción de custodia vinculada congela su liquidación pasando inmediatamente al estado `DISPUTED`.
   - La disputa entra en estado `OPENED`, permitiendo al solicitante adjuntar descripciones fácticas y enlaces de evidencia multimedia (capturas de chat, fotos, comprobantes).
   - Un operador de mediación / mesa de control administrativo toma el caso transicionando a `IN_REVIEW` y dictamina una resolución vinculante:
     * `RESOLVED_PASSENGER_REFUND`: Reembolso total o proporcional al pasajero (`escrow_transactions.status = REFUNDED_FULL` o `REFUNDED_PARTIAL`).
     * `RESOLVED_DRIVER_PAYOUT`: Desestimación del reclamo con liberación de los fondos custodiados al conductor (`escrow_transactions.status = RELEASED`).
     * `REJECTED`: Cierre de la disputa por falta de mérito o evidencia insuficiente.

---

## Requisitos Funcionales (Sintaxis EARS)

### 1. Finalización de Viaje y Liquidación de Escrow (Payouts)
* **RF-01 (Event-Driven):** CUANDO el conductor notifique la llegada a destino y solicite la conclusión del viaje (`POST /api/v1/trips/{id}/complete`), EL sistema DEBE verificar que el solicitante sea el conductor titular del viaje y que el viaje se encuentre en estado `IN_PROGRESS` o `PUBLISHED`/`FULL` habiendo alcanzado la hora prevista de llegada.
* **RF-02 (Event-Driven):** CUANDO un viaje transicione válidamente a estado `COMPLETED`, EL sistema DEBE transicionar de forma síncrona todas las reservas en estado `CONFIRMED` asociadas a dicho viaje al estado `COMPLETED`.
* **RF-03 (Event-Driven):** CUANDO una reserva pase a estado `COMPLETED` y no existan disputas activas, EL sistema DEBE programar o ejecutar la liquidación de la transacción de custodia asociada (`escrow_transactions.status = RELEASED`), registrando la marca temporal `released_at` y actualizando el balance contable del conductor.
* **RF-04 (State-Driven):** MIENTRAS una reserva o transacción de custodia se encuentre bajo disputa abierta (`DISPUTED`), EL sistema DEBE bloquear la liberación automática de los fondos en custodia hacia el conductor hasta que la mediación administrativa emita un dictamen formal.
* **RF-05 (Ubiquitous):** El sistema DEBE garantizar que la transición de fondos a `RELEASED` sea idempotente e inmutable, impidiendo dobles liquidaciones bancarias o contables.

### 2. Sistema de Calificaciones y Reputación Bidireccional
* **RF-06 (State-Driven):** MIENTRAS un viaje NO se encuentre en estado `COMPLETED`, EL sistema DEBE rechazar cualquier intento de envío de calificación o reseña con un error de precondición no cumplida (`400 Bad Request` / `ErrTripNotCompletedForReview`).
* **RF-07 (Event-Driven):** CUANDO un pasajero con reserva completada emita una calificación sobre el conductor del viaje, EL sistema DEBE validar que la puntuación sea un número entero comprendido entre 1 y 5 estrellas, que el calificado coincida con el conductor del viaje (`reviewee_id == driver_id`) y que no exista una reseña previa para dicha reserva y emisor.
* **RF-08 (Event-Driven):** CUANDO el conductor emita una calificación sobre un pasajero con reserva completada, EL sistema DEBE validar que la puntuación sea un número entero comprendido entre 1 y 5 estrellas, que el calificado sea el titular de la reserva (`reviewee_id == passenger_id`) y que no exista una reseña previa para dicha reserva y emisor.
* **RF-09 (Event-Driven):** CUANDO se inserte satisfactoriamente una nueva reseña en la tabla `reviews`, EL sistema DEBE disparar un procedimiento atómico en PostgreSQL para recalcular de inmediato el promedio de reputación (`rating_avg`) y el conteo acumulado de valoraciones (`rating_count`) del usuario evaluado en la tabla `users`.
* **RF-10 (Unwanted Behavior):** SI un usuario intenta calificarse a sí mismo (`reviewer_id == reviewee_id`) o intenta emitir más de una calificación por reserva, EL sistema DEBE rechazar la solicitud de forma inmediata con un error de operación no permitida (`422 Unprocessable Entity` / `409 Conflict`).

### 3. Protocolo de Reclamos y Disputas de Fondos en Custodia
* **RF-11 (Event-Driven):** CUANDO un pasajero o conductor detecte una anomalía grave en un viaje en curso o dentro de las 2 horas posteriores a su finalización estimada, EL sistema DEBE permitir la apertura de una disputa formal (`POST /api/v1/disputes`) vinculando la reserva, la transacción de custodia, el motivo tipificado, la descripción detallada y URLs de evidencias.
* **RF-12 (Event-Driven):** CUANDO se cree una disputa válida en estado `OPENED`, EL sistema DEBE transicionar inmediatamente la transacción de custodia vinculada a estado `DISPUTED`, congelando cualquier tentativa de desembolso hacia el conductor.
* **RF-13 (State-Driven):** MIENTRAS una disputa se encuentre en estado `OPENED` o `IN_REVIEW`, EL sistema DEBE notificar a la contraparte requerida e informar a los equipos de soporte y mediación de Thumbi.
* **RF-14 (Event-Driven):** CUANDO un administrador o mediador autorizado resuelva una disputa con dictamen `RESOLVED_PASSENGER_REFUND`, EL sistema DEBE emitir el reembolso total o parcial al pasajero (`escrow_transactions.status = REFUNDED_FULL`), generar el comprobante en `refund_transactions` y marcar la disputa como resuelta.
* **RF-15 (Event-Driven):** CUANDO un administrador o mediador autorizado resuelva una disputa con dictamen `RESOLVED_DRIVER_PAYOUT`, EL sistema DEBE desbloquear los fondos en custodia y transicionarlos a `RELEASED`, transfiriendo el importe a favor del conductor y registrando la fundamentación en `admin_notes`.
* **RF-16 (Event-Driven):** CUANDO una disputa sea desestimada administrativamente (`REJECTED`), EL sistema DEBE restituir la transacción de custodia a su flujo regular de liquidación según el estado del viaje.
* **RF-17 (Unwanted Behavior):** SI un usuario intenta abrir una disputa sobre una transacción de custodia que ya fue liberada o sobre la cual ya existe una disputa previa, EL sistema DEBE rechazar la petición con un error de conflicto (`409 Conflict`).

---

## Modelo de Datos y Entidades Principales

### 1. Entidad `Review` (Calificación y Reseña Reputacional)
* `id`: VARCHAR(64) (Primary Key)
* `trip_id`: VARCHAR(64) (Foreign Key -> `trips.id`, ON DELETE CASCADE)
* `booking_id`: VARCHAR(64) (Foreign Key -> `bookings.id`, ON DELETE CASCADE)
* `reviewer_id`: VARCHAR(64) (Foreign Key -> `users.id`, ON DELETE CASCADE - Quien emite la calificación)
* `reviewee_id`: VARCHAR(64) (Foreign Key -> `users.id`, ON DELETE CASCADE - Quien recibe la calificación)
* `rating`: INT (Check: `rating >= 1 AND rating <= 5`)
* `comment`: TEXT (Opcional, hasta 1000 caracteres)
* `created_at` / `updated_at`: TIMESTAMP WITH TIME ZONE
* **Constraints:**
  - `uq_review_booking_reviewer UNIQUE (booking_id, reviewer_id)`: Calificación única por reserva y calificador.
  - `chk_reviewer_not_reviewee CHECK (reviewer_id <> reviewee_id)`: Impide autocalificaciones.

### 2. Entidad `Dispute` (Disputa de Fondos en Custodia)
* `id`: VARCHAR(64) (Primary Key)
* `escrow_transaction_id`: VARCHAR(64) (Foreign Key -> `escrow_transactions.id`, UNIQUE)
* `booking_id`: VARCHAR(64) (Foreign Key -> `bookings.id`, ON DELETE CASCADE)
* `trip_id`: VARCHAR(64) (Foreign Key -> `trips.id`, ON DELETE CASCADE)
* `reporter_id`: VARCHAR(64) (Foreign Key -> `users.id` - Quien presenta el reclamo)
* `defendant_id`: VARCHAR(64) (Foreign Key -> `users.id` - Contraparte denunciada)
* `reason`: ENUM / VARCHAR(64) (`NO_SHOW`, `ROUTE_DEVIATION`, `RECKLESS_DRIVING`, `VEHICLE_MISMATCH`, `PASSENGER_MISCONDUCT`, `EXTRA_CHARGE_REQUESTED`, `OTHER`)
* `description`: TEXT (Fundamentación de los hechos denunciados)
* `evidence_urls`: TEXT[] (Arreglo de URLs con fotos, capturas o grabaciones)
* `status`: ENUM (`OPENED`, `IN_REVIEW`, `RESOLVED_PASSENGER_REFUND`, `RESOLVED_DRIVER_PAYOUT`, `REJECTED`)
* `admin_notes`: TEXT (Dictamen del mediador administrativo)
* `resolved_by`: VARCHAR(64) (Opcional, Foreign Key -> `users.id` del administrador)
* `resolved_at`: TIMESTAMP WITH TIME ZONE
* `created_at` / `updated_at`: TIMESTAMP WITH TIME ZONE

### 3. Actualización en Tabla `users` (Métricas Reputacionales)
* `rating_avg`: NUMERIC(3, 2) NOT NULL DEFAULT 5.00 (Promedio histórico de estrellas, 1.00 a 5.00)
* `rating_count`: INT NOT NULL DEFAULT 0 (Total acumulado de calificaciones recibidas)

---

## Máquinas de Estados (State Machines)

### 1. Ciclo de Vida de la Disputa (`DisputeStatus`)
```
                     [Reportar Incidencia]
                              │
                              ▼
                           OPENED
                              │
                    (Mesa de Mediación toma caso)
                              │
                              ▼
                          IN_REVIEW
                         /    │    \
   (Dictamen Pasajero)  /     │     \  (Dictamen Conductor)
                       /      │      \
                      ▼       │       ▼
  RESOLVED_PASSENGER_REFUND   │   RESOLVED_DRIVER_PAYOUT
 (Reembolso de Fondos Escrow) │  (Liberación fondos al Conductor)
                              │
                              ▼
                           REJECTED
                    (Reclamo Desestimado)
```

### 2. Flujo Integrado de Custodia y Payout en Módulo 4
```
                     CONFIRMED (Escrow HELD)
                                │
                        (Viaje Finalizado)
                                │
                                ▼
                       TRIP COMPLETED
                                │
                   ¿Disputa dentro de ventana?
                       /                 \
                 [SÍ] /                   \ [NO / Arribo Conforme]
                     ▼                     ▼
               Escrow DISPUTED       Escrow RELEASED
              (Revisión Manual)     (Payout a Conductor)
                     │
         Resolver Disputa
         ├── Reembolso Pasajero  ──► Escrow REFUNDED_FULL / PARTIAL
         └── Liberación Conductor ─► Escrow RELEASED
```

---

## Contrato de API REST (Endpoints HTTP)

### 1. Finalización de Viaje y Payouts (`/api/v1/trips`, `/api/v1/escrow`)
* `POST /api/v1/trips/{id}/complete`: Finalizar el viaje por parte del conductor; cambia el estado del viaje a `COMPLETED`, completa las reservas y dispara la liquidación de custodia a `RELEASED`.
* `POST /api/v1/escrow/{id}/release`: Liquidación manual o programada de fondos en custodia.

### 2. Calificaciones y Reseñas (`/api/v1/reviews`)
* `POST /api/v1/reviews`: Crear una nueva reseña (valida que el viaje y la reserva estén en `COMPLETED`, rating entre 1 y 5, y unicidad por reserva/calificador).
* `GET /api/v1/users/{user_id}/reviews`: Listar las opiniones y puntuaciones recibidas por un usuario con paginación.
* `GET /api/v1/trips/{trip_id}/reviews`: Consultar las calificaciones asociadas a un viaje concluido.
* `GET /api/v1/bookings/{booking_id}/reviews`: Consultar las calificaciones cruzadas emitidas en una reserva.

### 3. Reclamos y Disputas (`/api/v1/disputes`)
* `POST /api/v1/disputes`: Abrir una disputa formal sobre una reserva/escrow (cambia la transacción a `DISPUTED` y la disputa a `OPENED`).
* `GET /api/v1/disputes/{id}`: Consultar el estado y detalle de una disputa específica.
* `GET /api/v1/disputes/my-disputes`: Listar disputas abiertas o recibidas por el usuario autenticado.
* `POST /api/v1/disputes/{id}/review`: (Admin) Asignar y marcar la disputa en estado `IN_REVIEW`.
* `POST /api/v1/disputes/{id}/resolve`: (Admin) Resolver la disputa dictaminando `RESOLVED_PASSENGER_REFUND`, `RESOLVED_DRIVER_PAYOUT` o `REJECTED`, ejecutando la acción contable correspondiente sobre el Escrow.

---

## Fuera de Alcance (Out of Scope - MVP Módulo 4)

* Sistema de apelación en segunda instancia ante resolución administrativa (queda sujeto a soporte al cliente manual).
* Respuestas públicas o réplicas a comentarios de reseñas (comentarios unidireccionales por reseña en el MVP).
* Algoritmos complejos de reputación ponderada con decaimiento temporal bayesiano (se computa promedio aritmético directo `AVG(rating)`).
* Disputas iniciadas fuera de la ventana de tiempo límite permitida sin previa autorización de soporte.

---

## Criterios de Finalización (Definition of Done)

1. **Especificación Técnica:** Documento `SPEC_MODULE_4.md` detallado en sintaxis EARS reflejando el flujo de finalización de viajes, liquidación automática de pagos, sistema de reseñas y gestión de disputas.
2. **Migración de Base de Datos:**
   - Script `migrations/000004_add_reviews_and_disputes.up.sql` con tipo `dispute_status_enum`, tabla `reviews`, tabla `disputes`, columnas de reputación en `users`, restricciones de integridad y triggers de actualización atómica.
   - Script `migrations/000004_add_reviews_and_disputes.down.sql` para reversión limpia de tablas, índices, columnas, funciones y disparadores.
3. **Entidades del Dominio en Go:**
   - `internal/core/domain/review.go`: entidad `Review`, invariantes de rating (1..5), restricciones de autoevaluación y validación de estado completado.
   - `internal/core/domain/dispute.go`: entidad `Dispute`, enumeración `DisputeStatus`, motivos tipificados y métodos de transición de estado administrativo.
   - Actualización en `internal/core/domain/user.go` con campos `RatingAvg` y `RatingCount`.
   - Nuevos errores de dominio tipados en `internal/core/domain/errors.go`.
4. **Pruebas Unitarias de Dominio:**
   - Tests unitarios que certifiquen las invariantes de puntuaciones válidas (1..5), bloqueo de autoevaluación, ciclo de vida de disputas y liquidación de fondos en custodia.
