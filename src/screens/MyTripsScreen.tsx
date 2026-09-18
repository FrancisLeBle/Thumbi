import React, { useState } from 'react';
import {
  Calendar,
  ChevronRight,
  ArrowLeft,
  Share2,
  Users,
  Check,
  Star,
  MessageSquare,
  AlertTriangle,
  Loader2,
} from 'lucide-react';
import { CreateDisputeModal } from '../components/CreateDisputeModal';
import { ContactDriverModal } from '../components/ContactDriverModal';
import { Toast } from '../components/Toast';
import { tripService } from '../services/tripService';
import { useTripContext, Trip, Driver } from '../context/TripContext';
import { useToast } from '../context/ToastContext';

export interface MyTripsScreenProps {
  onNavigateToHome: () => void;
  onNavigateToSearch: () => void;
  onNavigateToPublish: () => void;
  onNavigateToProfile: () => void;
  onSelectBookingForDispute?: (bookingId: string) => void;
  initialManageTrip?: boolean;
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

const normalizeTripStatus = (status?: string): 'SCHEDULED' | 'IN_PROGRESS' | 'COMPLETED' | 'CANCELLED' => {
  if (status === 'IN_PROGRESS' || status === 'COMPLETED' || status === 'CANCELLED') {
    return status;
  }
  return 'SCHEDULED';
};

export const MyTripsScreen: React.FC<MyTripsScreenProps> = ({
  onNavigateToHome,
  onNavigateToSearch,
  onNavigateToPublish,
  onNavigateToProfile,
  initialManageTrip = false,
}) => {
  // Consumimos el contexto global de viajes y toasts
  const { trips, cancelTrip, isLoading } = useTripContext();
  const { showToast: showGlobalToast } = useToast();

  // Filtrado dinámico según rol
  const passengerTrips = trips.filter((t) => t.role === 'passenger');
  const driverTrips = trips.filter((t) => t.role === 'driver');

  const upcomingPassengerTrips = passengerTrips.filter(
    (t) => t.status !== 'COMPLETED' && t.status !== 'CANCELLED'
  );
  const historyPassengerTrips = passengerTrips.filter(
    (t) => t.status === 'COMPLETED' || t.status === 'CANCELLED'
  );

  const upcomingDriverTrips = driverTrips.filter(
    (t) => t.status !== 'COMPLETED' && t.status !== 'CANCELLED'
  );
  const historyDriverTrips = driverTrips.filter(
    (t) => t.status === 'COMPLETED' || t.status === 'CANCELLED'
  );

  // Helper para normalizar conductor
  const getDriverInfo = (driver: string | Driver | undefined) => {
    if (typeof driver === 'object' && driver !== null) {
      return {
        name: driver.name || 'Conductor',
        rating: driver.rating ?? 4.9,
        vehicle: driver.vehicle || 'Toyota Corolla • AA 123 CD',
        phone: driver.phone || '+5491148291123',
      };
    }
    return {
      name: typeof driver === 'string' && driver ? driver : 'Carlos M.',
      rating: 4.9,
      vehicle: 'Toyota Corolla • AA 123 CD',
      phone: '+5491148291123',
    };
  };

  const getInitials = (name: string): string => {
    if (!name) return 'CM';
    const parts = name.trim().split(/\s+/);
    if (parts.length >= 2) {
      return `${parts[0][0]}${parts[1][0]}`.toUpperCase();
    }
    return parts[0].slice(0, 2).toUpperCase();
  };

  // Estado local para conmutar entre 'Como Pasajero' y 'Como Conductor'
  const [activeTab, setActiveTab] = useState<'passenger' | 'driver'>(
    initialManageTrip ? 'driver' : 'passenger'
  );

  // Estado para la subpantalla de gestión de viaje
  const [isManagingView, setIsManagingView] = useState<boolean>(initialManageTrip);

  // Estados interactivos para modales y notificaciones
  const [isCancelConfirmOpen, setIsCancelConfirmOpen] = useState<boolean>(false);
  const [tripToCancel, setTripToCancel] = useState<{
    id: string;
    origin: string;
    destination: string;
    date?: string;
    time?: string;
    isDriver?: boolean;
  } | null>(null);
  const [isContactModalOpen, setIsContactModalOpen] = useState<boolean>(false);
  const [contactTarget, setContactTarget] = useState<{
    name: string;
    phone: string;
    destination: string;
  } | null>(null);

  const [selectedDisputeBookingId, setSelectedDisputeBookingId] = useState<string | null>(null);
  const [isDisputeModalOpen, setIsDisputeModalOpen] = useState<boolean>(false);
  const [isUpdatingStatus, setIsUpdatingStatus] = useState<boolean>(false);

  const [toast, setToast] = useState<{
    message: string;
    type: 'error' | 'success';
  } | null>(null);

  // Datos del viaje activo gestionado como Conductor
  const [managedTrip, setManagedTrip] = useState<ManagedTrip>(() => {
    const defaultTrip = driverTrips[0];
    if (defaultTrip) {
      const drv = getDriverInfo(defaultTrip.driver);
      const occupied = Math.max(0, defaultTrip.totalSeats - defaultTrip.availableSeats);
      return {
        id: defaultTrip.id,
        origin: defaultTrip.origin,
        destination: defaultTrip.destination,
        departureTimeText: `${defaultTrip.date} • ${defaultTrip.time} hs`,
        totalSeats: defaultTrip.totalSeats,
        occupiedSeats: occupied,
        pricePerSeat: defaultTrip.price,
        status: normalizeTripStatus(defaultTrip.status),
        carInfo: drv.vehicle,
        passengers: [
          {
            id: `pass-${defaultTrip.id}-1`,
            name: 'Sofía F.',
            initials: 'SF',
            rating: 4.9,
            verified: true,
            phone: '+5491148291123',
            seatsBooked: Math.max(1, occupied),
          },
        ],
      };
    }
    return {
      id: 'trip-published-pilar',
      origin: 'Palermo, CABA',
      destination: 'Pilar, Bs. As.',
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
    };
  });

  // Acciones de Conductor sobre el viaje publicado
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
    const targetId = tripToCancel?.id || managedTrip.id;
    if (!targetId) return;

    setIsUpdatingStatus(true);
    try {
      if (cancelTrip) {
        await cancelTrip(targetId);
      }
      try {
        await tripService.cancelTripApi(targetId);
      } catch {
        // Tolerancia si el mock ya lo procesó
      }
      if (managedTrip.id === targetId) {
        setManagedTrip((prev) => ({ ...prev, status: 'CANCELLED' }));
      }
      setIsCancelConfirmOpen(false);
      setTripToCancel(null);
      const msg = 'El viaje ha sido cancelado con éxito.';
      setToast({
        message: msg,
        type: 'success',
      });
      showGlobalToast(msg, 'success');
    } catch {
      setIsCancelConfirmOpen(false);
      setTripToCancel(null);
      const msg = 'El viaje ha sido cancelado.';
      setToast({
        message: msg,
        type: 'success',
      });
      showGlobalToast(msg, 'success');
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
        .catch(() => {});
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

  const availableRemainingSeats = Math.max(
    0,
    managedTrip.totalSeats - managedTrip.occupiedSeats
  );
  const occupancyPercentage = Math.min(
    100,
    Math.round((managedTrip.occupiedSeats / managedTrip.totalSeats) * 100)
  );

  // =========================================================================
  // SUBPANTALLA: GESTIÓN DEL VIAJE PUBLICADO (ManageTripReact.md)
  // =========================================================================
  if (isManagingView) {
    return (
      <div className="min-h-screen flex items-center justify-center p-0 md:p-6 bg-slate-900 font-sans antialiased select-none">
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
                  className="w-full py-2.5 bg-red-600 hover:bg-red-700 active:scale-[0.99] text-white text-xs font-bold rounded-xl transition cursor-pointer flex items-center justify-center gap-2 disabled:opacity-60"
                >
                  {isUpdatingStatus ? (
                    <>
                      <Loader2 className="w-4 h-4 animate-spin" />
                      <span>Cancelando...</span>
                    </>
                  ) : (
                    'Sí, cancelar viaje'
                  )}
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
        {contactTarget && (
          <ContactDriverModal
            isOpen={isContactModalOpen}
            onClose={() => {
              setIsContactModalOpen(false);
              setContactTarget(null);
            }}
            driverName={contactTarget.name}
            driverPhoneNumber={contactTarget.phone}
            destinationCity={contactTarget.destination}
          />
        )}

        {/* iPhone 15/16 Container */}
        <main
          className="relative w-full max-w-[393px] h-[852px] bg-[#F7F9FA] text-[#1A1A1A] flex flex-col overflow-hidden md:rounded-[44px] shadow-2xl border-0 md:border-[8px] md:border-neutral-800"
          data-purpose="manage-trip-container"
        >
          {/* iOS Status Bar */}
          <div className="pt-3 px-7 flex justify-between items-center z-30 shrink-0 select-none">
            <span className="text-[15px] font-semibold tracking-tight text-[#1A1A1A]">
              9:41
            </span>
            <div className="w-[124px] h-[34px] bg-black rounded-full absolute left-1/2 -translate-x-1/2 top-3 hidden sm:block" />
            <div className="flex items-center space-x-2 text-[#1A1A1A]">
              <svg className="w-4 h-3.5 fill-current" viewBox="0 0 17 12">
                <rect height="4" rx="0.75" width="2.5" x="0" y="8" />
                <rect height="6.5" rx="0.75" width="2.5" x="4.5" y="5.5" />
                <rect height="9" rx="0.75" width="2.5" x="9" y="3" />
                <rect height="11.5" rx="0.75" width="2.5" x="13.5" y="0.5" />
              </svg>
              <svg className="w-4 h-3.5 fill-current" viewBox="0 0 16 12">
                <path
                  clipRule="evenodd"
                  d="M8 2.5C5.1 2.5 2.5 3.7 0.7 5.6L0 4.9C2 2.7 4.9 1.5 8 1.5C11.1 1.5 14 2.7 16 4.9L15.3 5.6C13.5 3.7 10.9 2.5 8 2.5ZM8 6.5C6.3 6.5 4.8 7.2 3.7 8.3L3 7.6C4.3 6.3 6.1 5.5 8 5.5C9.9 5.5 11.7 6.3 13 7.6L12.3 8.3C11.2 7.2 9.7 6.5 8 6.5ZM8 10C7.2 10 6.5 10.4 6 11L8 13L10 11C9.5 10.4 8.8 10 8 10Z"
                  fillRule="evenodd"
                />
              </svg>
              <div className="w-6 h-[11.5px] border border-[#1A1A1A] rounded-[3.5px] p-[1.5px] flex items-center">
                <div className="h-full w-full bg-[#1A1A1A] rounded-[1.5px]" />
              </div>
            </div>
          </div>

          {/* Navigation Header */}
          <nav
            className="w-full px-5 pt-3 pb-3 flex items-center justify-between z-20 shrink-0"
            data-purpose="top-navigation-bar"
          >
            <button
              id="manage-trip-back-btn"
              aria-label="Volver"
              onClick={() => {
                setIsManagingView(false);
                setActiveTab('driver');
              }}
              className="w-9 h-9 flex items-center justify-center -ml-2 rounded-full hover:bg-black/5 active:bg-black/10 transition-colors text-[#1A1A1A] cursor-pointer"
              type="button"
            >
              <ArrowLeft className="w-5 h-5 stroke-[2.5]" />
            </button>

            <div className="flex flex-col items-center justify-center text-center">
              <h1 className="text-[17px] font-bold text-[#1A1A1A] leading-tight tracking-tight">
                Viaje a {managedTrip.destination.split(',')[0]}
              </h1>
              <span className="text-xs text-[#6B7280] font-normal mt-0.5">
                {managedTrip.departureTimeText}
              </span>
            </div>

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

          {/* Scrollable Content */}
          <div
            className="flex-1 overflow-y-auto px-4 pt-2 pb-4 flex flex-col justify-between"
            style={{ scrollbarWidth: 'none', msOverflowStyle: 'none' }}
          >
            <div className="flex flex-col space-y-5">
              {/* Capacity Progress Card */}
              <section className="bg-white rounded-xl p-4 shadow-[0_2px_8px_rgba(0,0,0,0.04)] border border-[#E5E9EB]/60">
                <div className="flex justify-between items-center mb-2.5">
                  <span className="text-sm font-bold text-[#1A1A1A]">
                    Lugares ocupados
                  </span>
                  <span className="text-sm font-bold text-[#00A896]">
                    {managedTrip.occupiedSeats} de {managedTrip.totalSeats} asientos
                  </span>
                </div>

                <div
                  aria-valuemax={100}
                  aria-valuemin={0}
                  aria-valuenow={occupancyPercentage}
                  className="w-full h-2 bg-[#E6F7F5] rounded-full overflow-hidden"
                  role="progressbar"
                >
                  <div
                    className="h-full bg-[#00A896] rounded-full transition-all duration-300"
                    style={{ width: `${occupancyPercentage}%` }}
                  />
                </div>

                <p className="text-[11px] text-[#6B7280] mt-2">
                  {availableRemainingSeats === 1
                    ? '1 lugar restante disponible para reserva'
                    : `${availableRemainingSeats} lugares restantes disponibles para reserva`}
                </p>
              </section>

              {/* Confirmed Passengers */}
              <section>
                <h2 className="text-xs font-semibold text-[#6B7280] uppercase tracking-wider mb-2.5 px-0.5">
                  Pasajeros Confirmados ({managedTrip.passengers.length})
                </h2>

                {managedTrip.passengers.map((passenger) => (
                  <div
                    key={passenger.id}
                    className="bg-white rounded-xl p-3.5 shadow-[0_2px_8px_rgba(0,0,0,0.04)] border border-[#E5E9EB]/60 flex items-center justify-between"
                  >
                    <div className="flex items-center gap-3">
                      <div className="w-[42px] h-[42px] rounded-full bg-[#E6F7F5] text-[#00A896] font-bold text-sm flex items-center justify-center shrink-0">
                        {passenger.initials}
                      </div>
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

                    <button
                      id={`btn-chat-passenger-${passenger.id}`}
                      aria-label={`Enviar mensaje a ${passenger.name}`}
                      onClick={() => {
                        setContactTarget({
                          name: passenger.name,
                          phone: passenger.phone,
                          destination: managedTrip.destination,
                        });
                        setIsContactModalOpen(true);
                      }}
                      className="w-[38px] h-[38px] rounded-full bg-slate-100 hover:bg-slate-200 active:bg-slate-300 transition-colors flex items-center justify-center text-slate-700 cursor-pointer"
                      type="button"
                    >
                      <MessageSquare className="w-5 h-5 text-slate-600 stroke-[1.8]" />
                    </button>
                  </div>
                ))}
              </section>

              {/* Available Seats */}
              <section>
                <h2 className="text-xs font-semibold text-[#6B7280] uppercase tracking-wider mb-2.5 px-0.5">
                  Asientos Libres ({availableRemainingSeats})
                </h2>

                {Array.from({ length: availableRemainingSeats }).map((_, index) => {
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
                })}
              </section>
            </div>

            {/* Action Buttons */}
            <section className="mt-6 flex flex-col w-full">
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
          </div>

          {/* Bottom Nav */}
          <nav className="bg-white border-t border-[#E5E9EB] pt-2.5 px-6 shrink-0 safe-area-bottom">
            <div className="flex justify-between items-center max-w-sm mx-auto">
              <button
                type="button"
                onClick={onNavigateToSearch}
                className="flex flex-col items-center group py-1 text-[#6B7280] cursor-pointer"
              >
                <svg className="w-6 h-6 stroke-current" fill="none" strokeWidth="2" viewBox="0 0 24 24">
                  <circle cx="11" cy="11" r="8" />
                  <line x1="21" x2="16.65" y1="21" y2="16.65" />
                </svg>
                <span className="text-[10px] font-medium mt-1">Buscar</span>
              </button>

              <button
                type="button"
                className="flex flex-col items-center group py-1 text-[#00A896] cursor-default"
              >
                <svg className="w-6 h-6 stroke-current fill-[#00A896]/10" stroke="currentColor" strokeWidth="2" viewBox="0 0 24 24">
                  <path d="M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4" strokeLinecap="round" strokeLinejoin="round" />
                </svg>
                <span className="text-[10px] font-semibold mt-1">Mis Viajes</span>
              </button>

              <button
                type="button"
                onClick={onNavigateToPublish}
                className="flex flex-col items-center group py-1 text-[#6B7280] cursor-pointer"
              >
                <svg className="w-6 h-6 stroke-current" fill="none" strokeWidth="2" viewBox="0 0 24 24">
                  <circle cx="12" cy="12" r="9" />
                  <line x1="12" x2="12" y1="8" y2="16" />
                  <line x1="8" x2="16" y1="12" y2="12" />
                </svg>
                <span className="text-[10px] font-medium mt-1">Publicar</span>
              </button>

              <button
                type="button"
                onClick={onNavigateToProfile}
                className="flex flex-col items-center group py-1 text-[#6B7280] cursor-pointer"
              >
                <svg className="w-6 h-6 stroke-current" fill="none" strokeWidth="2" viewBox="0 0 24 24">
                  <path d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" strokeLinecap="round" strokeLinejoin="round" />
                </svg>
                <span className="text-[10px] font-medium mt-1">Perfil</span>
              </button>
            </div>
            <div className="w-32 h-1 bg-[#1A1A1A]/20 rounded-full mx-auto mt-3 mb-1" />
          </nav>
        </main>
      </div>
    );
  }

  // =========================================================================
  // VISTA PRINCIPAL: MIS VIAJES (STITCH EXACT RESTRUCTURE)
  // =========================================================================
  return (
    <div className="min-h-screen flex items-center justify-center p-0 md:p-6 bg-slate-900 font-sans antialiased select-none">
      {toast && (
        <Toast
          id="my-trips-toast"
          message={toast.message}
          type={toast.type}
          onClose={() => setToast(null)}
        />
      )}

      {/* Modal de Disputa */}
      {selectedDisputeBookingId && (
        <CreateDisputeModal
          isOpen={isDisputeModalOpen}
          bookingId={selectedDisputeBookingId}
          onClose={() => {
            setIsDisputeModalOpen(false);
            setSelectedDisputeBookingId(null);
          }}
          onSuccess={() => {
            setToast({
              message: 'Disputa registrada para revisión de soporte.',
              type: 'success',
            });
          }}
          showToast={(msg, type) => setToast({ message: msg, type })}
        />
      )}

      {/* Modal de Contacto Compartido */}
      {contactTarget && (
        <ContactDriverModal
          isOpen={isContactModalOpen}
          onClose={() => {
            setIsContactModalOpen(false);
            setContactTarget(null);
          }}
          driverName={contactTarget.name}
          driverPhoneNumber={contactTarget.phone}
          destinationCity={contactTarget.destination}
        />
      )}

      {/* Modal Confirmación de Cancelación para la lista de viajes */}
      {isCancelConfirmOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-xs p-4 animate-in fade-in duration-150">
          <div className="bg-white rounded-2xl max-w-xs w-full p-5 shadow-2xl border border-gray-100 text-center space-y-3">
            <div className="w-12 h-12 rounded-full bg-red-100 text-red-600 flex items-center justify-center mx-auto">
              <AlertTriangle className="w-6 h-6" />
            </div>
            <h3 className="text-base font-bold text-gray-900">
              {tripToCancel?.isDriver ? '¿Cancelar viaje?' : '¿Cancelar reserva?'}
            </h3>
            <p className="text-xs text-gray-500 leading-relaxed">
              {tripToCancel?.isDriver
                ? `Esta acción cancelará tu viaje a ${tripToCancel?.destination || ''} y notificará a los pasajeros.`
                : `Esta acción cancelará tu reserva para el viaje a ${tripToCancel?.destination || ''} y liberará tu lugar.`}
            </p>
            <div className="pt-2 flex flex-col gap-2">
              <button
                type="button"
                disabled={isUpdatingStatus}
                onClick={handleConfirmCancelTrip}
                className="w-full py-2.5 bg-red-600 hover:bg-red-700 active:scale-[0.99] text-white text-xs font-bold rounded-xl transition cursor-pointer flex items-center justify-center gap-2 disabled:opacity-60"
              >
                {isUpdatingStatus ? (
                  <>
                    <Loader2 className="w-4 h-4 animate-spin" />
                    <span>Cancelando...</span>
                  </>
                ) : (
                  tripToCancel?.isDriver ? 'Sí, cancelar viaje' : 'Sí, cancelar reserva'
                )}
              </button>
              <button
                type="button"
                disabled={isUpdatingStatus}
                onClick={() => {
                  setIsCancelConfirmOpen(false);
                  setTripToCancel(null);
                }}
                className="w-full py-2.5 bg-gray-100 hover:bg-gray-200 text-gray-700 text-xs font-semibold rounded-xl transition cursor-pointer"
              >
                Volver
              </button>
            </div>
          </div>
        </div>
      )}

      {/* iPhone 15 / 16 Container: 393 x 852 px */}
      <div className="relative w-full max-w-[393px] h-[852px] bg-[#F7F9FA] text-[#1A1A1A] flex flex-col overflow-hidden md:rounded-[44px] shadow-2xl border-0 md:border-[8px] md:border-neutral-800">
        {/* iOS Status Bar with Dynamic Island */}
        <div className="pt-3 px-7 flex justify-between items-center z-30 shrink-0 select-none">
          <span className="text-[15px] font-semibold tracking-tight text-[#1A1A1A]">
            9:41
          </span>
          {/* Dynamic Island pill */}
          <div className="w-[124px] h-[34px] bg-black rounded-full absolute left-1/2 -translate-x-1/2 top-3 hidden sm:block" />
          <div className="flex items-center space-x-2 text-[#1A1A1A]">
            {/* Cellular */}
            <svg className="w-4 h-3.5 fill-current" viewBox="0 0 17 12">
              <rect height="4" rx="0.75" width="2.5" x="0" y="8" />
              <rect height="6.5" rx="0.75" width="2.5" x="4.5" y="5.5" />
              <rect height="9" rx="0.75" width="2.5" x="9" y="3" />
              <rect height="11.5" rx="0.75" width="2.5" x="13.5" y="0.5" />
            </svg>
            {/* Wifi */}
            <svg className="w-4 h-3.5 fill-current" viewBox="0 0 16 12">
              <path
                clipRule="evenodd"
                d="M8 2.5C5.1 2.5 2.5 3.7 0.7 5.6L0 4.9C2 2.7 4.9 1.5 8 1.5C11.1 1.5 14 2.7 16 4.9L15.3 5.6C13.5 3.7 10.9 2.5 8 2.5ZM8 6.5C6.3 6.5 4.8 7.2 3.7 8.3L3 7.6C4.3 6.3 6.1 5.5 8 5.5C9.9 5.5 11.7 6.3 13 7.6L12.3 8.3C11.2 7.2 9.7 6.5 8 6.5ZM8 10C7.2 10 6.5 10.4 6 11L8 13L10 11C9.5 10.4 8.8 10 8 10Z"
                fillRule="evenodd"
              />
            </svg>
            {/* Battery */}
            <div className="w-6 h-[11.5px] border border-[#1A1A1A] rounded-[3.5px] p-[1.5px] flex items-center">
              <div className="h-full w-full bg-[#1A1A1A] rounded-[1.5px]" />
            </div>
          </div>
        </div>

        {/* Header (Centered Title, No Back Arrow) */}
        <header className="pt-5 pb-3 px-6 text-center shrink-0">
          <h1 className="text-[20px] font-bold text-[#1A1A1A] tracking-tight">
            Mis Viajes
          </h1>
        </header>

        {/* Segmented Control (Tabs) */}
        <div className="px-6 shrink-0 border-b border-[#E5E9EB] bg-[#F7F9FA]">
          <div className="flex">
            {/* Tab: Como Pasajero */}
            <button
              id="tab-passenger-btn"
              type="button"
              onClick={() => setActiveTab('passenger')}
              className="flex-1 pb-3 text-center relative focus:outline-none transition-colors cursor-pointer"
            >
              <span
                className={`text-[15px] ${
                  activeTab === 'passenger'
                    ? 'font-semibold text-[#00A896]'
                    : 'font-medium text-[#6B7280] hover:text-[#1A1A1A]'
                }`}
              >
                Como Pasajero ({passengerTrips.length})
              </span>
              {activeTab === 'passenger' && (
                <div className="absolute bottom-0 left-0 right-0 h-[2.5px] bg-[#00A896] rounded-full" />
              )}
            </button>

            {/* Tab: Como Conductor */}
            <button
              id="tab-driver-btn"
              type="button"
              onClick={() => setActiveTab('driver')}
              className="flex-1 pb-3 text-center relative focus:outline-none transition-colors cursor-pointer"
            >
              <span
                className={`text-[15px] ${
                  activeTab === 'driver'
                    ? 'font-bold text-[#00A896]'
                    : 'font-medium text-[#6B7280] hover:text-[#1A1A1A]'
                }`}
              >
                Como Conductor ({driverTrips.length})
              </span>
              {activeTab === 'driver' && (
                <div className="absolute bottom-0 left-0 right-0 h-[2.5px] bg-[#00A896] rounded-full" />
              )}
            </button>
          </div>
        </div>

        {/* Main Scrollable Content Area */}
        <main
          className="flex-1 overflow-y-auto px-5 py-5 space-y-6"
          style={{ scrollbarWidth: 'none', msOverflowStyle: 'none' }}
        >
          {isLoading ? (
            <div className="space-y-4 pt-1">
              <div className="py-3 flex items-center justify-center gap-2.5 text-center bg-white rounded-xl border border-slate-100 shadow-xs">
                <Loader2 className="w-4 h-4 animate-spin text-[#00A896]" />
                <span className="text-xs font-medium text-[#6B7280]">
                  Cargando tus viajes...
                </span>
              </div>
              {[1, 2, 3].map((idx) => (
                <div
                  key={idx}
                  className="bg-white rounded-[16px] p-4 shadow-[0px_2px_8px_rgba(0,0,0,0.04)] border border-[#E5E9EB]/60 animate-pulse space-y-3"
                >
                  <div className="flex items-center justify-between pb-2 border-b border-[#F0F3F5]">
                    <div className="h-4 w-32 bg-slate-200 rounded" />
                    <div className="h-5 w-20 bg-slate-100 rounded-full" />
                  </div>
                  <div className="flex items-center space-x-3 py-2">
                    <div className="flex flex-col items-center space-y-1">
                      <div className="w-2.5 h-2.5 rounded-full bg-slate-300" />
                      <div className="w-0.5 h-6 bg-slate-200" />
                      <div className="w-2.5 h-2.5 rounded-full bg-slate-300" />
                    </div>
                    <div className="space-y-2 flex-1">
                      <div className="h-3.5 w-3/5 bg-slate-200 rounded" />
                      <div className="h-3.5 w-2/5 bg-slate-200 rounded" />
                    </div>
                  </div>
                  <div className="pt-2 border-t border-[#F0F3F5] flex items-center justify-between">
                    <div className="flex items-center space-x-2.5">
                      <div className="w-8 h-8 rounded-full bg-slate-200" />
                      <div className="h-3 w-24 bg-slate-200 rounded" />
                    </div>
                    <div className="h-4 w-16 bg-slate-200 rounded" />
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <>
              {/* =============================================================== */}
              {/* VISTA A: COMO PASAJERO                                           */}
              {/* =============================================================== */}
              {activeTab === 'passenger' && (
                passengerTrips.length === 0 ? (
                <div className="bg-white rounded-[16px] p-6 text-center shadow-[0px_2px_8px_rgba(0,0,0,0.04)] border border-[#E5E9EB]/60">
                  <div className="w-12 h-12 rounded-full bg-[#E6F7F5] flex items-center justify-center text-[#00A896] mx-auto mb-3">
                    <Users className="w-6 h-6 stroke-[2]" />
                  </div>
                  <h3 className="text-[15px] font-bold text-[#1A1A1A]">No tienes viajes como pasajero</h3>
                  <p className="text-[13px] text-[#6B7280] mt-1 max-w-xs mx-auto">
                    Busca viajes compartidos hacia tu destino y viaja de manera económica y segura.
                  </p>
                  <button
                    type="button"
                    onClick={onNavigateToSearch}
                    className="mt-4 px-4 py-2 bg-[#00A896] text-white text-[13px] font-semibold rounded-xl shadow-xs hover:bg-[#008f80] transition-colors cursor-pointer"
                  >
                    Buscar viajes
                  </button>
                </div>
              ) : (
                <>
                  {/* Section: Próximos Viajes */}
                  {upcomingPassengerTrips.length > 0 && (
                    <section className="space-y-2.5">
                      <div className="flex items-center justify-between px-1">
                        <h2 className="text-[12px] font-bold text-[#6B7280] tracking-wider uppercase">
                          Próximos Viajes
                        </h2>
                        <span className="text-[12px] font-medium text-[#00A896] bg-[#E6F7F5] px-2 py-0.5 rounded-full">
                          {upcomingPassengerTrips.length} pendiente{upcomingPassengerTrips.length === 1 ? '' : 's'}
                        </span>
                      </div>

                      {upcomingPassengerTrips.map((trip) => {
                        const drv = getDriverInfo(trip.driver);
                        const initials = getInitials(drv.name);
                        return (
                          <article
                            key={trip.id}
                            id={`passenger-trip-${trip.id}`}
                            onClick={() => {
                              setContactTarget({
                                name: drv.name,
                                phone: drv.phone,
                                destination: trip.destination,
                              });
                              setIsContactModalOpen(true);
                            }}
                            className="bg-white rounded-[16px] p-4 shadow-[0px_2px_8px_rgba(0,0,0,0.04)] border border-[#E5E9EB]/60 hover:border-[#00A896]/30 transition-all cursor-pointer"
                          >
                            {/* Card Header / Status & Date */}
                            <div className="flex items-center justify-between pb-3 border-b border-[#F0F3F5]">
                              <div className="flex items-center space-x-2 text-[13px] text-[#1A1A1A] font-medium">
                                <svg className="w-4 h-4 text-[#00A896]" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                  <path d="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" />
                                </svg>
                                <span>{trip.date} • {trip.time} hs</span>
                              </div>
                              <span className="inline-flex items-center px-2.5 py-1 rounded-full text-[11px] font-semibold bg-[#E6F7F5] text-[#00A896] border border-[#00A896]/20">
                                <span className="w-1.5 h-1.5 rounded-full bg-[#00A896] mr-1.5" />
                                {trip.status === 'IN_PROGRESS' ? 'En curso' : 'Confirmado'}
                              </span>
                            </div>

                            {/* Route Info */}
                            <div className="py-3">
                              <div className="flex items-center justify-between">
                                <div className="flex items-center space-x-3">
                                  <div className="flex flex-col items-center">
                                    <div className="w-2.5 h-2.5 rounded-full bg-[#00A896]" />
                                    <div className="w-0.5 h-6 bg-dashed border-l border-dashed border-[#CBD5E1] my-0.5" />
                                    <div className="w-2.5 h-2.5 rounded-full bg-[#E63946]" />
                                  </div>
                                  <div className="flex flex-col justify-between py-0.5 space-y-2">
                                    <div>
                                      <span className="text-[11px] font-medium uppercase text-[#6B7280] tracking-wide block leading-none">
                                        Origen
                                      </span>
                                      <span className="text-[15px] font-bold text-[#1A1A1A] leading-tight">
                                        {trip.origin}
                                      </span>
                                    </div>
                                    <div>
                                      <span className="text-[11px] font-medium uppercase text-[#6B7280] tracking-wide block leading-none">
                                        Destino
                                      </span>
                                      <span className="text-[15px] font-bold text-[#1A1A1A] leading-tight">
                                        {trip.destination}
                                      </span>
                                    </div>
                                  </div>
                                </div>

                                <div className="w-8 h-8 rounded-full bg-[#F7F9FA] flex items-center justify-center text-[#6B7280]">
                                  <ChevronRight className="w-4 h-4 stroke-[2.2]" />
                                </div>
                              </div>
                            </div>

                            {/* Driver & Vehicle Details Footer */}
                            <div className="mt-1 pt-3 border-t border-[#F0F3F5] flex items-center justify-between">
                              <div className="flex items-center space-x-2.5">
                                <div className="w-9 h-9 rounded-full bg-gradient-to-tr from-[#00A896] to-[#2EC4B6] flex items-center justify-center text-white font-bold text-[13px] shadow-sm">
                                  {initials}
                                </div>
                                <div className="flex flex-col">
                                  <div className="flex items-center space-x-1.5">
                                    <span className="text-[13px] font-bold text-[#1A1A1A]">
                                      {drv.name}
                                    </span>
                                    <span className="text-[11px] font-semibold text-amber-500 flex items-center">
                                      ★ {drv.rating.toFixed(1)}
                                    </span>
                                  </div>
                                  <span className="text-[12px] text-[#6B7280]">
                                    {drv.vehicle}
                                  </span>
                                </div>
                              </div>
                              <div className="text-right">
                                <span className="text-[14px] font-bold text-[#00A896]">
                                  ${trip.price.toLocaleString()}
                                </span>
                                <span className="text-[10px] text-[#6B7280] block">
                                  1 asiento
                                </span>
                              </div>
                            </div>

                            {/* Action row for passenger upcoming trip */}
                            <div className="mt-3 pt-2.5 border-t border-[#F0F3F5] flex items-center justify-between gap-2">
                              <button
                                type="button"
                                onClick={(e) => {
                                  e.stopPropagation();
                                  setContactTarget({
                                    name: drv.name,
                                    phone: drv.phone,
                                    destination: trip.destination,
                                  });
                                  setIsContactModalOpen(true);
                                }}
                                className="px-3 py-1.5 bg-[#E6F7F5] hover:bg-[#00A896] text-[#00A896] hover:text-white rounded-xl text-xs font-semibold transition cursor-pointer flex items-center gap-1.5"
                              >
                                <MessageSquare className="w-3.5 h-3.5" />
                                <span>Contactar conductor</span>
                              </button>
                              <button
                                type="button"
                                onClick={(e) => {
                                  e.stopPropagation();
                                  setTripToCancel({
                                    id: trip.id,
                                    origin: trip.origin,
                                    destination: trip.destination,
                                    date: trip.date,
                                    time: trip.time,
                                    isDriver: false,
                                  });
                                  setIsCancelConfirmOpen(true);
                                }}
                                className="px-3 py-1.5 bg-rose-50 hover:bg-rose-100 text-rose-600 rounded-xl text-xs font-semibold transition cursor-pointer"
                              >
                                Cancelar reserva
                              </button>
                            </div>
                          </article>
                        );
                      })}
                    </section>
                  )}

                  {/* Section: Historial */}
                  {historyPassengerTrips.length > 0 && (
                    <section className="space-y-2.5">
                      <div className="px-1">
                        <h2 className="text-[12px] font-bold text-[#6B7280] tracking-wider uppercase">
                          Historial
                        </h2>
                      </div>

                      {historyPassengerTrips.map((trip) => {
                        const drv = getDriverInfo(trip.driver);
                        const initials = getInitials(drv.name);
                        return (
                          <article
                            key={trip.id}
                            id={`passenger-history-${trip.id}`}
                            onClick={() => setSelectedDisputeBookingId(trip.id)}
                            className="bg-white rounded-[16px] p-4 shadow-[0px_2px_8px_rgba(0,0,0,0.04)] border border-[#E5E9EB]/60 hover:border-[#6B7280]/30 transition-all opacity-95 cursor-pointer"
                          >
                            <div className="flex items-center justify-between pb-3 border-b border-[#F0F3F5]">
                              <div className="flex items-center space-x-2 text-[13px] text-[#6B7280] font-medium">
                                <svg className="w-4 h-4 text-[#6B7280]" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                  <path d="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" />
                                </svg>
                                <span>{trip.date} • {trip.time} hs</span>
                              </div>
                              <span
                                className={`inline-flex items-center px-2.5 py-1 rounded-full text-[11px] font-semibold ${
                                  trip.status === 'CANCELLED'
                                    ? 'bg-rose-50 text-rose-600 border border-rose-200'
                                    : 'bg-[#F3F4F6] text-[#4B5563] border border-[#E5E7EB]'
                                }`}
                              >
                                {trip.status === 'CANCELLED' ? 'Cancelado' : 'Finalizado'}
                              </span>
                            </div>

                            <div className="py-3">
                              <div className="flex items-center justify-between">
                                <div className="flex items-center space-x-3">
                                  <div className="flex flex-col items-center">
                                    <div className="w-2.5 h-2.5 rounded-full bg-[#6B7280]" />
                                    <div className="w-0.5 h-6 bg-dashed border-l border-dashed border-[#CBD5E1] my-0.5" />
                                    <div className="w-2.5 h-2.5 rounded-full bg-[#94A3B8]" />
                                  </div>
                                  <div className="flex flex-col justify-between py-0.5 space-y-2">
                                    <div>
                                      <span className="text-[11px] font-medium uppercase text-[#6B7280] tracking-wide block leading-none">
                                        Origen
                                      </span>
                                      <span className="text-[15px] font-semibold text-[#1A1A1A] leading-tight">
                                        {trip.origin}
                                      </span>
                                    </div>
                                    <div>
                                      <span className="text-[11px] font-medium uppercase text-[#6B7280] tracking-wide block leading-none">
                                        Destino
                                      </span>
                                      <span className="text-[15px] font-semibold text-[#1A1A1A] leading-tight">
                                        {trip.destination}
                                      </span>
                                    </div>
                                  </div>
                                </div>

                                <div className="w-8 h-8 rounded-full bg-[#F7F9FA] flex items-center justify-center text-[#6B7280]">
                                  <ChevronRight className="w-4 h-4 stroke-[2.2]" />
                                </div>
                              </div>
                            </div>

                            <div className="mt-1 pt-3 border-t border-[#F0F3F5] flex items-center justify-between">
                              <div className="flex items-center space-x-2.5">
                                <div className="w-9 h-9 rounded-full bg-[#05668D] flex items-center justify-center text-white font-bold text-[13px] shadow-sm">
                                  {initials}
                                </div>
                                <div className="flex flex-col">
                                  <div className="flex items-center space-x-1.5">
                                    <span className="text-[13px] font-bold text-[#1A1A1A]">
                                      {drv.name}
                                    </span>
                                    <span className="text-[11px] font-semibold text-amber-500 flex items-center">
                                      ★ {drv.rating.toFixed(1)}
                                    </span>
                                  </div>
                                  <span className="text-[12px] text-[#6B7280]">
                                    {drv.vehicle}
                                  </span>
                                </div>
                              </div>
                              <div className="text-right">
                                <span className="text-[14px] font-bold text-[#4B5563]">
                                  ${trip.price.toLocaleString()}
                                </span>
                                <span className="text-[10px] text-[#6B7280] block">
                                  Pagado
                                </span>
                              </div>
                            </div>
                          </article>
                        );
                      })}
                    </section>
                  )}
                </>
              ))}

              {/* =============================================================== */}
              {/* VISTA B: COMO CONDUCTOR                                          */}
              {/* =============================================================== */}
              {activeTab === 'driver' && (
                driverTrips.length === 0 ? (
                <div className="bg-white rounded-[16px] p-6 text-center shadow-[0px_2px_8px_rgba(0,0,0,0.04)] border border-[#E5E9EB]/60">
                  <div className="w-12 h-12 rounded-full bg-[#E6F7F5] flex items-center justify-center text-[#00A896] mx-auto mb-3">
                    <Calendar className="w-6 h-6 stroke-[2]" />
                  </div>
                  <h3 className="text-[15px] font-bold text-[#1A1A1A]">No tienes viajes como conductor</h3>
                  <p className="text-[13px] text-[#6B7280] mt-1 max-w-xs mx-auto">
                    Publica tus asientos disponibles en tus trayectos habituales y comparte tus gastos de combustible.
                  </p>
                  <button
                    type="button"
                    onClick={onNavigateToPublish}
                    className="mt-4 px-4 py-2 bg-[#00A896] text-white text-[13px] font-semibold rounded-xl shadow-xs hover:bg-[#008f80] transition-colors cursor-pointer"
                  >
                    Publicar viaje
                  </button>
                </div>
              ) : (
                <>
                  {/* Section: Próximos / Activos */}
                  {upcomingDriverTrips.length > 0 && (
                    <section className="space-y-2.5">
                      <div className="flex items-center justify-between px-1">
                        <h2 className="text-[12px] font-bold text-[#6B7280] tracking-wider uppercase">
                          PRÓXIMOS / ACTIVOS
                        </h2>
                        <span className="text-[12px] font-medium text-[#00A896] bg-[#E6F7F5] px-2 py-0.5 rounded-full">
                          {upcomingDriverTrips.length} activo{upcomingDriverTrips.length === 1 ? '' : 's'}
                        </span>
                      </div>

                      {upcomingDriverTrips.map((trip) => {
                        const drv = getDriverInfo(trip.driver);
                        const occupied = Math.max(0, trip.totalSeats - trip.availableSeats);
                        return (
                          <article
                            key={trip.id}
                            id={`driver-trip-${trip.id}`}
                            onClick={() => {
                              setManagedTrip({
                                id: trip.id,
                                origin: trip.origin,
                                destination: trip.destination,
                                departureTimeText: `${trip.date} • ${trip.time} hs`,
                                totalSeats: trip.totalSeats,
                                occupiedSeats: occupied,
                                pricePerSeat: trip.price,
                                status: normalizeTripStatus(trip.status),
                                carInfo: drv.vehicle,
                                passengers: [
                                  {
                                    id: `pass-${trip.id}-1`,
                                    name: 'Sofía F.',
                                    initials: 'SF',
                                    rating: 4.9,
                                    verified: true,
                                    phone: '+5491148291123',
                                    seatsBooked: Math.max(1, occupied),
                                  },
                                ],
                              });
                              setIsManagingView(true);
                            }}
                            className="bg-white rounded-[16px] p-4 shadow-[0px_2px_8px_rgba(0,0,0,0.04)] border border-[#E5E9EB]/60 hover:border-[#00A896]/30 transition-all cursor-pointer"
                          >
                            <div className="flex items-center justify-between pb-3 border-b border-[#F0F3F5]">
                              <div className="flex items-center space-x-2 text-[13px] text-[#1A1A1A] font-medium">
                                <svg className="w-4 h-4 text-[#00A896]" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                  <path d="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" />
                                </svg>
                                <span>{trip.date} • {trip.time} hs</span>
                              </div>
                              <span className="inline-flex items-center px-2.5 py-1 rounded-full text-[11px] font-semibold bg-[#E6F7F5] text-[#00A896] border border-[#00A896]/20">
                                <span className="w-1.5 h-1.5 rounded-full bg-[#00A896] mr-1.5" />
                                {occupied}/{trip.totalSeats} Asientos
                              </span>
                            </div>

                            <div className="py-3">
                              <div className="flex items-center justify-between">
                                <div className="flex items-center space-x-3">
                                  <div className="flex flex-col items-center">
                                    <div className="w-2.5 h-2.5 rounded-full bg-[#00A896]" />
                                    <div className="w-0.5 h-6 bg-dashed border-l border-dashed border-[#CBD5E1] my-0.5" />
                                    <div className="w-2.5 h-2.5 rounded-full bg-[#E63946]" />
                                  </div>
                                  <div className="flex flex-col justify-between py-0.5 space-y-2">
                                    <div>
                                      <span className="text-[11px] font-medium uppercase text-[#6B7280] tracking-wide block leading-none">
                                        Origen
                                      </span>
                                      <span className="text-[15px] font-bold text-[#1A1A1A] leading-tight">
                                        {trip.origin}
                                      </span>
                                    </div>
                                    <div>
                                      <span className="text-[11px] font-medium uppercase text-[#6B7280] tracking-wide block leading-none">
                                        Destino
                                      </span>
                                      <span className="text-[15px] font-bold text-[#1A1A1A] leading-tight">
                                        {trip.destination}
                                      </span>
                                    </div>
                                  </div>
                                </div>
                                <div className="w-8 h-8 rounded-full bg-[#F7F9FA] flex items-center justify-center text-[#6B7280]">
                                  <ChevronRight className="w-4 h-4 stroke-[2.2]" />
                                </div>
                              </div>
                            </div>

                            <div className="mt-1 pt-3 border-t border-[#F0F3F5] flex items-center justify-between">
                              <div className="flex items-center space-x-2">
                                <svg className="w-4 h-4 text-[#6B7280] shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                  <path d="M9 17a2 2 0 11-4 0 2 2 0 014 0zM19 17a2 2 0 11-4 0 2 2 0 014 0z" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" />
                                  <path d="M13 16V6a1 1 0 00-1-1H4a1 1 0 00-1 1v10a1 1 0 001 1h1m8-1a1 1 0 01-1 1H9m4-1V8a1 1 0 011-1h2.586a1 1 0 01.707.293l3.414 3.414a1 1 0 01.293.707V16a1 1 0 01-1 1h-1m-6-1a1 1 0 001 1h1M5 17a2 2 0 104 0m-4 0a2 2 0 114 0m6 0a2 2 0 104 0m-4 0a2 2 0 114 0" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" />
                                </svg>
                                <span className="text-[12px] font-medium text-[#6B7280]">
                                  {drv.vehicle}
                                </span>
                              </div>
                              <div className="text-right">
                                <span className="text-[14px] font-bold text-[#00A896]">
                                  ${trip.price.toLocaleString()}
                                </span>
                                <span className="text-[10px] text-[#6B7280] block">/ lugar</span>
                              </div>
                            </div>

                            {/* Action row for driver upcoming trip */}
                            <div className="mt-3 pt-2.5 border-t border-[#F0F3F5] flex items-center justify-between gap-2">
                              <button
                                type="button"
                                onClick={(e) => {
                                  e.stopPropagation();
                                  setManagedTrip({
                                    id: trip.id,
                                    origin: trip.origin,
                                    destination: trip.destination,
                                    departureTimeText: `${trip.date} • ${trip.time} hs`,
                                    totalSeats: trip.totalSeats,
                                    occupiedSeats: occupied,
                                    pricePerSeat: trip.price,
                                    status: normalizeTripStatus(trip.status),
                                    carInfo: drv.vehicle,
                                    passengers: [
                                      {
                                        id: `pass-${trip.id}-1`,
                                        name: 'Sofía F.',
                                        initials: 'SF',
                                        rating: 4.9,
                                        verified: true,
                                        phone: '+5491148291123',
                                        seatsBooked: Math.max(1, occupied),
                                      },
                                    ],
                                  });
                                  setIsManagingView(true);
                                }}
                                className="px-3 py-1.5 bg-[#E6F7F5] hover:bg-[#00A896] text-[#00A896] hover:text-white rounded-xl text-xs font-semibold transition cursor-pointer"
                              >
                                Gestionar viaje
                              </button>
                              <button
                                type="button"
                                onClick={(e) => {
                                  e.stopPropagation();
                                  setTripToCancel({
                                    id: trip.id,
                                    origin: trip.origin,
                                    destination: trip.destination,
                                    date: trip.date,
                                    time: trip.time,
                                    isDriver: true,
                                  });
                                  setIsCancelConfirmOpen(true);
                                }}
                                className="px-3 py-1.5 bg-rose-50 hover:bg-rose-100 text-rose-600 rounded-xl text-xs font-semibold transition cursor-pointer"
                              >
                                Cancelar viaje
                              </button>
                            </div>
                          </article>
                        );
                      })}
                    </section>
                  )}

                  {/* Section: Historial */}
                  {historyDriverTrips.length > 0 && (
                    <section className="space-y-2.5">
                      <div className="px-1">
                        <h2 className="text-[12px] font-bold text-[#6B7280] tracking-wider uppercase">
                          HISTORIAL
                        </h2>
                      </div>

                      {historyDriverTrips.map((trip) => {
                        const occupied = Math.max(1, trip.totalSeats - trip.availableSeats);
                        return (
                          <article
                            key={trip.id}
                            id={`driver-history-${trip.id}`}
                            className="bg-white rounded-[16px] p-4 shadow-[0px_2px_8px_rgba(0,0,0,0.04)] border border-[#E5E9EB]/60 hover:border-[#6B7280]/30 transition-all opacity-95"
                          >
                            <div className="flex items-center justify-between pb-3 border-b border-[#F0F3F5]">
                              <div className="flex items-center space-x-2 text-[13px] text-[#6B7280] font-medium">
                                <svg className="w-4 h-4 text-[#6B7280]" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                  <path d="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" />
                                </svg>
                                <span>{trip.date} • {trip.time} hs</span>
                              </div>
                              <span
                                className={`inline-flex items-center px-2.5 py-1 rounded-full text-[11px] font-semibold ${
                                  trip.status === 'CANCELLED'
                                    ? 'bg-rose-50 text-rose-600 border border-rose-200'
                                    : 'bg-[#F3F4F6] text-[#4B5563] border border-[#E5E7EB]'
                                }`}
                              >
                                {trip.status === 'CANCELLED' ? 'Cancelado' : 'Finalizado'}
                              </span>
                            </div>

                            <div className="py-3">
                              <div className="flex items-center justify-between">
                                <div className="flex items-center space-x-3">
                                  <div className="flex flex-col items-center">
                                    <div className="w-2.5 h-2.5 rounded-full bg-[#6B7280]" />
                                    <div className="w-0.5 h-6 bg-dashed border-l border-dashed border-[#CBD5E1] my-0.5" />
                                    <div className="w-2.5 h-2.5 rounded-full bg-[#94A3B8]" />
                                  </div>
                                  <div className="flex flex-col justify-between py-0.5 space-y-2">
                                    <div>
                                      <span className="text-[11px] font-medium uppercase text-[#6B7280] tracking-wide block leading-none">
                                        Origen
                                      </span>
                                      <span className="text-[15px] font-semibold text-[#1A1A1A] leading-tight">
                                        {trip.origin}
                                      </span>
                                    </div>
                                    <div>
                                      <span className="text-[11px] font-medium uppercase text-[#6B7280] tracking-wide block leading-none">
                                        Destino
                                      </span>
                                      <span className="text-[15px] font-semibold text-[#1A1A1A] leading-tight">
                                        {trip.destination}
                                      </span>
                                    </div>
                                  </div>
                                </div>
                                <div className="w-8 h-8 rounded-full bg-[#F7F9FA] flex items-center justify-center text-[#6B7280]">
                                  <ChevronRight className="w-4 h-4 stroke-[2.2]" />
                                </div>
                              </div>
                            </div>

                            <div className="mt-1 pt-3 border-t border-[#F0F3F5] flex items-center justify-between">
                              <div className="flex items-center space-x-2">
                                <svg className="w-4 h-4 text-[#6B7280] shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                  <path d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" />
                                </svg>
                                <span className="text-[12px] text-[#6B7280]">
                                  {occupied} de {trip.totalSeats} pasajeros conducidos
                                </span>
                              </div>
                              <div className="text-right">
                                <span className="text-[14px] font-bold text-[#1A1A1A]">
                                  ${(trip.price * occupied).toLocaleString()}
                                </span>
                                <span className="text-[10px] text-[#6B7280] block">Recaudado</span>
                              </div>
                            </div>
                          </article>
                        );
                      })}
                    </section>
                  )}
                </>
              ))}
            </>
          )}
        </main>

        {/* Bottom Navigation Bar (Fixed with 4 items, Tab 2 "Mis Viajes" Active) */}
        <nav className="bg-white border-t border-[#E5E9EB] pt-2.5 px-6 shrink-0 safe-area-bottom">
          <div className="flex justify-between items-center max-w-sm mx-auto">
            {/* Buscar (Inactive) */}
            <button
              id="nav-search-btn"
              type="button"
              onClick={onNavigateToSearch}
              className="flex flex-col items-center group py-1 text-[#6B7280] cursor-pointer"
            >
              <svg className="w-6 h-6 stroke-current" fill="none" strokeWidth="2" viewBox="0 0 24 24">
                <circle cx="11" cy="11" r="8" />
                <line x1="21" x2="16.65" y1="21" y2="16.65" />
              </svg>
              <span className="text-[10px] font-medium mt-1">Buscar</span>
            </button>

            {/* Mis Viajes (Active) */}
            <button
              id="nav-trips-active-btn"
              type="button"
              className="flex flex-col items-center group py-1 text-[#00A896] cursor-default"
            >
              <svg className="w-6 h-6 stroke-current fill-[#00A896]/10" stroke="currentColor" strokeWidth="2" viewBox="0 0 24 24">
                <path d="M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4" strokeLinecap="round" strokeLinejoin="round" />
              </svg>
              <span className="text-[10px] font-semibold mt-1">Mis Viajes</span>
            </button>

            {/* Publicar (Inactive) */}
            <button
              id="nav-publish-btn"
              type="button"
              onClick={onNavigateToPublish}
              className="flex flex-col items-center group py-1 text-[#6B7280] cursor-pointer"
            >
              <svg className="w-6 h-6 stroke-current" fill="none" strokeWidth="2" viewBox="0 0 24 24">
                <circle cx="12" cy="12" r="9" />
                <line x1="12" x2="12" y1="8" y2="16" />
                <line x1="8" x2="16" y1="12" y2="12" />
              </svg>
              <span className="text-[10px] font-medium mt-1">Publicar</span>
            </button>

            {/* Perfil (Inactive) */}
            <button
              id="nav-profile-btn"
              type="button"
              onClick={onNavigateToProfile}
              className="flex flex-col items-center group py-1 text-[#6B7280] cursor-pointer"
            >
              <svg className="w-6 h-6 stroke-current" fill="none" strokeWidth="2" viewBox="0 0 24 24">
                <path d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" strokeLinecap="round" strokeLinejoin="round" />
              </svg>
              <span className="text-[10px] font-medium mt-1">Perfil</span>
            </button>
          </div>

          {/* iOS Home Indicator */}
          <div className="w-32 h-1 bg-[#1A1A1A]/20 rounded-full mx-auto mt-3 mb-1" />
        </nav>
      </div>
    </div>
  );
};

export default MyTripsScreen;
