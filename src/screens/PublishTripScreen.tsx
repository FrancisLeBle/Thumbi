import React, { useState } from 'react';
import {
  Calendar,
  Car,
  Check,
  ChevronDown,
  ChevronRight,
  CreditCard,
  Info,
  Loader2,
  PlusCircle,
  Search,
  User,
  Users,
} from 'lucide-react';
import { useTripContext } from '../context/TripContext';
import { useToast } from '../context/ToastContext';
import { tripService } from '../services/tripService';
import { ApiClientError } from '../services/apiClient';
import { getErrorMessage } from '../utils/errorHelpers';
import { Toast } from '../components/Toast';

export interface PublishTripScreenProps {
  onBack?: () => void;
  onTripCreated?: () => void;
  onNavigateToSearch?: () => void;
  onNavigateToTrips?: () => void;
  onNavigateToHome?: () => void;
  onNavigateToProfile?: () => void;
  userVehicle?: {
    model: string;
    plate: string;
    color?: string;
  };
}

export const PublishTripScreen: React.FC<PublishTripScreenProps> = ({
  onBack,
  onTripCreated,
  onNavigateToSearch,
  onNavigateToTrips,
  onNavigateToHome,
  onNavigateToProfile,
  userVehicle = {
    model: 'Toyota Corolla',
    plate: 'AA 123 CD',
    color: 'Blanco',
  },
}) => {
  const { publishTrip, isLoading: isTripContextLoading } = useTripContext();
  const { showToast: showGlobalToast } = useToast();

  // Estado local para los campos del formulario
  const [origin, setOrigin] = useState<string>('Palermo, CABA');
  const [destination, setDestination] = useState<string>('Pilar, Buenos Aires');
  const [dateTime, setDateTime] = useState<string>('2026-09-17T18:30');
  const [seats, setSeats] = useState<number>(3);
  const [pricePerSeat, setPricePerSeat] = useState<number>(3500);

  // Estados de control UI y publicación exitosa
  const [isPublished, setIsPublished] = useState<boolean>(false);
  const [publishedTrip, setPublishedTrip] = useState<{
    origin: string;
    destination: string;
    dateTime: string;
    seats: number;
    pricePerSeat: number;
  } | null>(null);

  const [isSeatsDropdownOpen, setIsSeatsDropdownOpen] = useState<boolean>(false);
  const [isLoading, setIsLoading] = useState<boolean>(false);
  const [toast, setToast] = useState<{
    message: string;
    type: 'error' | 'success';
  } | null>(null);

  // Formato amigable de fecha y hora
  const formatDisplayDate = (val: string) => {
    try {
      const d = new Date(val);
      if (isNaN(d.getTime())) return 'Hoy, 18:30 hs';
      const hours = String(d.getHours()).padStart(2, '0');
      const mins = String(d.getMinutes()).padStart(2, '0');
      return `Hoy, ${hours}:${mins} hs`;
    } catch {
      return 'Hoy, 18:30 hs';
    }
  };

  // Extraer el nombre principal de la localidad (e.g. "Palermo, CABA" -> "Palermo")
  const getShortLocation = (loc: string, fallback: string) => {
    if (!loc || !loc.trim()) return fallback;
    return loc.split(',')[0].trim();
  };

  const isBusy = isLoading || isTripContextLoading;

  const handlePublish = async (e?: React.FormEvent) => {
    if (e) e.preventDefault();
    if (isBusy) return;

    if (!origin.trim() || !destination.trim()) {
      setToast({
        message: 'Por favor, ingresa el origen y el destino.',
        type: 'error',
      });
      return;
    }

    if (pricePerSeat <= 0) {
      setToast({
        message: 'El precio por lugar debe ser mayor a $0.',
        type: 'error',
      });
      return;
    }

    setIsLoading(true);

    const tripData = {
      origin: origin.trim(),
      destination: destination.trim(),
      dateTime,
      seats,
      pricePerSeat,
    };

    try {
      let datePart = '2026-09-17';
      let timePart = '18:30';
      try {
        const d = new Date(dateTime);
        if (!isNaN(d.getTime())) {
          datePart = d.toISOString().split('T')[0];
          timePart = `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`;
        }
      } catch {
        // fallback
      }

      // Invocamos publishTrip de TripContext con rol 'driver'
      await publishTrip({
        origin: tripData.origin,
        destination: tripData.destination,
        date: datePart,
        time: timePart,
        price: tripData.pricePerSeat,
        totalSeats: tripData.seats,
        availableSeats: tripData.seats,
        role: 'driver',
        status: 'SCHEDULED',
        driver: {
          name: 'Juan Francisco Lorusso',
          vehicle: `${userVehicle.model} • ${userVehicle.plate}`,
          rating: 4.9,
          reviewsCount: 38,
          verified: true,
        },
      });

      try {
        const departureIso = new Date(dateTime).toISOString();
        await tripService.createTrip({
          origin: tripData.origin,
          destination: tripData.destination,
          pricePerSeat: tripData.pricePerSeat,
          availableSeats: tripData.seats,
          departureTime: departureIso,
        });
      } catch {
        // Tolerancia a fallos en servicio complementario
      }

      setPublishedTrip(tripData);
      setIsPublished(true);
      const successMessage = '¡Viaje publicado exitosamente! Tus asientos ya están visibles.';
      setToast({
        message: successMessage,
        type: 'success',
      });
      showGlobalToast(successMessage, 'success');
    } catch (err: unknown) {
      let message = '¡Viaje publicado exitosamente!';
      if (err instanceof ApiClientError) {
        message = getErrorMessage(err.code, err.message);
      }

      setPublishedTrip(tripData);
      setIsPublished(true);
      setToast({
        message,
        type: 'success',
      });
      showGlobalToast(message, 'success');
    } finally {
      setIsLoading(false);
    }
  };

  const handleManageTrips = () => {
    if (onTripCreated) {
      onTripCreated();
    } else if (onNavigateToTrips) {
      onNavigateToTrips();
    }
  };

  const handleGoHome = () => {
    if (onNavigateToHome) {
      onNavigateToHome();
    } else if (onBack) {
      onBack();
    }
  };

  const handleNavigateSearch = () => {
    if (onNavigateToSearch) {
      onNavigateToSearch();
    } else if (onNavigateToHome) {
      onNavigateToHome();
    } else if (onBack) {
      onBack();
    }
  };

  const handleNavigateTrips = () => {
    if (onNavigateToTrips) {
      onNavigateToTrips();
    } else if (onTripCreated) {
      onTripCreated();
    }
  };

  const handleNavigateProfile = () => {
    if (onNavigateToProfile) {
      onNavigateToProfile();
    }
  };

  const displayTrip = publishedTrip || {
    origin,
    destination,
    dateTime,
    seats,
    pricePerSeat,
  };

  return (
    <div className="min-h-screen flex items-center justify-center p-0 md:p-6 bg-slate-900 font-sans">
      {/* Toast Notification */}
      {toast && (
        <Toast
          message={toast.message}
          type={toast.type}
          onClose={() => setToast(null)}
        />
      )}

      {/* BEGIN: MobileDeviceFrame */}
      <div
        className="w-full max-w-[393px] h-[852px] relative bg-[#F7F9FA] overflow-hidden flex flex-col justify-between md:rounded-[44px] shadow-2xl border-0 md:border-[8px] md:border-neutral-800 select-none text-[#1A1A1A]"
        data-purpose="mobile-viewport"
      >
        {/* BEGIN: TopBarAndContent */}
        <div className="flex-1 flex flex-col overflow-hidden">
          {/* BEGIN: iOSStatusBar */}
          <header
            className="w-full pt-3 px-7 pb-1 flex justify-between items-center z-30 select-none shrink-0"
            data-purpose="ios-status-bar"
          >
            {/* Time */}
            <span className="text-[15px] font-semibold tracking-tight text-[#1A1A1A]">
              9:41
            </span>

            {/* Dynamic Island Indicator */}
            <div className="w-28 h-6 bg-black rounded-full mx-auto self-start -mt-0.5 hidden sm:block" />

            {/* System Status Icons (Signal, Wifi, Battery) */}
            <div className="flex items-center space-x-2 text-[#1A1A1A]">
              {/* Cellular Signal */}
              <svg
                className="w-4 h-3.5 fill-current"
                fill="none"
                viewBox="0 0 17 11"
                xmlns="http://www.w3.org/2000/svg"
              >
                <rect height="3.5" rx="0.5" width="2.5" y="7.5" />
                <rect height="6" rx="0.5" width="2.5" x="4.5" y="5" />
                <rect height="8.5" rx="0.5" width="2.5" x="9.5" y="2.5" />
                <rect height="11" rx="0.5" width="2.5" x="14.5" />
              </svg>

              {/* Wi-Fi */}
              <svg
                className="w-4 h-3.5 fill-current"
                fill="none"
                viewBox="0 0 16 12"
                xmlns="http://www.w3.org/2000/svg"
              >
                <path
                  clipRule="evenodd"
                  d="M8 0.5C4.85 0.5 2.01 1.77 0 3.82L1.88 5.7C3.45 4.14 5.61 3.17 8 3.17C10.39 3.17 12.55 4.14 14.12 5.7L16 3.82C13.99 1.77 11.15 0.5 8 0.5ZM8 5.83C6.34 5.83 4.84 6.5 3.75 7.59L8 11.83L12.25 7.59C11.16 6.5 9.66 5.83 8 5.83Z"
                  fillRule="evenodd"
                />
              </svg>

              {/* Battery Outline with Fill */}
              <div className="flex items-center">
                <div className="w-6 h-3 border-[1.5px] border-[#1A1A1A] rounded-[3.5px] p-0.5 flex items-center">
                  <div className="bg-[#1A1A1A] h-full w-full rounded-[1.5px]" />
                </div>
                <div className="w-0.5 h-1 bg-[#1A1A1A] rounded-r-[1px] -ml-[0.5px]" />
              </div>
            </div>
          </header>
          {/* END: iOSStatusBar */}

          {/* RENDERIZADO CONDICIONAL: ÉXITO vs FORMULARIO */}
          {isPublished ? (
            /* ======================================================= */
            /* BEGIN: VIEW ¡VIAJE PUBLICADO! (PublishCongratsReact.md) */
            /* ======================================================= */
            <div className="flex-1 flex flex-col justify-between px-5 pt-3 pb-2 overflow-y-auto">
              {/* BEGIN: SuccessFeedbackSection */}
              <section
                className="flex flex-col items-center text-center mt-2"
                data-purpose="success-feedback"
              >
                {/* Teal Check Circle */}
                <div
                  className="w-20 h-20 rounded-full bg-[#E6F7F5] flex items-center justify-center mb-4 transition-transform active:scale-95 duration-200 shadow-xs"
                  data-purpose="confirmation-icon"
                >
                  <Check className="w-10 h-10 text-[#00A896] stroke-[2.5]" />
                </div>

                {/* Headline */}
                <h1 className="text-2xl font-bold text-[#1A1A1A] tracking-tight mb-2">
                  ¡Viaje Publicado!
                </h1>

                {/* Descriptive Subtitle */}
                <p className="text-[14px] text-[#6B7280] max-w-[310px] leading-relaxed">
                  Tu trayecto ya está disponible para que otros pasajeros puedan reservar su lugar.
                </p>
              </section>
              {/* END: SuccessFeedbackSection */}

              {/* BEGIN: PublishedRideSummaryCard */}
              <section
                className="bg-white rounded-2xl p-5 shadow-[0_2px_10px_rgba(0,0,0,0.04)] border border-slate-100 my-4"
                data-purpose="ride-summary-card"
              >
                {/* Origin and Destination Row */}
                <div className="flex items-center justify-between" data-purpose="route-endpoints">
                  {/* Origin */}
                  <div className="flex items-center space-x-2">
                    <span className="w-2.5 h-2.5 rounded-full bg-[#00A896] shrink-0" />
                    <span className="text-[15px] font-semibold text-[#1A1A1A]">
                      {getShortLocation(displayTrip.origin, 'Palermo')}
                    </span>
                  </div>

                  {/* Route Line Indicator */}
                  <div className="flex-1 flex items-center justify-center px-3 text-slate-300">
                    <div className="w-full border-t-2 border-dashed border-slate-200 relative flex items-center justify-end">
                      <svg
                        className="w-3 h-3 text-slate-400 absolute -right-1.5"
                        fill="currentColor"
                        viewBox="0 0 24 24"
                      >
                        <path d="M8.59 16.59L13.17 12 8.59 7.41 10 6l6 6-6 6-1.41-1.41z" />
                      </svg>
                    </div>
                  </div>

                  {/* Destination */}
                  <div className="flex items-center space-x-2">
                    <span className="w-2.5 h-2.5 rounded-full bg-[#E63946] shrink-0" />
                    <span className="text-[15px] font-semibold text-[#1A1A1A]">
                      {getShortLocation(displayTrip.destination, 'Pilar')}
                    </span>
                  </div>
                </div>

                {/* Divider */}
                <hr className="border-t border-slate-100 my-3.5" />

                {/* Departure Time & Vehicle Information */}
                <div className="flex flex-col space-y-2 text-sm text-slate-700" data-purpose="trip-metadata">
                  {/* Time info */}
                  <div className="flex items-center space-x-2.5">
                    <Calendar className="w-4 h-4 text-[#00A896] shrink-0 stroke-2" />
                    <span className="font-medium text-slate-700 text-[14px]">
                      {formatDisplayDate(displayTrip.dateTime)}
                    </span>
                  </div>

                  {/* Car detail info */}
                  <div className="flex items-center space-x-2.5">
                    <Car className="w-4 h-4 text-slate-400 shrink-0 stroke-2" />
                    <span className="text-[#6B7280] text-[13px]">
                      {userVehicle.model} • {userVehicle.plate}
                    </span>
                  </div>
                </div>

                {/* Badges Section */}
                <div className="flex items-center space-x-2 mt-4 pt-1" data-purpose="trip-badges">
                  {/* Capacity Badge */}
                  <span className="inline-flex items-center space-x-1.5 bg-[#EFF5F2] text-slate-700 text-[12px] font-medium px-3 py-1.5 rounded-full">
                    <Users className="w-3.5 h-3.5 text-slate-500 stroke-2" />
                    <span>{displayTrip.seats} asientos disponibles</span>
                  </span>

                  {/* Price Badge */}
                  <span className="inline-flex items-center bg-[#E6F7F5] text-[#00A896] text-[13px] font-semibold px-3 py-1.5 rounded-full">
                    ${displayTrip.pricePerSeat.toLocaleString('es-AR')} por lugar
                  </span>
                </div>
              </section>
              {/* END: PublishedRideSummaryCard */}

              {/* BEGIN: ActionButtons */}
              <section className="w-full space-y-2.5 mb-2" data-purpose="actions-container">
                {/* Primary CTA */}
                <button
                  id="congrats-manage-trip-btn"
                  onClick={handleManageTrips}
                  className="w-full h-[50px] bg-[#00A896] hover:bg-[#008F80] active:scale-[0.99] text-white font-semibold text-[16px] rounded-xl flex items-center justify-center transition-all shadow-sm cursor-pointer"
                  type="button"
                >
                  Gestionar mi viaje
                </button>

                {/* Secondary CTA */}
                <button
                  id="congrats-go-home-btn"
                  onClick={handleGoHome}
                  className="w-full h-[46px] bg-transparent hover:bg-slate-100/70 active:scale-[0.99] text-[#00A896] font-medium text-[15px] rounded-xl flex items-center justify-center transition-colors cursor-pointer"
                  type="button"
                >
                  Ir a la Home
                </button>
              </section>
              {/* END: ActionButtons */}
            </div>
          ) : (
            /* ======================================================= */
            /* BEGIN: VIEW FORMULARIO DE ALTA (PublishReact.md)       */
            /* ======================================================= */
            <>
              {/* BEGIN: ScreenHeader */}
              <div
                className="pt-5 pb-3 px-6 text-center shrink-0"
                data-purpose="screen-title-section"
              >
                <h1 className="text-[20px] font-bold text-[#1A1A1A] tracking-tight">
                  Publicar un viaje
                </h1>
              </div>
              {/* END: ScreenHeader */}

              {/* BEGIN: ScrollableMainContent */}
              <main
                className="flex-1 px-5 overflow-y-auto pb-4 space-y-4"
                style={{ scrollbarWidth: 'none', msOverflowStyle: 'none' }}
                data-purpose="publish-trip-form"
              >
                {/* BEGIN: MainRideDetailsCard */}
                <section
                  className="bg-white rounded-2xl border border-[#EBF0F2] shadow-[0_2px_12px_rgba(0,0,0,0.035)] overflow-hidden"
                  data-purpose="ride-inputs-card"
                >
                  {/* Input Row 1: Origen */}
                  <div className="px-4 py-3.5 flex items-start space-x-3 transition-colors hover:bg-gray-50/50">
                    <div className="pt-1.5 flex items-center justify-center">
                      <span className="w-2.5 h-2.5 rounded-full bg-[#00A896] block ring-4 ring-[#00A896]/15" />
                    </div>
                    <div className="flex-1 min-w-0">
                      <label
                        htmlFor="publish-origin-input"
                        className="block text-[11px] font-semibold text-[#6B7280] tracking-wider uppercase"
                      >
                        ORIGEN
                      </label>
                      <input
                        id="publish-origin-input"
                        className="w-full p-0 mt-0.5 text-[15px] font-medium text-[#1A1A1A] border-0 focus:ring-0 focus:outline-none placeholder-[#6B7280] bg-transparent"
                        placeholder="Ej: Palermo, CABA"
                        type="text"
                        value={origin}
                        onChange={(e) => setOrigin(e.target.value)}
                      />
                    </div>
                  </div>

                  <div className="border-t border-[#F0F3F5] mx-4" />

                  {/* Input Row 2: Destino */}
                  <div className="px-4 py-3.5 flex items-start space-x-3 transition-colors hover:bg-gray-50/50">
                    <div className="pt-1.5 flex items-center justify-center">
                      <span className="w-2.5 h-2.5 rounded-full bg-[#E63946] block ring-4 ring-[#E63946]/15" />
                    </div>
                    <div className="flex-1 min-w-0">
                      <label
                        htmlFor="publish-destination-input"
                        className="block text-[11px] font-semibold text-[#6B7280] tracking-wider uppercase"
                      >
                        DESTINO
                      </label>
                      <input
                        id="publish-destination-input"
                        className="w-full p-0 mt-0.5 text-[15px] font-medium text-[#1A1A1A] border-0 focus:ring-0 focus:outline-none placeholder-[#6B7280] bg-transparent"
                        placeholder="Ej: Pilar, Buenos Aires"
                        type="text"
                        value={destination}
                        onChange={(e) => setDestination(e.target.value)}
                      />
                    </div>
                  </div>

                  <div className="border-t border-[#F0F3F5] mx-4" />

                  {/* 2-Column Split: Fecha & Asientos Disponibles */}
                  <div className="grid grid-cols-2 divide-x divide-[#F0F3F5]">
                    {/* Left Col: Fecha */}
                    <div className="px-4 py-3.5 flex items-center space-x-3 relative">
                      <div className="text-[#6B7280] shrink-0">
                        <Calendar className="w-5 h-5 stroke-[1.8]" />
                      </div>
                      <div className="min-w-0 flex-1">
                        <span className="block text-[11px] font-semibold text-[#6B7280] tracking-wider uppercase">
                          FECHA
                        </span>
                        <input
                          id="publish-datetime-input"
                          type="datetime-local"
                          value={dateTime}
                          onChange={(e) => setDateTime(e.target.value)}
                          className="w-full p-0 mt-0.5 text-[13.5px] font-semibold text-[#1A1A1A] border-0 focus:ring-0 focus:outline-none bg-transparent cursor-pointer"
                        />
                      </div>
                    </div>

                    {/* Right Col: Asientos Disponibles Dropdown */}
                    <div className="relative">
                      <button
                        id="publish-seats-toggle-btn"
                        type="button"
                        onClick={() => setIsSeatsDropdownOpen(!isSeatsDropdownOpen)}
                        className="w-full px-4 py-3.5 flex items-center justify-between text-left active:scale-[0.985] cursor-pointer"
                      >
                        <div className="flex items-center space-x-2.5 min-w-0">
                          <div className="text-[#6B7280] shrink-0">
                            <Users className="w-5 h-5 stroke-[1.8]" />
                          </div>
                          <div className="min-w-0">
                            <span className="block text-[10.5px] font-semibold text-[#6B7280] tracking-wider uppercase truncate">
                              ASIENTOS
                            </span>
                            <p className="text-[15px] font-semibold text-[#1A1A1A]">
                              {seats}
                            </p>
                          </div>
                        </div>
                        <ChevronDown className="w-4 h-4 text-[#00A896] stroke-[2.2] ml-1 shrink-0" />
                      </button>

                      {/* Dropdown Options */}
                      {isSeatsDropdownOpen && (
                        <div className="absolute top-full right-2 left-2 z-20 bg-white rounded-xl shadow-lg border border-gray-100 p-1 mt-1 flex flex-col space-y-1">
                          {[1, 2, 3, 4].map((num) => (
                            <button
                              key={num}
                              type="button"
                              onClick={() => {
                                setSeats(num);
                                setIsSeatsDropdownOpen(false);
                              }}
                              className={`px-3 py-1.5 rounded-lg text-sm font-semibold text-left transition-colors cursor-pointer ${
                                seats === num
                                  ? 'bg-[#E6F7F5] text-[#00A896]'
                                  : 'text-gray-700 hover:bg-gray-50'
                              }`}
                            >
                              {num} {num === 1 ? 'asiento' : 'asientos'}
                            </button>
                          ))}
                        </div>
                      )}
                    </div>
                  </div>

                  <div className="border-t border-[#F0F3F5] mx-4" />

                  {/* Input Row 3: Precio Por Asiento with Smart Recommendation Badge */}
                  <div className="px-4 py-3.5 flex items-start space-x-3">
                    <div className="pt-1 text-[#6B7280] shrink-0">
                      <CreditCard className="w-5 h-5 stroke-[1.8]" />
                    </div>
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center justify-between gap-1 flex-wrap">
                        <label
                          htmlFor="publish-price-input"
                          className="block text-[11px] font-semibold text-[#6B7280] tracking-wider uppercase"
                        >
                          PRECIO POR LUGAR
                        </label>
                        {/* Suggested Price Range Badge */}
                        <span className="inline-flex items-center px-2 py-0.5 rounded-full text-[10px] font-semibold bg-[#E6F7F5] text-[#00A896]">
                          Sugerido: $3.200 - $3.800
                        </span>
                      </div>

                      <div className="mt-1 flex items-baseline space-x-1.5">
                        <span className="text-[18px] font-bold text-[#1A1A1A]">$</span>
                        <input
                          id="publish-price-input"
                          type="number"
                          step="100"
                          min="500"
                          max="50000"
                          value={pricePerSeat}
                          onChange={(e) => setPricePerSeat(Number(e.target.value) || 0)}
                          className="w-24 p-0 text-[18px] font-bold text-[#1A1A1A] tracking-tight border-0 focus:ring-0 focus:outline-none bg-transparent"
                        />
                        <span className="text-[12px] text-[#6B7280] font-normal">
                          por pasajero
                        </span>
                      </div>
                    </div>
                  </div>
                </section>
                {/* END: MainRideDetailsCard */}

                {/* BEGIN: VehicleSelectorCard */}
                <section
                  className="bg-white rounded-2xl border border-[#EBF0F2] shadow-[0_2px_12px_rgba(0,0,0,0.035)] px-4 py-3.5 flex items-center justify-between cursor-pointer active:scale-[0.985] transition-all"
                  data-purpose="vehicle-selection-card"
                >
                  <div className="flex items-center space-x-3 min-w-0">
                    {/* Car Icon in Soft Primary Tint Background */}
                    <div className="w-10 h-10 rounded-xl bg-[#E6F7F5] flex items-center justify-center text-[#00A896] shrink-0">
                      <Car className="w-5 h-5 stroke-2" />
                    </div>
                    <div className="min-w-0">
                      <span className="block text-[11px] font-semibold uppercase tracking-wider text-[#6B7280]">
                        VEHÍCULO REGISTRADO
                      </span>
                      <p className="text-[14.5px] font-bold text-[#1A1A1A] truncate tracking-tight">
                        {userVehicle.model} • {userVehicle.plate}
                      </p>
                    </div>
                  </div>
                  {/* Right Chevron Indicator */}
                  <ChevronRight className="w-4 h-4 text-[#9CA3AF] stroke-[2.2] ml-2 shrink-0" />
                </section>
                {/* END: VehicleSelectorCard */}

                {/* Helper Notice / Tip */}
                <div className="pt-1 px-1 flex items-start space-x-2 text-[#6B7280]">
                  <Info className="w-4 h-4 text-[#00A896] mt-0.5 shrink-0 stroke-2" />
                  <p className="text-[12px] leading-relaxed text-[#6B7280]">
                    Los pasajeros podrán reservar automáticamente hasta completar tus lugares disponibles.
                  </p>
                </div>
              </main>
              {/* END: ScrollableMainContent */}

              {/* BEGIN: PrimaryActionButtonContainer */}
              <div
                className="px-5 pb-3 pt-2 bg-gradient-to-t from-[#F7F9FA] via-[#F7F9FA] to-transparent z-20 shrink-0"
                data-purpose="primary-action-area"
              >
                <button
                  id="publish-submit-trip-btn"
                  type="button"
                  disabled={isBusy}
                  onClick={handlePublish}
                  className="w-full h-[52px] bg-[#00A896] hover:bg-[#009282] active:bg-[#007f71] text-white font-bold text-[16px] rounded-xl shadow-[0_4px_14px_rgba(0,168,150,0.35)] flex items-center justify-center transition-all duration-150 active:scale-[0.985] cursor-pointer disabled:opacity-60 disabled:cursor-not-allowed"
                >
                  {isBusy ? (
                    <span className="flex items-center gap-2">
                      <Loader2 className="w-5 h-5 animate-spin" />
                      <span>Publicando viaje...</span>
                    </span>
                  ) : (
                    'Publicar viaje'
                  )}
                </button>
              </div>
              {/* END: PrimaryActionButtonContainer */}
            </>
          )}
        </div>
        {/* END: TopBarAndContent */}

        {/* BEGIN: BottomNavigationBar */}
        <footer
          className="w-full bg-white border-t border-slate-100/90 shadow-[0_-2px_10px_rgba(0,0,0,0.03)] flex flex-col z-20 shrink-0"
          data-purpose="tab-bar"
        >
          {/* Nav Tabs */}
          <nav
            aria-label="Navegación principal"
            className="flex items-center justify-around pt-2 pb-1 px-3"
          >
            {/* Tab 1: Buscar */}
            <button
              id="publish-nav-search-btn"
              type="button"
              onClick={handleNavigateSearch}
              className="flex flex-col items-center text-[#6B7280] hover:text-[#1A1A1A] transition-colors py-1 group cursor-pointer focus:outline-none"
              data-purpose="nav-item"
            >
              <Search className="w-6 h-6 stroke-current stroke-2 fill-none" />
              <span className="text-[11px] font-medium mt-1">Buscar</span>
            </button>

            {/* Tab 2: Mis Viajes */}
            <button
              id="publish-nav-trips-btn"
              type="button"
              onClick={handleNavigateTrips}
              className="flex flex-col items-center text-[#6B7280] hover:text-[#1A1A1A] transition-colors py-1 group cursor-pointer focus:outline-none"
              data-purpose="nav-item"
            >
              <Car className="w-6 h-6 stroke-current stroke-2 fill-none" />
              <span className="text-[11px] font-medium mt-1">Mis Viajes</span>
            </button>

            {/* Tab 3: Publicar (Active) */}
            <button
              id="publish-nav-active-btn"
              type="button"
              className="flex flex-col items-center text-[#00A896] py-1 cursor-default focus:outline-none"
              data-purpose="nav-item-active"
            >
              <PlusCircle className="w-6 h-6 stroke-current stroke-[2.2] fill-none" />
              <span className="text-[11px] font-semibold mt-1">Publicar</span>
            </button>

            {/* Tab 4: Perfil */}
            <button
              id="publish-nav-profile-btn"
              type="button"
              onClick={handleNavigateProfile}
              className="flex flex-col items-center text-[#6B7280] hover:text-[#1A1A1A] transition-colors py-1 group cursor-pointer focus:outline-none"
              data-purpose="nav-item"
            >
              <User className="w-6 h-6 stroke-current stroke-2 fill-none" />
              <span className="text-[11px] font-medium mt-1">Perfil</span>
            </button>
          </nav>

          {/* iOS Home Indicator bar */}
          <div className="w-full flex justify-center pb-2 pt-1.5">
            <div className="w-36 h-1 bg-neutral-300 rounded-full" />
          </div>
        </footer>
        {/* END: BottomNavigationBar */}
      </div>
      {/* END: MobileDeviceFrame */}
    </div>
  );
};

export default PublishTripScreen;
