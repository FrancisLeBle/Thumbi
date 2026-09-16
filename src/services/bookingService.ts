import { apiClient } from './apiClient';
import { Booking } from '../types/api';

export interface CreateBookingPayload {
  tripId: string;
  seatsRequested: number;
}

/**
 * Crea una reserva de asientos para un viaje existente.
 * Endpoint: POST /v1/bookings
 */
export async function createBooking(payload: CreateBookingPayload): Promise<Booking> {
  return apiClient.post<Booking>('/v1/bookings', {
    trip_id: payload.tripId,
    seats_requested: payload.seatsRequested,
  });
}

export const bookingService = {
  createBooking,
};

export default bookingService;
