export * from './api';

export interface Vehicle {
  id: string;
  brand: string;
  model: string;
  color?: string;
  plate: string;
  isActive: boolean;
  status?: 'APPROVED' | 'PENDING_VERIFICATION' | 'REJECTED';
}

export type UserVehicle = Vehicle;

export interface UserProfile {
  id?: string;
  name: string;
  firstName?: string;
  lastName?: string;
  fullName?: string;
  email?: string;
  phone?: string;
  rating: number;
  reviewsCount: number;
  reviewCount?: number;
  isVerified: boolean;
  isIdentityVerified?: boolean;
  kycStatus?: 'APPROVED' | 'PENDING_VERIFICATION' | 'REJECTED' | 'NOT_STARTED';
  role?: 'PASSENGER' | 'DRIVER';
  vehicles: Vehicle[];
}

export interface Driver {
  name: string;
  avatar?: string;
  rating?: number;
  reviewsCount?: number;
  verified?: boolean;
  vehicle?: string;
  phone?: string;
}

export interface StoredTrip {
  id: string;
  origin: string;
  destination: string;
  date: string;
  time: string;
  price: number;
  totalSeats: number;
  availableSeats: number;
  driver: string | Driver;
  status: 'SCHEDULED' | 'IN_PROGRESS' | 'COMPLETED' | 'CANCELLED' | string;
  role: 'driver' | 'passenger';
}

export interface PublishTripPayload {
  origin: string;
  destination: string;
  date: string;
  time: string;
  price: number;
  totalSeats: number;
  availableSeats?: number;
  driver?: string | Driver;
  status?: string;
  role?: 'driver' | 'passenger';
}

export interface CreateTripPayload {
  origin: string;
  destination: string;
  pricePerSeat: number;
  availableSeats: number;
  departureTime: string;
}

export interface SearchTripsParams {
  origin?: string;
  destination?: string;
  date?: string;
  seats?: number;
  filter?: 'cheapest' | 'earliest' | 'verified_only';
}

export interface TripSearchResult {
  id: string;
  driverId: string;
  driverName: string;
  driverRating: number;
  driverReviewsCount: number;
  driverVerified: boolean;
  driverPhoneNumber?: string;
  driverAvatarUrl?: string;
  carModel: string;
  carColor?: string;
  durationMinutes: number;
  origin: string;
  destination: string;
  pricePerSeat: number;
  availableSeats: number;
  departureTime: string;
  status: 'SCHEDULED' | 'IN_PROGRESS' | 'COMPLETED' | 'CANCELLED';
}

export interface VehicleFormData {
  brand: string;
  model: string;
  color: string;
  plate: string;
}
