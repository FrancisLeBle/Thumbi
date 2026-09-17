import React, { useState } from 'react';
import { ChevronLeft, Info, CheckCircle2 } from 'lucide-react';
import { userService } from '../services/userService';

export interface AddVehicleScreenProps {
  onBack: () => void;
  onVehicleAdded?: () => void;
}

export interface VehicleFormData {
  brand: string;
  model: string;
  color: string;
  plate: string;
}

export const AddVehicleScreen: React.FC<AddVehicleScreenProps> = ({
  onBack,
  onVehicleAdded,
}) => {
  // Estado local para los campos del formulario
  const [vehicleData, setVehicleData] = useState<VehicleFormData>({
    brand: '',
    model: '',
    color: '',
    plate: '',
  });

  const [isSubmitting, setIsSubmitting] = useState<boolean>(false);
  const [toastMessage, setToastMessage] = useState<string | null>(null);

  // Validación: Marca, Modelo y Patente son requeridos
  const isValid =
    vehicleData.brand.trim().length > 0 &&
    vehicleData.model.trim().length > 0 &&
    vehicleData.plate.trim().length >= 5;

  const showToast = (msg: string) => {
    setToastMessage(msg);
    setTimeout(() => {
      setToastMessage(null);
    }, 2800);
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!isValid || isSubmitting) return;

    setIsSubmitting(true);
    try {
      await userService.addVehicle({
        brand: vehicleData.brand.trim(),
        model: vehicleData.model.trim(),
        color: vehicleData.color.trim() || 'Blanco',
        plate: vehicleData.plate.trim().toUpperCase(),
      });

      showToast('¡Vehículo registrado con éxito!');
      if (onVehicleAdded) {
        onVehicleAdded();
      }
      setTimeout(() => {
        onBack();
      }, 700);
    } catch {
      showToast('Error al registrar el vehículo. Intente nuevamente.');
      setIsSubmitting(false);
    }
  };

  return (
    <div
      id="add-vehicle-screen"
      className="w-full max-w-md mx-auto min-h-screen bg-[#F7F9FA] px-5 pt-4 pb-28 space-y-4 flex flex-col justify-between select-none relative"
      style={{
        fontFamily:
          '-apple-system, BlinkMacSystemFont, "Plus Jakarta Sans", "Inter", "SF Pro Text", "Segoe UI", Roboto, sans-serif',
      }}
    >
      {/* Toast Notification */}
      {toastMessage && (
        <div className="fixed top-5 left-1/2 -translate-x-1/2 z-50 bg-[#1A1A1A] text-white px-4 py-2.5 rounded-full text-xs font-medium shadow-lg flex items-center space-x-2 animate-in fade-in slide-in-from-top-2 duration-200">
          <CheckCircle2 className="w-4 h-4 text-[#00A896]" />
          <span>{toastMessage}</span>
        </div>
      )}

      {/* Top Section: Header & Form Content */}
      <div className="space-y-4">
        {/* Navigation Header */}
        <header className="relative pt-1 pb-1 flex items-center justify-between z-20">
          <button
            id="add-vehicle-back-btn"
            type="button"
            onClick={onBack}
            className="w-10 h-10 -ml-2 rounded-full flex items-center justify-center text-[#1A1A1A] hover:bg-[#EFF5F2] active:scale-95 transition-all cursor-pointer"
            aria-label="Volver a Perfil"
          >
            <ChevronLeft className="w-6 h-6 stroke-[2.4] text-[#1A1A1A]" />
          </button>
          <h1
            id="add-vehicle-header-title"
            className="text-[18px] font-bold text-[#1A1A1A] text-center tracking-tight flex-1 mr-8"
          >
            Agregar vehículo
          </h1>
        </header>

        {/* Main Form Area */}
        <main className="space-y-5 pt-1">
          {/* Form Card */}
          <section
            id="vehicle-form-card"
            className="bg-white rounded-2xl shadow-[0_2px_8px_rgba(0,0,0,0.04)] border border-neutral-100 overflow-hidden divide-y divide-[#EEF2F6]"
          >
            {/* Field 1: Marca */}
            <div className="px-4 py-3">
              <label
                htmlFor="car-brand"
                className="block text-[11px] font-bold tracking-wider text-[#6B7280] uppercase"
              >
                Marca
              </label>
              <input
                id="car-brand"
                type="text"
                value={vehicleData.brand}
                onChange={(e) =>
                  setVehicleData((prev) => ({ ...prev, brand: e.target.value }))
                }
                placeholder="Ej: Toyota"
                className="w-full mt-0.5 p-0 bg-transparent border-0 text-[15px] font-medium text-[#1A1A1A] placeholder:text-neutral-400 focus:ring-0 focus:outline-none"
              />
            </div>

            {/* Field 2: Modelo */}
            <div className="px-4 py-3">
              <label
                htmlFor="car-model"
                className="block text-[11px] font-bold tracking-wider text-[#6B7280] uppercase"
              >
                Modelo
              </label>
              <input
                id="car-model"
                type="text"
                value={vehicleData.model}
                onChange={(e) =>
                  setVehicleData((prev) => ({ ...prev, model: e.target.value }))
                }
                placeholder="Ej: Corolla"
                className="w-full mt-0.5 p-0 bg-transparent border-0 text-[15px] font-medium text-[#1A1A1A] placeholder:text-neutral-400 focus:ring-0 focus:outline-none"
              />
            </div>

            {/* Field 3: Color */}
            <div className="px-4 py-3">
              <label
                htmlFor="car-color"
                className="block text-[11px] font-bold tracking-wider text-[#6B7280] uppercase"
              >
                Color
              </label>
              <input
                id="car-color"
                type="text"
                value={vehicleData.color}
                onChange={(e) =>
                  setVehicleData((prev) => ({ ...prev, color: e.target.value }))
                }
                placeholder="Ej: Blanco"
                className="w-full mt-0.5 p-0 bg-transparent border-0 text-[15px] font-medium text-[#1A1A1A] placeholder:text-neutral-400 focus:ring-0 focus:outline-none"
              />
            </div>

            {/* Field 4: Patente / Dominio */}
            <div className="px-4 py-3">
              <label
                htmlFor="car-plate"
                className="block text-[11px] font-bold tracking-wider text-[#6B7280] uppercase"
              >
                Patente / Dominio
              </label>
              <input
                id="car-plate"
                type="text"
                value={vehicleData.plate}
                onChange={(e) =>
                  setVehicleData((prev) => ({
                    ...prev,
                    plate: e.target.value.toUpperCase(),
                  }))
                }
                placeholder="Ej: AA 123 CD"
                className="w-full mt-0.5 p-0 bg-transparent border-0 text-[15px] font-medium text-[#1A1A1A] uppercase placeholder:normal-case placeholder:text-neutral-400 focus:ring-0 focus:outline-none font-mono"
              />
            </div>
          </section>

          {/* Banner Informativo */}
          <section
            id="vehicle-info-banner"
            className="bg-[#E6F7F5] rounded-2xl p-4 flex items-start gap-3.5 border border-[#BCEEE6] shadow-xs"
          >
            <div className="shrink-0 mt-0.5 text-[#00A896]">
              <Info className="w-5 h-5 stroke-[2.2]" />
            </div>
            <p className="text-[13px] leading-[1.45] text-[#028090] font-normal">
              Asegurate de registrar el vehículo con el que realizarás los viajes para que tus pasajeros puedan identificarte fácilmente.
            </p>
          </section>
        </main>
      </div>

      {/* Bottom Action Area: Botón de Guardado */}
      <div id="save-vehicle-container" className="pt-4">
        <button
          id="btn-save-vehicle"
          type="button"
          disabled={!isValid || isSubmitting}
          onClick={handleSubmit}
          className={`w-full py-3.5 rounded-2xl font-semibold text-[16px] flex items-center justify-center shadow-sm transition-all duration-150 cursor-pointer ${
            isValid && !isSubmitting
              ? 'bg-[#00A896] hover:bg-[#028090] active:scale-[0.99] text-white'
              : 'bg-slate-200 text-slate-400 cursor-not-allowed opacity-80'
          }`}
        >
          {isSubmitting ? 'Guardando...' : 'Guardar vehículo'}
        </button>
      </div>
    </div>
  );
};

export default AddVehicleScreen;
