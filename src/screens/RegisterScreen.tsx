import React, { useState } from 'react';
import { Mail, Lock, User, Loader2, UserPlus, ArrowLeft, ShieldCheck, Check } from 'lucide-react';
import { authService, AuthResponse } from '../services/authService';
import { ApiClientError } from '../services/apiClient';
import { getErrorMessage } from '../utils/errorHelpers';
import { Toast } from '../components/Toast';

export interface RegisterScreenProps {
  onRegisterSuccess?: (authData: AuthResponse) => void;
  onNavigateToLogin?: () => void;
}

export const RegisterScreen: React.FC<RegisterScreenProps> = ({
  onRegisterSuccess,
  onNavigateToLogin,
}) => {
  const [firstName, setFirstName] = useState<string>('');
  const [lastName, setLastName] = useState<string>('');
  const [email, setEmail] = useState<string>('');
  const [password, setPassword] = useState<string>('');
  const [confirmPassword, setConfirmPassword] = useState<string>('');
  const [isLoading, setIsLoading] = useState<boolean>(false);
  const [toast, setToast] = useState<{
    message: string;
    type: 'error' | 'success';
  } | null>(null);

  const passwordsMatch = password === confirmPassword;
  const isFormValid =
    firstName.trim() !== '' &&
    lastName.trim() !== '' &&
    email.trim() !== '' &&
    password.length >= 8 &&
    passwordsMatch;

  const handleSubmit = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    if (!passwordsMatch) {
      setToast({
        message: 'Las contraseñas no coinciden. Por favor verifica ambos campos.',
        type: 'error',
      });
      return;
    }

    if (!isFormValid || isLoading) return;

    setIsLoading(true);

    try {
      const response = await authService.registerWithEmail({
        email: email.trim(),
        password,
        firstName: firstName.trim(),
        lastName: lastName.trim(),
      });

      setToast({
        message: '¡Cuenta creada con éxito! Bienvenido a Thumbi. Tu rol base es Pasajero.',
        type: 'success',
      });

      if (onRegisterSuccess) {
        onRegisterSuccess(response);
      }
    } catch (err: unknown) {
      let message = 'No se pudo completar el registro. Inténtalo nuevamente.';

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
      id="register-screen"
      className="min-h-screen py-10 px-4 sm:px-6 lg:px-8 flex flex-col justify-center items-center"
      style={{
        backgroundColor: '#F7F9FA',
        fontFamily: '-apple-system, BlinkMacSystemFont, "SF Pro Text", "Segoe UI", Roboto, sans-serif',
      }}
    >
      <div className="w-full max-w-lg">
        {/* Cabecera / Identidad */}
        <div className="mb-8 text-center">
          <div className="inline-flex items-center justify-center w-12 h-12 rounded-full bg-[#E6F6F4] text-[#00A896] mb-3">
            <UserPlus className="w-6 h-6" />
          </div>
          <h1 className="text-2xl sm:text-3xl font-bold tracking-tight text-[#1A1A1A]">
            Crear Cuenta
          </h1>
          <p className="mt-2 text-sm text-[#666666]">
            Regístrate en la red de viajes compartidos segura y verificada.
          </p>
        </div>

        {/* Tarjeta de Formulario */}
        <div
          id="register-card"
          className="rounded-[12px] p-6 sm:p-8 bg-[#FFFFFF]"
          style={{
            boxShadow: '0px 2px 8px rgba(0, 0, 0, 0.04)',
            border: '1px solid #EFEFEF',
          }}
        >
          <form id="register-form" onSubmit={handleSubmit} className="space-y-4">
            {/* Nombre y Apellido */}
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <div>
                <label
                  htmlFor="register-firstname-input"
                  className="block text-xs font-semibold uppercase tracking-wider text-[#4A4A4A] mb-1.5"
                >
                  Nombre
                </label>
                <div className="relative rounded-lg">
                  <div className="absolute inset-y-0 left-0 pl-3.5 flex items-center pointer-events-none text-[#888888]">
                    <User className="w-4 h-4" />
                  </div>
                  <input
                    id="register-firstname-input"
                    name="firstName"
                    type="text"
                    required
                    autoComplete="given-name"
                    placeholder="Ej: Juan"
                    value={firstName}
                    onChange={(e) => setFirstName(e.target.value)}
                    disabled={isLoading}
                    className="w-full pl-10 pr-4 py-2.5 bg-[#FFFFFF] border border-[#E0E0E0] rounded-lg text-sm text-[#1A1A1A] placeholder-[#999999] focus:outline-none focus:border-[#00A896] focus:ring-1 focus:ring-[#00A896] transition-colors"
                  />
                </div>
              </div>

              <div>
                <label
                  htmlFor="register-lastname-input"
                  className="block text-xs font-semibold uppercase tracking-wider text-[#4A4A4A] mb-1.5"
                >
                  Apellido
                </label>
                <div className="relative rounded-lg">
                  <div className="absolute inset-y-0 left-0 pl-3.5 flex items-center pointer-events-none text-[#888888]">
                    <User className="w-4 h-4" />
                  </div>
                  <input
                    id="register-lastname-input"
                    name="lastName"
                    type="text"
                    required
                    autoComplete="family-name"
                    placeholder="Ej: Pérez"
                    value={lastName}
                    onChange={(e) => setLastName(e.target.value)}
                    disabled={isLoading}
                    className="w-full pl-10 pr-4 py-2.5 bg-[#FFFFFF] border border-[#E0E0E0] rounded-lg text-sm text-[#1A1A1A] placeholder-[#999999] focus:outline-none focus:border-[#00A896] focus:ring-1 focus:ring-[#00A896] transition-colors"
                  />
                </div>
              </div>
            </div>

            {/* Correo Electrónico (autoComplete="username") */}
            <div>
              <label
                htmlFor="register-email-input"
                className="block text-xs font-semibold uppercase tracking-wider text-[#4A4A4A] mb-1.5"
              >
                Correo Electrónico
              </label>
              <div className="relative rounded-lg">
                <div className="absolute inset-y-0 left-0 pl-3.5 flex items-center pointer-events-none text-[#888888]">
                  <Mail className="w-4 h-4" />
                </div>
                <input
                  id="register-email-input"
                  name="email"
                  type="email"
                  required
                  autoComplete="username"
                  placeholder="ejemplo@thumbi.com"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  disabled={isLoading}
                  className="w-full pl-10 pr-4 py-2.5 bg-[#FFFFFF] border border-[#E0E0E0] rounded-lg text-sm text-[#1A1A1A] placeholder-[#999999] focus:outline-none focus:border-[#00A896] focus:ring-1 focus:ring-[#00A896] transition-colors"
                />
              </div>
            </div>

            {/* Contraseña (autoComplete="new-password") */}
            <div>
              <div className="flex items-center justify-between mb-1.5">
                <label
                  htmlFor="register-password-input"
                  className="block text-xs font-semibold uppercase tracking-wider text-[#4A4A4A]"
                >
                  Contraseña
                </label>
                <span className="text-[11px] text-[#888888]">Mínimo 8 caracteres</span>
              </div>
              <div className="relative rounded-lg">
                <div className="absolute inset-y-0 left-0 pl-3.5 flex items-center pointer-events-none text-[#888888]">
                  <Lock className="w-4 h-4" />
                </div>
                <input
                  id="register-password-input"
                  name="password"
                  type="password"
                  required
                  autoComplete="new-password"
                  placeholder="••••••••"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  disabled={isLoading}
                  className="w-full pl-10 pr-4 py-2.5 bg-[#FFFFFF] border border-[#E0E0E0] rounded-lg text-sm text-[#1A1A1A] placeholder-[#999999] focus:outline-none focus:border-[#00A896] focus:ring-1 focus:ring-[#00A896] transition-colors"
                />
              </div>
            </div>

            {/* Confirmar Contraseña (autoComplete="new-password") */}
            <div>
              <label
                htmlFor="register-confirm-password-input"
                className="block text-xs font-semibold uppercase tracking-wider text-[#4A4A4A] mb-1.5"
              >
                Confirmar Contraseña
              </label>
              <div className="relative rounded-lg">
                <div className="absolute inset-y-0 left-0 pl-3.5 flex items-center pointer-events-none text-[#888888]">
                  <Lock className="w-4 h-4" />
                </div>
                <input
                  id="register-confirm-password-input"
                  name="confirmPassword"
                  type="password"
                  required
                  autoComplete="new-password"
                  placeholder="••••••••"
                  value={confirmPassword}
                  onChange={(e) => setConfirmPassword(e.target.value)}
                  disabled={isLoading}
                  className="w-full pl-10 pr-4 py-2.5 bg-[#FFFFFF] border border-[#E0E0E0] rounded-lg text-sm text-[#1A1A1A] placeholder-[#999999] focus:outline-none focus:border-[#00A896] focus:ring-1 focus:ring-[#00A896] transition-colors"
                />
              </div>
              {confirmPassword && !passwordsMatch && (
                <p className="mt-1 text-xs text-rose-500 font-medium">
                  Las contraseñas no coinciden.
                </p>
              )}
            </div>

            {/* Botón Principal (type="submit") */}
            <div className="pt-3">
              <button
                id="submit-register-btn"
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
                    Creando cuenta...
                  </>
                ) : (
                  'Registrarse'
                )}
              </button>
            </div>
          </form>

          {/* Enlace hacia Iniciar Sesión */}
          <div className="mt-6 pt-5 border-t border-[#F0F0F0] text-center">
            <p className="text-xs text-[#666666]">
              ¿Ya tienes una cuenta creada?{' '}
              <button
                id="nav-to-login-btn"
                type="button"
                onClick={onNavigateToLogin}
                className="font-semibold text-[#00A896] hover:underline inline-flex items-center gap-1 ml-1 cursor-pointer"
              >
                <ArrowLeft className="w-3 h-3" />
                <span>Iniciar sesión</span>
              </button>
            </p>
          </div>
        </div>

        {/* Términos e Identidad de Seguridad */}
        <div className="mt-6 space-y-2 text-center">
          <div className="flex items-center justify-center gap-1.5 text-xs text-[#888888]">
            <ShieldCheck className="w-4 h-4 text-[#00A896]" />
            <span>Datos cifrados y validación biométrica para conductores</span>
          </div>
          <p className="text-[11px] text-[#999999]">
            Al registrarte aceptas las políticas de uso responsable y protección de pagos en Escrow.
          </p>
        </div>
      </div>

      {toast && (
        <Toast
          id="register-toast"
          message={toast.message}
          type={toast.type}
          onClose={() => setToast(null)}
        />
      )}
    </div>
  );
};

export default RegisterScreen;
