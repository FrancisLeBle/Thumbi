import React from 'react';
import {
  User,
  ShieldCheck,
  Car,
  CreditCard,
  Bell,
  LogOut,
  ChevronRight,
  CheckCircle2,
  FileCheck,
  Star,
  Settings,
  Lock,
} from 'lucide-react';
import { BottomNav, BottomNavTab } from '../components/BottomNav';

export interface ProfileScreenProps {
  userName?: string;
  userEmail?: string;
  role?: 'PASSENGER' | 'DRIVER';
  onNavigateToHome: () => void;
  onNavigateToSearch: () => void;
  onNavigateToPublish: () => void;
  onNavigateToTrips: () => void;
  onLogout: () => void;
  onToggleRole?: () => void;
}

export const ProfileScreen: React.FC<ProfileScreenProps> = ({
  userName = 'Sofía Martínez',
  userEmail = 'sofia.martinez@ejemplo.com',
  role = 'PASSENGER',
  onNavigateToHome,
  onNavigateToSearch,
  onNavigateToPublish,
  onNavigateToTrips,
  onLogout,
  onToggleRole,
}) => {
  const handleBottomNavChange = (tab: BottomNavTab) => {
    if (tab === 'home') onNavigateToHome();
    else if (tab === 'search') onNavigateToSearch();
    else if (tab === 'publish') onNavigateToPublish();
    else if (tab === 'trips') onNavigateToTrips();
  };

  return (
    <div
      id="profile-screen"
      className="min-h-screen pb-28 pt-6 px-4 sm:px-6 max-w-xl mx-auto"
      style={{
        backgroundColor: '#F7F9FA',
        fontFamily: '-apple-system, BlinkMacSystemFont, "SF Pro Text", "Segoe UI", Roboto, sans-serif',
      }}
    >
      {/* Encabezado de Perfil */}
      <div className="bg-[#FFFFFF] border border-[#EFEFEF] rounded-[12px] p-6 shadow-sm mb-5 text-center sm:text-left">
        <div className="flex flex-col sm:flex-row items-center gap-4">
          <div className="relative">
            <div className="w-16 h-16 rounded-full bg-[#E6F6F4] text-[#00A896] flex items-center justify-center font-bold text-2xl border-2 border-[#00A896]">
              {userName.charAt(0)}
            </div>
            <div className="absolute bottom-0 right-0 bg-[#00A896] text-white p-1 rounded-full shadow-sm">
              <CheckCircle2 className="w-3.5 h-3.5" />
            </div>
          </div>

          <div className="flex-1 space-y-1">
            <div className="flex flex-wrap items-center justify-center sm:justify-start gap-2">
              <h2 className="text-lg font-bold text-[#1A1A1A]">{userName}</h2>
              <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-[11px] font-semibold bg-emerald-50 text-emerald-700 border border-emerald-200">
                <ShieldCheck className="w-3 h-3 text-[#00A896]" />
                <span>Identidad Validada</span>
              </span>
            </div>
            <p className="text-xs text-[#666666]">{userEmail}</p>
            <div className="flex items-center justify-center sm:justify-start gap-3 pt-1 text-xs text-[#777777]">
              <span className="flex items-center gap-1 font-semibold text-[#1A1A1A]">
                <Star className="w-3.5 h-3.5 text-amber-500 fill-amber-500" />
                4.95 (28 reseñas)
              </span>
              <span>•</span>
              <span>Rol actual: <strong className="text-[#00A896]">{role === 'DRIVER' ? 'Conductor' : 'Pasajero'}</strong></span>
            </div>
          </div>
        </div>

        {/* Botón para cambiar de rol Conductor / Pasajero */}
        {onToggleRole && (
          <div className="mt-5 pt-4 border-t border-[#F0F0F0] flex items-center justify-between">
            <div>
              <p className="text-xs font-semibold text-[#1A1A1A]">
                {role === 'DRIVER' ? 'Modo Conductor Activo' : 'Modo Pasajero Activo'}
              </p>
              <p className="text-[11px] text-[#888888]">
                {role === 'DRIVER' ? 'Puedes publicar viajes y recibir pasajeros' : 'Busca y reserva asientos en viajes de confianza'}
              </p>
            </div>
            <button
              type="button"
              onClick={onToggleRole}
              className="px-3 py-1.5 rounded-lg border border-[#00A896] text-[#00A896] text-xs font-semibold hover:bg-[#E6F6F4] transition cursor-pointer"
            >
              Cambiar a {role === 'DRIVER' ? 'Pasajero' : 'Conductor'}
            </button>
          </div>
        )}
      </div>

      {/* Secciones de Ajustes y Verificación */}
      <div className="space-y-4">
        {/* Verificación de Seguridad y Documentos */}
        <div className="bg-[#FFFFFF] border border-[#EFEFEF] rounded-[12px] p-4 shadow-sm space-y-3">
          <h3 className="text-xs font-bold text-[#888888] uppercase tracking-wider">
            Seguridad y Documentación
          </h3>

          <div className="flex items-center justify-between py-2 border-b border-[#F5F5F5] text-xs">
            <div className="flex items-center gap-3">
              <div className="w-8 h-8 rounded-lg bg-emerald-50 text-emerald-600 flex items-center justify-center">
                <FileCheck className="w-4 h-4" />
              </div>
              <div>
                <p className="font-semibold text-[#1A1A1A]">DNI Frente y Dorso</p>
                <p className="text-[11px] text-emerald-600 font-medium">Verificado y cifrado</p>
              </div>
            </div>
            <span className="text-[11px] text-[#888888]">Auditado</span>
          </div>

          <div className="flex items-center justify-between py-2 border-b border-[#F5F5F5] text-xs">
            <div className="flex items-center gap-3">
              <div className="w-8 h-8 rounded-lg bg-emerald-50 text-emerald-600 flex items-center justify-center">
                <ShieldCheck className="w-4 h-4" />
              </div>
              <div>
                <p className="font-semibold text-[#1A1A1A]">Contrato de Custodia Escrow</p>
                <p className="text-[11px] text-[#888888]">Protección de pagos activa en cada reserva</p>
              </div>
            </div>
            <span className="text-[11px] text-emerald-600 font-bold">Activo</span>
          </div>

          <div className="flex items-center justify-between py-2 text-xs">
            <div className="flex items-center gap-3">
              <div className="w-8 h-8 rounded-lg bg-[#E6F6F4] text-[#00A896] flex items-center justify-center">
                <Lock className="w-4 h-4" />
              </div>
              <div>
                <p className="font-semibold text-[#1A1A1A]">Sesión Segura JWT</p>
                <p className="text-[11px] text-[#888888]">Firma criptográfica con expiración de 20 min</p>
              </div>
            </div>
            <span className="text-[11px] text-[#888888]">Vigente</span>
          </div>
        </div>

        {/* Acciones Rápidas */}
        <div className="bg-[#FFFFFF] border border-[#EFEFEF] rounded-[12px] p-2 shadow-sm divide-y divide-[#F5F5F5]">
          <button
            type="button"
            onClick={onNavigateToTrips}
            className="w-full flex items-center justify-between p-3 text-xs text-[#1A1A1A] hover:bg-slate-50 rounded-lg transition cursor-pointer"
          >
            <div className="flex items-center gap-3">
              <Car className="w-4 h-4 text-[#00A896]" />
              <span className="font-medium">Historial completo de reservas y viajes</span>
            </div>
            <ChevronRight className="w-4 h-4 text-[#888888]" />
          </button>

          <button
            type="button"
            onClick={onNavigateToPublish}
            className="w-full flex items-center justify-between p-3 text-xs text-[#1A1A1A] hover:bg-slate-50 rounded-lg transition cursor-pointer"
          >
            <div className="flex items-center gap-3">
              <Car className="w-4 h-4 text-[#00A896]" />
              <span className="font-medium">Ofrecer un trayecto como conductor</span>
            </div>
            <ChevronRight className="w-4 h-4 text-[#888888]" />
          </button>
        </div>

        {/* Botón Cerrar Sesión */}
        <div className="pt-2">
          <button
            type="button"
            onClick={onLogout}
            className="w-full flex items-center justify-center gap-2 py-3 px-4 rounded-[12px] bg-rose-50 text-rose-600 hover:bg-rose-100 font-semibold text-xs transition cursor-pointer"
          >
            <LogOut className="w-4 h-4" />
            <span>Cerrar Sesión</span>
          </button>
        </div>
      </div>

      {/* Barra de Navegación Inferior Fija */}
      <BottomNav
        activeTab="profile"
        onTabChange={handleBottomNavChange}
      />
    </div>
  );
};

export default ProfileScreen;
