# SPEC_MODULE_2.md — Módulo 2: Core Match, Publicación y Búsqueda de Viajes (PostGIS & Cap Pricing)

## Proyecto
Plataforma de carpooling de mediana y larga distancia (**Thumbi**) basada en economía colaborativa no lucrativa con modelo de gastos compartidos. Arquitectura de microservicios con backend en Go, base de datos PostgreSQL con extensión PostGIS, frontend Web en React y aplicaciones móviles iOS/Android en React Native. Infraestructura orientada a eventos en AWS (API Gateway, Lambda, EKS) gestionada 100% mediante IaC con Terraform.

---

## Contexto de Dominio & Principios Rectores

1. **Economía Colaborativa No Lucrativa & Cap Pricing:**
   - La plataforma no permite que el conductor genere lucro comercial. El precio por plaza se calcula en base a los costos reales operativos estimados del trayecto (combustible según distancia/consumo de referencia y peajes prorrateados) dividido entre la capacidad de asientos ofrecidos.
   - El sistema calcula un precio sugerido y un **tope máximo estricto (Cap Price)** por asiento. Cualquier intento de publicar una tarifa superior al Cap Price es bloqueado automáticamente.
2. **Geometría y Matching Espaciotemporal con PostGIS:**
   - Cada viaje publicado almacena el trayecto como una geometría geoespacial (`LineString` con SRID 4326/WGS 84) generada a partir de los puntos de origen, paradas intermedias (waypoints) y destino final.
   - La búsqueda de viajes permite a un pasajero ingresar un punto de origen y un punto de destino con una ventana temporal de tolerancia.
   - PostGIS evalúa la proximidad de los puntos de abordaje y descenso a la traza del viaje (`ST_DWithin`) y verifica el sentido del trayecto mediante la proyección lineal (`ST_LineLocatePoint(route, origin) < ST_LineLocatePoint(route, destination)`).
3. **Control de Acceso por Rol Verificado:**
   - Solo los usuarios con perfil de conductor activo y verificado (`is_driver_active: true`, KYC aprobado y vehículo validado en Módulo 1) pueden publicar viajes.
   - Cualquier usuario autenticado puede buscar viajes y consultar trayectos.

---

## Requisitos Funcionales (Sintaxis EARS)

### 1. Validación de Roles y Permisos de Publicación
* **RF-01 (State-Driven):** MIENTRAS un usuario no posea el estado de conductor activo y verificado (`is_driver_active: false`), EL sistema DEBE bloquear la creación y publicación de viajes, retornando un error de autorización (`403 Forbidden`) indicando los pasos pendientes de verificación.
* **RF-02 (State-Driven):** MIENTRAS un conductor tenga su vehículo en estado `PENDING_VERIFICATION` o `REJECTED`, EL sistema DEBE impedir la selección de dicho vehículo para la publicación de viajes en ruta.
* **RF-03 (Event-Driven):** CUANDO un conductor con verificación activa (`is_driver_active: true`) inicie el flujo de publicación de viaje, EL sistema DEBE requerir la selección de su vehículo aprobado, los puntos de origen y destino, la fecha y hora de partida, los asientos disponibles ofrecidos (respetando la capacidad homologada del vehículo), las paradas intermedias opcionales y las políticas de equipaje.

### 2. Cálculo de Trayecto, Distancias y Cap Pricing (Tope No Lucrativo)
* **RF-04 (Event-Driven):** CUANDO el conductor defina el itinerario (origen, paradas y destino), EL sistema DEBE calcular la distancia total en kilómetros y el tiempo estimado de viaje mediante el servicio de rutas, generando la geometría vectorial (`LineString`).
* **RF-05 (Event-Driven):** CUANDO se compute la ruta del viaje, EL sistema DEBE calcular automáticamente la matriz de costos compartidos (costo de combustible de referencia por km según consumo promedio vehicular + estimación de peajes oficiales de la traza).
* **RF-06 (Ubiquitous):** El sistema DEBE establecer un **Cap Price (Precio Tope Máximo)** por asiento para cada tramo y para el trayecto total, calculado como:
  $$\text{CapPrice} = \frac{\text{CostoTotalEstimado}}{\text{AsientosTotalesOfertados}} \times (1 + \text{MargenToleranciaImprevistos})$$
  *(donde el margen de imprevistos no superará el 10% y no constituirá beneficio o lucro comercial).*
* **RF-07 (Event-Driven):** CUANDO el conductor intente ingresar un precio por asiento superior al Cap Price calculado por el sistema, EL sistema DEBE rechazar la publicación, informando la tarifa máxima permitida y el desglose de costos asociados.
* **RF-08 (Event-Driven):** CUANDO el conductor ingrese un precio menor o igual al Cap Price, EL sistema DEBE permitir la confirmación y registrar el viaje en estado `PUBLISHED`.

### 3. Almacenamiento e Indexación Geoespacial (PostGIS)
* **RF-09 (Event-Driven):** CUANDO un viaje sea publicado exitosamente, EL sistema DEBE persistir la ruta completa en PostgreSQL utilizando tipos de datos espaciales (`GEOGRAPHY(LineString, 4326)` o `GEOMETRY(LineString, 4326)` con índices espaciales `GIST`).
* **RF-10 (Ubiquitous):** El sistema DEBE almacenar los nodos de origen, destino y cada parada intermedia con sus coordenadas geográficas exactas (`Point(longitude, latitude)`), su dirección textual normalizada y su hora estimada de paso.

### 4. Búsqueda Geoespacial y Matching de Trayectos
* **RF-11 (Event-Driven):** CUANDO un pasajero busque viajes indicando coordenadas de origen, coordenadas de destino, fecha deseada y cantidad de asientos requeridos, EL sistema DEBE consultar viajes activos en estado `PUBLISHED` con asientos disponibles suficientes.
* **RF-12 (Event-Driven):** CUANDO el sistema ejecute la consulta espacial de búsqueda, EL sistema DEBE aplicar un radio de tolerancia de desvío configurable (por defecto hasta $5\text{ km}$ en zonas urbanas y hasta $15\text{ km}$ en corredores interurbanos) mediante `ST_DWithin` respecto al trazado del viaje.
* **RF-13 (State-Driven):** MIENTRAS se evalúe la coincidencia de un viaje potencial, EL sistema DEBE validar que la proyección lineal del punto de origen del pasajero sobre la polilínea ocurra estrictamente antes que la proyección lineal del punto de destino (`ST_LineLocatePoint(route, origin_point) < ST_LineLocatePoint(route, destination_point)`), descartando trayectos en sentido inverso.
* **RF-14 (Event-Driven):** CUANDO una búsqueda retorne resultados coincidentes, EL sistema DEBE ordenar los viajes según proximidad geográfica de los puntos de encuentro, cercanía horaria y menor costo por asiento.
* **RF-15 (Event-Driven):** CUANDO no existan viajes que coincidan directamente dentro del radio de tolerancia estándar, EL sistema DEBE devolver una lista vacía con sugerencias de puntos de encuentro alternativos o ampliación de radio de búsqueda.

### 5. Gestión del Ciclo de Vida del Viaje
* **RF-16 (Event-Driven):** CUANDO el conductor decida cancelar un viaje publicado sin reservas confirmadas, EL sistema DEBE actualizar su estado a `CANCELLED` y removerlo de los índices de búsqueda activa inmediatamente.
* **RF-17 (State-Driven):** MIENTRAS la fecha y hora de partida de un viaje sea alcanzada o superada sin haber iniciado, EL sistema DEBE transicionar automáticamente el estado del viaje a `EXPIRED` o `IN_PROGRESS` según la confirmación del conductor.

---

## Modelo de Datos Geoespacial y Entidades Principales

### Entidad `Trip` (Viaje)
* `id`: UUID (Primary Key)
* `driver_id`: UUID (Foreign Key -> Users, `is_driver_active = true`)
* `vehicle_id`: UUID (Foreign Key -> Vehicles, `status = APPROVED`)
* `origin_title`: VARCHAR(255)
* `origin_geom`: GEOMETRY(Point, 4326)
* `destination_title`: VARCHAR(255)
* `destination_geom`: GEOMETRY(Point, 4326)
* `route_polyline`: GEOMETRY(LineString, 4326) con índice `GIST`
* `departure_time`: TIMESTAMP WITH TIME ZONE
* `estimated_arrival_time`: TIMESTAMP WITH TIME ZONE
* `total_distance_km`: NUMERIC(8, 2)
* `total_duration_minutes`: INTEGER
* `available_seats`: SMALLINT
* `total_seats_offered`: SMALLINT
* `price_per_seat`: NUMERIC(10, 2)
* `cap_price_per_seat`: NUMERIC(10, 2)
* `estimated_fuel_cost`: NUMERIC(10, 2)
* `estimated_toll_cost`: NUMERIC(10, 2)
* `status`: ENUM (`PUBLISHED`, `FULL`, `IN_PROGRESS`, `COMPLETED`, `CANCELLED`, `EXPIRED`)
* `created_at` / `updated_at`: TIMESTAMP WITH TIME ZONE

### Entidad `TripWaypoint` (Paradas Intermedias)
* `id`: UUID (Primary Key)
* `trip_id`: UUID (Foreign Key -> Trip)
* `stop_order`: SMALLINT (Secuencia 1, 2, 3...)
* `location_title`: VARCHAR(255)
* `location_geom`: GEOMETRY(Point, 4326)
* `estimated_arrival_time`: TIMESTAMP WITH TIME ZONE

---

## Fuera de Alcance (Out of Scope - MVP Módulo 2)

* Sistema de reservas, pagos en escrow y pasarela de pago (corresponde a Módulo 3: Bookings & Escrow Payments).
* Chat en tiempo real y mensajería instantánea entre conductor y pasajero (Módulo 4).
* Desvíos dinámicos en tiempo real con recálculo algorítmico de ruta durante el viaje (ruta fija precalculada en MVP).
* Sistema de calificaciones y reputación post-viaje (Módulo 5).
* Soporte para múltiples conductores compartiendo un mismo tramo.

---

## Criterios de Finalización (Definition of Done)

1. **Especificación Aprobada:** Documento `SPEC_MODULE_2.md` completo en sintaxis EARS reflejando todas las reglas de negocio, Cap Pricing y PostGIS.
2. **Migración de Base de Datos:** Scripts SQL (`000002_add_trips_and_postgis.up.sql` y `.down.sql`) habilitando `CREATE EXTENSION IF NOT EXISTS postgis`, tablas `trips` y `trip_waypoints`, e índices GIST espaciales.
3. **Lógica de Dominio y Cap Pricing:**
   - Entidades de Dominio `Trip`, `Waypoint`, `RouteGeometry`, `PricingCap`.
   - Cálculo determinista del costo operativo por km y enforcement estricto de no superación del Cap Price.
4. **Servicio de Búsqueda PostGIS:**
   - Repositorio con consultas SQL nativas utilizando funciones espaciales `ST_DWithin`, `ST_Distance` y `ST_LineLocatePoint`.
   - Validación comprobada de sentido de recorrido (origen antes que destino).
5. **Endpoints HTTP REST:**
   - `POST /api/v1/trips`: Publicación de viajes con validación de conductor activo y Cap Price.
   - `GET /api/v1/trips/search`: Búsqueda geoespacial por coordenadas de origen, destino, fecha y radio de tolerancia.
   - `GET /api/v1/trips/{id}`: Detalle de viaje con waypoints y desglose transparente de costos.
   - `DELETE /api/v1/trips/{id}`: Cancelación de viaje publicado.
6. **Calidad de Código:** Cobertura de pruebas unitarias y de integración $\ge 80\%$, y linter sin advertencias.
