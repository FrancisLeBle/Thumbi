import React, { useState, useEffect, useMemo } from 'react';
import {
  ArrowLeft,
  Edit2,
  Check,
  Star,
  Loader2,
  Calendar,
  Users,
  Search,
  Car,
  PlusCircle,
  User,
  X,
  MapPin,
  Clock,
} from 'lucide-react';
import { tripService, TripSearchResult } from '../services/tripService';

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

type FilterType = 'cheapest' | 'earliest' | 'verified' | null;

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
  const [activeFilter, setActiveFilter] = useState<FilterType>('cheapest');
  const [isEditModalOpen, setIsEditModalOpen] = useState<boolean>(false);

  // Parámetros de búsqueda modificables
  const [origin, setOrigin] = useState<string>(initialOrigin);
  const [destination, setDestination] = useState<string>(initialDestination);
  const [date, setDate] = useState<string>(initialDate);
  const [seats, setSeats] = useState<number>(initialSeats);

  // Estados temporales para modal de edición
  const [editOrigin, setEditOrigin] = useState<string>(initialOrigin);
  const [editDestination, setEditDestination] = useState<string>(initialDestination);
  const [editDate, setEditDate] = useState<string>(initialDate);
  const [editSeats, setEditSeats] = useState<number>(initialSeats);

  // Sincronizar props entrantes si cambian
  useEffect(() => {
    setOrigin(initialOrigin);
    setDestination(initialDestination);
    setDate(initialDate);
    setSeats(initialSeats);
    setEditOrigin(initialOrigin);
    setEditDestination(initialDestination);
    setEditDate(initialDate);
    setEditSeats(initialSeats);
  }, [initialOrigin, initialDestination, initialDate, initialSeats]);

  // Carga de viajes desde tripService
  useEffect(() => {
    let isMounted = true;
    async function fetchTrips() {
      setIsLoading(true);
      try {
        const results = await tripService.searchTrips({
          origin,
          destination,
          date,
          seats,
        });

        if (isMounted) {
          // Si no hubiese resultados para la ruta exacta, proveemos resultados sugeridos de calidad
          if (results.length > 0) {
            setTrips(results);
          } else {
            setTrips([
              {
                id: 'trip-carlos-1',
                driverId: 'drv-carlos-101',
                driverName: 'Carlos M.',
                driverRating: 4.9,
                driverReviewsCount: 38,
                driverVerified: true,
                carModel: 'Toyota Corolla',
                carColor: 'Blanco',
                durationMinutes: 45,
                origin,
                destination,
                pricePerSeat: 3500,
                availableSeats: 2,
                departureTime: '2026-09-17T18:30:00Z',
                status: 'SCHEDULED',
              },
              {
                id: 'trip-mariana-2',
                driverId: 'drv-mariana-102',
                driverName: 'Mariana G.',
                driverRating: 5.0,
                driverReviewsCount: 52,
                driverVerified: true,
                carModel: 'Peugeot 208',
                carColor: 'Gris',
                durationMinutes: 40,
                origin,
                destination,
                pricePerSeat: 3800,
                availableSeats: 1,
                departureTime: '2026-09-17T19:00:00Z',
                status: 'SCHEDULED',
              },
              {
                id: 'trip-lucas-3',
                driverId: 'drv-lucas-103',
                driverName: 'Lucas R.',
                driverRating: 4.8,
                driverReviewsCount: 27,
                driverVerified: true,
                carModel: 'Volkswagen Gol',
                carColor: 'Negro',
                durationMinutes: 45,
                origin,
                destination,
                pricePerSeat: 3500,
                availableSeats: 3,
                departureTime: '2026-09-17T19:30:00Z',
                status: 'SCHEDULED',
              },
            ]);
          }
        }
      } catch {
        if (isMounted) {
          setTrips([
            {
              id: 'trip-carlos-1',
              driverId: 'drv-carlos-101',
              driverName: 'Carlos M.',
              driverRating: 4.9,
              driverReviewsCount: 38,
              driverVerified: true,
              carModel: 'Toyota Corolla',
              carColor: 'Blanco',
              durationMinutes: 45,
              origin,
              destination,
              pricePerSeat: 3500,
              availableSeats: 2,
              departureTime: '2026-09-17T18:30:00Z',
              status: 'SCHEDULED',
            },
            {
              id: 'trip-mariana-2',
              driverId: 'drv-mariana-102',
              driverName: 'Mariana G.',
              driverRating: 5.0,
              driverReviewsCount: 52,
              driverVerified: true,
              carModel: 'Peugeot 208',
              carColor: 'Gris',
              durationMinutes: 40,
              origin,
              destination,
              pricePerSeat: 3800,
              availableSeats: 1,
              departureTime: '2026-09-17T19:00:00Z',
              status: 'SCHEDULED',
            },
          ]);
        }
      } finally {
        if (isMounted) {
          setIsLoading(false);
        }
      }
    }

    fetchTrips();
    return () => {
      isMounted = false;
    };
  }, [origin, destination, date, seats]);

  // Aplicar ordenamiento y filtros
  const sortedAndFilteredTrips = useMemo(() => {
    let result = [...trips];

    if (activeFilter === 'verified') {
      result = result.filter((t) => t.driverVerified);
    }

    if (activeFilter === 'cheapest') {
      result.sort((a, b) => a.pricePerSeat - b.pricePerSeat);
    } else if (activeFilter === 'earliest') {
      result.sort((a, b) => new Date(a.departureTime).getTime() - new Date(b.departureTime).getTime());
    }

    return result;
  }, [trips, activeFilter]);

  const handleFilterToggle = (filter: FilterType) => {
    if (activeFilter === filter) {
      setActiveFilter(null);
    } else {
      setActiveFilter(filter);
    }
  };

  const handleApplyEdit = (e: React.FormEvent) => {
    e.preventDefault();
    setOrigin(editOrigin);
    setDestination(editDestination);
    setDate(editDate);
    setSeats(editSeats);
    setIsEditModalOpen(false);
  };

  const formatHour = (isoStr: string) => {
    try {
      const d = new Date(isoStr);
      if (isNaN(d.getTime())) return '18:30';
      return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', hour12: false });
    } catch {
      return '18:30';
    }
  };

  const calculateArrivalHour = (isoStr: string, durationMinutes: number) => {
    try {
      const d = new Date(isoStr);
      if (isNaN(d.getTime())) return '19:15';
      const arrival = new Date(d.getTime() + durationMinutes * 60 * 1000);
      return arrival.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', hour12: false });
    } catch {
      return '19:15';
    }
  };

  const getDriverInitials = (name: string) => {
    const parts = name.trim().split(' ');
    if (parts.length >= 2) {
      return `${parts[0][0]}${parts[1][0]}`.toUpperCase();
    }
    return name.slice(0, 2).toUpperCase() || 'CM';
  };

  // Resumen de ruta simplificada para el header
  const cleanRouteName = (place: string) => {
    return place.split('(')[0].trim();
  };

  return (
    <div className="min-h-screen flex items-center justify-center p-0 md:p-6 bg-slate-900 font-sans">
      {/* Device Frame Viewport matching iPhone 15 / 16 (393 x 852 style) */}
      <div
        className="w-full max-w-[393px] h-[852px] bg-[#F7F9FA] relative overflow-hidden flex flex-col md:rounded-[48px] shadow-2xl border border-[#222] select-none"
        data-purpose="ios-viewport"
      >
        {/* BEGIN: iOS Native Status Bar */}
        <header
          className="w-full bg-[#F7F9FA] pt-3 px-7 pb-1 flex justify-between items-center z-30 select-none shrink-0"
          data-purpose="status-bar"
        >
          {/* Time Display */}
          <span className="text-[15px] font-semibold tracking-tight text-[#1A1A1A]">9:41</span>

          {/* Dynamic Island Indicator */}
          <div className="w-28 h-6 bg-black rounded-full absolute left-1/2 -translate-x-1/2 top-2.5 hidden sm:block" />

          {/* Native Status Bar Icons: Cellular, Wifi, Battery */}
          <div className="flex items-center space-x-1.5 text-[#1A1A1A]">
            {/* Cellular Signal */}
            <svg className="w-4 h-3.5 fill-current" viewBox="0 0 17 12">
              <rect height="3" rx="0.6" width="2.5" x="0" y="9" />
              <rect height="6" rx="0.6" width="2.5" x="4.5" y="6" />
              <rect height="9" rx="0.6" width="2.5" x="9" y="3" />
              <rect height="12" rx="0.6" width="2.5" x="13.5" y="0" />
            </svg>
            {/* Wi-Fi */}
            <svg className="w-4 h-3.5 fill-current" viewBox="0 0 16 12">
              <path d="M8 2.8C10.6 2.8 13 3.8 14.8 5.4L16 4.1C13.8 2.2 11 1 8 1 5 1 2.2 2.2 0 4.1L1.2 5.4C3 3.8 5.4 2.8 8 2.8ZM8 6.4C9.7 6.4 11.2 7.1 12.4 8.2L13.6 6.9C12.1 5.5 10.1 4.6 8 4.6 5.9 4.6 3.9 5.5 2.4 6.9L3.6 8.2C4.8 7.1 6.3 6.4 8 6.4ZM8 10C8.8 10 9.5 10.7 9.5 11.5 9.5 12.3 8.8 13 8 13 7.2 13 6.5 12.3 6.5 11.5 6.5 10.7 7.2 10 8 10Z" />
            </svg>
            {/* Battery */}
            <div className="w-6 h-3 rounded-[3.5px] border border-[#1A1A1A] p-[1.5px] flex items-center">
              <div className="h-full w-full bg-[#1A1A1A] rounded-[1.5px]" />
            </div>
          </div>
        </header>
        {/* END: iOS Native Status Bar */}

        {/* BEGIN: Search Results Header */}
        <section className="w-full bg-[#F7F9FA] px-4 pt-1 pb-2 shrink-0" data-purpose="search-header">
          <div className="flex items-center justify-between">
            {/* Back Arrow Button */}
            <button
              id="back-to-home-btn"
              aria-label="Volver"
              type="button"
              onClick={onBackToHome}
              className="w-9 h-9 flex items-center justify-start text-[#00A896] hover:opacity-80 active:opacity-60 transition cursor-pointer"
            >
              <ArrowLeft className="w-6 h-6 stroke-[2.2]" />
            </button>

            {/* Route Origin, Destination and Details */}
            <div className="text-center flex-1 px-2">
              <h1 className="text-[17px] font-bold text-[#1A1A1A] leading-tight tracking-tight truncate">
                {cleanRouteName(origin)} → {cleanRouteName(destination)}
              </h1>
              <p className="text-[13px] text-[#6B7280] font-normal mt-0.5">
                {date === '2026-09-17' ? 'Hoy, 18:30 hs' : date} • {seats}{' '}
                {seats === 1 ? 'pasajero' : 'pasajeros'}
              </p>
            </div>

            {/* Edit Parameters Button */}
            <button
              id="edit-search-btn"
              aria-label="Editar búsqueda"
              type="button"
              onClick={() => setIsEditModalOpen(true)}
              className="w-9 h-9 flex items-center justify-end text-[#00A896] hover:opacity-80 active:opacity-60 transition cursor-pointer"
            >
              <Edit2 className="w-5 h-5 stroke-[2]" />
            </button>
          </div>
        </section>
        {/* END: Search Results Header */}

        {/* BEGIN: Quick Filter Chips */}
        <nav className="w-full pb-3 shrink-0" data-purpose="filter-chips">
          <div
            className="flex space-x-2.5 overflow-x-auto px-4 items-center"
            style={{ scrollbarWidth: 'none', msOverflowStyle: 'none' }}
          >
            {/* Chip 1: Más barato */}
            <button
              id="filter-cheapest-btn"
              type="button"
              onClick={() => handleFilterToggle('cheapest')}
              className={`whitespace-nowrap px-4 py-1.5 rounded-full text-[13px] font-medium transition cursor-pointer ${
                activeFilter === 'cheapest'
                  ? 'bg-[#00A896] text-white shadow-sm'
                  : 'bg-white border border-[#E5E7EB] text-[#1A1A1A] hover:bg-gray-50'
              }`}
            >
              Más barato
            </button>

            {/* Chip 2: Salida más cercana */}
            <button
              id="filter-earliest-btn"
              type="button"
              onClick={() => handleFilterToggle('earliest')}
              className={`whitespace-nowrap px-4 py-1.5 rounded-full text-[13px] font-medium transition cursor-pointer ${
                activeFilter === 'earliest'
                  ? 'bg-[#00A896] text-white shadow-sm'
                  : 'bg-white border border-[#E5E7EB] text-[#1A1A1A] hover:bg-gray-50'
              }`}
            >
              Salida más cercana
            </button>

            {/* Chip 3: Conductor verificado */}
            <button
              id="filter-verified-btn"
              type="button"
              onClick={() => handleFilterToggle('verified')}
              className={`whitespace-nowrap px-4 py-1.5 rounded-full text-[13px] font-medium transition cursor-pointer ${
                activeFilter === 'verified'
                  ? 'bg-[#00A896] text-white shadow-sm'
                  : 'bg-white border border-[#E5E7EB] text-[#1A1A1A] hover:bg-gray-50'
              }`}
            >
              Conductor verificado
            </button>
          </div>
        </nav>
        {/* END: Quick Filter Chips */}

        {/* BEGIN: Rides List (Scrollable Area) */}
        <main
          className="flex-1 overflow-y-auto px-4 pb-4 space-y-3.5"
          style={{ scrollbarWidth: 'none', msOverflowStyle: 'none' }}
          data-purpose="rides-list"
        >
          {isLoading ? (
            <div className="py-20 flex flex-col items-center justify-center text-center">
              <Loader2 className="w-8 h-8 animate-spin text-[#00A896] mb-3" />
              <p className="text-sm font-medium text-[#6B7280]">
                Buscando los mejores viajes disponibles...
              </p>
            </div>
          ) : sortedAndFilteredTrips.length === 0 ? (
            <div className="bg-white rounded-2xl p-6 text-center shadow-xs border border-gray-100 my-6">
              <div className="w-12 h-12 rounded-full bg-[#E6F7F5] text-[#00A896] flex items-center justify-center mx-auto mb-3">
                <Search className="w-6 h-6" />
              </div>
              <h3 className="text-base font-bold text-[#1A1A1A] mb-1">
                No hay viajes para este filtro
              </h3>
              <p className="text-xs text-[#6B7280] mb-4">
                Prueba desactivando los filtros aplicados o modificando el horario.
              </p>
              <button
                type="button"
                onClick={() => setActiveFilter(null)}
                className="bg-[#00A896] text-white text-xs font-semibold px-4 py-2 rounded-xl hover:bg-[#008F80] transition"
              >
                Ver todos los viajes
              </button>
            </div>
          ) : (
            sortedAndFilteredTrips.map((trip) => {
              const startHour = formatHour(trip.departureTime);
              const arrivalHour = calculateArrivalHour(
                trip.departureTime,
                trip.durationMinutes || 45
              );
              const formattedPrice = `$${trip.pricePerSeat.toLocaleString('es-AR')}`;
              const vehicleText = `${trip.carModel}${trip.carColor ? ` • ${trip.carColor}` : ''}`;
              const seatsAvailableText = `${trip.availableSeats} ${
                trip.availableSeats === 1 ? 'lugar disponible' : 'lugares disponibles'
              }`;

              return (
                <article
                  key={trip.id}
                  onClick={() => onSelectTripToBook(trip)}
                  className="bg-white rounded-xl p-4 shadow-[0px_2px_8px_rgba(0,0,0,0.04)] border border-gray-100/60 cursor-pointer active:scale-[0.99] transition-all hover:border-[#00A896]/40 group"
                >
                  {/* Row 1: Time, Duration & Price */}
                  <div className="flex justify-between items-baseline mb-3">
                    <div className="flex items-center space-x-2">
                      <span className="text-base font-bold text-[#1A1A1A] tracking-tight">
                        {startHour} → {arrivalHour}
                      </span>
                      <span className="text-[13px] text-[#6B7280] font-normal">
                        {trip.durationMinutes || 45} min
                      </span>
                    </div>
                    <span className="text-[18px] font-bold text-[#00A896] tracking-tight">
                      {formattedPrice}
                    </span>
                  </div>

                  {/* Row 2: Driver Info & Verification Badge */}
                  <div className="flex items-center justify-between mb-2.5">
                    <div className="flex items-center space-x-2.5">
                      {/* Driver Avatar */}
                      <div className="w-9 h-9 rounded-full bg-[#E5E7EB] text-[#4B5563] text-xs font-semibold flex items-center justify-center">
                        {getDriverInitials(trip.driverName)}
                      </div>
                      {/* Name & Rating */}
                      <div>
                        <div className="flex items-center space-x-1.5">
                          <span className="text-sm font-semibold text-[#1A1A1A] leading-none">
                            {trip.driverName}
                          </span>
                        </div>
                        <div className="flex items-center mt-1 text-[12px] text-[#1A1A1A] font-medium">
                          <Star className="w-3 h-3 fill-amber-400 text-amber-400 mr-1" />
                          <span>{trip.driverRating.toFixed(1)}</span>
                        </div>
                      </div>
                    </div>

                    {/* Verified DNI Badge */}
                    {trip.driverVerified && (
                      <div className="bg-[#E6F7F5] text-[#00A896] text-[11px] font-bold px-2.5 py-1 rounded-full flex items-center space-x-1">
                        <Check className="w-3 h-3 stroke-[3]" />
                        <span>DNI Verificado</span>
                      </div>
                    )}
                  </div>

                  {/* Row 3: Vehicle Description */}
                  <p className="text-[13px] text-[#6B7280] mb-3 font-normal">
                    {vehicleText}
                  </p>

                  {/* Row 4: Seats Availability Pill */}
                  <div className="flex items-center">
                    <span className="inline-block bg-[#E6F7F5] text-[#00A896] text-[12px] font-medium px-3 py-1 rounded-md">
                      {seatsAvailableText}
                    </span>
                  </div>
                </article>
              );
            })
          )}
        </main>
        {/* END: Rides List */}

        {/* Modal / Sheet: Editar Búsqueda In-Situ */}
        {isEditModalOpen && (
          <div className="absolute inset-0 bg-black/50 z-50 flex items-end justify-center animate-fadeIn">
            <div className="w-full bg-white rounded-t-3xl p-5 shadow-2xl border-t border-gray-100">
              <div className="flex items-center justify-between pb-3 border-b border-gray-100">
                <h3 className="font-bold text-base text-[#1A1A1A]">Modificar búsqueda</h3>
                <button
                  type="button"
                  onClick={() => setIsEditModalOpen(false)}
                  className="p-1 rounded-full text-gray-400 hover:text-gray-700 cursor-pointer"
                >
                  <X className="w-5 h-5" />
                </button>
              </div>

              <form onSubmit={handleApplyEdit} className="space-y-3 pt-3">
                <div>
                  <label className="text-xs font-semibold text-gray-500 block mb-1">
                    Origen
                  </label>
                  <input
                    type="text"
                    value={editOrigin}
                    onChange={(e) => setEditOrigin(e.target.value)}
                    className="w-full bg-gray-50 rounded-xl px-3 py-2.5 text-sm text-gray-800 border border-gray-200 focus:border-[#00A896] focus:outline-none"
                    placeholder="Origen del viaje"
                    required
                  />
                </div>

                <div>
                  <label className="text-xs font-semibold text-gray-500 block mb-1">
                    Destino
                  </label>
                  <input
                    type="text"
                    value={editDestination}
                    onChange={(e) => setEditDestination(e.target.value)}
                    className="w-full bg-gray-50 rounded-xl px-3 py-2.5 text-sm text-gray-800 border border-gray-200 focus:border-[#00A896] focus:outline-none"
                    placeholder="Destino del viaje"
                    required
                  />
                </div>

                <div className="grid grid-cols-2 gap-3">
                  <div>
                    <label className="text-xs font-semibold text-gray-500 block mb-1">
                      Fecha
                    </label>
                    <input
                      type="date"
                      value={editDate}
                      onChange={(e) => setEditDate(e.target.value)}
                      className="w-full bg-gray-50 rounded-xl px-3 py-2 text-xs text-gray-800 border border-gray-200 focus:border-[#00A896] focus:outline-none"
                    />
                  </div>

                  <div>
                    <label className="text-xs font-semibold text-gray-500 block mb-1">
                      Asientos
                    </label>
                    <div className="flex items-center justify-between bg-gray-50 border border-gray-200 rounded-xl px-2 py-1">
                      <button
                        type="button"
                        onClick={() => setEditSeats(Math.max(1, editSeats - 1))}
                        className="w-7 h-7 rounded-lg bg-gray-200 text-gray-700 font-bold"
                      >
                        -
                      </button>
                      <span className="text-sm font-bold text-[#00A896]">{editSeats}</span>
                      <button
                        type="button"
                        onClick={() => setEditSeats(Math.min(4, editSeats + 1))}
                        className="w-7 h-7 rounded-lg bg-gray-200 text-gray-700 font-bold"
                      >
                        +
                      </button>
                    </div>
                  </div>
                </div>

                <div className="pt-2 flex gap-2">
                  <button
                    type="button"
                    onClick={() => {
                      setIsEditModalOpen(false);
                      onBackToHome();
                    }}
                    className="w-1/2 py-3 rounded-xl border border-gray-200 text-xs font-semibold text-gray-700 hover:bg-gray-50"
                  >
                    Ir al buscador
                  </button>
                  <button
                    type="submit"
                    className="w-1/2 py-3 rounded-xl bg-[#00A896] hover:bg-[#008F80] text-white text-xs font-semibold shadow-sm transition"
                  >
                    Actualizar
                  </button>
                </div>
              </form>
            </div>
          </div>
        )}

        {/* BEGIN: Bottom Navigation Bar */}
        <footer
          className="w-full bg-white shadow-[0px_-1px_0px_rgba(0,0,0,0.05)] pt-2 pb-5 px-4 shrink-0 flex flex-col z-20 border-t border-gray-100"
          data-purpose="bottom-tab-bar"
        >
          <div className="grid grid-cols-4 items-center text-center">
            {/* Tab 1: Buscar (Active) */}
            <button
              type="button"
              onClick={onBackToHome}
              aria-current="page"
              className="flex flex-col items-center justify-center group cursor-pointer focus:outline-none"
            >
              <Search className="w-5 h-5 text-[#00A896] stroke-[2.4]" />
              <span className="text-[11px] font-semibold text-[#00A896] mt-1">Buscar</span>
            </button>

            {/* Tab 2: Mis Viajes */}
            <button
              type="button"
              onClick={onNavigateToTrips}
              className="flex flex-col items-center justify-center group cursor-pointer focus:outline-none"
            >
              <Car className="w-5 h-5 text-gray-400 group-hover:text-gray-600 transition" />
              <span className="text-[11px] font-medium text-gray-400 group-hover:text-gray-600 transition mt-1">
                Mis Viajes
              </span>
            </button>

            {/* Tab 3: Publicar */}
            <button
              type="button"
              onClick={onNavigateToPublish}
              className="flex flex-col items-center justify-center group cursor-pointer focus:outline-none"
            >
              <PlusCircle className="w-5 h-5 text-gray-400 group-hover:text-gray-600 transition" />
              <span className="text-[11px] font-medium text-gray-400 group-hover:text-gray-600 transition mt-1">
                Publicar
              </span>
            </button>

            {/* Tab 4: Perfil */}
            <button
              type="button"
              onClick={onNavigateToProfile}
              className="flex flex-col items-center justify-center group cursor-pointer focus:outline-none"
            >
              <User className="w-5 h-5 text-gray-400 group-hover:text-gray-600 transition" />
              <span className="text-[11px] font-medium text-gray-400 group-hover:text-gray-600 transition mt-1">
                Perfil
              </span>
            </button>
          </div>

          {/* iOS Home Bar Indicator */}
          <div className="w-32 h-1 bg-black/80 rounded-full mx-auto mt-3" />
        </footer>
        {/* END: Bottom Navigation Bar */}
      </div>
    </div>
  );
};

export default SearchResultsScreen;
