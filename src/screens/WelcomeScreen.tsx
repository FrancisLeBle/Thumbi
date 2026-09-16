import React, { useState } from 'react';
import { ShieldCheck, Users, Car, Sparkles, ArrowRight, LogIn, UserPlus, CheckCircle2 } from 'lucide-react';

export interface WelcomeScreenProps {
  onNavigateToLogin: () => void;
  onNavigateToRegister: () => void;
}

export const WelcomeScreen: React.FC<WelcomeScreenProps> = ({
  onNavigateToLogin,
  onNavigateToRegister,
}) => {
  const [activeSlide, setActiveSlide] = useState<number>(0);

  const slides = [
    {
      title: 'Viaja seguro y compartido',
      subtitle: 'Conecta con conductores y pasajeros verificados con validación biométrica y DNI.',
      tag: 'Seguridad Garantizada',
      icon: ShieldCheck,
      color: '#00A896',
      bgLight: '#E6F6F4',
    },
    {
      title: 'Ahorra en cada trayecto',
      subtitle: 'Divide los costos reales de peajes y combustible bajo el modelo de economía colaborativa.',
      tag: 'Precios Justos & Cap Price',
      icon: Car,
      color: '#028090',
      bgLight: '#E3F2FD',
    },
    {
      title: 'Pagos con custodia Escrow',
      subtitle: 'Tus fondos están protegidos en depósito de custodia hasta que el viaje concluye satisfactoriamente.',
      tag: 'Protección Total',
      icon: Sparkles,
      color: '#00A896',
      bgLight: '#E6F6F4',
    },
  ];

  const currentSlide = slides[activeSlide];
  const IconComponent = currentSlide.icon;

  return (
    <div
      id="welcome-screen"
      className="min-h-screen flex flex-col justify-between py-8 px-4 sm:px-6 lg:px-8 max-w-md mx-auto"
      style={{
        backgroundColor: '#F7F9FA',
        fontFamily: '-apple-system, BlinkMacSystemFont, "SF Pro Text", "SF Pro Display", "Segoe UI", Roboto, sans-serif',
      }}
    >
      {/* Barra Superior con Logo */}
      <header className="flex items-center justify-between pt-2">
        <div className="flex items-center gap-2">
          <div className="w-9 h-9 rounded-xl bg-[#00A896] flex items-center justify-center text-white shadow-sm">
            <Car className="w-5 h-5 stroke-[2.2]" />
          </div>
          <div>
            <span className="text-xl font-bold tracking-tight text-[#1A1A1A]">Thumbi</span>
            <span className="text-xs ml-1.5 px-2 py-0.5 rounded-full bg-[#E6F6F4] text-[#00A896] font-semibold">
              Carpooling
            </span>
          </div>
        </div>
      </header>

      {/* Ilustración / Tarjeta Visual Central */}
      <div className="my-auto py-6">
        <div
          className="relative rounded-[20px] p-6 sm:p-8 bg-[#FFFFFF] overflow-hidden transition-all duration-300"
          style={{
            boxShadow: '0px 4px 16px rgba(0, 0, 0, 0.05)',
            border: '1px solid #ECECEC',
          }}
        >
          {/* Gráfico decorativo de fondo */}
          <div
            className="w-32 h-32 rounded-full absolute -top-10 -right-10 opacity-30 pointer-events-none"
            style={{ backgroundColor: currentSlide.color }}
          />

          {/* Icono Principal */}
          <div
            className="w-16 h-16 rounded-2xl flex items-center justify-center mb-6 transition-colors"
            style={{ backgroundColor: currentSlide.bgLight, color: currentSlide.color }}
          >
            <IconComponent className="w-8 h-8 stroke-[2]" />
          </div>

          {/* Badge de Tag */}
          <div className="inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-semibold mb-3 bg-[#F0F2F5] text-[#333333]">
            <CheckCircle2 className="w-3.5 h-3.5 text-[#00A896]" />
            <span>{currentSlide.tag}</span>
          </div>

          {/* Títulos */}
          <h2 className="text-2xl font-bold tracking-tight text-[#1A1A1A] mb-3">
            {currentSlide.title}
          </h2>
          <p className="text-sm text-[#555555] leading-relaxed">
            {currentSlide.subtitle}
          </p>

          {/* Ilustración Vectorial / Carpooling Scene */}
          <div className="mt-6 pt-5 border-t border-[#F2F2F2] flex items-center justify-between text-xs text-[#777777]">
            <div className="flex items-center gap-2">
              <div className="flex -space-x-2 overflow-hidden">
                <span className="inline-block h-7 w-7 rounded-full ring-2 ring-white bg-[#00A896] text-white text-[10px] font-bold flex items-center justify-center">
                  SO
                </span>
                <span className="inline-block h-7 w-7 rounded-full ring-2 ring-white bg-[#028090] text-white text-[10px] font-bold flex items-center justify-center">
                  CA
                </span>
                <span className="inline-block h-7 w-7 rounded-full ring-2 ring-white bg-[#02C39A] text-white text-[10px] font-bold flex items-center justify-center">
                  MA
                </span>
              </div>
              <span className="font-medium text-[#444444]">+2.4k miembros activos</span>
            </div>
            <span className="text-[11px] font-semibold text-[#00A896]">100% Verificados</span>
          </div>
        </div>

        {/* Indicador de Carrusel (Dots) */}
        <div className="flex items-center justify-center gap-2 mt-6">
          {slides.map((_, idx) => (
            <button
              key={idx}
              type="button"
              onClick={() => setActiveSlide(idx)}
              aria-label={`Ir al slide ${idx + 1}`}
              className={`h-2 rounded-full transition-all duration-300 cursor-pointer ${
                activeSlide === idx
                  ? 'w-8 bg-[#00A896]'
                  : 'w-2 bg-[#D1D5DB] hover:bg-[#9CA3AF]'
              }`}
            />
          ))}
        </div>
      </div>

      {/* Botones de Acción */}
      <footer className="w-full space-y-3 pb-4">
        {/* Iniciar Sesión */}
        <button
          id="welcome-login-btn"
          type="button"
          onClick={onNavigateToLogin}
          className="w-full flex items-center justify-center gap-2 py-3.5 px-4 rounded-[12px] text-white font-semibold text-sm transition-all focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-[#00A896] cursor-pointer shadow-sm"
          style={{ backgroundColor: '#00A896' }}
        >
          <LogIn className="w-4 h-4" />
          <span>Iniciar Sesión</span>
        </button>

        {/* Crear Cuenta */}
        <button
          id="welcome-register-btn"
          type="button"
          onClick={onNavigateToRegister}
          className="w-full flex items-center justify-center gap-2 py-3.5 px-4 rounded-[12px] text-[#1A1A1A] font-semibold text-sm bg-[#FFFFFF] border border-[#E0E0E0] hover:bg-[#F9F9F9] transition-all focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-[#00A896] cursor-pointer shadow-xs"
        >
          <UserPlus className="w-4 h-4 text-[#00A896]" />
          <span>Crear cuenta</span>
        </button>

        <p className="text-center text-[11px] text-[#888888] pt-2">
          Al continuar aceptas nuestros Términos de Servicio y Política de Privacidad.
        </p>
      </footer>
    </div>
  );
};

export default WelcomeScreen;
