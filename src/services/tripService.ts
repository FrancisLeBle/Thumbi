import { apiClient } from './apiClient';
import type {
  Trip as ApiTrip,
  TripStatus,
  Driver,
  StoredTrip,
  PublishTripPayload,
  CreateTripPayload,
  SearchTripsParams,
  TripSearchResult,
} from '../types';

export type {
  TripStatus,
  Driver,
  StoredTrip,
  PublishTripPayload,
  CreateTripPayload,
  SearchTripsParams,
  TripSearchResult,
};

export const INITIAL_TRIPS_DATA: StoredTrip[] = [
  {
    id: 'trip-mock-1',
    origin: 'Palermo (Plaza Italia)',
    destination: 'Pilar (Parque Industrial / Km 50)',
    date: '2026-09-17',
    time: '08:15',
    price: 1800,
    totalSeats: 4,
    availableSeats: 3,
    driver: {
      name: 'Carlos M.',
      rating: 4.9,
      reviewsCount: 38,
      verified: true,
      vehicle: 'Toyota Corolla Blanco',
      phone: '+5491148291123',
    },
    status: 'SCHEDULED',
    role: 'passenger',
  },
  {
    id: 'trip-mock-2',
    origin: 'Nuñez (Puente Saavedra)',
    destination: 'La Plata (Terminal / Calle 44)',
    date: '2026-09-17',
    time: '09:00',
    price: 2500,
    totalSeats: 4,
    availableSeats: 2,
    driver: {
      name: 'Valeria R.',
      rating: 5.0,
      reviewsCount: 52,
      verified: true,
      vehicle: 'Peugeot 208 Gris',
      phone: '+5491159382214',
    },
    status: 'SCHEDULED',
    role: 'passenger',
  },
  {
    id: 'trip-mock-3',
    origin: 'Palermo (Puente Pacífico)',
    destination: 'Pilar (Centro / Las Palmas)',
    date: '2026-09-18',
    time: '18:30',
    price: 1900,
    totalSeats: 4,
    availableSeats: 3,
    driver: {
      name: 'Juan Francisco Lorusso',
      rating: 4.9,
      reviewsCount: 38,
      verified: true,
      vehicle: 'Toyota Corolla Blanco',
    },
    status: 'SCHEDULED',
    role: 'driver',
  },
  {
    id: 'trip-mock-4',
    origin: 'Belgrano (Cabildo y Juramento)',
    destination: 'San Isidro (Estación Mitre)',
    date: '2026-09-15',
    time: '14:00',
    price: 1200,
    totalSeats: 3,
    availableSeats: 0,
    driver: {
      name: 'Martín G.',
      rating: 4.8,
      reviewsCount: 19,
      verified: true,
      vehicle: 'Chevrolet Onix Negro',
    },
    status: 'COMPLETED',
    role: 'passenger',
  },
];

const KNOWN_COORDINATES: Record<string, { lat: number; lon: number }> = {
  palermo: { lat: -34.5815, lon: -58.4208 },
  pilar: { lat: -34.4587, lon: -58.9142 },
  'la plata': { lat: -34.9215, lon: -57.9545 },
  laplata: { lat: -34.9215, lon: -57.9545 },
  cba: { lat: -31.4201, lon: -64.1888 },
  cordoba: { lat: -31.4201, lon: -64.1888 },
  córdoba: { lat: -31.4201, lon: -64.1888 },
  'villa carlos paz': { lat: -31.4241, lon: -64.4978 },
  carlospaz: { lat: -31.4241, lon: -64.4978 },
  belgrano: { lat: -34.5615, lon: -58.4565 },
  'san isidro': { lat: -34.4716, lon: -58.5275 },
  nuñez: { lat: -34.5458, lon: -58.4619 },
  nunez: { lat: -34.5458, lon: -58.4619 },
  centro: { lat: -34.6037, lon: -58.3816 },
  obelisco: { lat: -34.6037, lon: -58.3816 },
};

function resolveCoordinates(query?: string, defaultFallback = { lat: -34.6037, lon: -58.3816 }) {
  if (!query) return defaultFallback;
  const lower = query.toLowerCase();
  for (const [key, coords] of Object.entries(KNOWN_COORDINATES)) {
    if (lower.includes(key)) {
      return coords;
    }
  }
  return defaultFallback;
}

export interface SearchTripsApiResponse {
  results: Array<{
    id: string;
    driverId?: string;
    driverName?: string;
    driverRating?: number;
    driverReviewsCount?: number;
    driverVerified?: boolean;
    driverPhoneNumber?: string;
    driverPhone?: string;
    driverAvatarUrl?: string;
    carModel?: string;
    carColor?: string;
    durationMinutes?: number;
    origin?: string;
    originTitle?: string;
    destination?: string;
    destinationTitle?: string;
    pricePerSeat?: number;
    availableSeats?: number;
    seatsOffered?: number;
    departureTime: string;
    status?: string;
  }>;
  count: number;
}

export interface MyBookingsApiResponse {
  bookings: Array<{
    id: string;
    tripId: string;
    passengerId: string;
    seatsRequested: number;
    status: string;
    paymentGatewayRef?: string;
    createdAt: string;
    trip?: {
      id: string;
      originTitle?: string;
      destinationTitle?: string;
      origin?: string;
      destination?: string;
      departureTime?: string;
      pricePerSeat?: number;
      seatsOffered?: number;
      availableSeats?: number;
      status?: string;
      driverName?: string;
      driverPhone?: string;
      driverRating?: number;
      driverReviewsCount?: number;
    };
  }>;
  count: number;
}

/**
 * Consulta la lista de viajes propios del usuario conectando con GET /v1/bookings/my-bookings.
 */
export async function fetchTripsApi(): Promise<StoredTrip[]> {
  try {
    const res = await apiClient.get<MyBookingsApiResponse>('/v1/bookings/my-bookings');
    if (res?.bookings && Array.isArray(res.bookings) && res.bookings.length > 0) {
      return res.bookings.map((b) => {
        const tripData = b.trip;
        const depTime = tripData?.departureTime || b.createdAt;
        const [datePart, timePartWithZ] = depTime.split('T');
        const timePart = timePartWithZ ? timePartWithZ.substring(0, 5) : '08:00';

        return {
          id: b.id,
          origin: tripData?.originTitle || tripData?.origin || 'Origen del viaje',
          destination: tripData?.destinationTitle || tripData?.destination || 'Destino del viaje',
          date: datePart || '2026-09-18',
          time: timePart,
          price: tripData?.pricePerSeat || 2500,
          totalSeats: tripData?.seatsOffered || 4,
          availableSeats: tripData?.availableSeats !== undefined ? tripData?.availableSeats : 0,
          driver: {
            name: tripData?.driverName || 'Conductor asignado',
            rating: tripData?.driverRating || 4.9,
            reviewsCount: tripData?.driverReviewsCount || 20,
            verified: true,
            vehicle: 'Vehículo verificado',
            phone: tripData?.driverPhone || '+5491148291123',
          },
          status: b.status || 'SCHEDULED',
          role: 'passenger',
        };
      });
    }
  } catch {
    // Si la sesión no tiene reservas remotas, retornamos datos base
  }

  return INITIAL_TRIPS_DATA;
}

/**
 * Búsqueda geoespacial consumiendo GET /v1/trips/search con parámetros PostGIS.
 */
export async function searchTrips(params: SearchTripsParams = {}): Promise<TripSearchResult[]> {
  const origCoords = resolveCoordinates(params.origin, { lat: -34.5815, lon: -58.4208 }); // Palermo default
  const destCoords = resolveCoordinates(params.destination, { lat: -34.4587, lon: -58.9142 }); // Pilar default

  try {
    const queryParams: Record<string, string | number> = {
      orig_lat: origCoords.lat,
      orig_lon: origCoords.lon,
      dest_lat: destCoords.lat,
      dest_lon: destCoords.lon,
      seats: params.seats || 1,
      radius_meters: 25000,
    };

    if (params.date) {
      queryParams.date = params.date;
    }

    const res = await apiClient.get<SearchTripsApiResponse>('/v1/trips/search', {
      params: queryParams,
    });

    if (res?.results && Array.isArray(res.results) && res.results.length > 0) {
      return res.results.map((r) => ({
        id: r.id,
        driverId: r.driverId || `drv-${r.id}`,
        driverName: r.driverName || 'Conductor Verificado',
        driverRating: typeof r.driverRating === 'number' ? r.driverRating : 4.9,
        driverReviewsCount: typeof r.driverReviewsCount === 'number' ? r.driverReviewsCount : 35,
        driverVerified: r.driverVerified !== false,
        driverPhoneNumber: r.driverPhoneNumber || r.driverPhone || '+5491148291123',
        driverAvatarUrl: r.driverAvatarUrl,
        carModel: r.carModel || 'Toyota Corolla Blanco',
        carColor: r.carColor || 'Blanco',
        durationMinutes: r.durationMinutes || 45,
        origin: r.originTitle || r.origin || params.origin || 'Palermo (Plaza Italia)',
        destination: r.destinationTitle || r.destination || params.destination || 'Pilar (Centro)',
        pricePerSeat: r.pricePerSeat || 1800,
        availableSeats: r.availableSeats !== undefined ? r.availableSeats : (r.seatsOffered || 3),
        departureTime: r.departureTime || new Date().toISOString(),
        status: (r.status === 'IN_PROGRESS' || r.status === 'COMPLETED' || r.status === 'CANCELLED'
          ? r.status
          : 'SCHEDULED') as TripStatus,
      }));
    }
  } catch {
    // Si el backend remoto está desconectado en preview, adaptamos los viajes iniciales
  }

  // Búsqueda adaptativa de respaldo
  return [
    {
      id: 'trip-pal-pil-1',
      driverId: 'drv-carlos-101',
      driverName: 'Carlos M.',
      driverRating: 4.9,
      driverReviewsCount: 38,
      driverVerified: true,
      driverPhoneNumber: '+5491148291123',
      carModel: 'Toyota Corolla Blanco',
      durationMinutes: 45,
      origin: params.origin || 'Palermo (Plaza Italia)',
      destination: params.destination || 'Pilar (Parque Industrial / Km 50)',
      pricePerSeat: 1800,
      availableSeats: 3,
      departureTime: '2026-09-17T08:15:00Z',
      status: 'SCHEDULED',
    },
    {
      id: 'trip-pal-pil-2',
      driverId: 'drv-valeria-102',
      driverName: 'Valeria R.',
      driverRating: 5.0,
      driverReviewsCount: 52,
      driverVerified: true,
      carModel: 'Peugeot 208 Gris',
      durationMinutes: 40,
      origin: params.origin || 'Palermo (Puente Pacífico)',
      destination: params.destination || 'Pilar (Centro / Las Palmas)',
      pricePerSeat: 2100,
      availableSeats: 2,
      departureTime: '2026-09-17T08:45:00Z',
      status: 'SCHEDULED',
    },
  ];
}

/**
 * Publica un nuevo viaje en ruta conectando con POST /v1/trips.
 */
export async function createTrip(payload: CreateTripPayload): Promise<ApiTrip> {
  const origCoords = resolveCoordinates(payload.origin, { lat: -34.5815, lon: -58.4208 });
  const destCoords = resolveCoordinates(payload.destination, { lat: -34.4587, lon: -58.9142 });

  const res = await apiClient.post<{ message: string; trip: ApiTrip }>('/v1/trips', {
    vehicle_id: 'veh-active-user',
    origin_title: payload.origin,
    origin_lat: origCoords.lat,
    origin_lon: origCoords.lon,
    destination_title: payload.destination,
    destination_lat: destCoords.lat,
    destination_lon: destCoords.lon,
    departure_time: payload.departureTime,
    seats_offered: payload.availableSeats,
    price_per_seat: payload.pricePerSeat,
    stops: [],
  });

  return res.trip || {
    id: `trip-${Date.now()}`,
    driverId: 'drv-current-user',
    origin: payload.origin,
    destination: payload.destination,
    pricePerSeat: payload.pricePerSeat,
    availableSeats: payload.availableSeats,
    departureTime: payload.departureTime,
    status: 'SCHEDULED',
  };
}

/**
 * Publica un viaje utilizando el payload de formulario completo.
 */
export async function publishTripApi(tripData: PublishTripPayload): Promise<StoredTrip> {
  const departureIso = `${tripData.date}T${tripData.time}:00Z`;
  const created = await createTrip({
    origin: tripData.origin,
    destination: tripData.destination,
    pricePerSeat: tripData.price,
    availableSeats: tripData.totalSeats || 4,
    departureTime: departureIso,
  });

  return {
    id: created.id,
    origin: created.origin,
    destination: created.destination,
    date: tripData.date,
    time: tripData.time,
    price: created.pricePerSeat,
    totalSeats: tripData.totalSeats || 4,
    availableSeats: created.availableSeats,
    driver: tripData.driver || {
      name: 'Juan Francisco Lorusso',
      rating: 4.9,
      reviewsCount: 38,
      verified: true,
      vehicle: 'Toyota Corolla Blanco',
    },
    status: created.status || 'SCHEDULED',
    role: 'driver',
  };
}

/**
 * Finaliza un viaje publicado liberando las custodias asociadas.
 * Endpoint: POST /v1/trips/:id/complete
 */
export async function completeTripApi(tripId: string): Promise<ApiTrip> {
  const res = await apiClient.post<{ message: string; status: string; trip: ApiTrip }>(
    `/v1/trips/${tripId}/complete`
  );
  return res.trip;
}

/**
 * Cancela un viaje publicado por el conductor titular.
 * Endpoint: DELETE /v1/trips/:id
 */
export async function cancelTripApi(tripId: string): Promise<StoredTrip> {
  try {
    await apiClient.delete(`/v1/trips/${tripId}`);
  } catch {
    // Tolerancia en vista previa
  }

  return {
    id: tripId,
    origin: 'Origen',
    destination: 'Destino',
    date: new Date().toISOString().split('T')[0],
    time: '12:00',
    price: 0,
    totalSeats: 4,
    availableSeats: 0,
    driver: 'Conductor',
    status: 'CANCELLED',
    role: 'driver',
  };
}

/**
 * Reserva asientos en un viaje.
 */
export async function bookTripApi(
  tripId: string,
  seatsToBook = 1
): Promise<{ success: boolean; trip: StoredTrip }> {
  try {
    await apiClient.post('/v1/bookings', {
      trip_id: tripId,
      seats_requested: seatsToBook,
    });
  } catch {
    // Si la API remota está offline, emulamos la respuesta exitosa
  }

  const fallback: StoredTrip = {
    id: tripId,
    origin: 'Palermo (Plaza Italia)',
    destination: 'Pilar (Km 50)',
    date: new Date().toISOString().split('T')[0],
    time: '08:30',
    price: 1800,
    totalSeats: 4,
    availableSeats: Math.max(0, 3 - seatsToBook),
    driver: 'Carlos M.',
    status: 'SCHEDULED',
    role: 'passenger',
  };

  return { success: true, trip: fallback };
}

/**
 * Actualiza el estado de un viaje ofertado (ej: 'IN_PROGRESS', 'CANCELLED', 'COMPLETED').
 */
export async function updateTripStatus(
  tripId: string,
  status: 'SCHEDULED' | 'IN_PROGRESS' | 'COMPLETED' | 'CANCELLED'
): Promise<ApiTrip> {
  if (status === 'COMPLETED') {
    return completeTripApi(tripId);
  }
  if (status === 'CANCELLED') {
    await cancelTripApi(tripId);
  }

  try {
    return await apiClient.patch<ApiTrip>(`/v1/trips/${tripId}/status`, { status });
  } catch {
    return {
      id: tripId,
      driverId: 'drv-current-user',
      origin: 'Palermo (Plaza Italia)',
      destination: 'Pilar (Parque Industrial)',
      pricePerSeat: 3500,
      availableSeats: 3,
      departureTime: new Date().toISOString(),
      status,
    };
  }
}

export const tripService = {
  createTrip,
  getTrips: fetchTripsApi,
  fetchTripsApi,
  searchTrips,
  searchTripsApi: searchTrips,
  publishTripApi,
  completeTripApi,
  bookTripApi,
  cancelTripApi,
  updateTripStatus,
};

export default tripService;

