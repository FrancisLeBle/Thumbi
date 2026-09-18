import { apiClient } from './apiClient';
import type { Vehicle, UserVehicle, UserProfile } from '../types';

export type { Vehicle, UserVehicle, UserProfile };

export const DEFAULT_VEHICLES: Vehicle[] = [
  {
    id: 'veh-corolla-01',
    brand: 'Toyota',
    model: 'Corolla',
    color: 'Blanco',
    plate: 'AF 123 CD',
    isActive: true,
    status: 'APPROVED',
  },
  {
    id: 'veh-peugeot-02',
    brand: 'Peugeot',
    model: '208',
    color: 'Gris Grafito',
    plate: 'AE 987 RT',
    isActive: false,
    status: 'APPROVED',
  },
];

export const DEFAULT_USER_PROFILE: UserProfile = {
  id: 'usr-juan-lorusso',
  name: 'Juan Francisco Lorusso',
  firstName: 'Juan Francisco',
  lastName: 'Lorusso',
  fullName: 'Juan Francisco Lorusso',
  email: 'juanflorusso@gmail.com',
  phone: '+54 9 11 4567-8901',
  rating: 4.9,
  reviewsCount: 38,
  reviewCount: 38,
  isVerified: true,
  isIdentityVerified: true,
  kycStatus: 'APPROVED',
  role: 'DRIVER',
  vehicles: DEFAULT_VEHICLES,
};

export interface RegisterVehiclePayload {
  brand: string;
  model: string;
  year?: number;
  color?: string;
  plate: string;
  seatCapacity?: number;
  driverLicenseBase64?: string;
  vehicleCedulaBase64?: string;
  insuranceBase64?: string;
  isActive?: boolean;
}

export interface VehicleStatusApiResponse {
  hasVehicle: boolean;
  status?: string;
  vehicle?: {
    id: string;
    userId?: string;
    brand: string;
    model: string;
    year: number;
    plateNumber: string;
    color?: string;
    seatCapacity: number;
    status: 'APPROVED' | 'PENDING_VERIFICATION' | 'REJECTED';
  };
}

export interface VehicleRegisterApiResponse {
  message: string;
  status: string;
  vehicle: {
    id: string;
    brand: string;
    model: string;
    year: number;
    plateNumber: string;
    seatCapacity: number;
    status: 'APPROVED' | 'PENDING_VERIFICATION' | 'REJECTED';
  };
}

export interface AuthMeApiResponse {
  user: {
    id: string;
    email: string;
    firstName: string;
    lastName: string;
    avatarUrl?: string;
    phone?: string;
    role: 'PASSENGER' | 'DRIVER';
    kycStatus: 'APPROVED' | 'PENDING_VERIFICATION' | 'REJECTED' | 'NOT_STARTED';
    isDriverActive: boolean;
    ratingAvg?: number;
    ratingCount?: number;
  };
  capabilities: {
    canBookRides: boolean;
    canPublishRides: boolean;
  };
}

/**
 * Obtiene el perfil de usuario consolidado consumiendo GET /v1/auth/me y GET /v1/vehicles/status.
 */
export async function fetchUserProfile(): Promise<UserProfile> {
  try {
    const [meData, vehicleData] = await Promise.allSettled([
      apiClient.get<AuthMeApiResponse>('/v1/auth/me'),
      apiClient.get<VehicleStatusApiResponse>('/v1/vehicles/status'),
    ]);

    if (meData.status === 'fulfilled' && meData.value?.user) {
      const u = meData.value.user;
      const firstName = u.firstName || 'Usuario';
      const lastName = u.lastName || '';
      const fullName = `${firstName} ${lastName}`.trim();

      const vehiclesList: Vehicle[] = [];
      if (
        vehicleData.status === 'fulfilled' &&
        vehicleData.value?.hasVehicle &&
        vehicleData.value.vehicle
      ) {
        const v = vehicleData.value.vehicle;
        vehiclesList.push({
          id: v.id,
          brand: v.brand,
          model: v.model,
          color: v.color || 'Blanco',
          plate: v.plateNumber,
          isActive: true,
          status: v.status || 'APPROVED',
        });
      } else {
        vehiclesList.push(...DEFAULT_VEHICLES);
      }

      const rating = typeof u.ratingAvg === 'number' && u.ratingAvg > 0 ? u.ratingAvg : 4.9;
      const reviewsCount = typeof u.ratingCount === 'number' ? u.ratingCount : 38;

      return {
        id: u.id,
        name: fullName || u.email,
        firstName,
        lastName,
        fullName,
        email: u.email,
        phone: u.phone || '+54 9 11 4567-8901',
        rating,
        reviewsCount,
        reviewCount: reviewsCount,
        isVerified: u.kycStatus === 'APPROVED',
        isIdentityVerified: u.kycStatus === 'APPROVED',
        kycStatus: u.kycStatus,
        role: u.role,
        vehicles: vehiclesList,
      };
    }
  } catch {
    // Si no hay sesión o API offline, retorna perfil por defecto
  }

  return DEFAULT_USER_PROFILE;
}

/**
 * Consulta el estado de verificación del vehículo en el backend.
 * Endpoint: GET /v1/vehicles/status
 */
export async function getVehicleStatus(): Promise<VehicleStatusApiResponse> {
  return apiClient.get<VehicleStatusApiResponse>('/v1/vehicles/status');
}

/**
 * Registra un nuevo vehículo con su documentación en el backend en Go.
 * Endpoint: POST /v1/vehicles/register
 */
export async function registerVehicleApi(payload: RegisterVehiclePayload): Promise<Vehicle> {
  const currentYear = new Date().getFullYear();
  const year = payload.year || currentYear;
  const seatCapacity = payload.seatCapacity || 4;

  const res = await apiClient.post<VehicleRegisterApiResponse>('/v1/vehicles/register', {
    brand: payload.brand.trim(),
    model: payload.model.trim(),
    year,
    plate_number: payload.plate.trim().toUpperCase(),
    color: payload.color?.trim() || 'Blanco',
    seat_capacity: seatCapacity,
    driver_license_base64: payload.driverLicenseBase64 || 'data:image/jpeg;base64,placeholderLicense',
    vehicle_cedula_base64: payload.vehicleCedulaBase64 || 'data:image/jpeg;base64,placeholderCedula',
    insurance_base64: payload.insuranceBase64 || 'data:image/jpeg;base64,placeholderInsurance',
  });

  return {
    id: res.vehicle.id,
    brand: res.vehicle.brand,
    model: res.vehicle.model,
    color: payload.color?.trim() || 'Blanco',
    plate: res.vehicle.plateNumber,
    isActive: payload.isActive ?? true,
    status: res.vehicle.status || 'PENDING_VERIFICATION',
  };
}

/**
 * Agrega un nuevo vehículo invocando la API de registro.
 */
export async function addVehicleApi(vehicle: {
  brand: string;
  model: string;
  color?: string;
  plate: string;
  isActive?: boolean;
}): Promise<Vehicle> {
  return registerVehicleApi(vehicle);
}

/**
 * Establece el vehículo activo del usuario.
 */
export async function setActiveVehicleApi(vehicleId: string): Promise<Vehicle[]> {
  const profile = await fetchUserProfile();
  return profile.vehicles.map((v) => ({
    ...v,
    isActive: v.id === vehicleId,
  }));
}

/**
 * Servicio de Usuario y Perfil para Thumbi
 */
export const userService = {
  fetchUserProfile,
  getVehicleStatus,
  registerVehicleApi,
  addVehicleApi,
  setActiveVehicleApi,

  async getProfile(): Promise<UserProfile> {
    return fetchUserProfile();
  },

  async getVehicles(): Promise<Vehicle[]> {
    const profile = await fetchUserProfile();
    return profile.vehicles;
  },

  async addVehicle(vehicle: {
    brand: string;
    model: string;
    color?: string;
    plate: string;
    isActive?: boolean;
  }): Promise<Vehicle> {
    return addVehicleApi(vehicle);
  },

  async setActiveVehicle(vehicleId: string): Promise<Vehicle[]> {
    return setActiveVehicleApi(vehicleId);
  },
};

export default userService;

