export type TripStatus =
  | 'SCHEDULED'
  | 'IN_PROGRESS'
  | 'COMPLETED'
  | 'CANCELLED';

export type BookingStatus = 'PENDING' | 'CONFIRMED' | 'CANCELLED';

export type EscrowStatus = 'HELD' | 'RELEASED' | 'REFUNDED';

export type DisputeStatus = 'OPENED' | 'IN_REVIEW' | 'RESOLVED' | 'REJECTED';

export interface Trip {
  id: string;
  driverId: string;
  origin: string;
  destination: string;
  pricePerSeat: number;
  availableSeats: number;
  departureTime: string;
  status: TripStatus;
}

export interface Booking {
  id: string;
  tripId: string;
  passengerId: string;
  seatsRequested: number;
  status: BookingStatus;
  paymentGatewayRef: string;
  createdAt: string;
}

export interface EscrowTransaction {
  id: string;
  bookingId: string;
  tripId: string;
  payerId: string;
  payeeId: string;
  amount: number;
  currency: string;
  paymentGatewayRef: string;
  status: EscrowStatus;
}

export interface Dispute {
  id: string;
  escrowTransactionId: string;
  bookingId: string;
  tripId: string;
  reporterId: string;
  defendantId: string;
  reason: string;
  description: string;
  evidenceUrls: string[] | null;
  status: DisputeStatus;
  createdAt: string;
  updatedAt: string;
}

export interface ApiErrorResponse {
  code: string;
  message: string;
  details?: unknown;
}
