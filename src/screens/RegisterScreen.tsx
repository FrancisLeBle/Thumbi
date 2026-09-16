import React, { useState } from 'react';
import { Loader2 } from 'lucide-react';
import { authService, AuthResponse } from '../services/authService';
import { ApiClientError } from '../services/apiClient';
import { getErrorMessage } from '../utils/errorHelpers';
import { Toast } from '../components/Toast';

export interface RegisterScreenProps {
  onRegisterSuccess?: (authData: AuthResponse) => void;
  onNavigateToLogin?: () => void;
  onNavigateToWelcome?: () => void;
  onContinueToVerification?: (userData: { email: string; name: string; role: 'PASSENGER' | 'DRIVER' }) => void;
}

export const RegisterScreen: React.FC<RegisterScreenProps> = ({
  onRegisterSuccess,
  onNavigateToLogin,
  onNavigateToWelcome,
  onContinueToVerification,
}) => {
  const [firstName, setFirstName] = useState<string>('Sofía');
  const [lastName, setLastName] = useState<string>('Fernández');
  const [email, setEmail] = useState<string>('sofia.f@email.com');
  const [password, setPassword] = useState<string>('Password123!');
  const [phone, setPhone] = useState<string>('+54 9 11 1234-5678');
  const [role, setRole] = useState<'PASSENGER' | 'DRIVER'>('PASSENGER');
  const [showPassword, setShowPassword] = useState<boolean>(false);
  const [isLoading, setIsLoading] = useState<boolean>(false);
  const [toast, setToast] = useState<{
    message: string;
    type: 'error' | 'success';
  } | null>(null);

  const handleBack = () => {
    if (onNavigateToLogin) {
      onNavigateToLogin();
    } else if (onNavigateToWelcome) {
      onNavigateToWelcome();
    }
  };

  const handleRegister = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    if (isLoading) return;

    if (!firstName.trim() || !lastName.trim() || !email.trim()) {
      setToast({
        message: 'Por favor completa todos los datos obligatorios.',
        type: 'error',
      });
      return;
    }

    if (password.length < 8) {
      setToast({
        message: 'La contraseña debe tener al menos 8 caracteres.',
        type: 'error',
      });
      return;
    }

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
        message: '¡Registro exitoso! Pasando a verificación de identidad.',
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
    <div className="min-h-screen flex items-center justify-center p-0 md:p-6 bg-slate-900 font-sans">
      {/* Toast Notification */}
      {toast && (
        <Toast
          message={toast.message}
          type={toast.type}
          onClose={() => setToast(null)}
        />
      )}

      {/* BEGIN: MobileDeviceFrame */}
      <div
        className="relative w-full max-w-[393px] h-[852px] bg-white flex flex-col justify-between overflow-hidden shadow-2xl md:rounded-[44px] border border-gray-200"
        data-purpose="mobile-viewport"
      >
        {/* Form Wrapping Whole Content and Actions */}
        <form
          onSubmit={handleRegister}
          className="w-full h-full flex flex-col justify-between"
          noValidate={false}
        >
          {/* BEGIN: TopContainer */}
          <div className="w-full flex flex-col overflow-y-auto">
            {/* BEGIN: iOSStatusBar */}
            <div
              className="w-full h-11 px-7 flex items-center justify-between pt-1 text-black font-semibold text-[14px] tracking-tight shrink-0 select-none"
              data-purpose="ios-status-bar"
            >
              <span className="font-medium">9:41</span>
              <div className="flex items-center space-x-2">
                {/* Cellular Icon */}
                <svg
                  className="w-4 h-3.5 fill-current text-black"
                  viewBox="0 0 17 11"
                  xmlns="http://www.w3.org/2000/svg"
                >
                  <rect height="3.5" rx="0.7" width="2.5" x="0" y="7.5" />
                  <rect height="6" rx="0.7" width="2.5" x="4.5" y="5" />
                  <rect height="8.5" rx="0.7" width="2.5" x="9" y="2.5" />
                  <rect height="11" rx="0.7" width="2.5" x="13.5" y="0" />
                </svg>
                {/* Wi-Fi Icon */}
                <svg
                  className="w-4 h-3.5 fill-current text-black"
                  viewBox="0 0 16 12"
                  xmlns="http://www.w3.org/2000/svg"
                >
                  <path d="M8 10.5a1.5 1.5 0 100-3 1.5 1.5 0 000 3zm-4.2-3.8a6.002 6.002 0 018.4 0 .8.8 0 101.1-1.15 7.6 7.6 0 00-10.6 0 .8.8 0 101.1 1.15zm-2.4-2.4a9.5 9.5 0 0113.2 0 .8.8 0 101.1-1.15 11.1 11.1 0 00-15.4 0 .8.8 0 001.1 1.15z" />
                </svg>
                {/* Battery Icon */}
                <div className="flex items-center">
                  <div className="w-5 h-2.5 border border-black rounded-[4px] p-0.5 flex items-center">
                    <div className="w-full h-full bg-black rounded-[1.5px]" />
                  </div>
                  <div className="w-0.5 h-1 bg-black rounded-r-sm -ml-[0.5px]" />
                </div>
              </div>
            </div>
            {/* END: iOSStatusBar */}

            {/* BEGIN: NavigationHeader */}
            <header
              className="relative px-5 py-3 flex items-center justify-between shrink-0"
              data-purpose="navigation-header"
            >
              <button
                aria-label="Volver"
                onClick={handleBack}
                className="text-gray-900 p-1 -ml-1 flex items-center justify-center focus:outline-none hover:opacity-75 transition-opacity cursor-pointer"
                type="button"
              >
                <svg className="w-6 h-6 stroke-current stroke-2" fill="none" viewBox="0 0 24 24">
                  <path
                    d="M10.5 19.5L3 12m0 0l7.5-7.5M3 12h18"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                  />
                </svg>
              </button>

              <h1 className="absolute left-1/2 -translate-x-1/2 text-base font-bold text-gray-900 tracking-tight">
                Crea tu cuenta
              </h1>
              <div className="w-6" />
            </header>
            {/* END: NavigationHeader */}

            {/* BEGIN: ProgressIndicator */}
            <section className="px-6 pt-1 pb-3 shrink-0" data-purpose="step-progress">
              <div className="flex justify-between items-center text-xs mb-2">
                <span className="font-semibold text-[#00A896]">Paso 1 de 2</span>
                <span className="text-gray-500 font-normal">Datos personales</span>
              </div>
              <div className="w-full h-1 bg-gray-200 rounded-full overflow-hidden">
                <div className="w-1/2 h-full bg-[#00A896] rounded-full transition-all duration-300" />
              </div>
            </section>
            {/* END: ProgressIndicator */}

            {/* BEGIN: FormContent */}
            <main className="px-6 flex flex-col space-y-3">
              {/* Field Row 1: Nombre & Apellido */}
              <div className="grid grid-cols-2 gap-3.5">
                {/* First Name */}
                <div className="flex flex-col">
                  <label className="text-xs font-semibold text-gray-900 mb-1.5" htmlFor="firstName">
                    Nombre
                  </label>
                  <input
                    className="w-full h-11 px-3.5 text-sm bg-gray-100 border border-transparent rounded-xl text-gray-900 focus:bg-white focus:border-[#00A896] focus:outline-none transition-all"
                    id="firstName"
                    name="firstName"
                    type="text"
                    autoComplete="given-name"
                    required
                    value={firstName}
                    onChange={(e) => setFirstName(e.target.value)}
                  />
                </div>
                {/* Last Name */}
                <div className="flex flex-col">
                  <label className="text-xs font-semibold text-gray-900 mb-1.5" htmlFor="lastName">
                    Apellido
                  </label>
                  <input
                    className="w-full h-11 px-3.5 text-sm bg-gray-100 border border-transparent rounded-xl text-gray-900 focus:bg-white focus:border-[#00A896] focus:outline-none transition-all"
                    id="lastName"
                    name="lastName"
                    type="text"
                    autoComplete="family-name"
                    required
                    value={lastName}
                    onChange={(e) => setLastName(e.target.value)}
                  />
                </div>
              </div>

              {/* Field Row 2: Correo Electrónico con autoComplete="username" */}
              <div className="flex flex-col">
                <label className="text-xs font-semibold text-gray-900 mb-1.5" htmlFor="email">
                  Correo electrónico
                </label>
                <input
                  className="w-full h-11 px-3.5 text-sm bg-gray-100 border border-transparent rounded-xl text-gray-900 focus:bg-white focus:border-[#00A896] focus:outline-none transition-all"
                  id="email"
                  name="email"
                  type="email"
                  autoComplete="username"
                  required
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                />
              </div>

              {/* Field Row 3: Contraseña con autoComplete="new-password" */}
              <div className="flex flex-col">
                <label className="text-xs font-semibold text-gray-900 mb-1.5" htmlFor="password">
                  Contraseña
                </label>
                <div className="relative flex items-center w-full">
                  <span className="absolute left-3.5 text-gray-400 flex items-center justify-center pointer-events-none">
                    <svg className="w-4 h-4 fill-none stroke-current stroke-2" viewBox="0 0 24 24">
                      <rect height="11" rx="2" ry="2" width="18" x="3" y="11" />
                      <path d="M7 11V7a5 5 0 0110 0v4" strokeLinecap="round" strokeLinejoin="round" />
                    </svg>
                  </span>
                  <input
                    className="w-full h-11 pl-10 pr-10 text-sm bg-gray-100 border border-transparent rounded-xl text-gray-900 focus:bg-white focus:border-[#00A896] focus:outline-none transition-all"
                    id="password"
                    name="password"
                    placeholder="••••••••"
                    type={showPassword ? 'text' : 'password'}
                    autoComplete="new-password"
                    required
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                  />
                  <button
                    aria-label="Mostrar u ocultar contraseña"
                    className="absolute right-3.5 text-gray-400 hover:text-gray-700 focus:outline-none flex items-center justify-center cursor-pointer"
                    type="button"
                    onClick={() => setShowPassword(!showPassword)}
                  >
                    {showPassword ? (
                      <svg className="w-4 h-4 fill-none stroke-current stroke-2" viewBox="0 0 24 24">
                        <path d="M17.94 17.94A10.07 10.07 0 0112 20c-7 0-10-8-10-8a18.45 18.45 0 015.06-5.94M9.9 4.24A9.12 9.12 0 0112 4c7 0 10 8 10 8a18.5 18.5 0 01-2.16 3.19m-6.72-1.07a3 3 0 11-4.24-4.24" strokeLinecap="round" strokeLinejoin="round" />
                        <line x1="1" y1="1" x2="23" y2="23" strokeLinecap="round" strokeLinejoin="round" />
                      </svg>
                    ) : (
                      <svg className="w-4 h-4 fill-none stroke-current stroke-2" viewBox="0 0 24 24">
                        <path d="M2 12s3-7 10-7 10 7 10 7-3 7-10 7-10-7-10-7z" strokeLinecap="round" strokeLinejoin="round" />
                        <circle cx="12" cy="12" r="3" />
                      </svg>
                    )}
                  </button>
                </div>
              </div>

              {/* Field Row 4: Teléfono Móvil */}
              <div className="flex flex-col">
                <label className="text-xs font-semibold text-gray-900 mb-1.5" htmlFor="phone">
                  Teléfono móvil
                </label>
                <input
                  className="w-full h-11 px-3.5 text-sm bg-gray-100 border border-transparent rounded-xl text-gray-900 focus:bg-white focus:border-[#00A896] focus:outline-none transition-all"
                  id="phone"
                  name="phone"
                  type="tel"
                  autoComplete="tel"
                  value={phone}
                  onChange={(e) => setPhone(e.target.value)}
                />
              </div>

              {/* BEGIN: RoleSelector */}
              <div className="pt-2 flex flex-col space-y-2.5" data-purpose="role-selection">
                <h2 className="text-sm font-bold text-gray-900">¿Cómo deseas viajar hoy?</h2>

                {/* Option 1: Pasajero */}
                <div
                  role="button"
                  tabIndex={0}
                  onClick={() => setRole('PASSENGER')}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter' || e.key === ' ') setRole('PASSENGER');
                  }}
                  className={`relative flex items-center p-3 rounded-xl cursor-pointer shadow-xs transition-all ${
                    role === 'PASSENGER'
                      ? 'bg-[#F0F9F8] border-[1.5px] border-[#00A896]'
                      : 'bg-white border border-gray-200 hover:border-gray-300'
                  }`}
                >
                  <div
                    className={`w-10 h-10 rounded-full flex items-center justify-center shrink-0 mr-3 ${
                      role === 'PASSENGER'
                        ? 'bg-white text-[#00A896] shadow-xs'
                        : 'bg-gray-100 text-gray-500'
                    }`}
                  >
                    <svg className="w-5 h-5 fill-none stroke-current stroke-2" viewBox="0 0 24 24">
                      <path
                        d="M15.75 6a3.75 3.75 0 11-7.5 0 3.75 3.75 0 017.5 0zM4.501 20.118a7.5 7.5 0 0114.998 0A17.933 17.933 0 0112 21.75c-2.676 0-5.216-.584-7.499-1.632z"
                        strokeLinecap="round"
                        strokeLinejoin="round"
                      />
                    </svg>
                  </div>
                  <div className="flex flex-col grow pr-2">
                    <span className="text-[13px] font-bold text-gray-900 leading-tight">
                      Quiero viajar como Pasajero
                    </span>
                    <span className="text-[11px] text-gray-500 mt-0.5 leading-snug">
                      Busca conductores y comparte gastos diarios.
                    </span>
                  </div>
                  {role === 'PASSENGER' && (
                    <div className="w-5 h-5 rounded-full bg-[#00A896] flex items-center justify-center shrink-0 text-white ml-1">
                      <svg className="w-3 h-3 fill-none stroke-current stroke-[2.5]" viewBox="0 0 24 24">
                        <path d="M4.5 12.75l6 6 9-13.5" strokeLinecap="round" strokeLinejoin="round" />
                      </svg>
                    </div>
                  )}
                </div>

                {/* Option 2: Conductor */}
                <div
                  role="button"
                  tabIndex={0}
                  onClick={() => setRole('DRIVER')}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter' || e.key === ' ') setRole('DRIVER');
                  }}
                  className={`relative flex items-center p-3 rounded-xl cursor-pointer shadow-xs transition-all ${
                    role === 'DRIVER'
                      ? 'bg-[#F0F9F8] border-[1.5px] border-[#00A896]'
                      : 'bg-white border border-gray-200 hover:border-gray-300'
                  }`}
                >
                  <div
                    className={`w-10 h-10 rounded-full flex items-center justify-center shrink-0 mr-3 ${
                      role === 'DRIVER'
                        ? 'bg-white text-[#00A896] shadow-xs'
                        : 'bg-gray-100 text-gray-500'
                    }`}
                  >
                    <svg className="w-5 h-5 fill-none stroke-current stroke-2" viewBox="0 0 24 24">
                      <path
                        d="M8.25 18.75a1.5 1.5 0 01-3 0m3 0a1.5 1.5 0 00-3 0m3 0h6m-9 0H3.375a1.125 1.125 0 01-1.125-1.125V14.25m17.25 4.5a1.5 1.5 0 01-3 0m3 0a1.5 1.5 0 00-3 0m3 0h1.125c.621 0 1.129-.504 1.09-1.124a17.902 17.902 0 00-3.213-9.193 2.056 2.056 0 00-1.58-.86H6.388c-.62 0-1.19.33-1.493.86a17.902 17.902 0 00-3.213 9.193c-.039.62.469 1.124 1.09 1.124H5.25m14.25 0H5.25"
                        strokeLinecap="round"
                        strokeLinejoin="round"
                      />
                    </svg>
                  </div>
                  <div className="flex flex-col grow pr-2">
                    <span className="text-[13px] font-bold text-gray-900 leading-tight">
                      Quiero ofrecer viajes como Conductor
                    </span>
                    <span className="text-[11px] text-gray-500 mt-0.5 leading-snug">
                      Publica tus rutas y ahorra combustible.
                    </span>
                  </div>
                  {role === 'DRIVER' && (
                    <div className="w-5 h-5 rounded-full bg-[#00A896] flex items-center justify-center shrink-0 text-white ml-1">
                      <svg className="w-3 h-3 fill-none stroke-current stroke-[2.5]" viewBox="0 0 24 24">
                        <path d="M4.5 12.75l6 6 9-13.5" strokeLinecap="round" strokeLinejoin="round" />
                      </svg>
                    </div>
                  )}
                </div>
              </div>
              {/* END: RoleSelector */}
            </main>
            {/* END: FormContent */}
          </div>
          {/* END: TopContainer */}

          {/* BEGIN: BottomActionsArea */}
          <footer
            className="w-full px-6 pb-2 pt-2 flex flex-col items-center shrink-0 bg-white"
            data-purpose="footer-actions"
          >
            {/* Primary Action Button (type="submit") */}
            <button
              id="register-submit-button"
              type="submit"
              disabled={isLoading}
              className="w-full h-12 bg-[#00A896] hover:bg-[#008F80] active:scale-[0.985] text-white text-sm font-semibold rounded-xl flex items-center justify-center shadow-md transition-all cursor-pointer disabled:opacity-50"
            >
              {isLoading ? (
                <span className="flex items-center gap-2">
                  <Loader2 className="w-4 h-4 animate-spin" />
                  <span>Procesando...</span>
                </span>
              ) : (
                'Continuar a Verificación'
              )}
            </button>

            {/* Iniciar sesión si ya tiene cuenta */}
            {onNavigateToLogin && (
              <p className="mt-2 text-center text-xs text-gray-500">
                ¿Ya tienes una cuenta?{' '}
                <button
                  type="button"
                  onClick={onNavigateToLogin}
                  className="text-[#00A896] font-semibold hover:underline cursor-pointer"
                >
                  Inicia sesión
                </button>
              </p>
            )}

            {/* Terms & Privacy Legal Disclaimer */}
            <p className="mt-2 text-center text-[10px] text-gray-400 leading-normal max-w-[280px]">
              Al registrarte aceptas nuestros{' '}
              <a className="text-[#00A896] font-medium hover:underline" href="#">
                Términos y Condiciones
              </a>{' '}
              y{' '}
              <a className="text-[#00A896] font-medium hover:underline" href="#">
                Políticas de Privacidad
              </a>
              .
            </p>

            {/* iOS Home Indicator */}
            <div className="w-32 h-1 bg-black/80 rounded-full mt-3 mb-1" />
          </footer>
          {/* END: BottomActionsArea */}
        </form>
      </div>
      {/* END: MobileDeviceFrame */}
    </div>
  );
};

export default RegisterScreen;
