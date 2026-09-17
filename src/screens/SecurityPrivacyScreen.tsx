import React, { useState } from 'react';
import {
  ChevronLeft,
  Key,
  ShieldCheck,
  Eye,
  FileText,
  Trash2,
  ChevronRight,
  X,
  CheckCircle2,
  AlertTriangle,
  Lock,
  Check,
} from 'lucide-react';

export interface SecurityPrivacyScreenProps {
  onBack: () => void;
  onLogout?: () => void;
}

export type PhoneVisibilityOption =
  | 'Solo reservas confirmadas'
  | 'Visible para miembros verificados'
  | 'Oculto siempre (chat interno)';

export const SecurityPrivacyScreen: React.FC<SecurityPrivacyScreenProps> = ({
  onBack,
  onLogout,
}) => {
  // Estado local para autenticación biométrica (Face ID / Huella)
  const [isBiometricEnabled, setIsBiometricEnabled] = useState<boolean>(true);

  // Estado local para la visibilidad del número telefónico
  const [phoneVisibility, setPhoneVisibility] =
    useState<PhoneVisibilityOption>('Solo reservas confirmadas');

  // Modales de interacción
  const [isPasswordModalOpen, setIsPasswordModalOpen] = useState<boolean>(false);
  const [isPhoneModalOpen, setIsPhoneModalOpen] = useState<boolean>(false);
  const [isTermsModalOpen, setIsTermsModalOpen] = useState<boolean>(false);
  const [isDeleteModalOpen, setIsDeleteModalOpen] = useState<boolean>(false);

  // Formulario de cambio de contraseña
  const [currentPassword, setCurrentPassword] = useState<string>('');
  const [newPassword, setNewPassword] = useState<string>('');
  const [confirmPassword, setConfirmPassword] = useState<string>('');
  const [passwordError, setPasswordError] = useState<string | null>(null);

  // Feedback toast flotante
  const [toastMessage, setToastMessage] = useState<string | null>(null);

  const showToast = (msg: string) => {
    setToastMessage(msg);
    setTimeout(() => {
      setToastMessage(null);
    }, 3200);
  };

  const handleToggleBiometric = () => {
    const nextState = !isBiometricEnabled;
    setIsBiometricEnabled(nextState);
    showToast(
      nextState
        ? 'Autenticación biométrica (Face ID / Huella) activada'
        : 'Autenticación biométrica desactivada'
    );
  };

  const handlePasswordSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (newPassword.length < 6) {
      setPasswordError('La nueva contraseña debe tener al menos 6 caracteres');
      return;
    }
    if (newPassword !== confirmPassword) {
      setPasswordError('Las contraseñas no coinciden');
      return;
    }

    setPasswordError(null);
    setIsPasswordModalOpen(false);
    setCurrentPassword('');
    setNewPassword('');
    setConfirmPassword('');
    showToast('Contraseña actualizada con éxito');
  };

  const handleConfirmDeleteAccount = () => {
    setIsDeleteModalOpen(false);
    showToast('Tu cuenta ha sido dada de baja correctamente');
    setTimeout(() => {
      if (onLogout) {
        onLogout();
      } else {
        onBack();
      }
    }, 1200);
  };

  return (
    <div
      id="security-privacy-screen"
      className="w-full max-w-md mx-auto min-h-screen bg-[#F7F9FA] px-5 pt-4 pb-28 space-y-4 select-none relative"
      style={{
        fontFamily:
          '-apple-system, BlinkMacSystemFont, "Plus Jakarta Sans", "Inter", sans-serif',
      }}
    >
      {/* Toast Notification */}
      {toastMessage && (
        <div className="fixed top-5 left-1/2 -translate-x-1/2 z-50 bg-[#1A1A1A] text-white px-4 py-2.5 rounded-full text-xs font-medium shadow-lg flex items-center space-x-2 animate-in fade-in slide-in-from-top-2 duration-200">
          <CheckCircle2 className="w-4 h-4 text-[#00A896]" />
          <span>{toastMessage}</span>
        </div>
      )}

      {/* Screen Header */}
      <header className="relative pt-1 pb-1 flex items-center justify-between z-20">
        <button
          id="security-back-btn"
          type="button"
          onClick={onBack}
          className="w-10 h-10 -ml-2 rounded-full flex items-center justify-center text-[#1A1A1A] hover:bg-[#EFF5F2] active:scale-95 transition-all cursor-pointer"
          aria-label="Volver a Perfil"
        >
          <ChevronLeft className="w-6 h-6 stroke-[2.4] text-[#1A1A1A]" />
        </button>
        <h1
          id="security-header-title"
          className="text-[18px] font-bold text-[#1A1A1A] text-center tracking-tight flex-1 mr-8"
        >
          Seguridad y Privacidad
        </h1>
      </header>

      {/* Main Content Sections */}
      <main className="space-y-5 pt-1">
        {/* Section 1: ACCESO Y AUTENTICACIÓN */}
        <section id="auth-access-section" aria-labelledby="auth-access-heading">
          <h2
            id="auth-access-heading"
            className="text-[12px] font-bold text-[#6B7280] tracking-wider uppercase mb-2 px-1"
          >
            ACCESO Y AUTENTICACIÓN
          </h2>

          <div className="bg-white rounded-2xl shadow-[0_2px_8px_rgba(0,0,0,0.03)] border border-gray-100 overflow-hidden divide-y divide-gray-100">
            {/* Row 1: Cambiar contraseña */}
            <div
              id="row-change-password"
              onClick={() => setIsPasswordModalOpen(true)}
              className="flex items-center justify-between px-4 py-3.5 hover:bg-[#F9FBFC] active:bg-gray-50/80 transition cursor-pointer group"
            >
              <div className="flex items-center space-x-3.5">
                <div className="w-8 h-8 rounded-xl bg-[#E6F7F5] flex items-center justify-center text-[#00A896] flex-shrink-0">
                  <Key className="w-4 h-4 stroke-[2]" />
                </div>
                <span className="text-[15px] font-medium text-[#1A1A1A]">
                  Cambiar contraseña
                </span>
              </div>
              <ChevronRight className="w-4 h-4 text-gray-400 group-hover:text-[#00A896] transition-colors stroke-[2.2]" />
            </div>

            {/* Row 2: Autenticación biométrica con Switch Interactivo */}
            <div
              id="row-biometric-auth"
              className="flex items-center justify-between px-4 py-3.5 hover:bg-[#F9FBFC] transition"
            >
              <div className="flex items-center space-x-3.5">
                <div className="w-8 h-8 rounded-xl bg-[#E6F7F5] flex items-center justify-center text-[#00A896] flex-shrink-0">
                  <ShieldCheck className="w-4 h-4 stroke-[2]" />
                </div>
                <div>
                  <span className="text-[15px] font-medium text-[#1A1A1A] block">
                    Autenticación biométrica
                  </span>
                  <span className="text-[12px] text-[#6B7280]">
                    Face ID / Huella dactilar
                  </span>
                </div>
              </div>

              {/* Toggle Switch animado */}
              <button
                id="toggle-biometric-switch"
                type="button"
                role="switch"
                aria-checked={isBiometricEnabled}
                onClick={handleToggleBiometric}
                className={`relative inline-flex h-7 w-12 items-center rounded-full transition-colors duration-200 cursor-pointer focus:outline-none ${
                  isBiometricEnabled ? 'bg-[#00A896]' : 'bg-gray-300'
                }`}
              >
                <span
                  className={`inline-block h-5 w-5 transform rounded-full bg-white shadow-md transition-transform duration-200 ${
                    isBiometricEnabled ? 'translate-x-6' : 'translate-x-1'
                  }`}
                />
              </button>
            </div>
          </div>
        </section>

        {/* Section 2: PRIVACIDAD DE DATOS */}
        <section id="data-privacy-section" aria-labelledby="data-privacy-heading">
          <h2
            id="data-privacy-heading"
            className="text-[12px] font-bold text-[#6B7280] tracking-wider uppercase mb-2 px-1"
          >
            PRIVACIDAD DE DATOS
          </h2>

          <div className="bg-white rounded-2xl shadow-[0_2px_8px_rgba(0,0,0,0.03)] border border-gray-100 overflow-hidden divide-y divide-gray-100">
            {/* Row 1: Visibilidad de teléfono */}
            <div
              id="row-phone-visibility"
              onClick={() => setIsPhoneModalOpen(true)}
              className="flex items-center justify-between px-4 py-3.5 hover:bg-[#F9FBFC] active:bg-gray-50/80 transition cursor-pointer group"
            >
              <div className="flex items-center space-x-3.5 pr-2 min-w-0">
                <div className="w-8 h-8 rounded-xl bg-blue-50 flex items-center justify-center text-[#05668D] flex-shrink-0">
                  <Eye className="w-4 h-4 stroke-[2]" />
                </div>
                <span className="text-[15px] font-medium text-[#1A1A1A] truncate">
                  Visibilidad de teléfono
                </span>
              </div>

              <div className="flex items-center space-x-1.5 text-[#6B7280] flex-shrink-0">
                <span className="text-[13px] font-medium text-[#475569] truncate max-w-[140px] text-right">
                  {phoneVisibility}
                </span>
                <ChevronRight className="w-4 h-4 text-gray-400 group-hover:text-[#00A896] transition-colors stroke-[2.2]" />
              </div>
            </div>

            {/* Row 2: Términos y Políticas de Privacidad */}
            <div
              id="row-terms-privacy"
              onClick={() => setIsTermsModalOpen(true)}
              className="flex items-center justify-between px-4 py-3.5 hover:bg-[#F9FBFC] active:bg-gray-50/80 transition cursor-pointer group"
            >
              <div className="flex items-center space-x-3.5">
                <div className="w-8 h-8 rounded-xl bg-blue-50 flex items-center justify-center text-[#05668D] flex-shrink-0">
                  <FileText className="w-4 h-4 stroke-[2]" />
                </div>
                <span className="text-[15px] font-medium text-[#1A1A1A]">
                  Términos y Políticas de Privacidad
                </span>
              </div>
              <ChevronRight className="w-4 h-4 text-gray-400 group-hover:text-[#00A896] transition-colors stroke-[2.2]" />
            </div>
          </div>
        </section>

        {/* Section 3: Acciones de Cuenta (Eliminar mi cuenta) */}
        <section id="account-actions-section">
          <div className="bg-white rounded-2xl shadow-[0_2px_8px_rgba(0,0,0,0.03)] border border-red-100 overflow-hidden">
            <button
              id="delete-account-btn"
              type="button"
              onClick={() => setIsDeleteModalOpen(true)}
              className="w-full flex items-center justify-between px-4 py-3.5 bg-red-50/40 hover:bg-red-50 transition cursor-pointer text-left group"
            >
              <div className="flex items-center space-x-3.5">
                <div className="w-8 h-8 rounded-xl bg-red-100/80 flex items-center justify-center text-[#E63946] flex-shrink-0">
                  <Trash2 className="w-4 h-4 stroke-[2]" />
                </div>
                <div>
                  <span className="text-[15px] font-semibold text-[#E63946] block">
                    Eliminar mi cuenta
                  </span>
                  <span className="text-[12px] text-red-400">
                    Baja permanente de perfil y datos
                  </span>
                </div>
              </div>
              <ChevronRight className="w-4 h-4 text-[#E63946]/60 group-hover:text-[#E63946] stroke-[2.2] transition-colors" />
            </button>
          </div>
        </section>

        {/* Footer Note */}
        <div className="pt-2 text-center">
          <p className="text-[12px] text-[#9CA3AF] font-medium">
            Thumbi Seguridad y Cifrado de Extremo a Extremo • v1.4.2
          </p>
        </div>
      </main>

      {/* Modal 1: Cambiar Contraseña */}
      {isPasswordModalOpen && (
        <div className="fixed inset-0 z-50 bg-black/40 backdrop-blur-xs flex items-end sm:items-center justify-center p-4">
          <div className="bg-white w-full max-w-md rounded-2xl p-5 shadow-xl border border-gray-100 animate-in fade-in slide-in-from-bottom-4 duration-200">
            <div className="flex items-center justify-between pb-3 border-b border-gray-100">
              <div className="flex items-center space-x-2">
                <div className="w-8 h-8 rounded-lg bg-[#E6F7F5] text-[#00A896] flex items-center justify-center">
                  <Lock className="w-4 h-4" />
                </div>
                <h3 className="text-base font-bold text-[#1A1A1A]">Cambiar Contraseña</h3>
              </div>
              <button
                type="button"
                onClick={() => {
                  setIsPasswordModalOpen(false);
                  setPasswordError(null);
                }}
                className="w-8 h-8 rounded-full text-gray-400 hover:bg-gray-100 flex items-center justify-center cursor-pointer"
              >
                <X className="w-4 h-4" />
              </button>
            </div>

            <form onSubmit={handlePasswordSubmit} className="space-y-3 mt-4">
              {passwordError && (
                <div className="p-2.5 bg-red-50 border border-red-200 rounded-xl text-xs text-red-600 font-medium flex items-center space-x-1.5">
                  <AlertTriangle className="w-4 h-4 flex-shrink-0" />
                  <span>{passwordError}</span>
                </div>
              )}

              <div>
                <label className="block text-xs font-semibold text-gray-700 mb-1">
                  Contraseña actual
                </label>
                <input
                  type="password"
                  required
                  value={currentPassword}
                  onChange={(e) => setCurrentPassword(e.target.value)}
                  placeholder="••••••••"
                  className="w-full px-3 py-2 text-sm border border-gray-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-[#00A896]/30 focus:border-[#00A896]"
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-gray-700 mb-1">
                  Nueva contraseña
                </label>
                <input
                  type="password"
                  required
                  value={newPassword}
                  onChange={(e) => setNewPassword(e.target.value)}
                  placeholder="Mínimo 6 caracteres"
                  className="w-full px-3 py-2 text-sm border border-gray-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-[#00A896]/30 focus:border-[#00A896]"
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-gray-700 mb-1">
                  Confirmar nueva contraseña
                </label>
                <input
                  type="password"
                  required
                  value={confirmPassword}
                  onChange={(e) => setConfirmPassword(e.target.value)}
                  placeholder="Repetir contraseña"
                  className="w-full px-3 py-2 text-sm border border-gray-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-[#00A896]/30 focus:border-[#00A896]"
                />
              </div>

              <div className="pt-2 flex items-center space-x-2.5">
                <button
                  type="button"
                  onClick={() => {
                    setIsPasswordModalOpen(false);
                    setPasswordError(null);
                  }}
                  className="w-1/2 py-2.5 text-xs font-semibold text-gray-600 bg-gray-100 hover:bg-gray-200 rounded-xl transition cursor-pointer"
                >
                  Cancelar
                </button>
                <button
                  type="submit"
                  className="w-1/2 py-2.5 text-xs font-semibold text-white bg-[#00A896] hover:bg-[#028090] rounded-xl shadow-sm transition cursor-pointer"
                >
                  Actualizar
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Modal 2: Selector de Visibilidad de Teléfono */}
      {isPhoneModalOpen && (
        <div className="fixed inset-0 z-50 bg-black/40 backdrop-blur-xs flex items-end sm:items-center justify-center p-4">
          <div className="bg-white w-full max-w-md rounded-2xl p-5 shadow-xl border border-gray-100 animate-in fade-in slide-in-from-bottom-4 duration-200">
            <div className="flex items-center justify-between pb-3 border-b border-gray-100">
              <div className="flex items-center space-x-2">
                <div className="w-8 h-8 rounded-lg bg-blue-50 text-[#05668D] flex items-center justify-center">
                  <Eye className="w-4 h-4" />
                </div>
                <h3 className="text-base font-bold text-[#1A1A1A]">Visibilidad de Teléfono</h3>
              </div>
              <button
                type="button"
                onClick={() => setIsPhoneModalOpen(false)}
                className="w-8 h-8 rounded-full text-gray-400 hover:bg-gray-100 flex items-center justify-center cursor-pointer"
              >
                <X className="w-4 h-4" />
              </button>
            </div>

            <div className="space-y-2.5 mt-4">
              {(
                [
                  {
                    value: 'Solo reservas confirmadas',
                    desc: 'Recomendado. Solo los pasajeros o conductores con reserva pagada verán tu número.',
                  },
                  {
                    value: 'Visible para miembros verificados',
                    desc: 'Cualquier usuario con DNI validado podrá ver tu teléfono de contacto.',
                  },
                  {
                    value: 'Oculto siempre (chat interno)',
                    desc: 'Tu número nunca se comparte. Toda coordinación se realizará por chat.',
                  },
                ] as const
              ).map((opt) => {
                const isSelected = phoneVisibility === opt.value;
                return (
                  <button
                    key={opt.value}
                    type="button"
                    onClick={() => {
                      setPhoneVisibility(opt.value);
                      setIsPhoneModalOpen(false);
                      showToast(`Visibilidad actualizada: "${opt.value}"`);
                    }}
                    className={`w-full p-3.5 rounded-xl border text-left flex items-start justify-between transition cursor-pointer ${
                      isSelected
                        ? 'border-[#00A896] bg-[#E6F7F5]/40'
                        : 'border-gray-200 hover:bg-gray-50'
                    }`}
                  >
                    <div className="pr-3">
                      <p
                        className={`text-sm font-semibold ${
                          isSelected ? 'text-[#00A896]' : 'text-gray-800'
                        }`}
                      >
                        {opt.value}
                      </p>
                      <p className="text-xs text-gray-500 mt-0.5 leading-snug">{opt.desc}</p>
                    </div>
                    {isSelected && (
                      <Check className="w-4 h-4 text-[#00A896] flex-shrink-0 mt-0.5 stroke-[2.5]" />
                    )}
                  </button>
                );
              })}
            </div>

            <div className="pt-3">
              <button
                type="button"
                onClick={() => setIsPhoneModalOpen(false)}
                className="w-full py-2.5 text-xs font-semibold text-gray-600 bg-gray-100 hover:bg-gray-200 rounded-xl transition cursor-pointer"
              >
                Cerrar
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Modal 3: Términos y Políticas de Privacidad */}
      {isTermsModalOpen && (
        <div className="fixed inset-0 z-50 bg-black/40 backdrop-blur-xs flex items-end sm:items-center justify-center p-4">
          <div className="bg-white w-full max-w-md rounded-2xl p-5 shadow-xl border border-gray-100 animate-in fade-in slide-in-from-bottom-4 duration-200 max-h-[85vh] flex flex-col">
            <div className="flex items-center justify-between pb-3 border-b border-gray-100 flex-shrink-0">
              <div className="flex items-center space-x-2">
                <div className="w-8 h-8 rounded-lg bg-blue-50 text-[#05668D] flex items-center justify-center">
                  <FileText className="w-4 h-4" />
                </div>
                <h3 className="text-base font-bold text-[#1A1A1A]">
                  Términos y Privacidad
                </h3>
              </div>
              <button
                type="button"
                onClick={() => setIsTermsModalOpen(false)}
                className="w-8 h-8 rounded-full text-gray-400 hover:bg-gray-100 flex items-center justify-center cursor-pointer"
              >
                <X className="w-4 h-4" />
              </button>
            </div>

            <div className="py-4 space-y-3 overflow-y-auto text-xs text-gray-600 leading-relaxed pr-1">
              <div className="p-3 bg-[#E6F7F5]/50 border border-[#BCEEE6] rounded-xl text-[#028090]">
                <p className="font-semibold text-xs mb-1">Protección de Datos Personales</p>
                <p className="text-[11px]">
                  Thumbi cumple con la Ley de Protección de Datos Personales (Ley 25.326). Tus datos biométricos y documentación de DNI se cifran con estándar AES-256.
                </p>
              </div>

              <div>
                <h4 className="font-bold text-gray-800 text-[13px] mb-1">1. Custodia de Fondos (Escrow)</h4>
                <p>
                  Los pagos por asientos quedan retenidos en una cuenta de custodia segura hasta la confirmación de llegada al destino. Los conductores reciben el pago únicamente al concluir el trayecto acordado.
                </p>
              </div>

              <div>
                <h4 className="font-bold text-gray-800 text-[13px] mb-1">2. Responsabilidad de Viaje</h4>
                <p>
                  Los viajes son compartidos entre particulares para prorrateo de gastos de combustible y peajes. No constituye un servicio de transporte público comercial.
                </p>
              </div>

              <div>
                <h4 className="font-bold text-gray-800 text-[13px] mb-1">3. Geolocalización y Seguridad</h4>
                <p>
                  Durante los trayectos activos, la app permite compartir la ubicación en tiempo real con contactos de confianza para máxima seguridad de pasajeros y choferes.
                </p>
              </div>
            </div>

            <div className="pt-2 border-t border-gray-100 flex-shrink-0">
              <button
                type="button"
                onClick={() => setIsTermsModalOpen(false)}
                className="w-full py-2.5 text-xs font-semibold text-white bg-[#00A896] hover:bg-[#028090] rounded-xl shadow-sm transition cursor-pointer"
              >
                Entendido
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Modal 4: Confirmación para Eliminar Cuenta */}
      {isDeleteModalOpen && (
        <div className="fixed inset-0 z-50 bg-black/50 backdrop-blur-xs flex items-center justify-center p-4">
          <div className="bg-white w-full max-w-sm rounded-2xl p-5 shadow-2xl border border-red-100 animate-in fade-in zoom-in-95 duration-200">
            <div className="w-12 h-12 rounded-2xl bg-red-100 text-[#E63946] flex items-center justify-center mx-auto mb-3">
              <AlertTriangle className="w-6 h-6 stroke-[2.2]" />
            </div>

            <h3 className="text-base font-bold text-center text-[#1A1A1A] mb-1">
              ¿Eliminar tu cuenta definitivamente?
            </h3>
            <p className="text-xs text-center text-[#6B7280] mb-4 leading-relaxed">
              Esta acción es irreversible. Se cancelarán tus reservas activas, se eliminarán tus vehículos registrados y se liquidarán los fondos pendientes a tu cuenta bancaria.
            </p>

            <div className="space-y-2">
              <button
                type="button"
                onClick={handleConfirmDeleteAccount}
                className="w-full py-2.5 text-xs font-semibold text-white bg-[#E63946] hover:bg-[#c92a37] rounded-xl shadow-sm transition cursor-pointer"
              >
                Sí, eliminar mi cuenta
              </button>
              <button
                type="button"
                onClick={() => setIsDeleteModalOpen(false)}
                className="w-full py-2.5 text-xs font-semibold text-gray-700 bg-gray-100 hover:bg-gray-200 rounded-xl transition cursor-pointer"
              >
                Cancelar y conservar cuenta
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

export default SecurityPrivacyScreen;
