import React, { useState } from 'react';
import { Mail, Lock, User, Phone, Car, Users, Loader2, UserPlus, ArrowLeft, ArrowRight, ShieldCheck, Check } from 'lucide-react';
import { authService, AuthResponse } from '../services/authService';
import { ApiClientError } from '../services/apiClient';
import { getErrorMessage } from '../utils/errorHelpers';
import { Toast } from '../components/Toast';

export interface RegisterScreenProps {
  onRegisterSuccess?: (authData: AuthResponse) => void;
  onNavigateToLogin?: () => void;
  onContinueToVerification?: (userData: { email: string; name: string; role: 'PASSENGER' | 'DRIVER' }) => void;
}

export const RegisterScreen: React.FC<RegisterScreenProps> = ({
  onRegisterSuccess,
  onNavigateToLogin,
  onContinueToVerification,
}) => {
  const [firstName, setFirstName] = useState<string>('');
  const [lastName, setLastName] = useState<string>('');
  const [email, setEmail] = useState<string>('');
  const [phone, setPhone] = useState<string>('');
  const [role, setRole] = useState<'PASSENGER' | 'DRIVER'>('PASSENGER');
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
        phone: phone.trim(),
        role,
      });

      setToast({
        message: `¡Cuenta pre-registrada! Continuando al Paso 2: Verificación de Identidad.`,
        type: 'success',
      });

      if (onRegisterSuccess) {
        onRegisterSuccess(response);
      }

      if (onContinueToVerification) {
        onContinueToVerification({
          email: email.trim(),
          name: firstName.trim(),
          role,
        });
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
        <div className="mb-6 text-center">
          <div className="flex items-center justify-between mb-4">
            {onNavigateToLogin && (
              <button
                type="button"
                onClick={onNavigateToLogin}
                className="p-2 -ml-2 rounded-lg text-[#666666] hover:bg-slate-200/60 transition cursor-pointer"
              >
                <ArrowLeft className="w-5 h-5" />
              </button>
            )}
            <span className="text-xs font-semibold text-[#00A896] bg-[#E6F6F4] px-2.5 py-1 rounded-full ml-auto">
              Paso 1 de 2 • Datos Personales
            </span>
          </div>
          <div className="inline-flex items-center justify-center w-12 h-12 rounded-full bg-[#E6F6F4] text-[#00A896] mb-2">
            <UserPlus className="w-6 h-6" />
          </div>
          <h1 className="text-2xl sm:text-3xl font-bold tracking-tight text-[#1A1A1A]">
            Crear Cuenta
          </h1>
          <p className="mt-1 text-sm text-[#666666]">
            Completa tus datos para unirte a la comunidad Thumbi.
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
            {/* Selector de Rol: Pasajero / Conductor */}
            <div>
              <label className="block text-xs font-semibold uppercase tracking-wider text-[#4A4A4A] mb-1.5">
                Quiero usar Thumbi como
              </label>
              <div className="grid grid-cols-2 gap-3">
                <button
                  id="role-passenger-btn"
                  type="button"
                  onClick={() => setRole('PASSENGER')}
                  className={`flex items-center justify-center gap-2 py-2.5 px-3 rounded-[10px] border text-xs font-semibold transition cursor-pointer ${
                    role === 'PASSENGER'
                      ? 'border-[#00A896] bg-[#E6F6F4] text-[#00A896] ring-1 ring-[#00A896]'
                      : 'border-[#E0E0E0] bg-[#FFFFFF] text-[#555555] hover:bg-slate-50'
                  }`}
                >
                  <Users className="w-4 h-4" />
                  <span>Pasajero</span>
                </button>

                <button
                  id="role-driver-btn"
                  type="button"
                  onClick={() => setRole('DRIVER')}
                  className={`flex items-center justify-center gap-2 py-2.5 px-3 rounded-[10px] border text-xs font-semibold transition cursor-pointer ${
                    role === 'DRIVER'
                      ? 'border-[#00A896] bg-[#E6F6F4] text-[#00A896] ring-1 ring-[#00A896]'
                      : 'border-[#E0E0E0] bg-[#FFFFFF] text-[#555555] hover:bg-slate-50'
                  }`}
                >
                  <Car className="w-4 h-4" />
                  <span>Conductor</span>
                </button>
              </div>
            </div>

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
                    placeholder="Ej: Sofía"
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
                    placeholder="Ej: Martínez"
                    value={lastName}
                    onChange={(e) => setLastName(e.target.value)}
                    disabled={isLoading}
                    className="w-full pl-10 pr-4 py-2.5 bg-[#FFFFFF] border border-[#E0E0E0] rounded-lg text-sm text-[#1A1A1A] placeholder-[#999999] focus:outline-none focus:border-[#00A896] focus:ring-1 focus:ring-[#00A896] transition-colors"
                  />
                </div>
              </div>
            </div>

            {/* Teléfono Móvil */}
            <div>
              <label
                htmlFor="register-phone-input"
                className="block text-xs font-semibold uppercase tracking-wider text-[#4A4A4A] mb-1.5"
              >
                Teléfono Móvil
              </label>
              <div className="relative rounded-lg">
                <div className="absolute inset-y-0 left-0 pl-3.5 flex items-center pointer-events-none text-[#888888]">
                  <Phone className="w-4 h-4" />
                </div>
                <input
                  id="register-phone-input"
                  name="phone"
                  type="tel"
                  autoComplete="tel"
                  placeholder="Ej: +54 9 11 2345-6789"
                  value={phone}
                  onChange={(e) => setPhone(e.target.value)}
                  disabled={isLoading}
                  className="w-full pl-10 pr-4 py-2.5 bg-[#FFFFFF] border border-[#E0E0E0] rounded-lg text-sm text-[#1A1A1A] placeholder-[#999999] focus:outline-none focus:border-[#00A896] focus:ring-1 focus:ring-[#00A896] transition-colors"
                />
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

            {/* Botón Principal: Continuar a Verificación (type="submit") */}
            <div className="pt-3">
              <button
                id="continue-to-verification-btn"
                type="submit"
                disabled={!isFormValid || isLoading}
                className="w-full flex items-center justify-center gap-2 py-3.5 px-4 rounded-[12px] text-white font-semibold text-sm transition-all focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-[#00A896] disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer shadow-sm"
                style={{
                  backgroundColor: '#00A896',
                }}
              >
                {isLoading ? (
                  <>
                    <Loader2 className="w-4 h-4 animate-spin" />
                    <span>Guardando datos...</span>
                  </>
                ) : (
                  <>
                    <span>Continuar a Verificación</span>
                    <ArrowRight className="w-4 h-4" />
                  </>
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
            <span>Datos protegidos bajo estrictas normas de privacidad</span>
          </div>
          <p className="text-[11px] text-[#999999]">
            En el siguiente paso validaremos tu documento de identidad oficial.
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
