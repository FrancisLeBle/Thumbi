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

export const MyTripsScreen: React.FC<MyTripsScreenProps> = ({
  onNavigateToHome,
  onNavigateToSearch,
  onNavigateToPublish,
  onNavigateToProfile,
  initialManageTrip = false,
}) => {
  // Estado local para conmutar entre 'Como Pasajero' y 'Como Conductor'
  const [activeTab, setActiveTab] = useState<'passenger' | 'driver'>('passenger');

  // Estado para la subpantalla de gestión de viaje
  const [isManagingView, setIsManagingView] = useState<boolean>(initialManageTrip);

  // Estados interactivos para modales y notificaciones
  const [isCancelConfirmOpen, setIsCancelConfirmOpen] = useState<boolean>(false);
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
  const [managedTrip, setManagedTrip] = useState<ManagedTrip>({
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
                Como Pasajero
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
                Como Conductor
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
          {/* =============================================================== */}
          {/* VISTA A: COMO PASAJERO (MyTripsPassengerReact.md)                */}
          {/* =============================================================== */}
          {activeTab === 'passenger' && (
            <>
              {/* Section: Próximos Viajes */}
              <section className="space-y-2.5">
                <div className="flex items-center justify-between px-1">
                  <h2 className="text-[12px] font-bold text-[#6B7280] tracking-wider uppercase">
                    Próximos Viajes
                  </h2>
                  <span className="text-[12px] font-medium text-[#00A896] bg-[#E6F7F5] px-2 py-0.5 rounded-full">
                    1 pendiente
                  </span>
                </div>

                {/* Passenger Card 1: Confirmado */}
                <article
                  onClick={() => {
                    setContactTarget({
                      name: 'Carlos M.',
                      phone: '+5491148291123',
                      destination: 'Pilar, Bs. As.',
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
                      <span>Mañana, 09:30 hs</span>
                    </div>
                    {/* Status Badge: Confirmado (Light teal) */}
                    <span className="inline-flex items-center px-2.5 py-1 rounded-full text-[11px] font-semibold bg-[#E6F7F5] text-[#00A896] border border-[#00A896]/20">
                      <span className="w-1.5 h-1.5 rounded-full bg-[#00A896] mr-1.5" />
                      Confirmado
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
                              Palermo, CABA
                            </span>
                          </div>
                          <div>
                            <span className="text-[11px] font-medium uppercase text-[#6B7280] tracking-wide block leading-none">
                              Destino
                            </span>
                            <span className="text-[15px] font-bold text-[#1A1A1A] leading-tight">
                              Pilar, Bs. As.
                            </span>
                          </div>
                        </div>
                      </div>

                      {/* Right Chevron Indicator */}
                      <div className="w-8 h-8 rounded-full bg-[#F7F9FA] flex items-center justify-center text-[#6B7280]">
                        <ChevronRight className="w-4 h-4 stroke-[2.2]" />
                      </div>
                    </div>
                  </div>

                  {/* Driver & Vehicle Details Footer */}
                  <div className="mt-1 pt-3 border-t border-[#F0F3F5] flex items-center justify-between">
                    <div className="flex items-center space-x-2.5">
                      {/* Driver Avatar "CM" */}
                      <div className="w-9 h-9 rounded-full bg-gradient-to-tr from-[#00A896] to-[#2EC4B6] flex items-center justify-center text-white font-bold text-[13px] shadow-sm">
                        CM
                      </div>
                      <div className="flex flex-col">
                        <div className="flex items-center space-x-1.5">
                          <span className="text-[13px] font-bold text-[#1A1A1A]">
                            Carlos M.
                          </span>
                          <span className="text-[11px] font-semibold text-amber-500 flex items-center">
                            ★ 4.9
                          </span>
                        </div>
                        <span className="text-[12px] text-[#6B7280]">
                          Toyota Corolla • AA 123 CD
                        </span>
                      </div>
                    </div>
                    <div className="text-right">
                      <span className="text-[14px] font-bold text-[#00A896]">
                        $3.500
                      </span>
                      <span className="text-[10px] text-[#6B7280] block">
                        1 asiento
                      </span>
                    </div>
                  </div>
                </article>
              </section>

              {/* Section: Historial */}
              <section className="space-y-2.5">
                <div className="px-1">
                  <h2 className="text-[12px] font-bold text-[#6B7280] tracking-wider uppercase">
                    Historial
                  </h2>
                </div>

                {/* Passenger Card 2: Finalizado */}
                <article
                  onClick={() => setSelectedDisputeBookingId('hist-rosario-01')}
                  className="bg-white rounded-[16px] p-4 shadow-[0px_2px_8px_rgba(0,0,0,0.04)] border border-[#E5E9EB]/60 hover:border-[#6B7280]/30 transition-all opacity-95 cursor-pointer"
                >
                  {/* Card Header / Status & Date */}
                  <div className="flex items-center justify-between pb-3 border-b border-[#F0F3F5]">
                    <div className="flex items-center space-x-2 text-[13px] text-[#6B7280] font-medium">
                      <svg className="w-4 h-4 text-[#6B7280]" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path d="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" />
                      </svg>
                      <span>12 Oct 2024, 15:15 hs</span>
                    </div>
                    {/* Status Badge: Finalizado (Grey) */}
                    <span className="inline-flex items-center px-2.5 py-1 rounded-full text-[11px] font-semibold bg-[#F3F4F6] text-[#4B5563] border border-[#E5E7EB]">
                      Finalizado
                    </span>
                  </div>

                  {/* Route Info */}
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
                              Buenos Aires
                            </span>
                          </div>
                          <div>
                            <span className="text-[11px] font-medium uppercase text-[#6B7280] tracking-wide block leading-none">
                              Destino
                            </span>
                            <span className="text-[15px] font-semibold text-[#1A1A1A] leading-tight">
                              Rosario, Santa Fe
                            </span>
                          </div>
                        </div>
                      </div>

                      {/* Right Chevron Indicator */}
                      <div className="w-8 h-8 rounded-full bg-[#F7F9FA] flex items-center justify-center text-[#6B7280]">
                        <ChevronRight className="w-4 h-4 stroke-[2.2]" />
                      </div>
                    </div>
                  </div>

                  {/* Driver & Vehicle Details Footer */}
                  <div className="mt-1 pt-3 border-t border-[#F0F3F5] flex items-center justify-between">
                    <div className="flex items-center space-x-2.5">
                      {/* Driver Avatar "SO" */}
                      <div className="w-9 h-9 rounded-full bg-[#05668D] flex items-center justify-center text-white font-bold text-[13px] shadow-sm">
                        SO
                      </div>
                      <div className="flex flex-col">
                        <div className="flex items-center space-x-1.5">
                          <span className="text-[13px] font-bold text-[#1A1A1A]">
                            Sofía O.
                          </span>
                          <span className="text-[11px] font-semibold text-amber-500 flex items-center">
                            ★ 5.0
                          </span>
                        </div>
                        <span className="text-[12px] text-[#6B7280]">
                          Ford Focus • AB 456 CD
                        </span>
                      </div>
                    </div>
                    <div className="text-right">
                      <span className="text-[14px] font-bold text-[#4B5563]">
                        $7.200
                      </span>
                      <span className="text-[10px] text-[#6B7280] block">
                        Pagado
                      </span>
                    </div>
                  </div>
                </article>
              </section>
            </>
          )}

          {/* =============================================================== */}
          {/* VISTA B: COMO CONDUCTOR (MyTripsDriverReact.md)                  */}
          {/* =============================================================== */}
          {activeTab === 'driver' && (
            <>
              {/* Section: Próximos Viajes */}
              <section className="space-y-2.5">
                <div className="flex items-center justify-between px-1">
                  <h2 className="text-[12px] font-bold text-[#6B7280] tracking-wider uppercase">
                    PRÓXIMOS / ACTIVOS
                  </h2>
                  <span className="text-[12px] font-medium text-[#00A896] bg-[#E6F7F5] px-2 py-0.5 rounded-full">
                    1 activo
                  </span>
                </div>

                <article
                  id="driver-active-trip-card"
                  onClick={() => setIsManagingView(true)}
                  className="bg-white rounded-[16px] p-4 shadow-[0px_2px_8px_rgba(0,0,0,0.04)] border border-[#E5E9EB]/60 hover:border-[#00A896]/30 transition-all cursor-pointer"
                >
                  <div className="flex items-center justify-between pb-3 border-b border-[#F0F3F5]">
                    <div className="flex items-center space-x-2 text-[13px] text-[#1A1A1A] font-medium">
                      <svg className="w-4 h-4 text-[#00A896]" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path d="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" />
                      </svg>
                      <span>{managedTrip.departureTimeText}</span>
                    </div>
                    <span className="inline-flex items-center px-2.5 py-1 rounded-full text-[11px] font-semibold bg-[#E6F7F5] text-[#00A896] border border-[#00A896]/20">
                      <span className="w-1.5 h-1.5 rounded-full bg-[#00A896] mr-1.5" />
                      {managedTrip.occupiedSeats}/{managedTrip.totalSeats} Asientos
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
                              {managedTrip.origin}
                            </span>
                          </div>
                          <div>
                            <span className="text-[11px] font-medium uppercase text-[#6B7280] tracking-wide block leading-none">
                              Destino
                            </span>
                            <span className="text-[15px] font-bold text-[#1A1A1A] leading-tight">
                              {managedTrip.destination}
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
                        {managedTrip.carInfo}
                      </span>
                    </div>
                    <div className="text-right">
                      <span className="text-[14px] font-bold text-[#00A896]">
                        ${managedTrip.pricePerSeat.toLocaleString()}
                      </span>
                      <span className="text-[10px] text-[#6B7280] block">/ lugar</span>
                    </div>
                  </div>
                </article>
              </section>

              {/* Section: Historial */}
              <section className="space-y-2.5">
                <div className="px-1">
                  <h2 className="text-[12px] font-bold text-[#6B7280] tracking-wider uppercase">
                    HISTORIAL
                  </h2>
                </div>

                <article className="bg-white rounded-[16px] p-4 shadow-[0px_2px_8px_rgba(0,0,0,0.04)] border border-[#E5E9EB]/60 hover:border-[#6B7280]/30 transition-all opacity-95">
                  <div className="flex items-center justify-between pb-3 border-b border-[#F0F3F5]">
                    <div className="flex items-center space-x-2 text-[13px] text-[#6B7280] font-medium">
                      <svg className="w-4 h-4 text-[#6B7280]" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path d="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" />
                      </svg>
                      <span>05 Nov 2024, 08:00 hs</span>
                    </div>
                    <span className="inline-flex items-center px-2.5 py-1 rounded-full text-[11px] font-semibold bg-[#F3F4F6] text-[#4B5563] border border-[#E5E7EB]">
                      Finalizado
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
                              Nuñez, CABA
                            </span>
                          </div>
                          <div>
                            <span className="text-[11px] font-medium uppercase text-[#6B7280] tracking-wide block leading-none">
                              Destino
                            </span>
                            <span className="text-[15px] font-semibold text-[#1A1A1A] leading-tight">
                              La Plata, Bs. As.
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
                        3 de 3 pasajeros conducidos
                      </span>
                    </div>
                    <div className="text-right">
                      <span className="text-[14px] font-bold text-[#1A1A1A]">
                        $10.500
                      </span>
                      <span className="text-[10px] text-[#6B7280] block">Recaudado</span>
                    </div>
                  </div>
                </article>
              </section>
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
