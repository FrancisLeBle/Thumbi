import React, { useState } from 'react';
import { Loader2 } from 'lucide-react';
import { authService, AuthResponse } from '../services/authService';
import { ApiClientError } from '../services/apiClient';
import { getErrorMessage } from '../utils/errorHelpers';
import { Toast } from '../components/Toast';

export interface LoginScreenProps {
  onLoginSuccess?: (authData: AuthResponse) => void;
  onNavigateToRegister?: () => void;
  onNavigateToWelcome?: () => void;
}

export const LoginScreen: React.FC<LoginScreenProps> = ({
  onLoginSuccess,
  onNavigateToRegister,
  onNavigateToWelcome,
}) => {
  const [email, setEmail] = useState<string>('sofia.f@email.com');
  const [password, setPassword] = useState<string>('password123');
  const [showPassword, setShowPassword] = useState<boolean>(false);
  const [isLoading, setIsLoading] = useState<boolean>(false);
  const [toast, setToast] = useState<{
    message: string;
    type: 'error' | 'success';
  } | null>(null);

  const handleBack = () => {
    if (onNavigateToWelcome) {
      onNavigateToWelcome();
    } else if (onNavigateToRegister) {
      onNavigateToRegister();
    }
  };

  const handleLogin = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    if (isLoading) return;

    if (!email.trim() || !password) {
      setToast({
        message: 'Por favor ingresa tu correo electrónico y contraseña.',
        type: 'error',
      });
      return;
    }

    setIsLoading(true);

    try {
      const response = await authService.login({
        email: email.trim(),
        password,
      });

      setToast({
        message: '¡Bienvenido de nuevo! Sesión iniciada con éxito.',
        type: 'success',
      });

      if (onLoginSuccess) {
        onLoginSuccess(response);
      }
    } catch (err: unknown) {
      let message = 'No se pudo iniciar sesión. Verifica tus credenciales.';

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

  const handleSocialLogin = async (provider: 'Google' | 'Apple') => {
    if (isLoading) return;
    setIsLoading(true);

    try {
      // Simula el flujo nativo de inicio de sesión social
      const response = await authService.login({
        email: provider === 'Google' ? 'sofia.google@gmail.com' : 'sofia.apple@icloud.com',
        password: 'social-session-token',
      });

      setToast({
        message: `Sesión iniciada con cuenta de ${provider}.`,
        type: 'success',
      });

      if (onLoginSuccess) {
        onLoginSuccess(response);
      }
    } catch {
      setToast({
        message: `No se pudo conectar con ${provider}. Inténtalo con tu correo.`,
        type: 'error',
      });
    } finally {
      setIsLoading(false);
    }
  };

  const handleForgotPassword = (e: React.MouseEvent) => {
    e.preventDefault();
    setToast({
      message: 'Te hemos enviado un correo para restablecer tu contraseña.',
      type: 'success',
    });
  };

  return (
    <div className="min-h-screen w-full flex flex-col justify-between bg-white text-[#1A1A1A] font-sans">
      {/* Toast de Notificaciones */}
      {toast && (
        <Toast
          message={toast.message}
          type={toast.type}
          onClose={() => setToast(null)}
        />
      )}

      {/* Main App Container */}
      <main className="w-full flex-1 flex flex-col justify-between max-w-xl mx-auto px-4 sm:px-6 py-4">
        {/* Top Scrollable Container */}
        <div className="w-full flex flex-col">
          {/* Header Bar with Back Arrow */}
          <div className="pt-2 pb-2 flex items-center shrink-0">
            <button
              aria-label="Volver"
              onClick={handleBack}
              className="w-10 h-10 -ml-2 rounded-full flex items-center justify-center text-[#1A1A1A] hover:bg-gray-100 active:scale-95 transition-all cursor-pointer"
              type="button"
            >
              <svg
                className="w-6 h-6"
                fill="none"
                stroke="currentColor"
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth="2.2"
                viewBox="0 0 24 24"
              >
                <path d="M15 18l-6-6 6-6" />
              </svg>
            </button>
          </div>

          {/* Form Content */}
          <div className="px-6 pt-1 flex flex-col">
            {/* Header Text */}
            <div className="mt-1 mb-6">
              <h1 className="text-[28px] font-bold text-[#1A1A1A] tracking-tight leading-tight mb-2">
                ¡Bienvenido de nuevo!
              </h1>
              <p className="text-[15px] text-[#6B7280] leading-relaxed">
                Ingresa tus credenciales para acceder a tu cuenta.
              </p>
            </div>

            {/* Form Fields wrapped in native <form> */}
            <form onSubmit={handleLogin} className="space-y-4">
              {/* Campo 1: Correo electrónico */}
              <div>
                <label
                  className="block text-[14px] font-semibold text-[#1A1A1A] mb-1.5"
                  htmlFor="email"
                >
                  Correo electrónico
                </label>
                <div className="relative flex items-center">
                  <span className="absolute left-4 text-[#9CA3AF] flex items-center pointer-events-none">
                    <svg
                      className="w-5 h-5"
                      fill="none"
                      stroke="currentColor"
                      strokeWidth="1.8"
                      viewBox="0 0 24 24"
                    >
                      <path
                        d="M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"
                        strokeLinecap="round"
                        strokeLinejoin="round"
                      />
                    </svg>
                  </span>
                  <input
                    id="email"
                    name="email"
                    type="email"
                    autoComplete="username"
                    required
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                    placeholder="sofia.f@email.com"
                    className="w-full h-[52px] pl-11 pr-4 bg-[#F7F9FA] text-[#1A1A1A] text-[15px] font-medium rounded-xl border border-transparent focus:border-[#00A896] focus:bg-white focus:outline-none transition-colors placeholder:text-[#9CA3AF]"
                  />
                </div>
              </div>

              {/* Campo 2: Contraseña */}
              <div>
                <label
                  className="block text-[14px] font-semibold text-[#1A1A1A] mb-1.5"
                  htmlFor="password"
                >
                  Contraseña
                </label>
                <div className="relative flex items-center">
                  <span className="absolute left-4 text-[#9CA3AF] flex items-center pointer-events-none">
                    <svg
                      className="w-5 h-5"
                      fill="none"
                      stroke="currentColor"
                      strokeWidth="1.8"
                      viewBox="0 0 24 24"
                    >
                      <path
                        d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"
                        strokeLinecap="round"
                        strokeLinejoin="round"
                      />
                    </svg>
                  </span>
                  <input
                    id="password"
                    name="password"
                    type={showPassword ? 'text' : 'password'}
                    autoComplete="current-password"
                    required
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    placeholder="••••••••"
                    className="w-full h-[52px] pl-11 pr-12 bg-[#F7F9FA] text-[#1A1A1A] text-[15px] font-medium rounded-xl border border-transparent focus:border-[#00A896] focus:bg-white focus:outline-none transition-colors placeholder:text-[#9CA3AF]"
                  />
                  <button
                    aria-label="Mostrar u ocultar contraseña"
                    type="button"
                    onClick={() => setShowPassword(!showPassword)}
                    className="absolute right-3.5 text-[#9CA3AF] hover:text-[#4B5563] p-1 flex items-center focus:outline-none cursor-pointer"
                  >
                    {showPassword ? (
                      <svg
                        className="w-5 h-5"
                        fill="none"
                        stroke="currentColor"
                        strokeWidth="1.8"
                        viewBox="0 0 24 24"
                      >
                        <path
                          d="M17.94 17.94A10.07 10.07 0 0112 20c-7 0-10-8-10-8a18.45 18.45 0 015.06-5.94M9.9 4.24A9.12 9.12 0 0112 4c7 0 10 8 10 8a18.5 18.5 0 01-2.16 3.19m-6.72-1.07a3 3 0 11-4.24-4.24"
                          strokeLinecap="round"
                          strokeLinejoin="round"
                        />
                        <line x1="1" y1="1" x2="23" y2="23" strokeLinecap="round" strokeLinejoin="round" />
                      </svg>
                    ) : (
                      <svg
                        className="w-5 h-5"
                        fill="none"
                        stroke="currentColor"
                        strokeWidth="1.8"
                        viewBox="0 0 24 24"
                      >
                        <path
                          d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"
                          strokeLinecap="round"
                          strokeLinejoin="round"
                        />
                        <path
                          d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z"
                          strokeLinecap="round"
                          strokeLinejoin="round"
                        />
                      </svg>
                    )}
                  </button>
                </div>

                {/* Link: ¿Olvidaste tu contraseña? */}
                <div className="flex justify-end mt-2">
                  <button
                    type="button"
                    onClick={handleForgotPassword}
                    className="text-[13px] font-semibold text-[#00A896] hover:text-[#028090] transition-colors cursor-pointer"
                  >
                    ¿Olvidaste tu contraseña?
                  </button>
                </div>
              </div>

              {/* Botón de Acción Principal (type="submit") */}
              <div className="pt-2">
                <button
                  id="login-submit-button"
                  type="submit"
                  disabled={isLoading}
                  className="w-full h-[52px] bg-[#00A896] hover:bg-[#028090] active:scale-[0.99] text-white font-bold text-[16px] rounded-xl flex items-center justify-center shadow-[0_4px_14px_rgba(0,168,150,0.3)] transition-all cursor-pointer disabled:opacity-50"
                >
                  {isLoading ? (
                    <span className="flex items-center gap-2">
                      <Loader2 className="w-5 h-5 animate-spin" />
                      <span>Iniciando sesión...</span>
                    </span>
                  ) : (
                    'Iniciar Sesión'
                  )}
                </button>
              </div>
            </form>

            {/* Divider sutil */}
            <div className="relative my-6 flex items-center justify-center">
              <div className="border-t border-gray-100 w-full" />
              <span className="bg-white px-3 text-[12px] text-[#9CA3AF] uppercase tracking-wider font-medium absolute">
                o
              </span>
            </div>

            {/* Botones de acceso rápido social */}
            <div className="grid grid-cols-2 gap-3">
              <button
                type="button"
                onClick={() => handleSocialLogin('Google')}
                disabled={isLoading}
                className="h-[46px] border border-gray-200 rounded-xl bg-white hover:bg-gray-50 flex items-center justify-center space-x-2 text-[13px] font-semibold text-[#1A1A1A] transition-colors cursor-pointer"
              >
                <svg className="w-4 h-4" viewBox="0 0 24 24">
                  <path
                    d="M12 5c1.6 0 3 .6 4.1 1.7l3.1-3.1C17.3 1.8 14.8 1 12 1 7.4 1 3.5 3.6 1.6 7.4l3.7 2.9C6.2 7.3 8.9 5 12 5z"
                    fill="#EA4335"
                  />
                  <path
                    d="M23.5 12.3c0-.8-.1-1.6-.2-2.3H12v4.5h6.5c-.3 1.5-1.1 2.8-2.4 3.7l3.7 2.9c2.2-2 3.7-5 3.7-8.8z"
                    fill="#4285F4"
                  />
                  <path
                    d="M5.3 14.7c-.2-.7-.4-1.5-.4-2.7 0-1.1.2-2 .4-2.7L1.6 6.4C.6 8.3 0 10.5 0 12.8s.6 4.5 1.6 6.4l3.7-4.5z"
                    fill="#FBBC05"
                  />
                  <path
                    d="M12 23.5c3.2 0 6-1.1 8-3l-3.7-2.9c-1.1.7-2.5 1.2-4.3 1.2-3.1 0-5.8-2.3-6.7-5.3L1.6 18.2C3.5 22 7.4 24.6 12 24.6z"
                    fill="#34A853"
                  />
                </svg>
                <span>Google</span>
              </button>

              <button
                type="button"
                onClick={() => handleSocialLogin('Apple')}
                disabled={isLoading}
                className="h-[46px] border border-gray-200 rounded-xl bg-white hover:bg-gray-50 flex items-center justify-center space-x-2 text-[13px] font-semibold text-[#1A1A1A] transition-colors cursor-pointer"
              >
                <svg className="w-4 h-4 fill-current" viewBox="0 0 24 24">
                  <path d="M18.71 19.5c-.83 1.24-1.71 2.45-3.05 2.47-1.34.03-1.77-.79-3.29-.79-1.53 0-2 .77-3.27.82-1.31.05-2.3-1.32-3.14-2.53C4.25 17 2.94 12.45 4.7 9.39c.87-1.52 2.43-2.48 4.12-2.51 1.28-.02 2.5.87 3.29.87.78 0 2.26-1.07 3.81-.91.65.03 2.47.26 3.64 1.98-.09.06-2.17 1.28-2.15 3.81.03 3.02 2.65 4.03 2.68 4.04-.03.07-.42 1.44-1.38 2.83M15.97 6.84c.62-.75 1.04-1.8 0.93-2.84-.9.04-2 .6-2.64 1.34-.56.65-.05 1.71.07 2.73 1.01.08 2.02-.48 2.64-1.23" />
                </svg>
                <span>Apple</span>
              </button>
            </div>
          </div>
        </div>

        {/* Footer: Acceso Directo a Registro */}
        <footer className="w-full px-6 pb-3 pt-4 shrink-0 bg-white flex flex-col items-center">
          <p className="text-[14px] text-[#6B7280]">
            ¿No tienes una cuenta?{' '}
            <button
              id="goto-register-btn"
              type="button"
              onClick={onNavigateToRegister}
              className="font-bold text-[#00A896] hover:text-[#028090] transition-colors ml-1 cursor-pointer"
            >
              Registrate
            </button>
          </p>
        </footer>
      </main>
    </div>
  );
};

export default LoginScreen;
