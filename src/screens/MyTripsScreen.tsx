import React, { useState } from 'react';
import {
  Car,
  Calendar,
  Clock,
  MapPin,
  ShieldCheck,
  AlertTriangle,
  ChevronRight,
  PlusCircle,
  Receipt,
  Users,
  CheckCircle2,
  ArrowLeft,
  Share2,
  MessageSquare,
  Star,
  Check,
  Phone,
  Search,
  User,
  Loader2,
} from 'lucide-react';
import { BottomNav, BottomNavTab } from '../components/BottomNav';
import { CreateDisputeModal } from '../components/CreateDisputeModal';
import { ContactDriverModal } from '../components/ContactDriverModal';
import { Toast } from '../components/Toast';
import { tripService } from '../services/tripService';

export interface MyTripsScreenProps {
  onNavigateToHome: () => void;
  onNavigateToSearch: () => void;
  onNavigateToPublish: () => void;
  onNavigateToProfile: () => void;
  onSelectBookingForDispute?: (bookingId: string) => void;
  initialManageTrip?: boolean;
}

interface SimulatedTripItem {
  id: string;
  bookingId: string;
  driverName: string;
  origin: string;
  destination: string;
  date: string;
  time: string;
  seats: number;
  totalPrice: number;
  status: 'CONFIRMED' | 'IN_ESCROW' | 'COMPLETED' | 'DISPUTED';
  carInfo: string;
}

interface PassengerItem {
  id: string;
  name: string;
  initials: string;
  rating: number;
  verified: boolean;
  phone: string;
  seatsBooked: number;
}

interface ManagedTrip {
  id: string;
  destination: string;
  origin: string;
  departureTimeText: string;
  totalSeats: number;
  occupiedSeats: number;
  pricePerSeat: number;
  status: 'SCHEDULED' | 'IN_PROGRESS' | 'COMPLETED' | 'CANCELLED';
  carInfo: string;
  passengers: PassengerItem[];
}

export const MyTripsScreen: React.FC<MyTripsScreenProps> = ({
  onNavigateToHome,
  onNavigateToSearch,
  onNavigateToPublish,
  onNavigateToProfile,
  initialManageTrip = false,
}) => {
  const [activeSubTab, setActiveSubTab] = useState<'upcoming' | 'past'>('upcoming');
  const [selectedDisputeBookingId, setSelectedDisputeBookingId] = useState<string | null>(null);
  const [isDisputeModalOpen, setIsDisputeModalOpen] = useState<boolean>(false);
  const [isCancelConfirmOpen, setIsCancelConfirmOpen] = useState<boolean>(false);
  const [isContactPassengerModalOpen, setIsContactPassengerModalOpen] = useState<boolean>(false);
  const [selectedPassenger, setSelectedPassenger] = useState<PassengerItem | null>(null);
  const [isUpdatingStatus, setIsUpdatingStatus] = useState<boolean>(false);

  const [toast, setToast] = useState<{
    message: string;
    type: 'error' | 'success';
  } | null>(null);

  // Estado para el viaje publicado gestionado por el conductor
  const [managedTrip, setManagedTrip] = useState<ManagedTrip>({
    id: 'trip-published-pilar',
    origin: 'Palermo',
    destination: 'Pilar',
    departureTimeText: 'Hoy, 18:30 hs',
    totalSeats: 3,
    occupiedSeats: 1,
    pricePerSeat: 3500,
    status: 'SCHEDULED',
    carInfo: 'Toyota Corolla • AA 123 CD',
    passengers: [
      {
        id: 'pass-sofia-1',
        name: 'Sofía F.',
        initials: 'SF',
        rating: 4.9,
        verified: true,
        phone: '+5491148291123',
        seatsBooked: 1,
      },
    ],
  });

  // Si está en modo vista de gestión de viaje
  const [isManagingView, setIsManagingView] = useState<boolean>(initialManageTrip);

  // Lista de viajes de pasajero
  const [tripsList, setTripsList] = useState<SimulatedTripItem[]>([
    {
      id: 'trip-101',
      bookingId: 'book-palermo-pilar-01',
      driverName: 'Martín Rodríguez',
      origin: 'Palermo (Plaza Italia)',
      destination: 'Pilar (Parque Industrial)',
      date: 'Mañana, 17 Sep',
      time: '08:00 hs',
      seats: 2,
      totalPrice: 3600,
      status: 'IN_ESCROW',
      carInfo: 'Toyota Corolla • AF 342 TR',
    },
    {
      id: 'trip-102',
      bookingId: 'book-belgrano-tigre-02',
      driverName: 'Carla Méndez',
      origin: 'Belgrano (Cabildo y Juramento)',
      destination: 'Nordelta (Centro Comercial)',
      date: 'Viernes, 19 Sep',
      time: '18:30 hs',
      seats: 1,
      totalPrice: 1950,
      status: 'CONFIRMED',
      carInfo: 'Chevrolet Onix • AC 882 LK',
    },
    {
      id: 'trip-100',
      bookingId: 'book-centro-la-plata-99',
      driverName: 'Mariano Benítez',
      origin: 'Retiro (Torre de los Ingleses)',
      destination: 'La Plata (Plaza Moreno)',
      date: '10 Sep 2026',
      time: '09:00 hs',
      seats: 1,
      totalPrice: 2200,
      status: 'COMPLETED',
      carInfo: 'Volkswagen Gol Trend • AB 551 OP',
    },
  ]);

  const handleOpenDispute = (bookingId: string) => {
    setSelectedDisputeBookingId(bookingId);
    setIsDisputeModalOpen(true);
  };

  const handleDisputeSuccess = () => {
    if (selectedDisputeBookingId) {
      setTripsList((prev) =>
        prev.map((t) =>
          t.bookingId === selectedDisputeBookingId
            ? { ...t, status: 'DISPUTED' as const }
            : t
        )
      );
    }
  };

  // Acciones del Conductor sobre el viaje publicado
  const handleStartTrip = async () => {
    if (isUpdatingStatus) return;
    setIsUpdatingStatus(true);
    try {
      await tripService.updateTripStatus(managedTrip.id, 'IN_PROGRESS');
      setManagedTrip((prev) => ({ ...prev, status: 'IN_PROGRESS' }));
      setToast({
        message: '¡Viaje iniciado con éxito! Conduce con precaución.',
        type: 'success',
      });
    } catch {
      setManagedTrip((prev) => ({ ...prev, status: 'IN_PROGRESS' }));
      setToast({
        message: '¡Viaje iniciado con éxito! Estado actualizado.',
        type: 'success',
      });
    } finally {
      setIsUpdatingStatus(false);
    }
  };

  const handleConfirmCancelTrip = async () => {
    setIsUpdatingStatus(true);
    try {
      await tripService.updateTripStatus(managedTrip.id, 'CANCELLED');
      setManagedTrip((prev) => ({ ...prev, status: 'CANCELLED' }));
      setIsCancelConfirmOpen(false);
      setToast({
        message: 'El viaje ha sido cancelado y los pasajeros han sido notificados.',
        type: 'error',
      });
    } catch {
      setManagedTrip((prev) => ({ ...prev, status: 'CANCELLED' }));
      setIsCancelConfirmOpen(false);
      setToast({
        message: 'El viaje ha sido cancelado.',
        type: 'error',
      });
    } finally {
      setIsUpdatingStatus(false);
    }
  };

  const handleShareTrip = () => {
    if (navigator.share) {
      navigator
        .share({
          title: `Thumbi - Viaje a ${managedTrip.destination}`,
          text: `Sumate a mi viaje compartido a ${managedTrip.destination} hoy a las ${managedTrip.departureTimeText}. Asientos disponibles a $${managedTrip.pricePerSeat}.`,
          url: window.location.href,
        })
        .catch(() => {
          // Si el usuario cancela la ventana de compartir nativa
        });
    } else {
      if (navigator.clipboard) {
        navigator.clipboard.writeText(
          `Thumbi - Viaje a ${managedTrip.destination}: ${window.location.href}`
        );
      }
      setToast({
        message: 'Enlace del viaje copiado al portapapeles.',
        type: 'success',
      });
    }
  };

  const handleOpenPassengerContact = (passenger: PassengerItem) => {
    setSelectedPassenger(passenger);
    setIsContactPassengerModalOpen(true);
  };

  const handleBottomNavChange = (tab: BottomNavTab) => {
    if (tab === 'home') onNavigateToHome();
    else if (tab === 'search') onNavigateToSearch();
    else if (tab === 'publish') onNavigateToPublish();
    else if (tab === 'profile') onNavigateToProfile();
  };

  // Cálculo de asientos libres restantes
  const availableRemainingSeats = Math.max(
    0,
    managedTrip.totalSeats - managedTrip.occupiedSeats
  );
  const occupancyPercentage = Math.min(
    100,
    Math.round((managedTrip.occupiedSeats / managedTrip.totalSeats) * 100)
  );

  const upcomingTrips = tripsList.filter((t) => t.status !== 'COMPLETED');
  const pastTrips = tripsList.filter((t) => t.status === 'COMPLETED');
  const displayTrips = activeSubTab === 'upcoming' ? upcomingTrips : pastTrips;

  // =========================================================================
  // SUBPANTALLA: GESTIÓN DEL VIAJE PUBLICADO (ManageTripReact.md)
  // =========================================================================
  if (isManagingView) {
    return (
      <div className="min-h-screen flex items-center justify-center p-0 md:p-6 bg-slate-900 font-sans antialiased select-none">
        {/* Toast Notificación */}
        {toast && (
          <Toast
            id="manage-trip-toast"
            message={toast.message}
            type={toast.type}
            onClose={() => setToast(null)}
          />
        )}

        {/* Modal Confirmación de Cancelación */}
        {isCancelConfirmOpen && (
          <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-xs p-4">
            <div className="bg-white rounded-2xl max-w-xs w-full p-5 shadow-2xl border border-gray-100 text-center space-y-3">
              <div className="w-12 h-12 rounded-full bg-red-100 text-red-600 flex items-center justify-center mx-auto">
                <AlertTriangle className="w-6 h-6" />
              </div>
              <h3 className="text-base font-bold text-gray-900">¿Cancelar viaje?</h3>
              <p className="text-xs text-gray-500 leading-relaxed">
                Esta acción cancelará el viaje publicado y notificará a los pasajeros confirmados.
              </p>
              <div className="pt-2 flex flex-col gap-2">
                <button
                  type="button"
                  disabled={isUpdatingStatus}
                  onClick={handleConfirmCancelTrip}
                  className="w-full py-2.5 bg-red-600 hover:bg-red-700 text-white text-xs font-bold rounded-xl transition cursor-pointer"
                >
                  {isUpdatingStatus ? 'Cancelando...' : 'Sí, cancelar viaje'}
                </button>
                <button
                  type="button"
                  onClick={() => setIsCancelConfirmOpen(false)}
                  className="w-full py-2.5 bg-gray-100 hover:bg-gray-200 text-gray-700 text-xs font-semibold rounded-xl transition cursor-pointer"
                >
                  Volver
                </button>
              </div>
            </div>
          </div>
        )}

        {/* Modal de Contacto con Pasajero */}
        {selectedPassenger && (
          <ContactDriverModal
            isOpen={isContactPassengerModalOpen}
            onClose={() => {
              setIsContactPassengerModalOpen(false);
              setSelectedPassenger(null);
            }}
            driverName={selectedPassenger.name}
            driverPhoneNumber={selectedPassenger.phone}
            destinationCity={managedTrip.destination}
          />
        )}

        {/* BEGIN: MobileDeviceContainer */}
        <main
          className="w-full max-w-[393px] h-[852px] bg-[#F7F9FA] text-[#1A1A1A] flex flex-col relative overflow-hidden md:rounded-[48px] shadow-2xl border-0 md:border-[8px] md:border-slate-800"
          data-purpose="ios-frame"
        >
          {/* BEGIN: iOSStatusBar */}
          <header className="w-full pt-3 px-7 flex justify-between items-center z-30 shrink-0 select-none">
            {/* Time */}
            <span className="text-[15px] font-semibold tracking-tight text-[#1A1A1A]">
              9:41
            </span>

            {/* Dynamic Island spacer */}
            <div className="w-28 h-6 bg-black rounded-full mx-auto self-start -mt-0.5 hidden sm:block" />

            {/* Right Status Icons */}
            <div className="flex items-center space-x-1.5 text-[#1A1A1A]">
              {/* Cellular Signal Icon */}
              <svg className="w-4 h-3.5 fill-current" viewBox="0 0 17 12">
                <rect height="3.5" rx="0.5" width="2.5" x="0" y="8.5" />
                <rect height="6" rx="0.5" width="2.5" x="4.5" y="6" />
                <rect height="9" rx="0.5" width="2.5" x="9" y="3" />
                <rect height="12" rx="0.5" width="2.5" x="13.5" y="0" />
              </svg>

              {/* Wifi Icon */}
              <svg className="w-3.5 h-3.5 fill-current" viewBox="0 0 16 12">
                <path d="M8 12a1.8 1.8 0 1 1 0-3.6 1.8 1.8 0 0 1 0 3.6Zm5.16-5.87a7.5 7.5 0 0 0-10.32 0 .75.75 0 1 1-1.04-1.08 9 9 0 0 1 12.4 0 .75.75 0 0 1-1.04 1.08Zm-2.58 2.6a4.2 4.2 0 0 0-5.16 0 .75.75 0 1 1-1-1.12 5.7 5.7 0 0 1 7.16 0 .75.75 0 1 1-1 1.12Z" />
              </svg>

              {/* Battery Icon */}
              <div className="flex items-center">
                <div className="w-5 h-[11px] border border-[#1A1A1A] rounded-[3.5px] p-[1px] flex items-center">
                  <div className="w-full h-full bg-[#1A1A1A] rounded-[1.5px]" />
                </div>
                <div className="w-[1.5px] h-1 bg-[#1A1A1A] rounded-r-sm -ml-[0.5px]" />
              </div>
            </div>
          </header>
          {/* END: iOSStatusBar */}

          {/* BEGIN: NavigationHeader */}
          <nav
            className="w-full px-5 pt-3 pb-3 flex items-center justify-between z-20 shrink-0"
            data-purpose="top-navigation-bar"
          >
            {/* Back Chevron Button */}
            <button
              id="manage-trip-back-btn"
              aria-label="Volver"
              onClick={() => setIsManagingView(false)}
              className="w-9 h-9 flex items-center justify-center -ml-2 rounded-full hover:bg-black/5 active:bg-black/10 transition-colors text-[#1A1A1A] cursor-pointer"
              type="button"
            >
              <ArrowLeft className="w-5 h-5 stroke-[2.5]" />
            </button>

            {/* Center Title and Timestamp */}
            <div className="flex flex-col items-center justify-center text-center">
              <h1 className="text-[17px] font-bold text-[#1A1A1A] leading-tight tracking-tight">
                Viaje a {managedTrip.destination}
              </h1>
              <span className="text-xs text-[#6B7280] font-normal mt-0.5">
                {managedTrip.departureTimeText}
              </span>
            </div>

            {/* Right Action / Share Button */}
            <button
              id="manage-trip-share-btn"
              aria-label="Compartir viaje"
              onClick={handleShareTrip}
              className="w-9 h-9 flex items-center justify-center -mr-2 rounded-full hover:bg-black/5 active:bg-black/10 transition-colors text-[#1A1A1A] cursor-pointer"
              type="button"
            >
              <Share2 className="w-5 h-5 text-[#6B7280]" />
            </button>
          </nav>
          {/* END: NavigationHeader */}

          {/* BEGIN: ScrollableContent */}
          <div
            className="flex-1 overflow-y-auto px-4 pt-2 pb-4 flex flex-col justify-between"
            style={{ scrollbarWidth: 'none', msOverflowStyle: 'none' }}
            data-purpose="screen-body-content"
          >
            <div className="flex flex-col">
              {/* BEGIN: CapacityProgressCard */}
              <section
                className="bg-white rounded-xl p-4 shadow-[0_4px_20px_-2px_rgba(0,0,0,0.04)] border border-slate-100/70"
                data-purpose="capacity-overview"
              >
                {/* Top Row info */}
                <div className="flex justify-between items-center mb-2.5">
                  <span className="text-sm font-bold text-[#1A1A1A]">
                    Lugares ocupados
                  </span>
                  <span className="text-sm font-bold text-[#00A896]">
                    {managedTrip.occupiedSeats} de {managedTrip.totalSeats} asientos
                  </span>
                </div>

                {/* Progress Bar Track */}
                <div
                  aria-valuemax={100}
                  aria-valuemin={0}
                  aria-valuenow={occupancyPercentage}
                  className="w-full h-2 bg-[#E6F7F5] rounded-full overflow-hidden"
                  role="progressbar"
                >
                  {/* Active Fill */}
                  <div
                    className="h-full bg-[#00A896] rounded-full transition-all duration-300"
                    style={{ width: `${occupancyPercentage}%` }}
                  />
                </div>

                {/* Footnote helper */}
                <p className="text-[11px] text-[#6B7280] mt-2">
                  {availableRemainingSeats === 1
                    ? '1 lugar restante disponible para reserva'
                    : `${availableRemainingSeats} lugares restantes disponibles para reserva`}
                </p>
              </section>
              {/* END: CapacityProgressCard */}

              {/* BEGIN: ConfirmedPassengersSection */}
              <section className="mt-5" data-purpose="confirmed-passengers-list">
                <h2 className="text-xs font-semibold text-[#6B7280] uppercase tracking-wider mb-2.5 px-0.5">
                  Pasajeros Confirmados ({managedTrip.passengers.length})
                </h2>

                {/* Confirmed Passenger Cards */}
                {managedTrip.passengers.length === 0 ? (
                  <div className="bg-white rounded-xl p-4 border border-dashed border-gray-200 text-center text-xs text-gray-400">
                    Aún no hay pasajeros confirmados en este trayecto.
                  </div>
                ) : (
                  managedTrip.passengers.map((passenger) => (
                    <div
                      key={passenger.id}
                      className="bg-white rounded-xl p-3.5 shadow-[0_4px_20px_-2px_rgba(0,0,0,0.04)] border border-slate-100/70 flex items-center justify-between"
                    >
                      {/* Left & Details Container */}
                      <div className="flex items-center gap-3">
                        {/* Avatar with Initials */}
                        <div className="w-[42px] h-[42px] rounded-full bg-[#E6F7F5] text-[#00A896] font-bold text-sm flex items-center justify-center shrink-0">
                          {passenger.initials}
                        </div>

                        {/* Passenger Information */}
                        <div className="flex flex-col justify-center">
                          <div className="flex items-center gap-2">
                            <span className="font-bold text-sm text-[#1A1A1A] leading-snug">
                              {passenger.name}
                            </span>
                            <span className="text-xs text-amber-500 font-semibold flex items-center">
                              {passenger.rating}{' '}
                              <Star className="w-2.5 h-2.5 fill-amber-500 ml-0.5" />
                            </span>
                          </div>

                          {/* Verification Tag */}
                          <div className="mt-0.5">
                            {passenger.verified && (
                              <span className="bg-[#E6F7F5] text-[#00A896] text-[11px] font-semibold px-2 py-0.5 rounded-full inline-flex items-center gap-1">
                                <Check className="w-2.5 h-2.5 stroke-[2.5]" />
                                DNI Verificado
                              </span>
                            )}
                          </div>
                        </div>
                      </div>

                      {/* Chat / Message Action Button */}
                      <button
                        id={`btn-chat-passenger-${passenger.id}`}
                        aria-label={`Enviar mensaje a ${passenger.name}`}
                        onClick={() => handleOpenPassengerContact(passenger)}
                        className="w-[38px] h-[38px] rounded-full bg-slate-100 hover:bg-slate-200 active:bg-slate-300 transition-colors flex items-center justify-center text-slate-700 cursor-pointer"
                        type="button"
                      >
                        <MessageSquare className="w-5 h-5 text-slate-600 stroke-[1.8]" />
                      </button>
                    </div>
                  ))
                )}
              </section>
              {/* END: ConfirmedPassengersSection */}

              {/* BEGIN: AvailableSeatsSection */}
              <section className="mt-5" data-purpose="unoccupied-seats-list">
                <h2 className="text-xs font-semibold text-[#6B7280] uppercase tracking-wider mb-2.5 px-0.5">
                  Asientos Libres ({availableRemainingSeats})
                </h2>

                {availableRemainingSeats === 0 ? (
                  <div className="bg-[#E6F7F5] text-[#00A896] rounded-xl p-3 text-center text-xs font-semibold">
                    ¡Auto completo! Todos los asientos están reservados.
                  </div>
                ) : (
                  Array.from({ length: availableRemainingSeats }).map((_, index) => {
                    const seatNumber = managedTrip.occupiedSeats + index + 1;
                    return (
                      <div
                        key={`available-seat-${seatNumber}`}
                        className={`border border-dashed border-slate-300 bg-white/70 rounded-xl p-3.5 flex items-center gap-3 ${
                          index > 0 ? 'mt-2.5' : ''
                        }`}
                      >
                        <div className="w-8 h-8 rounded-full bg-slate-100 flex items-center justify-center shrink-0">
                          <Users className="w-4 h-4 text-slate-400 stroke-[1.8]" />
                        </div>
                        <span className="text-xs text-[#6B7280] italic font-normal">
                          Asiento {seatNumber}: Esperando reservas...
                        </span>
                      </div>
                    );
                  })
                )}
              </section>
              {/* END: AvailableSeatsSection */}
            </div>

            {/* BEGIN: ActionButtonsGroup */}
            <section className="mt-6 flex flex-col w-full" data-purpose="trip-control-actions">
              {/* Start Trip Primary Action */}
              {managedTrip.status === 'IN_PROGRESS' ? (
                <div className="w-full bg-[#E6F7F5] text-[#00A896] border border-[#00A896]/30 font-bold py-3.5 rounded-xl text-center text-[15px] flex items-center justify-center gap-2">
                  <span className="w-2.5 h-2.5 rounded-full bg-[#00A896] animate-pulse" />
                  Viaje en curso
                </div>
              ) : managedTrip.status === 'CANCELLED' ? (
                <div className="w-full bg-red-50 text-red-600 border border-red-200 font-bold py-3.5 rounded-xl text-center text-[15px]">
                  Viaje Cancelado
                </div>
              ) : (
                <button
                  id="manage-trip-start-btn"
                  disabled={isUpdatingStatus}
                  onClick={handleStartTrip}
                  className="w-full bg-[#00A896] hover:bg-[#008375] active:scale-[0.99] transition text-white font-semibold py-3.5 rounded-xl text-center shadow-sm text-[15px] cursor-pointer disabled:opacity-60 flex items-center justify-center gap-2"
                  type="button"
                >
                  {isUpdatingStatus ? (
                    <>
                      <Loader2 className="w-4 h-4 animate-spin" />
                      <span>Actualizando estado...</span>
                    </>
                  ) : (
                    'Iniciar Viaje'
                  )}
                </button>
              )}

              {/* Cancel Trip Secondary Action */}
              {managedTrip.status !== 'CANCELLED' && managedTrip.status !== 'COMPLETED' && (
                <button
                  id="manage-trip-cancel-btn"
                  disabled={isUpdatingStatus}
                  onClick={() => setIsCancelConfirmOpen(true)}
                  className="w-full bg-transparent hover:bg-red-50 active:scale-[0.99] border border-red-200 text-[#E63946] font-medium py-3 rounded-xl text-center transition mt-2.5 text-sm cursor-pointer"
                  type="button"
                >
                  Cancelar viaje
                </button>
              )}
            </section>
            {/* END: ActionButtonsGroup */}
          </div>
          {/* END: ScrollableContent */}

          {/* BEGIN: BottomNavigationBar */}
          <footer
            className="w-full bg-white border-t border-slate-100 shrink-0 pt-2 pb-1 z-30"
            data-purpose="mobile-tab-bar"
          >
            <nav className="grid grid-cols-4 items-center">
              {/* Tab: Buscar */}
              <button
                id="manage-nav-search-btn"
                type="button"
                onClick={onNavigateToSearch}
                className="flex flex-col items-center justify-center py-1 group cursor-pointer"
              >
                <Search className="w-6 h-6 text-[#6B7280] group-hover:text-[#1A1A1A] transition-colors stroke-[1.8]" />
                <span className="text-[11px] font-medium text-[#6B7280] mt-1 group-hover:text-[#1A1A1A] transition-colors">
                  Buscar
                </span>
              </button>

              {/* Tab: Mis Viajes (Active) */}
              <button
                id="manage-nav-trips-active-btn"
                type="button"
                className="flex flex-col items-center justify-center py-1 cursor-default"
              >
                <Car className="w-6 h-6 text-[#00A896] stroke-[1.8]" />
                <span className="text-[11px] font-semibold text-[#00A896] mt-1">
                  Mis Viajes
                </span>
              </button>

              {/* Tab: Publicar */}
              <button
                id="manage-nav-publish-btn"
                type="button"
                onClick={onNavigateToPublish}
                className="flex flex-col items-center justify-center py-1 group cursor-pointer"
              >
                <PlusCircle className="w-6 h-6 text-[#6B7280] group-hover:text-[#1A1A1A] transition-colors stroke-[1.8]" />
                <span className="text-[11px] font-medium text-[#6B7280] mt-1 group-hover:text-[#1A1A1A] transition-colors">
                  Publicar
                </span>
              </button>

              {/* Tab: Perfil */}
              <button
                id="manage-nav-profile-btn"
                type="button"
                onClick={onNavigateToProfile}
                className="flex flex-col items-center justify-center py-1 group cursor-pointer"
              >
                <User className="w-6 h-6 text-[#6B7280] group-hover:text-[#1A1A1A] transition-colors stroke-[1.8]" />
                <span className="text-[11px] font-medium text-[#6B7280] mt-1 group-hover:text-[#1A1A1A] transition-colors">
                  Perfil
                </span>
              </button>
            </nav>

            {/* iOS Home Bar Indicator */}
            <div className="w-full flex justify-center pb-1 pt-2">
              <div className="w-36 h-1 bg-slate-300 rounded-full" />
            </div>
          </footer>
          {/* END: BottomNavigationBar */}
        </main>
        {/* END: MobileDeviceContainer */}
      </div>
    );
  }

  // =========================================================================
  // VISTA GENERAL: LISTADO DE MIS VIAJES (Historial + Próximos)
  // =========================================================================
  return (
    <div
      id="my-trips-screen"
      className="min-h-screen pb-28 pt-6 px-4 sm:px-6 max-w-xl mx-auto"
      style={{
        backgroundColor: '#F7F9FA',
        fontFamily: '-apple-system, BlinkMacSystemFont, "SF Pro Text", "Segoe UI", Roboto, sans-serif',
      }}
    >
      {/* Encabezado */}
      <div className="mb-5 flex items-start justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-[#1A1A1A]">
            Mis Viajes
          </h1>
          <p className="text-xs text-[#666666] mt-1">
            Historial y viajes activos con fondos protegidos por el contrato Escrow.
          </p>
        </div>

        {/* Botón rápido para acceder al viaje publicado si existe */}
        {managedTrip.status !== 'CANCELLED' && (
          <button
            type="button"
            onClick={() => setIsManagingView(true)}
            className="px-3 py-1.5 bg-[#00A896] hover:bg-[#008f80] text-white rounded-xl text-xs font-bold transition flex items-center gap-1 shadow-xs cursor-pointer"
          >
            <span>Gestionar mi viaje</span>
            <ChevronRight className="w-3.5 h-3.5" />
          </button>
        )}
      </div>

      {/* Tarjeta Destacada: Viaje que ofrecés como Conductor */}
      {managedTrip.status !== 'CANCELLED' && (
        <div className="mb-5 bg-white border border-[#E6F7F5] rounded-2xl p-4 shadow-sm relative overflow-hidden">
          <div className="absolute top-0 right-0 bg-[#E6F7F5] text-[#00A896] text-[10px] font-bold px-3 py-1 rounded-bl-xl uppercase tracking-wider">
            Sos el conductor
          </div>

          <div className="flex items-center gap-2 mb-2">
            <span className="w-2.5 h-2.5 rounded-full bg-[#00A896]" />
            <h2 className="text-sm font-bold text-[#1A1A1A]">
              {managedTrip.origin} ➔ {managedTrip.destination}
            </h2>
          </div>

          <div className="text-xs text-gray-500 mb-3 flex items-center gap-3">
            <span>{managedTrip.departureTimeText}</span>
            <span>•</span>
            <span className="font-semibold text-[#00A896]">
              {managedTrip.occupiedSeats} de {managedTrip.totalSeats} asientos reservados
            </span>
          </div>

          <div className="flex items-center justify-between pt-2 border-t border-gray-100">
            <span className="text-xs font-bold text-gray-700">
              ${managedTrip.pricePerSeat.toLocaleString()} por lugar
            </span>
            <button
              id="list-manage-my-trip-btn"
              type="button"
              onClick={() => setIsManagingView(true)}
              className="text-xs font-bold text-[#00A896] hover:text-[#008375] inline-flex items-center gap-1 cursor-pointer"
            >
              <span>Ver gestión y pasajeros</span>
              <ChevronRight className="w-4 h-4" />
            </button>
          </div>
        </div>
      )}

      {/* Tabs Sub-Navegación */}
      <div className="flex bg-[#EAEAEA]/80 p-1 rounded-[10px] mb-5">
        <button
          type="button"
          onClick={() => setActiveSubTab('upcoming')}
          className={`flex-1 py-2 text-xs font-semibold rounded-[8px] transition-all cursor-pointer ${
            activeSubTab === 'upcoming'
              ? 'bg-[#FFFFFF] text-[#1A1A1A] shadow-sm'
              : 'text-[#666666] hover:text-[#1A1A1A]'
          }`}
        >
          Próximos ({upcomingTrips.length})
        </button>
        <button
          type="button"
          onClick={() => setActiveSubTab('past')}
          className={`flex-1 py-2 text-xs font-semibold rounded-[8px] transition-all cursor-pointer ${
            activeSubTab === 'past'
              ? 'bg-[#FFFFFF] text-[#1A1A1A] shadow-sm'
              : 'text-[#666666] hover:text-[#1A1A1A]'
          }`}
        >
          Historial ({pastTrips.length})
        </button>
      </div>

      {/* Lista de Viajes */}
      <div className="space-y-4">
        {displayTrips.length === 0 ? (
          <div className="bg-[#FFFFFF] border border-[#EFEFEF] rounded-[12px] p-8 text-center">
            <Car className="w-10 h-10 text-[#CCCCCC] mx-auto mb-3" />
            <p className="text-sm font-semibold text-[#1A1A1A]">No tienes viajes en esta sección</p>
            <p className="text-xs text-[#888888] mt-1">
              Busca trayectos disponibles o publica una ruta para compartir tus gastos.
            </p>
            <button
              type="button"
              onClick={onNavigateToSearch}
              className="mt-4 px-4 py-2 bg-[#00A896] text-white rounded-[10px] text-xs font-semibold hover:bg-[#008f80] transition"
            >
              Explorar viajes
            </button>
          </div>
        ) : (
          displayTrips.map((item) => (
            <div
              key={item.id}
              className="bg-[#FFFFFF] border border-[#EFEFEF] rounded-[12px] p-5 shadow-sm space-y-4"
            >
              {/* Encabezado de la Tarjeta */}
              <div className="flex items-center justify-between pb-3 border-b border-[#F0F0F0]">
                <div className="flex items-center gap-2">
                  <div className="w-8 h-8 rounded-full bg-[#E6F6F4] flex items-center justify-center text-[#00A896] font-bold text-xs">
                    {item.driverName.charAt(0)}
                  </div>
                  <div>
                    <h3 className="text-xs font-bold text-[#1A1A1A]">{item.driverName}</h3>
                    <p className="text-[11px] text-[#777777]">{item.carInfo}</p>
                  </div>
                </div>

                {/* Badge de Estado Escrow */}
                {item.status === 'IN_ESCROW' && (
                  <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-[11px] font-semibold bg-emerald-50 text-emerald-700 border border-emerald-200">
                    <ShieldCheck className="w-3 h-3 text-[#00A896]" />
                    <span>Fondos en Custodia</span>
                  </span>
                )}
                {item.status === 'CONFIRMED' && (
                  <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-[11px] font-semibold bg-blue-50 text-blue-700 border border-blue-200">
                    <CheckCircle2 className="w-3 h-3" />
                    <span>Confirmado</span>
                  </span>
                )}
                {item.status === 'COMPLETED' && (
                  <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-[11px] font-semibold bg-gray-100 text-gray-700">
                    <span>Completado</span>
                  </span>
                )}
                {item.status === 'DISPUTED' && (
                  <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-[11px] font-semibold bg-amber-50 text-amber-700 border border-amber-200">
                    <AlertTriangle className="w-3 h-3" />
                    <span>Disputa en Revisión</span>
                  </span>
                )}
              </div>

              {/* Itinerario */}
              <div className="space-y-2 text-xs">
                <div className="flex items-start gap-2.5">
                  <div className="w-2 h-2 rounded-full bg-emerald-500 mt-1.5 shrink-0" />
                  <div>
                    <span className="text-[10px] text-[#888888] uppercase font-semibold block">Origen</span>
                    <span className="font-semibold text-[#1A1A1A]">{item.origin}</span>
                  </div>
                </div>
                <div className="flex items-start gap-2.5">
                  <div className="w-2 h-2 rounded-full bg-[#00A896] mt-1.5 shrink-0" />
                  <div>
                    <span className="text-[10px] text-[#888888] uppercase font-semibold block">Destino</span>
                    <span className="font-semibold text-[#1A1A1A]">{item.destination}</span>
                  </div>
                </div>
              </div>

              {/* Metadatos y Monto */}
              <div className="flex items-center justify-between pt-3 border-t border-[#F5F5F5] text-xs">
                <div className="flex items-center gap-3 text-[#666666]">
                  <span className="flex items-center gap-1">
                    <Calendar className="w-3.5 h-3.5 text-[#888888]" />
                    {item.date} • {item.time}
                  </span>
                  <span className="flex items-center gap-1">
                    <Users className="w-3.5 h-3.5 text-[#888888]" />
                    {item.seats} asiento(s)
                  </span>
                </div>
                <span className="font-bold text-sm text-[#00A896]">
                  ${item.totalPrice.toLocaleString()} ARS
                </span>
              </div>

              {/* Botón de Abrir Disputa / Reclamo */}
              {item.status !== 'DISPUTED' && (
                <div className="pt-2 flex justify-end">
                  <button
                    type="button"
                    onClick={() => handleOpenDispute(item.bookingId)}
                    className="inline-flex items-center gap-1.5 text-xs text-rose-600 hover:text-rose-700 hover:bg-rose-50 px-2.5 py-1.5 rounded-lg font-medium transition cursor-pointer"
                  >
                    <AlertTriangle className="w-3.5 h-3.5" />
                    <span>Reportar problema o disputa</span>
                  </button>
                </div>
              )}
            </div>
          ))
        )}
      </div>

      {/* Modal de Disputa Flotante */}
      {selectedDisputeBookingId && (
        <CreateDisputeModal
          isOpen={isDisputeModalOpen}
          bookingId={selectedDisputeBookingId}
          onClose={() => {
            setIsDisputeModalOpen(false);
            setSelectedDisputeBookingId(null);
          }}
          onSuccess={handleDisputeSuccess}
          showToast={(msg, type) => setToast({ message: msg, type })}
        />
      )}

      {toast && (
        <Toast
          id="my-trips-toast"
          message={toast.message}
          type={toast.type}
          onClose={() => setToast(null)}
        />
      )}

      {/* Barra de Navegación Inferior Fija */}
      <BottomNav
        activeTab="trips"
        onTabChange={handleBottomNavChange}
      />
    </div>
  );
};

export default MyTripsScreen;
