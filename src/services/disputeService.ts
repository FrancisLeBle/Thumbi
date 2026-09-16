import { apiClient } from './apiClient';
import { Dispute } from '../types/api';

export interface CreateDisputePayload {
  bookingId: string;
  reason: string;
  description: string;
}

/**
 * Inicia una disputa formal asociada a una reserva y su custodia/pago.
 * Endpoint: POST /v1/disputes
 */
export async function createDispute(payload: CreateDisputePayload): Promise<Dispute> {
  return apiClient.post<Dispute>('/v1/disputes', {
    booking_id: payload.bookingId,
    reason: payload.reason,
    description: payload.description,
  });
}

export const disputeService = {
  createDispute,
};

export default disputeService;
