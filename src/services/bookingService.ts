import { apiClient } from './apiClient';
import type { Booking, PlatformPricingConfig, BookingPricingQuote } from '../types';

export interface CreateBookingPayload {
  tripId: string;
  seatsRequested: number;
}

export interface BookingResponse {
  message?: string;
  booking: Booking;
}

export interface ConfirmPaymentResponse {
  message: string;
  status: string;
  booking: Booking;
}

export interface ContactLinkResponse {
  driverName?: string;
  driverPhone?: string;
  pin: string;
  whatsappUrl: string;
}

export interface MyBookingsResponse {
  bookings: Booking[];
  count: number;
}

/**
 * Crea una reserva de asientos para un viaje existente.
 * Endpoint: POST /v1/bookings
 */
export async function createBooking(payload: CreateBookingPayload): Promise<Booking> {
  const res = await apiClient.post<Booking | BookingResponse>('/v1/bookings', {
    trip_id: payload.tripId,
    seats_requested: payload.seatsRequested,
  });

  if ('booking' in res && res.booking) {
    return res.booking;
  }
  return res as Booking;
}

/**
 * Confirma el pago en garantía (Escrow) de una reserva.
 * Endpoint: POST /v1/bookings/:id/confirm-payment
 */
export async function confirmPayment(
  bookingId: string,
  paymentRef?: string
): Promise<ConfirmPaymentResponse> {
  return apiClient.post<ConfirmPaymentResponse>(`/v1/bookings/${bookingId}/confirm-payment`, {
    payment_ref: paymentRef || `pay-escrow-${Date.now()}`,
  });
}

/**
 * Obtiene el enlace de contacto seguro y directo de WhatsApp con el conductor y el PIN de viaje.
 * Endpoint: GET /v1/bookings/:id/contact-link
 */
export async function getContactLink(bookingId: string): Promise<ContactLinkResponse> {
  return apiClient.get<ContactLinkResponse>(`/v1/bookings/${bookingId}/contact-link`);
}

/**
 * Consulta todas las reservas activas e históricas del pasajero autenticado.
 * Endpoint: GET /v1/bookings/my-bookings
 */
export async function getMyBookings(): Promise<MyBookingsResponse> {
  return apiClient.get<MyBookingsResponse>('/v1/bookings/my-bookings');
}

/**
 * Cancela una reserva existente.
 * Endpoint: DELETE /v1/bookings/:id
 */
export async function cancelBooking(bookingId: string): Promise<void> {
  await apiClient.delete(`/v1/bookings/${bookingId}`);
}

/**
 * Obtiene la configuración activa de tarifas (Service Fee al pasajero y reglas de retención por cancelación).
 * Endpoint: GET /v1/pricing/config
 */
export async function getPricingConfig(): Promise<PlatformPricingConfig> {
  return apiClient.get<PlatformPricingConfig>('/v1/pricing/config');
}

/**
 * Solicita al backend el desglose transparente (Base + Service Fee = Total) para un viaje y plazas dadas.
 * Endpoint: GET /v1/pricing/quote
 */
export async function quoteBookingPrice(
  tripId: string,
  seatsRequested: number
): Promise<BookingPricingQuote> {
  return apiClient.get<BookingPricingQuote>(
    `/v1/pricing/quote?trip_id=${encodeURIComponent(tripId)}&seats=${encodeURIComponent(String(seatsRequested))}`
  );
}

export const bookingService = {
  createBooking,
  confirmPayment,
  getContactLink,
  getMyBookings,
  cancelBooking,
  getPricingConfig,
  quoteBookingPrice,
};

export default bookingService;
