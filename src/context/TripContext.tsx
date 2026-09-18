import React, { createContext, useContext, useState, useEffect, ReactNode, useCallback } from 'react';
import {
  tripService,
  StoredTrip as Trip,
  Driver,
  PublishTripPayload as PublishTripData,
  INITIAL_TRIPS_DATA,
} from '../services/tripService';

export type { Trip, Driver, PublishTripData };

export interface TripContextType {
  trips: Trip[];
  isLoading: boolean;
  error: string | null;
  publishTrip: (tripData: PublishTripData) => Promise<Trip>;
  bookTrip: (tripId: string, seatsToBook?: number) => Promise<boolean>;
  cancelTrip: (tripId: string) => Promise<void>;
  getTripById: (tripId: string) => Trip | undefined;
  refreshTrips: () => Promise<void>;
}

const TripContext = createContext<TripContextType | undefined>(undefined);

export const TripProvider: React.FC<{ children: ReactNode }> = ({ children }) => {
  const [trips, setTrips] = useState<Trip[]>(() => {
    try {
      const saved = localStorage.getItem('thumbi_trips');
      if (saved) {
        const parsed = JSON.parse(saved);
        if (Array.isArray(parsed) && parsed.length > 0) {
          return parsed;
        }
      }
    } catch {
      // Ignorar error de parsing
    }
    return INITIAL_TRIPS_DATA;
  });

  const [isLoading, setIsLoading] = useState<boolean>(false);
  const [error, setError] = useState<string | null>(null);

  const refreshTrips = useCallback(async () => {
    setIsLoading(true);
    setError(null);
    try {
      const data = await tripService.fetchTripsApi();
      if (Array.isArray(data) && data.length > 0) {
        setTrips(data);
      } else {
        setTrips(INITIAL_TRIPS_DATA);
      }
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Error al cargar los viajes';
      setError(msg);
      setTrips((prev) => (Array.isArray(prev) && prev.length > 0 ? prev : INITIAL_TRIPS_DATA));
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    refreshTrips();
  }, [refreshTrips]);

  // Función para agregar un nuevo viaje como conductor
  const publishTrip = async (tripData: PublishTripData): Promise<Trip> => {
    setIsLoading(true);
    setError(null);
    try {
      const newTrip = await tripService.publishTripApi(tripData);
      setTrips((prevTrips) => [newTrip, ...prevTrips.filter((t) => t.id !== newTrip.id)]);
      return newTrip;
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Error al publicar el viaje';
      setError(msg);
      throw err;
    } finally {
      setIsLoading(false);
    }
  };

  // Función para reservar plazas en un viaje como pasajero
  const bookTrip = async (tripId: string, seatsToBook = 1): Promise<boolean> => {
    setIsLoading(true);
    setError(null);
    try {
      const result = await tripService.bookTripApi(tripId, seatsToBook);
      if (result.success) {
        setTrips((prevTrips) =>
          prevTrips.map((trip) => (trip.id === tripId ? result.trip : trip))
        );
        return true;
      }
      return false;
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Error al reservar el viaje';
      setError(msg);
      return false;
    } finally {
      setIsLoading(false);
    }
  };

  const cancelTrip = async (tripId: string): Promise<void> => {
    setIsLoading(true);
    setError(null);
    try {
      const updatedTrip = await tripService.cancelTripApi(tripId);
      setTrips((prevTrips) =>
        prevTrips.map((trip) => (trip.id === tripId ? updatedTrip : trip))
      );
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Error al cancelar el viaje';
      setError(msg);
      // Fallback local en caso de error
      setTrips((prevTrips) =>
        prevTrips.map((trip) =>
          trip.id === tripId ? { ...trip, status: 'CANCELLED' } : trip
        )
      );
    } finally {
      setIsLoading(false);
    }
  };

  const getTripById = (tripId: string): Trip | undefined => {
    return trips.find((t) => t.id === tripId);
  };

  return (
    <TripContext.Provider
      value={{
        trips,
        isLoading,
        error,
        publishTrip,
        bookTrip,
        cancelTrip,
        getTripById,
        refreshTrips,
      }}
    >
      {children}
    </TripContext.Provider>
  );
};

export const useTrips = (): TripContextType => {
  const context = useContext(TripContext);
  if (!context) {
    throw new Error('useTrips debe utilizarse dentro de un TripProvider');
  }
  return context;
};

export const useTripContext = useTrips;

export { TripContext };

