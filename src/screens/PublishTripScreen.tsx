import React, { useState } from 'react';
import { MapPin, DollarSign, Users, Calendar, ArrowRight, Loader2 } from 'lucide-react';
import { tripService } from '../services/tripService';
import { ApiClientError } from '../services/apiClient';
import { getErrorMessage } from '../utils/errorHelpers';
import { Toast } from '../components/Toast';

interface FormState {
  origin: string;
  destination: string;
  pricePerSeat: string;
  availableSeats: string;
  departureTime: string;
}

const initialFormState: FormState = {
  origin: '',
  destination: '',
  pricePerSeat: '',
  availableSeats: '1',
  departureTime: '',
};

export const PublishTripScreen: React.FC = () => {
  const [formData, setFormData] = useState<FormState>(initialFormState);
  const [isLoading, setIsLoading] = useState<boolean>(false);
  const [toast, setToast] = useState<{
    message: string;
    type: 'error' | 'success';
  } | null>(null);

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const { name, value } = e.target;
    setFormData((prev) => ({ ...prev, [name]: value }));
  };

  const isFormValid =
    formData.origin.trim() !== '' &&
    formData.destination.trim() !== '' &&
    Number(formData.pricePerSeat) > 0 &&
    Number(formData.availableSeats) > 0 &&
    formData.departureTime.trim() !== '';

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!isFormValid || isLoading) return;

    setIsLoading(true);

    try {
      const departureDate = new Date(formData.departureTime);
      const isoDeparture = isNaN(departureDate.getTime())
        ? formData.departureTime
        : departureDate.toISOString();

      await tripService.createTrip({
        origin: formData.origin.trim(),
        destination: formData.destination.trim(),
        pricePerSeat: parseFloat(formData.pricePerSeat),
        availableSeats: parseInt(formData.availableSeats, 10),
        departureTime: isoDeparture,
      });

      setToast({
        message: '¡Viaje publicado exitosamente! Los pasajeros ya pueden reservar sus asientos.',
        type: 'success',
      });
      setFormData(initialFormState);
    } catch (err: unknown) {
      let message = 'No se pudo publicar el viaje. Inténtalo nuevamente.';

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
      id="publish-trip-screen"
      className="min-h-screen py-10 px-4 sm:px-6 lg:px-8 flex flex-col justify-center items-center"
      style={{ backgroundColor: '#F7F9FA' }}
    >
      <div className="w-full max-w-xl">
        <div className="mb-8 text-center sm:text-left">
          <h1 className="text-2xl sm:text-3xl font-bold tracking-tight text-[#1A1A1A]">
            Publicar un viaje
          </h1>
          <p className="mt-2 text-sm text-[#666666]">
            Comparte los gastos de tu trayecto ofreciendo asientos disponibles a otros pasajeros.
          </p>
        </div>

        <div
          id="publish-trip-card"
          className="rounded-[12px] p-6 sm:p-8 bg-[#FFFFFF]"
          style={{
            boxShadow: '0px 2px 8px rgba(0, 0, 0, 0.04)',
            border: '1px solid #EFEFEF',
          }}
        >
          <form id="publish-trip-form" onSubmit={handleSubmit} className="space-y-5">
            {/* Origen */}
            <div>
              <label
                htmlFor="origin-input"
                className="block text-xs font-semibold uppercase tracking-wider text-[#4A4A4A] mb-1.5"
              >
                Origen
              </label>
              <div className="relative rounded-lg">
                <div className="absolute inset-y-0 left-0 pl-3.5 flex items-center pointer-events-none text-[#888888]">
                  <MapPin className="w-4 h-4" />
                </div>
                <input
                  id="origin-input"
                  name="origin"
                  type="text"
                  required
                  placeholder="Ej: Córdoba Capital"
                  value={formData.origin}
                  onChange={handleChange}
                  className="w-full pl-10 pr-4 py-2.5 bg-[#FFFFFF] border border-[#E0E0E0] rounded-lg text-sm text-[#1A1A1A] placeholder-[#999999] focus:outline-none focus:border-[#00A896] focus:ring-1 focus:ring-[#00A896] transition-colors"
                />
              </div>
            </div>

            {/* Destino */}
            <div>
              <label
                htmlFor="destination-input"
                className="block text-xs font-semibold uppercase tracking-wider text-[#4A4A4A] mb-1.5"
              >
                Destino
              </label>
              <div className="relative rounded-lg">
                <div className="absolute inset-y-0 left-0 pl-3.5 flex items-center pointer-events-none text-[#888888]">
                  <ArrowRight className="w-4 h-4" />
                </div>
                <input
                  id="destination-input"
                  name="destination"
                  type="text"
                  required
                  placeholder="Ej: Villa Carlos Paz"
                  value={formData.destination}
                  onChange={handleChange}
                  className="w-full pl-10 pr-4 py-2.5 bg-[#FFFFFF] border border-[#E0E0E0] rounded-lg text-sm text-[#1A1A1A] placeholder-[#999999] focus:outline-none focus:border-[#00A896] focus:ring-1 focus:ring-[#00A896] transition-colors"
                />
              </div>
            </div>

            {/* Fila: Precio por Asiento y Asientos Disponibles */}
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <div>
                <label
                  htmlFor="price-input"
                  className="block text-xs font-semibold uppercase tracking-wider text-[#4A4A4A] mb-1.5"
                >
                  Precio por Asiento (USD)
                </label>
                <div className="relative rounded-lg">
                  <div className="absolute inset-y-0 left-0 pl-3.5 flex items-center pointer-events-none text-[#888888]">
                    <DollarSign className="w-4 h-4" />
                  </div>
                  <input
                    id="price-input"
                    name="pricePerSeat"
                    type="number"
                    min="1"
                    step="0.5"
                    required
                    placeholder="0.00"
                    value={formData.pricePerSeat}
                    onChange={handleChange}
                    className="w-full pl-10 pr-4 py-2.5 bg-[#FFFFFF] border border-[#E0E0E0] rounded-lg text-sm text-[#1A1A1A] placeholder-[#999999] focus:outline-none focus:border-[#00A896] focus:ring-1 focus:ring-[#00A896] transition-colors"
                  />
                </div>
              </div>

              <div>
                <label
                  htmlFor="seats-input"
                  className="block text-xs font-semibold uppercase tracking-wider text-[#4A4A4A] mb-1.5"
                >
                  Asientos Disponibles
                </label>
                <div className="relative rounded-lg">
                  <div className="absolute inset-y-0 left-0 pl-3.5 flex items-center pointer-events-none text-[#888888]">
                    <Users className="w-4 h-4" />
                  </div>
                  <input
                    id="seats-input"
                    name="availableSeats"
                    type="number"
                    min="1"
                    max="8"
                    required
                    value={formData.availableSeats}
                    onChange={handleChange}
                    className="w-full pl-10 pr-4 py-2.5 bg-[#FFFFFF] border border-[#E0E0E0] rounded-lg text-sm text-[#1A1A1A] placeholder-[#999999] focus:outline-none focus:border-[#00A896] focus:ring-1 focus:ring-[#00A896] transition-colors"
                  />
                </div>
              </div>
            </div>

            {/* Fecha y Hora de Salida */}
            <div>
              <label
                htmlFor="departure-time-input"
                className="block text-xs font-semibold uppercase tracking-wider text-[#4A4A4A] mb-1.5"
              >
                Fecha y Hora de Salida
              </label>
              <div className="relative rounded-lg">
                <div className="absolute inset-y-0 left-0 pl-3.5 flex items-center pointer-events-none text-[#888888]">
                  <Calendar className="w-4 h-4" />
                </div>
                <input
                  id="departure-time-input"
                  name="departureTime"
                  type="datetime-local"
                  required
                  value={formData.departureTime}
                  onChange={handleChange}
                  className="w-full pl-10 pr-4 py-2.5 bg-[#FFFFFF] border border-[#E0E0E0] rounded-lg text-sm text-[#1A1A1A] placeholder-[#999999] focus:outline-none focus:border-[#00A896] focus:ring-1 focus:ring-[#00A896] transition-colors"
                />
              </div>
            </div>

            {/* Botón Principal */}
            <div className="pt-2">
              <button
                id="submit-publish-trip-btn"
                type="submit"
                disabled={!isFormValid || isLoading}
                className="w-full flex items-center justify-center py-3 px-4 rounded-[8px] text-white font-medium text-sm transition-all focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-[#00A896] disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer"
                style={{
                  backgroundColor: '#00A896',
                }}
              >
                {isLoading ? (
                  <>
                    <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                    Publicando viaje...
                  </>
                ) : (
                  'Publicar Viaje'
                )}
              </button>
            </div>
          </form>
        </div>
      </div>

      {toast && (
        <Toast
          id="publish-trip-toast"
          message={toast.message}
          type={toast.type}
          onClose={() => setToast(null)}
        />
      )}
    </div>
  );
};

export default PublishTripScreen;
