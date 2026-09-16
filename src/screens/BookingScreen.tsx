import React, { useState } from 'react';
import {
  MapPin,
  Calendar,
  DollarSign,
  Users,
  ShieldCheck,
  CheckCircle2,
  Minus,
  Plus,
  ArrowRight,
  Loader2,
  Receipt,
  AlertTriangle
} from 'lucide-react';
import { Trip, Booking } from '../types/api';
import { bookingService } from '../services/bookingService';
import { ApiClientError } from '../services/apiClient';
import { formatCurrency, formatDateISOToLocal } from '../utils/apiHelpers';
import { getErrorMessage } from '../utils/errorHelpers';
import { Toast } from '../components/Toast';
import { CreateDisputeModal } from '../components/CreateDisputeModal';

export interface BookingScreenProps {
  trip?: Trip;
  onBookingSuccess?: (booking: Booking) => void;
  onBack?: () => void;
}

const defaultTrip: Trip = {
  id: 'trip-789-cor',
  driverId: 'driver-001',
  origin: 'Córdoba Capital (Terminal)',
  destination: 'Villa Carlos Paz (Centro)',
  pricePerSeat: 15.0,
  availableSeats: 3,
  departureTime: '2026-09-17T14:30:00Z',
  status: 'SCHEDULED',
};

export const BookingScreen: React.FC<BookingScreenProps> = ({
  trip = defaultTrip,
  onBookingSuccess,
  onBack,
}) => {
  const [seatsRequested, setSeatsRequested] = useState<number>(1);
  const [isLoading, setIsLoading] = useState<boolean>(false);
  const [confirmedBooking, setConfirmedBooking] = useState<Booking | null>(null);
  const [isDisputeModalOpen, setIsDisputeModalOpen] = useState<boolean>(false);
  const [toast, setToast] = useState<{
    message: string;
    type: 'error' | 'success';
  } | null>(null);

  const availableSeats = trip.availableSeats;
  const isTripAvailable = trip.status === 'SCHEDULED' && availableSeats > 0;

  const handleDecrement = () => {
    if (seatsRequested > 1) {
      setSeatsRequested((prev) => prev - 1);
    }
  };

  const handleIncrement = () => {
    if (seatsRequested < availableSeats) {
      setSeatsRequested((prev) => prev + 1);
    }
  };

  const totalAmount = seatsRequested * trip.pricePerSeat;

  const handleConfirmBooking = async () => {
    if (!isTripAvailable || isLoading || seatsRequested < 1) return;

    setIsLoading(true);

    try {
      const newBooking = await bookingService.createBooking({
        tripId: trip.id,
        seatsRequested,
      });

      setConfirmedBooking(newBooking);
      setToast({
        message: `¡Reserva confirmada con éxito! Se reservaron ${seatsRequested} asiento(s).`,
        type: 'success',
      });

      if (onBookingSuccess) {
        onBookingSuccess(newBooking);
      }
    } catch (err: unknown) {
      let message = 'No se pudo completar la reserva. Por favor, inténtalo nuevamente.';

      if (err instanceof ApiClientError) {
        message = getErrorMessage(err.code, err.message);
      } else if (err instanceof Error) {
        message = err.message;
      }

      setToast({
        message,
        type: 'error',
      });
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div
      id="booking-screen"
      className="min-h-screen py-10 px-4 sm:px-6 lg:px-8 flex flex-col justify-center items-center font-sans"
      style={{
        backgroundColor: '#F7F9FA',
        fontFamily: '-apple-system, BlinkMacSystemFont, "SF Pro Text", "Segoe UI", Roboto, sans-serif',
      }}
    >
      <div className="w-full max-w-lg">
        {/* Cabecera */}
        <div className="mb-6 text-center sm:text-left">
          {onBack && (
            <button
              id="booking-back-btn"
              type="button"
              onClick={onBack}
              className="text-xs font-semibold text-[#666666] hover:text-[#1A1A1A] mb-3 inline-flex items-center gap-1 transition-colors"
            >
              ← Volver a viajes
            </button>
          )}
          <h1 className="text-2xl sm:text-3xl font-bold tracking-tight text-[#1A1A1A]">
            Reservar Asientos
          </h1>
          <p className="mt-1.5 text-sm text-[#666666]">
            Revisa los detalles del trayecto y selecciona los cupos que necesitas.
          </p>
        </div>

        {/* Tarjeta de Resumen del Viaje */}
        <div
          id="trip-summary-card"
          className="rounded-[12px] p-6 sm:p-7 bg-[#FFFFFF] transition-all"
          style={{
            boxShadow: '0px 2px 8px rgba(0, 0, 0, 0.04)',
            border: '1px solid #EFEFEF',
          }}
        >
          {/* Ruta: Origen -> Destino */}
          <div className="flex items-start justify-between pb-5 border-b border-[#F0F0F0]">
            <div className="space-y-3 flex-1">
              <div className="flex items-center gap-2.5">
                <div className="w-6 h-6 rounded-full bg-emerald-50 flex items-center justify-center text-emerald-600 shrink-0">
                  <MapPin className="w-3.5 h-3.5" />
                </div>
                <div>
                  <span className="block text-[11px] font-semibold text-[#888888] uppercase tracking-wider">
                    Origen
                  </span>
                  <span className="text-sm font-semibold text-[#1A1A1A]">
                    {trip.origin}
                  </span>
                </div>
              </div>

              <div className="flex items-center gap-2.5">
                <div className="w-6 h-6 rounded-full bg-teal-50 flex items-center justify-center text-teal-600 shrink-0">
                  <ArrowRight className="w-3.5 h-3.5" />
                </div>
                <div>
                  <span className="block text-[11px] font-semibold text-[#888888] uppercase tracking-wider">
                    Destino
                  </span>
                  <span className="text-sm font-semibold text-[#1A1A1A]">
                    {trip.destination}
                  </span>
                </div>
              </div>
            </div>

            <div className="text-right pl-4">
              <span className="inline-block px-2.5 py-1 text-[11px] font-semibold rounded-full bg-[#E6F6F4] text-[#00A896]">
                {trip.status === 'SCHEDULED' ? 'Disponible' : trip.status}
              </span>
            </div>
          </div>

          {/* Detalles clave: Salida y Precio */}
          <div className="grid grid-cols-2 gap-4 py-5 border-b border-[#F0F0F0]">
            <div className="flex items-start gap-2.5">
              <Calendar className="w-4 h-4 text-[#888888] mt-0.5 shrink-0" />
              <div>
                <p className="text-[11px] font-semibold text-[#888888] uppercase tracking-wider">
                  Salida programada
                </p>
                <p className="text-sm font-medium text-[#1A1A1A] mt-0.5">
                  {formatDateISOToLocal(trip.departureTime)}
                </p>
              </div>
            </div>

            <div className="flex items-start gap-2.5">
              <DollarSign className="w-4 h-4 text-[#888888] mt-0.5 shrink-0" />
              <div>
                <p className="text-[11px] font-semibold text-[#888888] uppercase tracking-wider">
                  Precio por Asiento
                </p>
                <p className="text-sm font-bold text-[#1A1A1A] mt-0.5">
                  {formatCurrency(trip.pricePerSeat, 'USD')}
                </p>
              </div>
            </div>
          </div>

          {/* Asientos disponibles en el vehículo */}
          <div className="flex items-center justify-between py-4 border-b border-[#F0F0F0] text-sm">
            <div className="flex items-center gap-2 text-[#4A4A4A]">
              <Users className="w-4 h-4 text-[#888888]" />
              <span className="font-medium">Cupos totales disponibles</span>
            </div>
            <span className="font-semibold text-[#1A1A1A]">
              {availableSeats} {availableSeats === 1 ? 'asiento' : 'asientos'}
            </span>
          </div>

          {/* Selector de Asientos con botones - y + */}
          {!confirmedBooking && (
            <div className="pt-5 pb-2">
              <div className="flex items-center justify-between">
                <div>
                  <span className="block text-sm font-semibold text-[#1A1A1A]">
                    Cantidad a reservar
                  </span>
                  <span className="block text-xs text-[#888888] mt-0.5">
                    Máximo: {availableSeats} {availableSeats === 1 ? 'cupo' : 'cupos'}
                  </span>
                </div>

                <div className="flex items-center gap-3 bg-[#F7F9FA] p-1.5 rounded-lg border border-[#EAECEF]">
                  <button
                    id="decrease-seats-btn"
                    type="button"
                    onClick={handleDecrement}
                    disabled={seatsRequested <= 1 || isLoading}
                    aria-label="Disminuir asientos"
                    className="w-8 h-8 rounded-md bg-[#FFFFFF] text-[#1A1A1A] flex items-center justify-center font-bold text-sm shadow-xs hover:bg-[#F0F2F5] disabled:opacity-40 disabled:cursor-not-allowed transition"
                  >
                    <Minus className="w-3.5 h-3.5" />
                  </button>

                  <span
                    id="seats-requested-counter"
                    className="w-6 text-center font-bold text-base text-[#1A1A1A]"
                  >
                    {seatsRequested}
                  </span>

                  <button
                    id="increase-seats-btn"
                    type="button"
                    onClick={handleIncrement}
                    disabled={seatsRequested >= availableSeats || isLoading}
                    aria-label="Aumentar asientos"
                    className="w-8 h-8 rounded-md bg-[#FFFFFF] text-[#1A1A1A] flex items-center justify-center font-bold text-sm shadow-xs hover:bg-[#F0F2F5] disabled:opacity-40 disabled:cursor-not-allowed transition"
                  >
                    <Plus className="w-3.5 h-3.5" />
                  </button>
                </div>
              </div>

              {/* Total estimado */}
              <div className="mt-5 p-3.5 rounded-lg bg-[#F7F9FA] border border-[#EAECEF] flex items-center justify-between">
                <span className="text-xs font-semibold text-[#666666] uppercase tracking-wider">
                  Monto total a retener (Escrow)
                </span>
                <span className="text-base font-extrabold text-[#1A1A1A]">
                  {formatCurrency(totalAmount, 'USD')}
                </span>
              </div>
            </div>
          )}

          {/* Botón Principal: Confirmar Reserva */}
          {!confirmedBooking ? (
            <div className="mt-6">
              <button
                id="confirm-booking-btn"
                type="button"
                onClick={handleConfirmBooking}
                disabled={!isTripAvailable || isLoading}
                className="w-full flex items-center justify-center py-3.5 px-4 rounded-[8px] text-white font-medium text-sm transition-all focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-[#00A896] disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer"
                style={{
                  backgroundColor: '#00A896',
                }}
              >
                {isLoading ? (
                  <>
                    <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                    Procesando reserva y custodia...
                  </>
                ) : (
                  'Confirmar Reserva'
                )}
              </button>

              <div className="mt-3 flex items-center justify-center gap-1.5 text-[11px] text-[#666666]">
                <ShieldCheck className="w-3.5 h-3.5 text-[#00A896]" />
                <span>Pago protegido en Escrow hasta finalizar el viaje</span>
              </div>
            </div>
          ) : (
            /* Resumen y Detalle de Reserva Confirmada */
            <div
              id="booking-confirmed-card"
              className="mt-6 p-5 rounded-[8px] bg-emerald-50/70 border border-emerald-200"
            >
              <div className="flex items-center gap-2 text-emerald-800 font-semibold text-sm mb-3">
                <CheckCircle2 className="w-4 h-4 text-emerald-600" />
                <span>Reserva Completada con Éxito</span>
              </div>

              <div className="space-y-1.5 text-xs text-slate-700">
                <p>
                  <strong className="text-slate-900">ID de Reserva:</strong>{' '}
                  <span className="font-mono text-[11px]">{confirmedBooking.id}</span>
                </p>
                <p>
                  <strong className="text-slate-900">Asientos Reservados:</strong>{' '}
                  {confirmedBooking.seatsRequested}
                </p>
                <p>
                  <strong className="text-slate-900">Estado:</strong>{' '}
                  <span className="inline-block px-2 py-0.5 rounded bg-emerald-100 text-emerald-800 font-semibold text-[10px]">
                    {confirmedBooking.status}
                  </span>
                </p>
                {confirmedBooking.paymentGatewayRef && (
                  <p>
                    <strong className="text-slate-900">Ref. Pasarela:</strong>{' '}
                    <span className="font-mono text-[11px]">
                      {confirmedBooking.paymentGatewayRef}
                    </span>
                  </p>
                )}
              </div>

              <div className="mt-4 pt-3 border-t border-emerald-200/60 flex flex-wrap items-center justify-between gap-2">
                <div className="flex items-center gap-3">
                  <button
                    id="view-booking-receipt-btn"
                    type="button"
                    onClick={() => {
                      setToast({
                        message: `Comprobante de reserva ${confirmedBooking.id} listo para consultar.`,
                        type: 'success',
                      });
                    }}
                    className="inline-flex items-center gap-1 text-xs font-semibold text-emerald-800 hover:text-emerald-950 transition"
                  >
                    <Receipt className="w-3.5 h-3.5" />
                    <span>Ver comprobante digital</span>
                  </button>

                  <button
                    id="open-dispute-from-booking-btn"
                    type="button"
                    onClick={() => setIsDisputeModalOpen(true)}
                    className="inline-flex items-center gap-1 text-xs font-semibold text-[#E63946] hover:text-red-700 transition"
                  >
                    <AlertTriangle className="w-3.5 h-3.5" />
                    <span>Reportar problema</span>
                  </button>
                </div>

                <button
                  id="reset-booking-view-btn"
                  type="button"
                  onClick={() => {
                    setConfirmedBooking(null);
                    setSeatsRequested(1);
                  }}
                  className="text-xs font-medium text-slate-600 hover:text-slate-900 transition"
                >
                  Hacer otra reserva
                </button>
              </div>
            </div>
          )}
        </div>
      </div>

      {confirmedBooking && (
        <CreateDisputeModal
          isOpen={isDisputeModalOpen}
          bookingId={confirmedBooking.id}
          onClose={() => setIsDisputeModalOpen(false)}
          showToast={(message, type) => setToast({ message, type })}
        />
      )}

      {toast && (
        <Toast
          id="booking-toast"
          message={toast.message}
          type={toast.type}
          onClose={() => setToast(null)}
        />
      )}
    </div>
  );
};

export default BookingScreen;
