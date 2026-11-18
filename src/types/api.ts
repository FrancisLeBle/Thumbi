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

export interface MoneyAmount {
  amount: number;
  currency: string;
}

export interface Booking {
  id: string;
  tripId: string;
  passengerId: string;
  seatsRequested: number;
  status: BookingStatus;
  paymentGatewayRef: string;
  createdAt: string;
  base_amount?: MoneyAmount;
  platform_fee?: MoneyAmount;
  total_price?: MoneyAmount;
}

export interface PlatformPricingConfig {
  id: string;
  currency: string;
  passenger_service_fee_bps: number;
  passenger_service_fee_percent: number;
  passenger_fixed_fee: MoneyAmount;
  early_cancel_fee_retention_bps: number;
  early_cancel_fee_retention_percent: number;
  late_cancel_window_hours: number;
  late_cancel_fee_retention_bps: number;
  late_cancel_fee_retention_percent: number;
  late_cancel_passenger_refund_bps: number;
  late_cancel_passenger_refund_percent: number;
  late_cancel_driver_comp_bps: number;
  late_cancel_driver_comp_percent: number;
  late_cancel_platform_retention_bps: number;
  late_cancel_platform_retention_percent: number;
  is_active: boolean;
  updated_by?: string;
  updated_at: string;
}

export interface BookingPricingQuote {
  trip_id: string;
  seats_requested: number;
  currency: string;
  unit_price: MoneyAmount;
  base_amount: MoneyAmount;
  platform_fee: MoneyAmount;
  total_price: MoneyAmount;
  driver_payout: MoneyAmount;
  service_fee_bps: number;
  service_fee_percent: number;
  fixed_fee: MoneyAmount;
}

export interface RefundBreakdown {
  id: string;
  escrow_transaction_id: string;
  booking_id: string;
  passenger_refund_amount: MoneyAmount;
  driver_compensation_amount: MoneyAmount;
  platform_retained_amount: MoneyAmount;
  refund_type: string;
  gateway_refund_ref?: string;
  processed_at: string;
  created_at: string;
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
