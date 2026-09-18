import React, { createContext, useContext, useState, useEffect, ReactNode, useCallback } from 'react';
import {
  userService,
  UserProfile,
  Vehicle,
  DEFAULT_USER_PROFILE,
} from '../services/userService';

export type { UserProfile, Vehicle };

export interface UserContextType {
  user: UserProfile;
  isLoading: boolean;
  error: string | null;
  addVehicle: (vehicle: Omit<Vehicle, 'id'> | { brand: string; model: string; color?: string; plate: string; isActive?: boolean }) => Promise<Vehicle>;
  setActiveVehicle: (id: string) => Promise<void>;
  updateProfile: (updates: Partial<UserProfile>) => Promise<void>;
  refreshProfile: () => Promise<void>;
}

const UserContext = createContext<UserContextType | undefined>(undefined);

export const UserProvider: React.FC<{ children: ReactNode }> = ({ children }) => {
  const [user, setUser] = useState<UserProfile>(() => {
    try {
      const saved = localStorage.getItem('thumbi_user_profile');
      if (saved) {
        const parsed = JSON.parse(saved);
        if (parsed && typeof parsed === 'object' && parsed.name) {
          if (!parsed.vehicles || !Array.isArray(parsed.vehicles) || parsed.vehicles.length === 0) {
            parsed.vehicles = DEFAULT_USER_PROFILE.vehicles;
          }
          return parsed;
        }
      }
    } catch {
      // Fallback a perfil por defecto
    }
    return DEFAULT_USER_PROFILE;
  });

  const [isLoading, setIsLoading] = useState<boolean>(false);
  const [error, setError] = useState<string | null>(null);

  const refreshProfile = useCallback(async () => {
    setIsLoading(true);
    setError(null);
    try {
      const profile = await userService.fetchUserProfile();
      if (profile && typeof profile === 'object' && profile.name) {
        setUser(profile);
      } else {
        setUser(DEFAULT_USER_PROFILE);
      }
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Error al cargar el perfil de usuario';
      setError(msg);
      setUser((prev) => (prev && prev.name ? prev : DEFAULT_USER_PROFILE));
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    refreshProfile();
  }, [refreshProfile]);

  const addVehicle = async (
    vehicleData: Omit<Vehicle, 'id'> | { brand: string; model: string; color?: string; plate: string; isActive?: boolean }
  ): Promise<Vehicle> => {
    setIsLoading(true);
    setError(null);
    try {
      const newVehicle = await userService.addVehicleApi(vehicleData);
      setUser((prev) => {
        const updatedVehicles = newVehicle.isActive
          ? prev.vehicles.map((v) => ({ ...v, isActive: false }))
          : [...prev.vehicles];

        return {
          ...prev,
          vehicles: [newVehicle, ...updatedVehicles],
        };
      });
      return newVehicle;
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Error al registrar el nuevo vehículo';
      setError(msg);
      throw err;
    } finally {
      setIsLoading(false);
    }
  };

  const setActiveVehicle = async (id: string): Promise<void> => {
    setIsLoading(true);
    setError(null);
    try {
      const updatedVehicles = await userService.setActiveVehicleApi(id);
      setUser((prev) => ({
        ...prev,
        vehicles: updatedVehicles,
      }));
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Error al actualizar el vehículo activo';
      setError(msg);
      throw err;
    } finally {
      setIsLoading(false);
    }
  };

  const updateProfile = async (updates: Partial<UserProfile>): Promise<void> => {
    setIsLoading(true);
    setError(null);
    try {
      setUser((prev) => {
        const updated = { ...prev, ...updates };
        try {
          localStorage.setItem('thumbi_user_profile', JSON.stringify(updated));
        } catch {
          // Ignorar error de guardado
        }
        return updated;
      });
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Error al actualizar los datos de perfil';
      setError(msg);
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <UserContext.Provider
      value={{
        user,
        isLoading,
        error,
        addVehicle,
        setActiveVehicle,
        updateProfile,
        refreshProfile,
      }}
    >
      {children}
    </UserContext.Provider>
  );
};

export const useUser = (): UserContextType => {
  const context = useContext(UserContext);
  if (!context) {
    throw new Error('useUser debe utilizarse dentro de un UserProvider');
  }
  return context;
};

export const useUserContext = useUser;

export { UserContext };

