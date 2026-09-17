import { apiClient } from './apiClient';

export interface UserVehicle {
  id: string;
  brand: string;
  model: string;
  color?: string;
  plate: string;
  isActive: boolean;
  status: 'APPROVED' | 'PENDING_VERIFICATION' | 'REJECTED';
}

export interface UserProfile {
  id: string;
  firstName: string;
  lastName: string;
  fullName: string;
  email: string;
  phone?: string;
  rating: number;
  reviewCount: number;
  isIdentityVerified: boolean;
  kycStatus: 'APPROVED' | 'PENDING_VERIFICATION' | 'REJECTED' | 'NOT_STARTED';
  role: 'PASSENGER' | 'DRIVER';
}

const STORAGE_PROFILE_KEY = 'thumbi_user_profile';
const STORAGE_VEHICLES_KEY = 'thumbi_user_vehicles';

const DEFAULT_PROFILE: UserProfile = {
  id: 'usr-juan-lorusso',
  firstName: 'Juan Francisco',
  lastName: 'Lorusso',
  fullName: 'Juan Francisco Lorusso',
  email: 'juanflorusso@gmail.com',
  phone: '+54 9 11 4567-8901',
  rating: 4.9,
  reviewCount: 18,
  isIdentityVerified: true,
  kycStatus: 'APPROVED',
  role: 'DRIVER',
};

const DEFAULT_VEHICLES: UserVehicle[] = [
  {
    id: 'veh-001',
    brand: 'Toyota',
    model: 'Corolla',
    color: 'Blanco',
    plate: 'AA 123 CD',
    isActive: true,
    status: 'APPROVED',
  },
];

/**
 * Servicio de Usuario y Perfil para Thumbi
 */
export const userService = {
  /**
   * Obtiene el perfil del usuario activo (con sincronización API y fallback local seguro).
   */
  async getProfile(): Promise<UserProfile> {
    try {
      const res = await apiClient.get<UserProfile>('/v1/users/me');
      if (res && res.id) {
        localStorage.setItem(STORAGE_PROFILE_KEY, JSON.stringify(res));
        return res;
      }
    } catch {
      // Fallback local
    }

    const cached = localStorage.getItem(STORAGE_PROFILE_KEY);
    if (cached) {
      try {
        return JSON.parse(cached) as UserProfile;
      } catch {
        // Ignorar error de parseo
      }
    }

    localStorage.setItem(STORAGE_PROFILE_KEY, JSON.stringify(DEFAULT_PROFILE));
    return DEFAULT_PROFILE;
  },

  /**
   * Obtiene los vehículos registrados del usuario.
   */
  async getVehicles(): Promise<UserVehicle[]> {
    try {
      const res = await apiClient.get<UserVehicle[]>('/v1/users/me/vehicles');
      if (Array.isArray(res)) {
        localStorage.setItem(STORAGE_VEHICLES_KEY, JSON.stringify(res));
        return res;
      }
    } catch {
      // Fallback local
    }

    const cached = localStorage.getItem(STORAGE_VEHICLES_KEY);
    if (cached) {
      try {
        return JSON.parse(cached) as UserVehicle[];
      } catch {
        // Ignorar error de parseo
      }
    }

    localStorage.setItem(STORAGE_VEHICLES_KEY, JSON.stringify(DEFAULT_VEHICLES));
    return DEFAULT_VEHICLES;
  },

  /**
   * Registra un nuevo vehículo en la cuenta del usuario.
   */
  async addVehicle(vehicle: {
    brand: string;
    model: string;
    color?: string;
    plate: string;
  }): Promise<UserVehicle> {
    const newVehicle: UserVehicle = {
      id: 'veh-' + Math.random().toString(36).substring(2, 8),
      brand: vehicle.brand.trim(),
      model: vehicle.model.trim(),
      color: vehicle.color?.trim() || 'Blanco',
      plate: vehicle.plate.trim().toUpperCase(),
      isActive: true,
      status: 'APPROVED',
    };

    try {
      await apiClient.post('/v1/users/me/vehicles', newVehicle);
    } catch {
      // Persistir localmente en caso de modo prototipo / sin backend directo
    }

    const vehicles = await this.getVehicles();
    const updatedVehicles = [newVehicle, ...vehicles.map(v => ({ ...v, isActive: false }))];
    localStorage.setItem(STORAGE_VEHICLES_KEY, JSON.stringify(updatedVehicles));

    return newVehicle;
  },

  /**
   * Actualiza el vehículo principal o activo.
   */
  async setActiveVehicle(vehicleId: string): Promise<UserVehicle[]> {
    const vehicles = await this.getVehicles();
    const updated = vehicles.map(v => ({
      ...v,
      isActive: v.id === vehicleId,
    }));
    localStorage.setItem(STORAGE_VEHICLES_KEY, JSON.stringify(updated));
    return updated;
  },
};

export default userService;
