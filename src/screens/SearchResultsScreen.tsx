import React, { useState, useEffect } from 'react';
import {
  ArrowLeft,
  Filter,
  ShieldCheck,
  Star,
  Users,
  Clock,
  Car,
  ChevronRight,
  AlertCircle,
  Loader2,
  Calendar,
  Sparkles,
  ArrowRight,
} from 'lucide-react';
import { tripService, TripSearchResult, SearchTripsParams } from '../services/tripService';
import { formatCurrency } from '../utils/apiHelpers';
import { BottomNav, BottomNavTab } from '../components/BottomNav';

export interface SearchResultsScreenProps {
  initialOrigin?: string;
  initialDestination?: string;
  initialDate?: string;
  initialSeats?: number;
  onBackToHome: () => void;
  onSelectTripToBook: (trip: TripSearchResult) => void;
  onNavigateToPublish: () => void;
  onNavigateToTrips: () => void;
  onNavigateToProfile: () => void;
}

export const SearchResultsScreen: React.FC<SearchResultsScreenProps> = ({
  initialOrigin = 'Palermo',
  initialDestination = 'Pilar',
  initialDate = '2026-09-17',
  initialSeats = 1,
  onBackToHome,
  onSelectTripToBook,
  onNavigateToPublish,
  onNavigateToTrips,
  onNavigateToProfile,
}) => {
  const [trips, setTrips] = useState<TripSearchResult[]>([]);
  const [isLoading, setIsLoading] = useState<boolean>(true);
  const [error, setError] = useState<string | null>(null);
  const [activeFilter, setActiveFilter] = useState<'all' | 'cheapest' | 'earliest' | 'verified_only'>('all');
  const [activeNavTab, setActiveNavTab] = useState<BottomNavTab>('search');

  useEffect(() => {
    let isMounted = true;

    async function loadTrips() {
      setIsLoading(true);
      setError(null);
      try {
        const results = await tripService.searchTrips({
          origin: initialOrigin,
          destination: initialDestination,
          date: initialDate,
          seats: initialSeats,
        });

        if (isMounted) {
          setTrips(results);
        }
      } catch (err) {
        if (isMounted) {
          setError('No fue posible cargar los viajes. Mostrando rutas recomendadas.');
        }
      } finally {
        if (isMounted) {
          setIsLoading(false);
        }
      }
    }

    loadTrips();

    return () => {
      isMounted = false;
    };
  }, [initialOrigin, initialDestination, initialDate, initialSeats]);

  // Aplicar filtros dinámicos en memoria
  const filteredTrips = [...trips].filter((t) => {
    if (activeFilter === 'verified_only') {
      return t.driverVerified === true;
    }
    return true;
  }).sort((a, b) => {
    if (activeFilter === 'cheapest') {
      return a.pricePerSeat - b.pricePerSeat;
    }
    if (activeFilter === 'earliest') {
      return new Date(a.departureTime).getTime() - new Date(b.departureTime).getTime();
    }
    return 0;
  });

  const handleTabChange = (tab: BottomNavTab) => {
    setActiveNavTab(tab);
    if (tab === 'home') onBackToHome();
    else if (tab === 'publish') onNavigateToPublish();
    else if (tab === 'trips') onNavigateToTrips();
    else if (tab === 'profile') onNavigateToProfile();
  };

  const formatDepartureHour = (isoString: string) => {
    try {
      const date = new Date(isoString);
      return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
    } catch {
      return '08:30';
    }
  };

  return (
    <div
      id="search-results-screen"
      className="min-h-screen pb-24"
      style={{
        backgroundColor: '#F7F9FA',
        fontFamily: '-apple-system, BlinkMacSystemFont, "SF Pro Text", "SF Pro Display", "Segoe UI", Roboto, sans-serif',
      }}
    >
      {/* Barra Superior con botón Volver y Resumen de Ruta */}
      <header className="bg-[#FFFFFF] border-b border-[#ECECEC] px-4 py-3 sticky top-0 z-30 shadow-xs">
        <div className="max-w-md mx-auto flex items-center justify-between">
          <div className="flex items-center gap-3">
            <button
              id="results-back-btn"
              type="button"
              onClick={onBackToHome}
              aria-label="Volver a Inicio"
              className="p-2 -ml-2 rounded-lg text-[#666666] hover:bg-slate-100 transition cursor-pointer"
            >
              <ArrowLeft className="w-5 h-5" />
            </button>
            <div>
              <h1 className="text-base font-bold text-[#1A1A1A] flex items-center gap-1.5">
                <span>{initialOrigin}</span>
                <span className="text-[#00A896]">→</span>
                <span>{initialDestination}</span>
              </h1>
              <div className="flex items-center gap-2 text-xs text-[#777777]">
                <span>{initialDate}</span>
                <span>•</span>
                <span>{initialSeats} {initialSeats === 1 ? 'asiento' : 'asientos'}</span>
              </div>
            </div>
          </div>

          <span className="text-xs font-semibold px-2 py-0.5 rounded-full bg-[#E6F6F4] text-[#00A896]">
            {filteredTrips.length} disponibles
          </span>
        </div>
      </header>

      {/* Chips de Filtros */}
      <div className="bg-[#FFFFFF] border-b border-[#EFEFEF] px-4 py-2.5">
        <div className="max-w-md mx-auto flex items-center gap-2 overflow-x-auto scrollbar-none">
          <button
            id="filter-chip-all"
            type="button"
            onClick={() => setActiveFilter('all')}
            className={`px-3 py-1.5 rounded-full text-xs font-semibold whitespace-nowrap transition cursor-pointer ${
              activeFilter === 'all'
                ? 'bg-[#00A896] text-white shadow-xs'
                : 'bg-[#F3F4F6] text-[#4A4A4A] hover:bg-slate-200'
            }`}
          >
            Todos los viajes
          </button>

          <button
            id="filter-chip-cheapest"
            type="button"
            onClick={() => setActiveFilter('cheapest')}
            className={`px-3 py-1.5 rounded-full text-xs font-semibold whitespace-nowrap transition cursor-pointer ${
              activeFilter === 'cheapest'
                ? 'bg-[#00A896] text-white shadow-xs'
                : 'bg-[#F3F4F6] text-[#4A4A4A] hover:bg-slate-200'
            }`}
          >
            Más barato
          </button>

          <button
            id="filter-chip-earliest"
            type="button"
            onClick={() => setActiveFilter('earliest')}
            className={`px-3 py-1.5 rounded-full text-xs font-semibold whitespace-nowrap transition cursor-pointer ${
              activeFilter === 'earliest'
                ? 'bg-[#00A896] text-white shadow-xs'
                : 'bg-[#F3F4F6] text-[#4A4A4A] hover:bg-slate-200'
            }`}
          >
            Salida más cercana
          </button>

          <button
            id="filter-chip-verified"
            type="button"
            onClick={() => setActiveFilter('verified_only')}
            className={`px-3 py-1.5 rounded-full text-xs font-semibold whitespace-nowrap transition cursor-pointer flex items-center gap-1 ${
              activeFilter === 'verified_only'
                ? 'bg-[#00A896] text-white shadow-xs'
                : 'bg-[#F3F4F6] text-[#4A4A4A] hover:bg-slate-200'
            }`}
          >
            <ShieldCheck className="w-3.5 h-3.5" />
            <span>Conductor verificado</span>
          </button>
        </div>
      </div>

      {/* Lista de Resultados de Viajes */}
      <main className="max-w-md mx-auto px-4 pt-4 space-y-3.5">
        {isLoading && (
          <div className="py-12 flex flex-col items-center justify-center text-center">
            <Loader2 className="w-8 h-8 text-[#00A896] animate-spin mb-3" />
            <span className="text-sm font-semibold text-[#1A1A1A]">
              Buscando viajes compatibles en ruta...
            </span>
            <p className="text-xs text-[#777777] mt-1">
              Verificando asientos disponibles y conductores con KYC aprobado.
            </p>
          </div>
        )}

        {!isLoading && filteredTrips.length === 0 && (
          <div className="py-12 px-4 rounded-[16px] bg-[#FFFFFF] border border-[#ECECEC] text-center">
            <div className="w-12 h-12 rounded-full bg-[#F3F4F6] text-[#888888] flex items-center justify-center mx-auto mb-3">
              <Car className="w-6 h-6" />
            </div>
            <h3 className="text-base font-bold text-[#1A1A1A]">No se encontraron viajes</h3>
            <p className="text-xs text-[#666666] mt-1 max-w-xs mx-auto">
              Intenta cambiar los filtros o busca una fecha posterior para encontrar conductores en este trayecto.
            </p>
            <button
              type="button"
              onClick={() => setActiveFilter('all')}
              className="mt-4 px-4 py-2 rounded-lg bg-[#E6F6F4] text-[#00A896] text-xs font-semibold hover:bg-[#D4EFEA] transition"
            >
              Restablecer filtros
            </button>
          </div>
        )}

        {!isLoading &&
          filteredTrips.map((trip) => (
            <div
              key={trip.id}
              id={`trip-card-${trip.id}`}
              className="rounded-[16px] p-5 bg-[#FFFFFF] border border-[#E8EEF2] hover:border-[#00A896]/60 transition-all shadow-xs space-y-4"
            >
              {/* Encabezado de la Tarjeta: Conductor y Badge DNI Verificado */}
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <div className="w-10 h-10 rounded-full bg-[#00A896] text-white font-bold text-xs flex items-center justify-center">
                    {trip.driverName.substring(0, 2).toUpperCase()}
                  </div>
                  <div>
                    <div className="flex items-center gap-1.5">
                      <span className="text-sm font-bold text-[#1A1A1A]">{trip.driverName}</span>
                      {trip.driverVerified && (
                        <span
                          id={`badge-dni-verified-${trip.id}`}
                          className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-semibold bg-[#E6F6F4] text-[#00A896]"
                        >
                          <ShieldCheck className="w-3 h-3 text-[#00A896]" />
                          <span>DNI Verificado</span>
                        </span>
                      )}
                    </div>
                    <div className="flex items-center gap-1.5 text-xs text-[#777777] mt-0.5">
                      <div className="flex items-center text-amber-500 font-semibold">
                        <Star className="w-3 h-3 fill-amber-400 stroke-amber-400 mr-0.5" />
                        <span>{trip.driverRating.toFixed(1)}</span>
                      </div>
                      <span>({trip.driverReviewsCount} reseñas)</span>
                    </div>
                  </div>
                </div>

                {/* Precio por Asiento */}
                <div className="text-right">
                  <span className="text-lg font-bold text-[#00A896]">
                    {formatCurrency(trip.pricePerSeat)}
                  </span>
                  <span className="text-[10px] text-[#777777] block">por asiento</span>
                </div>
              </div>

              {/* Información de Trayecto e Itinerario */}
              <div className="p-3 rounded-[12px] bg-[#F9FAFB] border border-[#F0F0F0] space-y-2">
                <div className="flex items-center justify-between text-xs">
                  <div className="flex items-center gap-2 text-[#1A1A1A] font-semibold">
                    <Clock className="w-3.5 h-3.5 text-[#00A896]" />
                    <span>Salida {formatDepartureHour(trip.departureTime)} hs</span>
                  </div>
                  <span className="text-[11px] text-[#666666]">
                    ~{trip.durationMinutes} min de viaje
                  </span>
                </div>

                <div className="text-xs text-[#555555] space-y-1">
                  <div className="flex items-center gap-2">
                    <span className="w-2 h-2 rounded-full bg-[#00A896] shrink-0" />
                    <span className="truncate">{trip.origin}</span>
                  </div>
                  <div className="flex items-center gap-2">
                    <span className="w-2 h-2 rounded-full bg-[#E63946] shrink-0" />
                    <span className="truncate">{trip.destination}</span>
                  </div>
                </div>
              </div>

              {/* Pie de Tarjeta: Modelo de Auto, Plazas y Botón Reservar */}
              <div className="pt-1 flex items-center justify-between">
                <div className="text-xs text-[#666666]">
                  <div className="flex items-center gap-1.5 font-medium">
                    <Car className="w-3.5 h-3.5 text-[#888888]" />
                    <span>{trip.carModel}</span>
                  </div>
                  <div className="flex items-center gap-1 text-[11px] text-[#00A896] font-semibold mt-0.5">
                    <Users className="w-3 h-3" />
                    <span>{trip.availableSeats} {trip.availableSeats === 1 ? 'asiento disponible' : 'asientos disponibles'}</span>
                  </div>
                </div>

                <button
                  id={`book-trip-btn-${trip.id}`}
                  type="button"
                  onClick={() => onSelectTripToBook(trip)}
                  className="flex items-center gap-1 px-4 py-2 rounded-[10px] text-white font-semibold text-xs transition-all shadow-xs hover:opacity-95 cursor-pointer"
                  style={{ backgroundColor: '#00A896' }}
                >
                  <span>Reservar</span>
                  <ChevronRight className="w-3.5 h-3.5" />
                </button>
              </div>
            </div>
          ))}
      </main>

      {/* Barra de Navegación Inferior */}
      <BottomNav
        activeTab={activeNavTab}
        onTabChange={handleTabChange}
      />
    </div>
  );
};

export default SearchResultsScreen;
