import React, { useState } from 'react';
import { Mail, Lock, Loader2, LogIn, ArrowRight, ShieldCheck } from 'lucide-react';
import { authService, AuthResponse } from '../services/authService';
import { ApiClientError } from '../services/apiClient';
import { getErrorMessage } from '../utils/errorHelpers';
import { Toast } from '../components/Toast';

export interface LoginScreenProps {
  onLoginSuccess?: (authData: AuthResponse) => void;
  onNavigateToRegister?: () => void;
}

export const LoginScreen: React.FC<LoginScreenProps> = ({
  onLoginSuccess,
  onNavigateToRegister,
}) => {
  const [email, setEmail] = useState<string>('');
  const [password, setPassword] = useState<string>('');
  const [isLoading, setIsLoading] = useState<boolean>(false);
  const [toast, setToast] = useState<{
    message: string;
    type: 'error' | 'success';
  } | null>(null);

  const isFormValid = email.trim() !== '' && password.trim().length >= 6;

  const handleSubmit = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    if (!isFormValid || isLoading) return;

    setIsLoading(true);

    try {
      const response = await authService.loginWithEmail({
        email: email.trim(),
        password,
      });

      setToast({
        message: '¡Sesión iniciada correctamente! Bienvenido de vuelta a Thumbi.',
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

  return (
    <div
      id="login-screen"
      className="min-h-screen py-10 px-4 sm:px-6 lg:px-8 flex flex-col justify-center items-center"
      style={{
        backgroundColor: '#F7F9FA',
        fontFamily: '-apple-system, BlinkMacSystemFont, "SF Pro Text", "Segoe UI", Roboto, sans-serif',
      }}
    >
      <div className="w-full max-w-md">
        {/* Cabecera / Identidad */}
        <div className="mb-8 text-center">
          <div className="inline-flex items-center justify-center w-12 h-12 rounded-full bg-[#E6F6F4] text-[#00A896] mb-3">
            <LogIn className="w-6 h-6" />
          </div>
          <h1 className="text-2xl sm:text-3xl font-bold tracking-tight text-[#1A1A1A]">
            Iniciar Sesión
          </h1>
          <p className="mt-2 text-sm text-[#666666]">
            Ingresa a tu cuenta para gestionar tus viajes y reservas protegidas.
          </p>
        </div>

        {/* Tarjeta de Formulario */}
        <div
          id="login-card"
          className="rounded-[12px] p-6 sm:p-8 bg-[#FFFFFF]"
          style={{
            boxShadow: '0px 2px 8px rgba(0, 0, 0, 0.04)',
            border: '1px solid #EFEFEF',
          }}
        >
          <form id="login-form" onSubmit={handleSubmit} className="space-y-5">
            {/* Campo: Correo Electrónico (autoComplete="username") */}
            <div>
              <label
                htmlFor="login-email-input"
                className="block text-xs font-semibold uppercase tracking-wider text-[#4A4A4A] mb-1.5"
              >
                Correo Electrónico
              </label>
              <div className="relative rounded-lg">
                <div className="absolute inset-y-0 left-0 pl-3.5 flex items-center pointer-events-none text-[#888888]">
                  <Mail className="w-4 h-4" />
                </div>
                <input
                  id="login-email-input"
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

            {/* Campo: Contraseña (autoComplete="current-password") */}
            <div>
              <div className="flex items-center justify-between mb-1.5">
                <label
                  htmlFor="login-password-input"
                  className="block text-xs font-semibold uppercase tracking-wider text-[#4A4A4A]"
                >
                  Contraseña
                </label>
              </div>
              <div className="relative rounded-lg">
                <div className="absolute inset-y-0 left-0 pl-3.5 flex items-center pointer-events-none text-[#888888]">
                  <Lock className="w-4 h-4" />
                </div>
                <input
                  id="login-password-input"
                  name="password"
                  type="password"
                  required
                  autoComplete="current-password"
                  placeholder="••••••••"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  disabled={isLoading}
                  className="w-full pl-10 pr-4 py-2.5 bg-[#FFFFFF] border border-[#E0E0E0] rounded-lg text-sm text-[#1A1A1A] placeholder-[#999999] focus:outline-none focus:border-[#00A896] focus:ring-1 focus:ring-[#00A896] transition-colors"
                />
              </div>
            </div>

            {/* Botón Principal (type="submit") */}
            <div className="pt-2">
              <button
                id="submit-login-btn"
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
                    Iniciando sesión...
                  </>
                ) : (
                  'Iniciar Sesión'
                )}
              </button>
            </div>
          </form>

          {/* Enlace hacia Registro */}
          <div className="mt-6 pt-5 border-t border-[#F0F0F0] text-center">
            <p className="text-xs text-[#666666]">
              ¿No tienes una cuenta registrada?{' '}
              <button
                id="nav-to-register-btn"
                type="button"
                onClick={onNavigateToRegister}
                className="font-semibold text-[#00A896] hover:underline inline-flex items-center gap-1 ml-1 cursor-pointer"
              >
                <span>Crear cuenta</span>
                <ArrowRight className="w-3 h-3" />
              </button>
            </p>
          </div>
        </div>

        {/* Garantía de Seguridad */}
        <div className="mt-6 flex items-center justify-center gap-1.5 text-xs text-[#888888]">
          <ShieldCheck className="w-4 h-4 text-[#00A896]" />
          <span>Acceso seguro con token JWT firmado de 20 min de vigencia</span>
        </div>
      </div>

      {toast && (
        <Toast
          id="login-toast"
          message={toast.message}
          type={toast.type}
          onClose={() => setToast(null)}
        />
      )}
    </div>
  );
};

export default LoginScreen;
