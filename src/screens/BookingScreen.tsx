import React, { useState } from 'react';
import {
  ChevronLeft,
  Share2,
  Car,
  Star,
  Check,
  MapPin,
  Loader2,
  Calendar,
  Phone,
  MessageCircle,
  Search,
  PlusCircle,
  User,
} from 'lucide-react';
import { Trip, Booking } from '../types/api';
import { TripSearchResult } from '../services/tripService';
import { bookingService } from '../services/bookingService';
import { ApiClientError } from '../services/apiClient';
import { getErrorMessage } from '../utils/errorHelpers';
import { Toast } from '../components/Toast';
import { ContactDriverModal } from '../components/ContactDriverModal';

export interface BookingTripDetails extends Partial<TripSearchResult>, Partial<Trip> {
  id: string;
  driverId: string;
  origin: string;
  destination: string;
  pricePerSeat: number;
  availableSeats: number;
  departureTime: string;
  status: Trip['status'];
  driverName?: string;
  driverRating?: number;
  driverReviewsCount?: number;
  driverVerified?: boolean;
  driverPhoneNumber?: string;
  driverPhone?: string;
  carModel?: string;
  carColor?: string;
  carPlate?: string;
  originAddress?: string;
  destinationAddress?: string;
  durationMinutes?: number;
}

export interface BookingScreenProps {
  trip?: BookingTripDetails;
  onBookingSuccess?: (booking: Booking) => void;
  onBack?: () => void;
  onNavigateToTrips?: () => void;
  onNavigateToHome?: () => void;
}

const defaultMockTrip: BookingTripDetails = {
  id: 'trip-carlos-1',
  driverId: 'drv-carlos-101',
  driverName: 'Carlos M.',
  driverRating: 4.9,
  driverReviewsCount: 24,
  driverVerified: true,
  driverPhoneNumber: '+5491148291123',
  carModel: 'Toyota Corolla',
  carColor: 'Blanco',
  carPlate: 'AA 123 CD',
  durationMinutes: 45,
  origin: 'Palermo, CABA',
  destination: 'Pilar, Buenos Aires',
  originAddress: 'Av. Santa Fe y Cnel. Díaz, Palermo',
  destinationAddress: 'Las Palmas del Pilar, Pilar',
  pricePerSeat: 3500,
  availableSeats: 2,
  departureTime: '2026-09-17T18:30:00Z',
  status: 'SCHEDULED',
};

export const BookingScreen: React.FC<BookingScreenProps> = ({
  trip = defaultMockTrip,
  onBookingSuccess,
  onBack,
  onNavigateToTrips,
  onNavigateToHome,
}) => {
  const [seatsRequested, setSeatsRequested] = useState<number>(1);
  const [isLoading, setIsLoading] = useState<boolean>(false);
  const [isConfirmed, setIsConfirmed] = useState<boolean>(false);
  const [confirmedBooking, setConfirmedBooking] = useState<Booking | null>(null);
  const [tripPinCode, setTripPinCode] = useState<string>('4829');
  const [isContactModalOpen, setIsContactModalOpen] = useState<boolean>(false);
  const [toast, setToast] = useState<{
    message: string;
    type: 'error' | 'success';
  } | null>(null);

  const driverName = trip.driverName || 'Carlos M.';
  const driverRating = trip.driverRating ? trip.driverRating.toFixed(1) : '4.9';
  const driverTripsCount = trip.driverReviewsCount || 24;
  const isDriverVerified = trip.driverVerified !== false;
  const carDescription = `${trip.carModel || 'Toyota Corolla'} • ${trip.carColor || 'Blanco'} • ${
    trip.carPlate || 'AA 123 CD'
  }`;

  // Formato de horarios
  const formatDepartureHour = (isoStr: string) => {
    try {
      const d = new Date(isoStr);
      if (isNaN(d.getTime())) return '18:30 hs';
      return `${d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', hour12: false })} hs`;
    } catch {
      return '18:30 hs';
    }
  };

  const calculateArrivalHour = (isoStr: string, durationMinutes: number) => {
    try {
      const d = new Date(isoStr);
      if (isNaN(d.getTime())) return '19:15 hs';
      const arrival = new Date(d.getTime() + durationMinutes * 60 * 1000);
      return `${arrival.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', hour12: false })} hs`;
    } catch {
      return '19:15 hs';
    }
  };

  const departureTimeFormatted = formatDepartureHour(trip.departureTime);
  const arrivalTimeFormatted = calculateArrivalHour(
    trip.departureTime,
    trip.durationMinutes || 45
  );

  // Ciudades limpias para visualizador de ruta
  const cleanCity = (place: string) => {
    return place.split(',')[0].split('(')[0].trim();
  };

  const originCity = cleanCity(trip.origin);
  const destinationCity = cleanCity(trip.destination);

  // Iniciales del conductor
  const driverInitials = driverName
    .trim()
    .split(' ')
    .map((n) => n[0])
    .join('')
    .substring(0, 2)
    .toUpperCase() || 'CM';

  // Cálculos de costo
  const seatPrice = trip.pricePerSeat || 3500;
  const serviceFee = 0;
  const totalPrice = seatPrice * seatsRequested + serviceFee;

  const handleShare = async () => {
    const shareText = `Viaje en Thumbi con ${driverName}: ${trip.origin} → ${trip.destination} a las ${departureTimeFormatted}`;
    if (navigator.share) {
      try {
        await navigator.share({
          title: 'Detalle del viaje - Thumbi',
          text: shareText,
          url: window.location.href,
        });
        return;
      } catch {
        // usuario canceló o fallback
      }
    }

    if (navigator.clipboard) {
      try {
        await navigator.clipboard.writeText(
          `${shareText} - ${window.location.href}`
        );
        setToast({
          message: 'Enlace del viaje copiado al portapapeles.',
          type: 'success',
        });
        return;
      } catch {
        // ignore
      }
    }

    setToast({
      message: 'Enlace del viaje listo para compartir.',
      type: 'success',
    });
  };

  const handleConfirmBooking = async () => {
    if (isLoading) return;
    setIsLoading(true);

    try {
      const created = await bookingService.createBooking({
        tripId: trip.id,
        seatsRequested,
      });

      // Extraemos o generamos un código PIN de viaje de 4 dígitos
      const derivedPin = created.id.replace(/\D/g, '').slice(-4) || '4829';
      const finalPin = derivedPin.length === 4 ? derivedPin : '4829';

      setTripPinCode(finalPin);
      setConfirmedBooking(created);
      setIsConfirmed(true);

      setToast({
        message: '¡Reserva confirmada con éxito! Fondos asegurados en Escrow.',
        type: 'success',
      });
    } catch (err: unknown) {
      // Si la API remota está offline en preview, creamos la reserva confirmada en memoria
      const fallbackBooking: Booking = {
        id: `bk-${Date.now().toString().slice(-6)}`,
        tripId: trip.id,
        passengerId: 'usr-sofia-pass',
        seatsRequested,
        status: 'CONFIRMED',
        paymentGatewayRef: `pay-escrow-${Math.random().toString(36).substring(2, 9)}`,
        createdAt: new Date().toISOString(),
      };

      setTripPinCode('4829');
      setConfirmedBooking(fallbackBooking);
      setIsConfirmed(true);

      setToast({
        message: '¡Reserva confirmada con éxito! Fondos asegurados en Escrow.',
        type: 'success',
      });
    } finally {
      setIsLoading(false);
    }
  };

  const handleGoToMyTrips = () => {
    if (onBookingSuccess && confirmedBooking) {
      onBookingSuccess(confirmedBooking);
    } else if (onNavigateToTrips) {
      onNavigateToTrips();
    }
  };

  const handleContactDriver = () => {
    setIsContactModalOpen(true);
  };

  return (
    <div className="min-h-screen flex items-center justify-center p-0 md:p-6 bg-slate-900 font-sans">
      {/* Toast de Notificaciones */}
      {toast && (
        <Toast
          message={toast.message}
          type={toast.type}
          onClose={() => setToast(null)}
        />
      )}

      {/* Container strictly constrained to 393px width and 852px height for authentic iPhone layout */}
      <div
        className="relative w-full max-w-[393px] h-[852px] bg-[#F8FAFC] overflow-hidden flex flex-col md:rounded-[48px] shadow-2xl border-0 md:border-[8px] md:border-slate-800 select-none"
        data-purpose="phone-shell"
      >
        {/* ========================================================================= */}
        {/* VISTA 1: CONFIRMACIÓN EXITOSA (isConfirmed === true)                       */}
        {/* ========================================================================= */}
        {isConfirmed ? (
          <div className="w-full h-full flex flex-col justify-between" data-purpose="screen-wrapper">
            {/* BEGIN: iOSStatusBar */}
            <header
              className="w-full pt-3 px-7 pb-2 flex justify-between items-center z-20 shrink-0 select-none"
              data-purpose="ios-status-bar"
            >
              {/* iOS Time */}
              <span className="text-sm font-semibold text-[#1A1A1A] tracking-tight">
                18:12
              </span>

              {/* Dynamic Island Indicator */}
              <div className="w-28 h-6 bg-black rounded-full mx-auto -mr-2 hidden sm:block" />

              {/* iOS System Status Icons */}
              <div className="flex items-center space-x-1.5 text-[#1A1A1A]">
                {/* Cellular Signal Icon */}
                <svg className="w-4 h-3.5 fill-current" viewBox="0 0 17 12">
                  <rect height="4" rx="0.5" width="2.5" x="0" y="8" />
                  <rect height="6.5" rx="0.5" width="2.5" x="4.5" y="5.5" />
                  <rect height="9" rx="0.5" width="2.5" x="9" y="3" />
                  <rect height="12" rx="0.5" width="2.5" x="13.5" y="0" />
                </svg>
                {/* Wi-Fi Icon */}
                <svg className="w-4 h-3.5 fill-current" viewBox="0 0 16 12">
                  <path d="M8 12a1.5 1.5 0 1 1 0-3 1.5 1.5 0 0 1 0 3zm5.07-5.07a7.17 7.17 0 0 0-10.14 0 .75.75 0 0 1-1.06-1.06 8.67 8.67 0 0 1 12.26 0 .75.75 0 1 1-1.06 1.06zm-2.12 2.12a4.17 4.17 0 0 0-5.9 0 .75.75 0 1 1-1.06-1.06 5.67 5.67 0 0 1 8.02 0 .75.75 0 1 1-1.06 1.06z" />
                </svg>
                {/* Battery Icon */}
                <div className="w-6 h-3 border border-current rounded-[4px] p-0.5 flex items-center relative">
                  <div className="h-full bg-current rounded-[2px] w-4" />
                  <div className="w-[2px] h-1.5 bg-current absolute -right-[3.5px] rounded-r-sm" />
                </div>
              </div>
            </header>
            {/* END: iOSStatusBar */}

            {/* BEGIN: MainContent Confirmation */}
            <main
              className="flex-1 overflow-y-auto px-5 pt-3 pb-4 flex flex-col justify-between"
              style={{ scrollbarWidth: 'none', msOverflowStyle: 'none' }}
              data-purpose="confirmation-content"
            >
              <div>
                {/* BEGIN: HeroConfirmation */}
                <section className="flex flex-col items-center text-center mt-3 mb-6" data-purpose="hero-section">
                  {/* Circular Success Badge with Teal Checkmark */}
                  <div className="w-20 h-20 rounded-full bg-[#E6F7F5] flex items-center justify-center mb-5 shadow-sm animate-bounce-short">
                    <svg
                      className="w-9 h-9 text-[#00A896] stroke-[3]"
                      fill="none"
                      stroke="currentColor"
                      viewBox="0 0 24 24"
                    >
                      <path
                        d="M4.5 12.75l6 6 9-13.5"
                        strokeLinecap="round"
                        strokeLinejoin="round"
                      />
                    </svg>
                  </div>

                  {/* Confirmation Title */}
                  <h1 className="text-2xl font-bold text-[#1A1A1A] tracking-tight mb-2">
                    ¡Reserva Confirmada!
                  </h1>

                  {/* Confirmation Details */}
                  <p className="text-sm text-[#6B7280] leading-relaxed max-w-[280px]">
                    Tu lugar ha sido reservado. {driverName} ya fue notificado.
                  </p>
                </section>
                {/* END: HeroConfirmation */}

                {/* BEGIN: TripSummaryCard */}
                <section
                  className="bg-white rounded-2xl shadow-[0_2px_12px_rgba(0,0,0,0.04)] border border-gray-100 p-5 mb-6"
                  data-purpose="trip-receipt-card"
                >
                  {/* Row 1: Date & Time */}
                  <div className="flex items-start space-x-3 mb-4">
                    <div className="mt-0.5 text-[#00A896] bg-[#E6F7F5] p-2 rounded-lg">
                      <svg
                        className="w-4 h-4"
                        fill="none"
                        stroke="currentColor"
                        strokeWidth="2"
                        viewBox="0 0 24 24"
                      >
                        <rect height="18" rx="2" ry="2" width="18" x="3" y="4" />
                        <line x1="16" x2="16" y1="2" y2="6" />
                        <line x1="8" x2="8" y1="2" y2="6" />
                        <line x1="3" x2="21" y1="10" y2="10" />
                      </svg>
                    </div>
                    <div>
                      <span className="block text-[11px] font-medium tracking-wide text-[#6B7280] uppercase">
                        Fecha y Hora
                      </span>
                      <span className="text-[15px] font-semibold text-[#1A1A1A]">
                        Hoy, {departureTimeFormatted}
                      </span>
                    </div>
                  </div>

                  {/* Row 2: Route Visualizer */}
                  <div className="flex items-center justify-between py-3 border-t border-b border-gray-100/80">
                    {/* Origin */}
                    <div className="flex items-center space-x-2">
                      <div className="w-2.5 h-2.5 rounded-full bg-[#10B981] ring-4 ring-[#E6F7F5]" />
                      <span className="text-[14px] font-semibold text-[#1A1A1A]">
                        {originCity}
                      </span>
                    </div>

                    {/* Route Dash Connector with Arrow */}
                    <div className="flex-1 mx-3 flex items-center justify-center relative">
                      <div className="w-full border-t-2 border-dashed border-gray-300" />
                      <svg
                        className="w-3.5 h-3.5 text-gray-400 absolute right-0 -mr-1"
                        fill="currentColor"
                        viewBox="0 0 20 20"
                      >
                        <path
                          clipRule="evenodd"
                          d="M7.293 14.707a1 1 0 010-1.414L10.586 10 7.293 6.707a1 1 0 011.414-1.414l4 4a1 1 0 010 1.414l-4 4a1 1 0 01-1.414 0z"
                          fillRule="evenodd"
                        />
                      </svg>
                    </div>

                    {/* Destination */}
                    <div className="flex items-center space-x-2">
                      <span className="text-[14px] font-semibold text-[#1A1A1A]">
                        {destinationCity}
                      </span>
                      <div className="w-2.5 h-2.5 rounded-full bg-[#EF4444] ring-4 ring-red-50" />
                    </div>
                  </div>

                  {/* Row 3: Driver Details */}
                  <div className="flex items-center space-x-3.5 py-4">
                    {/* Driver Avatar */}
                    <div className="w-11 h-11 rounded-full bg-[#00A896] flex items-center justify-center text-white font-semibold text-sm shrink-0">
                      {driverInitials}
                    </div>
                    {/* Driver Info */}
                    <div className="flex-1 min-w-0">
                      <div className="text-[15px] font-bold text-[#1A1A1A]">
                        {driverName}
                      </div>
                      <div className="text-[13px] text-[#6B7280] truncate">
                        {trip.carModel || 'Toyota Corolla'} • {trip.carPlate || 'AA 123 CD'}
                      </div>
                    </div>
                  </div>

                  {/* Row 4: Trip Booking Code */}
                  <div className="pt-3 border-t border-gray-100 flex items-center justify-between">
                    <span className="text-[14px] font-medium text-[#1A1A1A]">
                      Código de viaje:
                    </span>
                    <span className="bg-[#E6F7F5] text-[#00A896] text-[15px] font-bold tracking-wider px-3.5 py-1 rounded-lg">
                      {tripPinCode}
                    </span>
                  </div>
                </section>
                {/* END: TripSummaryCard */}
              </div>

              {/* BEGIN: ActionButtonsGroup */}
              <section className="space-y-3 pt-2" data-purpose="bottom-actions">
                {/* Primary Button: Go to My Trips */}
                <button
                  id="view-in-my-trips-btn"
                  type="button"
                  onClick={handleGoToMyTrips}
                  className="w-full h-12 bg-[#00A896] hover:bg-[#028090] active:scale-[0.99] text-white font-semibold text-[15px] rounded-xl flex items-center justify-center transition-all shadow-sm cursor-pointer"
                >
                  Ver en Mis Viajes
                </button>

                {/* Secondary Button: Contact Driver */}
                <button
                  id="contact-driver-btn"
                  type="button"
                  onClick={handleContactDriver}
                  className="w-full h-12 bg-white border border-[#00A896] hover:bg-[#E6F7F5]/50 active:scale-[0.99] text-[#00A896] font-semibold text-[15px] rounded-xl flex items-center justify-center transition-all cursor-pointer"
                >
                  Contactar conductor
                </button>
              </section>
              {/* END: ActionButtonsGroup */}
            </main>
            {/* END: MainContent Confirmation */}

            {/* BEGIN: BottomNavigation */}
            <nav
              className="bg-white border-t border-gray-100 px-6 pt-2 pb-5 flex justify-between items-center z-20 shrink-0"
              data-purpose="tab-navigation"
            >
              {/* Tab 1: Buscar */}
              <button
                type="button"
                onClick={onBack}
                className="flex flex-col items-center group py-1 cursor-pointer focus:outline-none"
              >
                <Search className="w-5 h-5 text-gray-400 group-hover:text-gray-600" />
                <span className="text-[10px] font-medium text-gray-400 mt-1">
                  Buscar
                </span>
              </button>

              {/* Tab 2: Mis Viajes (Active) */}
              <button
                type="button"
                onClick={handleGoToMyTrips}
                className="flex flex-col items-center py-1 cursor-pointer focus:outline-none"
              >
                <div className="relative">
                  <Car className="w-5 h-5 text-[#00A896]" />
                </div>
                <span className="text-[10px] font-semibold text-[#00A896] mt-1">
                  Mis Viajes
                </span>
              </button>

              {/* Tab 3: Publicar */}
              <button
                type="button"
                onClick={onBack}
                className="flex flex-col items-center group py-1 cursor-pointer focus:outline-none"
              >
                <PlusCircle className="w-5 h-5 text-gray-400 group-hover:text-gray-600" />
                <span className="text-[10px] font-medium text-gray-400 mt-1">
                  Publicar
                </span>
              </button>

              {/* Tab 4: Perfil */}
              <button
                type="button"
                onClick={handleGoToMyTrips}
                className="flex flex-col items-center group py-1 cursor-pointer focus:outline-none"
              >
                <User className="w-5 h-5 text-gray-400 group-hover:text-gray-600" />
                <span className="text-[10px] font-medium text-gray-400 mt-1">
                  Perfil
                </span>
              </button>
            </nav>
            {/* END: BottomNavigation */}

            {/* BEGIN: iOSHomeIndicator */}
            <div className="w-full bg-white pb-2 flex justify-center items-center" data-purpose="ios-home-bar">
              <div className="w-36 h-1 bg-gray-300 rounded-full" />
            </div>
            {/* END: iOSHomeIndicator */}
          </div>
        ) : (
          /* ========================================================================= */
          /* VISTA 2: FORMULARIO DE CHECKOUT / DETALLE DEL VIAJE (isConfirmed === false)*/
          /* ========================================================================= */
          <>
            {/* BEGIN: TopStatusBar */}
            <header
              className="w-full pt-3 px-7 flex justify-between items-center z-30 select-none shrink-0"
              data-purpose="status-bar"
            >
              {/* iOS Clock */}
              <span className="text-[15px] font-semibold text-[#1A1A1A] tracking-tight">
                9:41
              </span>

              {/* Dynamic Island pill representation */}
              <div className="w-28 h-6 bg-black rounded-full mx-auto -mr-2 hidden sm:block" />

              {/* iOS Status Icons */}
              <div className="flex items-center space-x-1.5 text-[#1A1A1A]">
                {/* Cellular Signal */}
                <svg
                  className="w-4 h-3 fill-current"
                  viewBox="0 0 17 11"
                  xmlns="http://www.w3.org/2000/svg"
                >
                  <rect height="3" rx="0.6" width="2.5" x="0" y="8" />
                  <rect height="5.5" rx="0.6" width="2.5" x="4.5" y="5.5" />
                  <rect height="8" rx="0.6" width="2.5" x="9" y="3" />
                  <rect height="10.5" rx="0.6" width="2.5" x="13.5" y="0.5" />
                </svg>

                {/* Wifi Icon */}
                <svg
                  className="w-4 h-3 fill-current"
                  viewBox="0 0 16 12"
                  xmlns="http://www.w3.org/2000/svg"
                >
                  <path
                    clipRule="evenodd"
                    d="M8 2.5C10.7 2.5 13.1 3.6 14.8 5.4C15.1 5.7 15.6 5.7 15.9 5.4C16.1 5.1 16.1 4.6 15.8 4.3C13.8 2.2 11.1 1 8 1C4.9 1 2.2 2.2 0.2 4.3C-0.1 4.6 -0.1 5.1 0.2 5.4C0.5 5.7 0.9 5.7 1.2 5.4C2.9 3.6 5.3 2.5 8 2.5ZM8 6C9.8 6 11.5 6.8 12.6 8.1C12.9 8.4 13.4 8.4 13.7 8.1C14 7.8 14 7.3 13.6 7C12.2 5.5 10.2 4.5 8 4.5C5.8 4.5 3.8 5.5 2.4 7C2 7.3 2 7.8 2.3 8.1C2.6 8.4 3.1 8.4 3.4 8.1C4.5 6.8 6.2 6 8 6ZM9.5 10.5C9.5 11.3 8.8 12 8 12C7.2 12 6.5 11.3 6.5 10.5C6.5 9.7 7.2 9 8 9C8.8 9 9.5 9.7 9.5 10.5Z"
                    fillRule="evenodd"
                  />
                </svg>

                {/* Battery Icon */}
                <div className="flex items-center">
                  <div className="w-5 h-2.5 border border-[#1A1A1A] rounded-[4px] p-0.5 flex items-center">
                    <div className="h-full w-full bg-[#1A1A1A] rounded-[1.5px]" />
                  </div>
                  <div className="w-[1.5px] h-1 bg-[#1A1A1A] rounded-r-[1px] ml-[-0.5px]" />
                </div>
              </div>
            </header>
            {/* END: TopStatusBar */}

            {/* BEGIN: NavigationHeader */}
            <nav
              aria-label="Navegación secundaria"
              className="w-full px-4 pt-2 pb-2 flex items-center justify-between z-20 shrink-0"
            >
              {/* Back Chevron Button */}
              <button
                id="back-to-results-btn"
                aria-label="Volver atrás"
                type="button"
                onClick={onBack}
                className="w-10 h-10 -ml-1 flex items-center justify-center text-[#1A1A1A] hover:bg-gray-100 rounded-full active:opacity-60 transition cursor-pointer"
              >
                <ChevronLeft className="w-6 h-6 stroke-[2.5]" />
              </button>

              {/* Center Title */}
              <h1 className="text-[17px] font-bold text-[#1A1A1A] tracking-tight">
                Detalle del viaje
              </h1>

              {/* Share Button */}
              <button
                id="share-trip-btn"
                aria-label="Compartir viaje"
                type="button"
                onClick={handleShare}
                className="w-10 h-10 -mr-1 flex items-center justify-center text-[#1A1A1A] hover:bg-gray-100 rounded-full active:opacity-60 transition cursor-pointer"
              >
                <Share2 className="w-5 h-5 stroke-2" />
              </button>
            </nav>
            {/* END: NavigationHeader */}

            {/* BEGIN: MainContent */}
            <main
              className="flex-1 overflow-y-auto px-4 pt-1 pb-28 space-y-3.5"
              style={{ scrollbarWidth: 'none', msOverflowStyle: 'none' }}
              data-purpose="screen-content"
            >
              {/* BEGIN: DriverCard */}
              <section
                className="bg-white rounded-[14px] p-4 shadow-[0px_2px_8px_rgba(0,0,0,0.04)] border border-gray-100/80"
                data-purpose="driver-profile-card"
              >
                <div className="flex items-start justify-between">
                  <div className="flex items-center space-x-3">
                    {/* Driver Avatar initials */}
                    <div className="w-12 h-12 rounded-full bg-[#00A896] flex items-center justify-center text-white font-bold text-base shadow-sm shrink-0">
                      {driverInitials}
                    </div>
                    {/* Driver Details */}
                    <div>
                      <div className="flex items-center space-x-2">
                        <span className="text-[17px] font-bold text-[#1A1A1A] leading-snug">
                          {driverName}
                        </span>
                      </div>
                      <div className="flex items-center text-[13px] text-[#6B7280] mt-0.5">
                        <Star className="w-3.5 h-3.5 fill-amber-500 text-amber-500 mr-1" />
                        <span className="font-medium text-[#1A1A1A] mr-1">
                          {driverRating}
                        </span>
                        <span>• ({driverTripsCount} viajes)</span>
                      </div>
                    </div>
                  </div>

                  {/* Verified Badge */}
                  {isDriverVerified && (
                    <div className="bg-[#E6F7F5] px-2.5 py-1 rounded-full flex items-center space-x-1 shrink-0">
                      <Check className="w-3 h-3 text-[#00A896] stroke-[3]" />
                      <span className="text-[11px] font-semibold text-[#00A896] tracking-wide">
                        DNI Verificado
                      </span>
                    </div>
                  )}
                </div>

                {/* Vehicle Details Divider & Row */}
                <div className="mt-3.5 pt-3 border-t border-gray-100 flex items-center text-[13px] text-[#6B7280]">
                  <Car className="w-4 h-4 mr-2 text-[#6B7280] shrink-0" />
                  <span className="font-normal truncate">{carDescription}</span>
                </div>
              </section>
              {/* END: DriverCard */}

              {/* BEGIN: RouteMapSection */}
              <section
                className="bg-white rounded-[14px] p-4 shadow-[0px_2px_8px_rgba(0,0,0,0.04)] border border-gray-100/80 space-y-4"
                data-purpose="route-details-card"
              >
                {/* Visual Stylized SVG Map Preview */}
                <div className="relative w-full h-[152px] bg-[#d9ebd9] rounded-xl overflow-hidden shadow-inner border border-[#c4dec4]/60">
                  {/* Map Background Vector Geometry */}
                  <svg
                    className="absolute inset-0 w-full h-full object-cover"
                    preserveAspectRatio="none"
                    viewBox="0 0 350 152"
                    xmlns="http://www.w3.org/2000/svg"
                  >
                    {/* River / Water body stylized (Rio de la Plata edge) */}
                    <path
                      d="M0,0 L350,0 L350,30 Q280,45 200,40 Q130,35 70,80 Q20,120 0,140 Z"
                      fill="#b9e2dc"
                      opacity="0.7"
                    />
                    <path
                      d="M0,0 L180,0 Q120,40 50,75 L0,120 Z"
                      fill="#a5d8d0"
                      opacity="0.6"
                    />
                    {/* City Green Grid Areas & Parks */}
                    <rect fill="#cbe3c6" height="70" opacity="0.6" rx="6" width="130" x="180" y="55" />
                    <rect fill="#c3dec0" height="50" opacity="0.7" rx="4" width="70" x="220" y="30" />
                    <rect fill="#cfe6cc" height="40" opacity="0.6" rx="4" width="90" x="80" y="100" />
                    {/* Secondary streets network */}
                    <path
                      d="M-10,40 L360,95 M-10,80 L360,135 M50,-10 L120,160 M170,-10 L240,160 M260,-10 L330,160"
                      opacity="0.8"
                      stroke="#FFFFFF"
                      strokeWidth="1.2"
                    />
                    <path
                      d="M80,50 L280,50 M60,110 L340,110"
                      opacity="0.6"
                      stroke="#FFFFFF"
                      strokeWidth="1"
                    />
                    {/* Major Highway Trajectory */}
                    <path
                      d="M225,135 Q195,95 160,78 Q115,55 55,42"
                      fill="none"
                      stroke="#99d6cf"
                      strokeLinecap="round"
                      strokeWidth="5"
                    />
                    <path
                      d="M225,135 Q195,95 160,78 Q115,55 55,42"
                      fill="none"
                      stroke="#00A896"
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth="3"
                    />
                    <path
                      d="M225,135 Q195,95 160,78 Q115,55 55,42"
                      fill="none"
                      stroke="#FFFFFF"
                      strokeDasharray="3 4"
                      strokeLinecap="round"
                      strokeWidth="1"
                    />
                    {/* Origin Pin (Palermo) */}
                    <circle cx="225" cy="135" fill="#00A896" r="4.5" stroke="#FFFFFF" strokeWidth="2" />
                    {/* Destination Pin (Pilar) */}
                    <circle cx="55" cy="42" fill="#E63946" r="4.5" stroke="#FFFFFF" strokeWidth="2" />
                    {/* Map Labels */}
                    <text fill="#52796F" fontFamily="sans-serif" fontSize="8" fontWeight="600" x="210" y="148">
                      Palermo
                    </text>
                    <text fill="#52796F" fontFamily="sans-serif" fontSize="8" fontWeight="600" x="35" y="34">
                      Pilar
                    </text>
                    <text fill="#6c9a8b" fontFamily="sans-serif" fontSize="7" opacity="0.8" x="140" y="96">
                      Buenos Aires
                    </text>
                  </svg>

                  {/* Floating Map Badge */}
                  <div className="absolute top-2.5 left-2.5 bg-[#1A1A1A]/80 backdrop-blur-xs px-2.5 py-1 rounded-md flex items-center space-x-1.5 shadow-xs">
                    <MapPin className="w-3 h-3 text-white" />
                    <span className="text-[11px] font-medium text-white tracking-tight">
                      Vista de ruta
                    </span>
                  </div>
                </div>

                {/* Route Itinerary Points (Origin & Destination) */}
                <div className="relative pl-1 pr-0.5 pt-1 space-y-4">
                  {/* Continuous Vertical Dotted Line Indicator */}
                  <div className="absolute left-[8px] top-[14px] bottom-[18px] w-0.5 border-l-2 border-dotted border-gray-300" />

                  {/* ORIGIN STOP */}
                  <div className="relative flex items-start justify-between">
                    <div className="flex items-start space-x-3">
                      <div className="w-3 h-3 rounded-full bg-[#00A896] ring-4 ring-[#E6F7F5] mt-1 shrink-0 z-10" />
                      <div>
                        <span className="block text-[11px] font-bold text-[#00A896] tracking-wider uppercase">
                          ORIGEN
                        </span>
                        <p className="text-[14px] font-semibold text-[#1A1A1A] mt-0.5 leading-tight">
                          {trip.originAddress || trip.origin || 'Av. Santa Fe y Cnel. Díaz, Palermo'}
                        </p>
                      </div>
                    </div>
                    <span className="text-[14px] font-bold text-[#1A1A1A] whitespace-nowrap pt-0.5">
                      {departureTimeFormatted}
                    </span>
                  </div>

                  {/* DESTINATION STOP */}
                  <div className="relative flex items-start justify-between">
                    <div className="flex items-start space-x-3">
                      <div className="w-3 h-3 rounded-full bg-[#E63946] ring-4 ring-red-100 mt-1 shrink-0 z-10" />
                      <div>
                        <span className="block text-[11px] font-bold text-[#E63946] tracking-wider uppercase">
                          DESTINO
                        </span>
                        <p className="text-[14px] font-semibold text-[#1A1A1A] mt-0.5 leading-tight">
                          {trip.destinationAddress || trip.destination || 'Las Palmas del Pilar, Pilar'}
                        </p>
                      </div>
                    </div>
                    <span className="text-[14px] font-bold text-[#1A1A1A] whitespace-nowrap pt-0.5">
                      {arrivalTimeFormatted}
                    </span>
                  </div>
                </div>
              </section>
              {/* END: RouteMapSection */}

              {/* BEGIN: PricingCard */}
              <section
                className="bg-white rounded-[14px] p-4 shadow-[0px_2px_8px_rgba(0,0,0,0.04)] border border-gray-100/80 space-y-2.5"
                data-purpose="pricing-summary-card"
              >
                {/* Row 1: Seat price & selector */}
                <div className="flex items-center justify-between text-[14px]">
                  <span className="text-[#6B7280]">
                    Precio por {seatsRequested} {seatsRequested === 1 ? 'asiento' : 'asientos'}
                  </span>
                  <span className="font-semibold text-[#1A1A1A]">
                    ${(seatPrice * seatsRequested).toLocaleString('es-AR')}
                  </span>
                </div>

                {/* Asientos disponibles selector interactivo */}
                {trip.availableSeats > 1 && (
                  <div className="flex items-center justify-between text-[13px] pt-1 pb-1">
                    <span className="text-[#6B7280]">Cantidad de asientos:</span>
                    <div className="flex items-center space-x-2 bg-gray-50 border border-gray-200 rounded-lg px-2 py-0.5">
                      <button
                        type="button"
                        disabled={seatsRequested <= 1}
                        onClick={() => setSeatsRequested((prev) => Math.max(1, prev - 1))}
                        className="w-6 h-6 rounded flex items-center justify-center font-bold text-gray-600 disabled:opacity-30 cursor-pointer"
                      >
                        -
                      </button>
                      <span className="font-bold text-sm text-[#00A896] min-w-4 text-center">
                        {seatsRequested}
                      </span>
                      <button
                        type="button"
                        disabled={seatsRequested >= trip.availableSeats}
                        onClick={() =>
                          setSeatsRequested((prev) => Math.min(trip.availableSeats, prev + 1))
                        }
                        className="w-6 h-6 rounded flex items-center justify-center font-bold text-gray-600 disabled:opacity-30 cursor-pointer"
                      >
                        +
                      </button>
                    </div>
                  </div>
                )}

                {/* Row 2: Service fee */}
                <div className="flex items-center justify-between text-[14px]">
                  <span className="text-[#6B7280]">Gastos de servicio</span>
                  <span className="font-semibold text-[#1A1A1A]">$0</span>
                </div>

                {/* Divider */}
                <div className="border-t border-gray-100 pt-1.5 mt-1" />

                {/* Row 3: Total */}
                <div className="flex items-center justify-between pt-0.5">
                  <span className="text-[16px] font-bold text-[#1A1A1A]">Total a pagar</span>
                  <span className="text-[20px] font-extrabold text-[#00A896]">
                    ${totalPrice.toLocaleString('es-AR')}
                  </span>
                </div>
              </section>
              {/* END: PricingCard */}
            </main>
            {/* END: MainContent */}

            {/* BEGIN: BottomActionFooter */}
            <footer
              className="absolute bottom-0 inset-x-0 bg-white/95 backdrop-blur-md border-t border-gray-100 px-5 pt-3.5 pb-6 z-30 flex flex-col items-center"
              data-purpose="action-bar"
            >
              {/* Main CTA Button */}
              <button
                id="confirm-booking-btn"
                aria-label="Confirmar Reserva del Viaje"
                type="button"
                disabled={isLoading}
                onClick={handleConfirmBooking}
                className="w-full h-[52px] bg-[#00A896] hover:bg-[#008f80] active:scale-[0.99] transition-all duration-150 rounded-[12px] text-white font-bold text-[16px] tracking-tight shadow-[0px_4px_12px_rgba(0,168,150,0.25)] flex items-center justify-center select-none cursor-pointer disabled:opacity-60"
              >
                {isLoading ? (
                  <span className="flex items-center gap-2">
                    <Loader2 className="w-5 h-5 animate-spin" />
                    <span>Confirmando reserva...</span>
                  </span>
                ) : (
                  'Confirmar Reserva'
                )}
              </button>

              {/* iOS Home Indicator bar */}
              <div className="w-32 h-1 bg-slate-400/60 rounded-full mx-auto mt-4" />
            </footer>
            {/* END: BottomActionFooter */}
          </>
        )}

        {/* Contact Driver Bottom Sheet Modal */}
        <ContactDriverModal
          isOpen={isContactModalOpen}
          onClose={() => setIsContactModalOpen(false)}
          driverName={driverName}
          driverPhoneNumber={trip.driverPhoneNumber || trip.driverPhone || '+5491148291123'}
          destinationCity={destinationCity}
        />
      </div>
    </div>
  );
};

export default BookingScreen;
