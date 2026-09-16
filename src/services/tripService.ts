import { apiClient } from './apiClient';
import { Trip } from '../types/api';

export interface CreateTripPayload {
  origin: string;
  destination: string;
  pricePerSeat: number;
  availableSeats: number;
  departureTime: string;
}

/**
 * Publica un nuevo viaje ofertado por un conductor.
 * Endpoint: POST /v1/trips
 */
export async function createTrip(payload: CreateTripPayload): Promise<Trip> {
  return apiClient.post<Trip>('/v1/trips', {
    origin: payload.origin,
    destination: payload.destination,
    price_per_seat: payload.pricePerSeat,
    available_seats: payload.availableSeats,
    departure_time: payload.departureTime,
  });
}

/**
 * Obtiene la lista de viajes disponibles.
 * Endpoint: GET /v1/trips
 */
export async function getTrips(): Promise<Trip[]> {
  return apiClient.get<Trip[]>('/v1/trips');
}

export const tripService = {
  createTrip,
  getTrips,
};

export default tripService;
