import React, { useState } from 'react';
import { AlertTriangle, X, Loader2, ShieldAlert } from 'lucide-react';
import { disputeService } from '../services/disputeService';
import { ApiClientError } from '../services/apiClient';
import { getErrorMessage } from '../utils/errorHelpers';
import { Dispute } from '../types/api';

export type DisputeReason =
  | 'TRIP_CANCELLED'
  | 'DRIVER_NO_SHOW'
  | 'SAFETY_CONCERN'
  | 'OTHER';

export interface CreateDisputeModalProps {
  isOpen: boolean;
  bookingId: string;
  onClose: () => void;
  onSuccess?: (dispute: Dispute) => void;
  showToast: (message: string, type: 'error' | 'success') => void;
}

const REASON_OPTIONS: { value: DisputeReason; label: string }[] = [
  { value: 'TRIP_CANCELLED', label: 'Viaje cancelado por el conductor' },
  { value: 'DRIVER_NO_SHOW', label: 'El conductor no se presentó (No Show)' },
  { value: 'SAFETY_CONCERN', label: 'Problemas de seguridad o conducta indebida' },
  { value: 'OTHER', label: 'Otro motivo' },
];

export const CreateDisputeModal: React.FC<CreateDisputeModalProps> = ({
  isOpen,
  bookingId,
  onClose,
  onSuccess,
  showToast,
}) => {
  const [reason, setReason] = useState<DisputeReason>('TRIP_CANCELLED');
  const [description, setDescription] = useState<string>('');
  const [isSubmitting, setIsSubmitting] = useState<boolean>(false);
  const [touched, setTouched] = useState<boolean>(false);

  if (!isOpen) return null;

  const isDescriptionValid = description.trim().length >= 10;
  const showError = touched && !isDescriptionValid;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setTouched(true);

    if (!isDescriptionValid || isSubmitting) return;

    setIsSubmitting(true);

    try {
      const dispute = await disputeService.createDispute({
        bookingId,
        reason,
        description: description.trim(),
      });

      showToast(
        'Disputa abierta exitosamente. Los fondos en custodia (Escrow) se mantendrán retenidos mientras el equipo revisa el caso.',
        'success'
      );

      if (onSuccess) {
        onSuccess(dispute);
      }

      onClose();
    } catch (err: unknown) {
      let message = 'No se pudo iniciar la disputa. Por favor, inténtalo nuevamente.';

      if (err instanceof ApiClientError) {
        message = getErrorMessage(err.code, err.message);
      } else if (err instanceof Error) {
        message = err.message;
      }

      showToast(message, 'error');
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <div
      id="create-dispute-modal-overlay"
      className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50 backdrop-blur-xs transition-opacity"
      aria-modal="true"
      role="dialog"
      aria-labelledby="dispute-modal-title"
    >
      <div
        id="create-dispute-card"
        className="w-full max-w-lg rounded-[12px] bg-[#FFFFFF] p-6 sm:p-7 text-[#1A1A1A] transition-all relative"
        style={{
          boxShadow: '0px 2px 8px rgba(0, 0, 0, 0.04)',
          fontFamily: '-apple-system, BlinkMacSystemFont, "SF Pro Display", "SF Pro Text", "Segoe UI", Roboto, sans-serif',
        }}
      >
        {/* Botón cerrar */}
        <button
          id="close-dispute-modal-btn"
          type="button"
          onClick={onClose}
          disabled={isSubmitting}
          aria-label="Cerrar ventana de reclamo"
          className="absolute top-5 right-5 rounded-md p-1.5 text-[#888888] hover:text-[#1A1A1A] hover:bg-[#F4F4F5] transition-colors focus:outline-none focus:ring-2 focus:ring-black"
        >
          <X className="w-5 h-5" />
        </button>

        {/* Encabezado */}
        <div className="flex items-start gap-3.5 pr-8">
          <div className="w-10 h-10 rounded-full bg-red-50 flex items-center justify-center text-[#E63946] shrink-0">
            <AlertTriangle className="w-5 h-5" />
          </div>
          <div>
            <h2
              id="dispute-modal-title"
              className="text-lg sm:text-xl font-bold tracking-tight text-[#1A1A1A]"
            >
              Abrir Reclamo o Disputa
            </h2>
            <p className="mt-1 text-xs sm:text-sm text-[#666666] leading-relaxed">
              Si tuviste un inconveniente con el viaje o el conductor, inicia un reclamo. Los fondos de la reserva quedarán retenidos en custodia (Escrow).
            </p>
          </div>
        </div>

        {/* ID de Reserva informada */}
        <div className="mt-4 p-2.5 rounded-lg bg-[#F7F9FA] border border-[#EAECEF] flex items-center justify-between text-xs">
          <span className="text-[#666666] font-medium">Reserva afectada:</span>
          <span className="font-mono font-semibold text-[#1A1A1A]">{bookingId}</span>
        </div>

        {/* Formulario */}
        <form onSubmit={handleSubmit} className="mt-5 space-y-4">
          {/* Selector de Motivo */}
          <div>
            <label
              htmlFor="dispute-reason-select"
              className="block text-xs font-semibold uppercase tracking-wider text-[#4A4A4A] mb-1.5"
            >
              Motivo del reclamo
            </label>
            <select
              id="dispute-reason-select"
              name="reason"
              value={reason}
              onChange={(e) => setReason(e.target.value as DisputeReason)}
              disabled={isSubmitting}
              className="w-full px-3.5 py-2.5 bg-[#FFFFFF] border border-[#E0E0E0] rounded-lg text-sm text-[#1A1A1A] focus:outline-none focus:border-[#00A896] focus:ring-1 focus:ring-[#00A896] transition-colors"
            >
              {REASON_OPTIONS.map((opt) => (
                <option key={opt.value} value={opt.value}>
                  {opt.label}
                </option>
              ))}
            </select>
          </div>

          {/* Área de Texto: Descripción */}
          <div>
            <div className="flex items-center justify-between mb-1.5">
              <label
                htmlFor="dispute-description-input"
                className="block text-xs font-semibold uppercase tracking-wider text-[#4A4A4A]"
              >
                Descripción detallada
              </label>
              <span className="text-[11px] text-[#888888]">
                Mínimo 10 caracteres
              </span>
            </div>
            <textarea
              id="dispute-description-input"
              name="description"
              rows={4}
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              onBlur={() => setTouched(true)}
              placeholder="Explica qué ocurrió durante el viaje o con el conductor..."
              disabled={isSubmitting}
              className={`w-full px-3.5 py-2.5 bg-[#FFFFFF] rounded-lg text-sm text-[#1A1A1A] placeholder-[#999999] transition-colors resize-none focus:outline-none ${
                showError
                  ? 'border-2 border-[#E63946] focus:border-[#E63946] focus:ring-1 focus:ring-[#E63946]'
                  : 'border border-[#E0E0E0] focus:border-[#00A896] focus:ring-1 focus:ring-[#00A896]'
              }`}
            />
            {showError && (
              <p className="mt-1 text-xs text-[#E63946] font-medium">
                Por favor, describe con claridad lo ocurrido (al menos 10 caracteres).
              </p>
            )}
          </div>

          {/* Aviso de Protección Escrow */}
          <div className="p-3 rounded-lg bg-amber-50 border border-amber-200/80 flex items-start gap-2.5 text-xs text-amber-900">
            <ShieldAlert className="w-4 h-4 text-amber-600 mt-0.5 shrink-0" />
            <p className="leading-relaxed">
              Al abrir este reclamo, el pago al conductor se congelará automáticamente hasta que un mediador examine la evidencia.
            </p>
          </div>

          {/* Botones de acción */}
          <div className="pt-2 flex flex-col-reverse sm:flex-row items-center gap-3">
            <button
              id="cancel-dispute-btn"
              type="button"
              onClick={onClose}
              disabled={isSubmitting}
              className="w-full sm:w-auto flex-1 py-2.5 px-4 rounded-[8px] border border-[#E0E0E0] bg-[#FFFFFF] text-[#4A4A4A] hover:bg-[#F7F9FA] font-medium text-sm transition-colors focus:outline-none"
            >
              Cancelar
            </button>

            <button
              id="submit-dispute-btn"
              type="submit"
              disabled={isSubmitting || (touched && !isDescriptionValid)}
              className="w-full sm:w-auto flex-1 flex items-center justify-center py-2.5 px-4 rounded-[8px] text-white font-medium text-sm transition-all focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-[#00A896] disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer"
              style={{
                backgroundColor: '#00A896',
              }}
            >
              {isSubmitting ? (
                <>
                  <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                  Abriendo disputa...
                </>
              ) : (
                'Abrir Disputa'
              )}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};

export default CreateDisputeModal;
