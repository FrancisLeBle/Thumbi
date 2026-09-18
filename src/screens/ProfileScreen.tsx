import React, { useState } from 'react';
import {
  Car,
  Bell,
  CreditCard,
  Lock,
  HelpCircle,
  ChevronRight,
  LogOut,
  Plus,
  X,
  Check,
  ShieldCheck,
  Smartphone,
  MessageSquare,
  AlertCircle,
  CheckCircle2,
  Search,
  PlusCircle,
  User,
  Loader2,
} from 'lucide-react';
import { authService } from '../services/authService';
import { useUserContext } from '../context/UserContext';
import { useToast } from '../context/ToastContext';

export interface ProfileScreenProps {
  userName?: string;
  userEmail?: string;
  role?: 'PASSENGER' | 'DRIVER';
  onNavigateToHome: () => void;
  onNavigateToSearch: () => void;
  onNavigateToPublish: () => void;
  onNavigateToTrips: () => void;
  onLogout: () => void;
  onNavigateToWelcome?: () => void;
  onToggleRole?: () => void;
  onNavigateToHelpSupport?: () => void;
  onNavigateToSecurityPrivacy?: () => void;
  onNavigateToPaymentMethods?: () => void;
  onNavigateToNotifications?: () => void;
  onNavigateToAddVehicle?: () => void;
}

type ConfigModalType = 'notifications' | 'payments' | 'security' | 'help' | null;

export const ProfileScreen: React.FC<ProfileScreenProps> = ({
  userName: propUserName,
  userEmail: propUserEmail,
  onNavigateToHome,
  onNavigateToSearch,
  onNavigateToPublish,
  onNavigateToTrips,
  onLogout,
  onNavigateToWelcome,
  onNavigateToHelpSupport,
  onNavigateToSecurityPrivacy,
  onNavigateToPaymentMethods,
  onNavigateToNotifications,
  onNavigateToAddVehicle,
}) => {
  // Consumimos el contexto global de usuario
  const { user, setActiveVehicle, addVehicle, isLoading: isUserLoading } = useUserContext();
  const { showToast } = useToast();

  // Estados para modales de vehículo y configuración
  const [isAddVehicleOpen, setIsAddVehicleOpen] = useState<boolean>(false);
  const [activeConfigModal, setActiveConfigModal] = useState<ConfigModalType>(null);
  const [actionFeedback, setActionFeedback] = useState<string | null>(null);

  // Estado del formulario de nuevo vehículo
  const [newBrand, setNewBrand] = useState<string>('Toyota');
  const [newModel, setNewModel] = useState<string>('Yaris');
  const [newColor, setNewColor] = useState<string>('Gris Plata');
  const [newPlate, setNewPlate] = useState<string>('AF 456 XY');

  // Estados interactivos para las configuraciones
  const [pushEnabled, setPushEnabled] = useState<boolean>(true);
  const [whatsappAlerts, setWhatsappAlerts] = useState<boolean>(true);
  const [escrowAlerts, setEscrowAlerts] = useState<boolean>(true);

  // Cálculo de iniciales del nombre (ej. "Juan Francisco Lorusso" -> "JF")
  const getInitials = (name: string): string => {
    if (!name) return 'JF';
    const parts = name.trim().split(/\s+/);
    if (parts.length >= 2) {
      return `${parts[0][0]}${parts[1][0]}`.toUpperCase();
    }
    return parts[0].slice(0, 2).toUpperCase();
  };

  const displayName = user.name || propUserName || 'Juan Francisco Lorusso';
  const displayRating = typeof user.rating === 'number' ? user.rating.toFixed(1) : '4.9';
  const displayReviews = user.reviewsCount ?? 38;
  const isKycVerified = user.isVerified ?? true;
  const initials = getInitials(displayName);

  // Acción de agregar vehículo
  const handleAddVehicleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newBrand || !newModel || !newPlate) return;

    try {
      await addVehicle({
        brand: newBrand,
        model: newModel,
        color: newColor,
        plate: newPlate.toUpperCase(),
        isActive: true,
      });

      setIsAddVehicleOpen(false);
      setNewBrand('Toyota');
      setNewModel('');
      setNewPlate('');
      showFeedbackToast('Vehículo registrado y activado con éxito');
    } catch {
      showFeedbackToast('No se pudo guardar el vehículo. Intente nuevamente.');
    }
  };

  // Marcar vehículo como activo
  const handleSelectActiveVehicle = async (id: string) => {
    try {
      await setActiveVehicle(id);
      showFeedbackToast('Vehículo activo actualizado para tus viajes');
    } catch {
      showFeedbackToast('No se pudo actualizar el vehículo activo');
    }
  };

  // Cierre de sesión con authService.logout() y redirección a WelcomeScreen
  const handleLogoutClick = () => {
    authService.logout();
    if (onNavigateToWelcome) {
      onNavigateToWelcome();
    } else if (onLogout) {
      onLogout();
    }
  };

  const showFeedbackToast = (msg: string) => {
    setActionFeedback(msg);
    showToast(msg, 'success');
    setTimeout(() => {
      setActionFeedback(null);
    }, 3200);
  };

  return (
    <div className="min-h-screen flex items-center justify-center p-0 md:p-6 bg-slate-900 font-sans">
      {/* Mobile Device Frame */}
      <div
        id="profile-screen"
        className="w-full max-w-[393px] h-[852px] bg-[#F8FAFC] relative overflow-hidden flex flex-col md:rounded-[54px] shadow-2xl border border-slate-700/50 select-none text-[#1A1A1A]"
        data-purpose="iphone-mockup"
        style={{
          fontFamily:
            '-apple-system, BlinkMacSystemFont, "SF Pro Text", "SF Pro Display", "Inter", sans-serif',
        }}
      >
        {/* BEGIN: StatusBar */}
        <header className="w-full pt-3 px-7 flex justify-between items-center z-30 select-none bg-[#F8FAFC] shrink-0">
          {/* Time */}
          <span className="text-[15px] font-semibold text-gray-900 tracking-tight">9:41</span>

          {/* Dynamic Island Mockup */}
          <div className="w-28 h-7 bg-black rounded-full absolute left-1/2 -translate-x-1/2 top-2.5 flex items-center justify-between px-2.5 z-40">
            <div className="w-2.5 h-2.5 rounded-full bg-[#111] opacity-40" />
            <div className="w-2.5 h-2.5 rounded-full bg-[#0a192f] border border-[#1e293b]" />
          </div>

          {/* Network, WiFi & Battery Icons */}
          <div className="flex items-center space-x-2 text-gray-900">
            {/* Signal icon */}
            <svg className="w-4 h-4 fill-current" viewBox="0 0 24 24">
              <path d="M2 17h3v4H2zm5-4h3v8H7zm5-4h3v12h-3zm5-5h3v17h-3z" />
            </svg>
            {/* Wifi icon */}
            <svg className="w-4 h-4 fill-current" viewBox="0 0 24 24">
              <path d="M12 4C7.31 4 3.07 5.9 0 8.98L12 21 24 8.98C20.93 5.9 16.69 4 12 4zm0 3.32c3.84 0 7.33 1.54 9.89 4.04L12 18.9 2.11 11.36C4.67 8.86 8.16 7.32 12 7.32z" />
            </svg>
            {/* Battery icon */}
            <div className="flex items-center">
              <div className="w-6 h-3 rounded-[4px] border border-gray-900 p-0.5 flex items-center">
                <div className="w-4 h-full bg-gray-900 rounded-[1px]" />
              </div>
              <div className="w-0.5 h-1.5 bg-gray-900 rounded-r-sm" />
            </div>
          </div>
        </header>
        {/* END: StatusBar */}

        {/* Feedback flotante tipo Toast */}
        {actionFeedback && (
          <div className="absolute top-14 left-1/2 -translate-x-1/2 z-50 bg-[#1A1A1A] text-white px-4 py-2.5 rounded-full text-xs font-medium shadow-lg flex items-center space-x-2 transition-all">
            <CheckCircle2 className="w-4 h-4 text-[#00A896]" />
            <span>{actionFeedback}</span>
          </div>
        )}

        {/* Scrollable Screen Content */}
        <div className="flex-1 overflow-y-auto px-4 sm:px-6 pt-2 pb-6 space-y-4">
          {/* Encabezado de la pantalla */}
          <header className="pt-2 pb-1 text-center relative flex items-center justify-center">
            <h1
              id="profile-title"
              className="text-[20px] font-bold text-[#1A1A1A] tracking-tight"
            >
              Mi Perfil
            </h1>
          </header>

          {/* Tarjeta de Identidad del Usuario */}
        <div
          id="user-identity-card"
          className="bg-white rounded-2xl p-4 shadow-[0_2px_8px_rgba(0,0,0,0.04)] border border-[#F0F2F4] flex items-center space-x-4 transition-all"
        >
          {/* Avatar badge con iniciales dinámicas */}
          <div
            id="user-avatar-badge"
            className="w-16 h-16 rounded-full bg-gradient-to-tr from-[#00A896] to-[#2EC4B6] flex items-center justify-center text-white text-xl font-bold tracking-tight shadow-sm flex-shrink-0"
          >
            {initials}
          </div>

          {/* Información de Identidad */}
          <div className="flex-1 min-w-0">
            <h2
              id="user-full-name"
              className="text-[17px] font-bold text-[#1A1A1A] truncate"
            >
              {displayName}
            </h2>
            <div className="flex items-center space-x-1 mt-0.5">
              <span className="text-[13px] font-semibold text-[#F59E0B]">
                {displayRating} ★
              </span>
              <span className="text-[13px] text-[#6B7280]">
                ({displayReviews} opiniones)
              </span>
            </div>

            {/* Badge de Verificación de DNI */}
            {isKycVerified ? (
              <div
                id="dni-verification-badge"
                className="mt-2 inline-flex items-center px-2.5 py-1 rounded-full bg-[#E6F7F5] border border-[#BCEEE6]"
              >
                <Check className="w-3.5 h-3.5 text-[#00A896] stroke-[3] mr-1 flex-shrink-0" />
                <span className="text-[11px] font-semibold text-[#00A896] tracking-wide">
                  DNI Verificado
                </span>
              </div>
            ) : (
              <div className="mt-2 inline-flex items-center px-2.5 py-1 rounded-full bg-amber-50 border border-amber-200">
                <AlertCircle className="w-3.5 h-3.5 text-amber-600 mr-1 flex-shrink-0" />
                <span className="text-[11px] font-semibold text-amber-700 tracking-wide">
                  Verificación Pendiente
                </span>
              </div>
            )}
          </div>
        </div>

        {/* Sección de Vehículos */}
        <section id="vehicles-section" aria-labelledby="vehicles-heading">
          <div className="px-1 mb-1.5 flex justify-between items-center">
            <span
              id="vehicles-heading"
              className="text-[11px] font-bold tracking-wider text-[#6B7280] uppercase"
            >
              MIS VEHÍCULOS
            </span>
          </div>

          <div className="bg-white rounded-2xl p-4 shadow-[0_2px_8px_rgba(0,0,0,0.04)] border border-[#F0F2F4] space-y-3">
            {user.vehicles.length === 0 ? (
              <div className="text-center py-4">
                <Car className="w-8 h-8 text-gray-300 mx-auto mb-2" />
                <p className="text-xs text-[#6B7280]">Aún no tienes vehículos registrados</p>
              </div>
            ) : (
              user.vehicles.map((veh) => (
                <div
                  key={veh.id}
                  id={`vehicle-card-${veh.id}`}
                  onClick={() => handleSelectActiveVehicle(veh.id)}
                  className={`flex items-center justify-between cursor-pointer p-2 rounded-xl transition ${
                    veh.isActive ? 'bg-[#F0FDFB] border border-[#BCEEE6]' : 'hover:bg-slate-50/80 border border-transparent'
                  }`}
                >
                  <div className="flex items-center space-x-3.5 min-w-0">
                    <div className={`w-10 h-10 rounded-xl flex items-center justify-center flex-shrink-0 ${
                      veh.isActive ? 'bg-[#00A896] text-white' : 'bg-[#E6F7F5] text-[#00A896]'
                    }`}>
                      <Car className="w-5 h-5 stroke-[2]" />
                    </div>
                    <div className="truncate">
                      <p className="text-[14px] font-bold text-[#1A1A1A] truncate">
                        {veh.brand} {veh.model} {veh.color ? `• ${veh.color}` : ''}
                      </p>
                      <div className="mt-1 inline-block bg-[#F0F4F6] text-[#475569] font-mono text-[11px] font-semibold px-2 py-0.5 rounded border border-[#E2E8F0]">
                        {veh.plate}
                      </div>
                    </div>
                  </div>

                  {/* Estado Activo / Seleccionable */}
                  <div>
                    {veh.isActive ? (
                      <span className="text-[11px] font-bold text-[#00A896] bg-white border border-[#00A896]/30 px-2.5 py-1 rounded-full flex items-center gap-1 shadow-2xs">
                        <Check className="w-3 h-3 stroke-[3]" />
                        Activo
                      </span>
                    ) : (
                      <span className="text-[11px] font-medium text-[#6B7280] bg-gray-100 hover:bg-gray-200 px-2.5 py-1 rounded-full transition-colors">
                        Elegir
                      </span>
                    )}
                  </div>
                </div>
              ))
            )}

            {/* Botón de acción: + Agregar vehículo */}
            <div className="pt-3 border-t border-[#F5F7F8] flex justify-end">
              <button
                id="add-vehicle-btn"
                type="button"
                onClick={() => {
                  if (onNavigateToAddVehicle) {
                    onNavigateToAddVehicle();
                  } else {
                    setIsAddVehicleOpen(true);
                  }
                }}
                className="text-[13px] font-semibold text-[#00A896] hover:text-[#028090] transition-colors flex items-center space-x-1 cursor-pointer"
              >
                <Plus className="w-3.5 h-3.5 stroke-[2.5]" />
                <span>Agregar vehículo</span>
              </button>
            </div>
          </div>
        </section>

        {/* Sección Menú de Configuración */}
        <section id="settings-section" aria-labelledby="settings-heading">
          <div className="px-1 mb-1.5 flex justify-between items-center">
            <span
              id="settings-heading"
              className="text-[11px] font-bold tracking-wider text-[#6B7280] uppercase"
            >
              CONFIGURACIÓN
            </span>
          </div>

          <div className="bg-white rounded-2xl shadow-[0_2px_8px_rgba(0,0,0,0.04)] border border-[#F0F2F4] divide-y divide-[#F5F7F8] overflow-hidden">
            {/* Opción 1: Notificaciones */}
            <div
              id="setting-notifications-item"
              onClick={() => {
                if (onNavigateToNotifications) {
                  onNavigateToNotifications();
                } else {
                  setActiveConfigModal('notifications');
                }
              }}
              className="px-4 py-3.5 flex items-center justify-between active:bg-[#FAFAFA] hover:bg-[#F9FBFC] cursor-pointer transition-colors"
            >
              <div className="flex items-center space-x-3.5">
                <div className="w-8 h-8 rounded-lg bg-[#F3F4F6] flex items-center justify-center text-[#6B7280]">
                  <Bell className="w-4 h-4 text-[#6B7280] stroke-[2]" />
                </div>
                <span className="text-[14px] font-medium text-[#1A1A1A]">
                  Notificaciones
                </span>
              </div>
              <ChevronRight className="w-4 h-4 text-[#6B7280] opacity-60 stroke-[2.5]" />
            </div>

            {/* Opción 2: Métodos de pago y cobro */}
            <div
              id="setting-payments-item"
              onClick={() => {
                if (onNavigateToPaymentMethods) {
                  onNavigateToPaymentMethods();
                } else {
                  setActiveConfigModal('payments');
                }
              }}
              className="px-4 py-3.5 flex items-center justify-between active:bg-[#FAFAFA] hover:bg-[#F9FBFC] cursor-pointer transition-colors"
            >
              <div className="flex items-center space-x-3.5">
                <div className="w-8 h-8 rounded-lg bg-[#F3F4F6] flex items-center justify-center text-[#6B7280]">
                  <CreditCard className="w-4 h-4 text-[#6B7280] stroke-[2]" />
                </div>
                <span className="text-[14px] font-medium text-[#1A1A1A]">
                  Métodos de pago y cobro
                </span>
              </div>
              <ChevronRight className="w-4 h-4 text-[#6B7280] opacity-60 stroke-[2.5]" />
            </div>

            {/* Opción 3: Seguridad y Privacidad */}
            <div
              id="setting-security-item"
              onClick={() => {
                if (onNavigateToSecurityPrivacy) {
                  onNavigateToSecurityPrivacy();
                } else {
                  setActiveConfigModal('security');
                }
              }}
              className="px-4 py-3.5 flex items-center justify-between active:bg-[#FAFAFA] hover:bg-[#F9FBFC] cursor-pointer transition-colors"
            >
              <div className="flex items-center space-x-3.5">
                <div className="w-8 h-8 rounded-lg bg-[#F3F4F6] flex items-center justify-center text-[#6B7280]">
                  <Lock className="w-4 h-4 text-[#6B7280] stroke-[2]" />
                </div>
                <span className="text-[14px] font-medium text-[#1A1A1A]">
                  Seguridad y Privacidad
                </span>
              </div>
              <ChevronRight className="w-4 h-4 text-[#6B7280] opacity-60 stroke-[2.5]" />
            </div>

            {/* Opción 4: Ayuda y Soporte */}
            <div
              id="setting-help-item"
              onClick={() => {
                if (onNavigateToHelpSupport) {
                  onNavigateToHelpSupport();
                } else {
                  setActiveConfigModal('help');
                }
              }}
              className="px-4 py-3.5 flex items-center justify-between active:bg-[#FAFAFA] hover:bg-[#F9FBFC] cursor-pointer transition-colors"
            >
              <div className="flex items-center space-x-3.5">
                <div className="w-8 h-8 rounded-lg bg-[#F3F4F6] flex items-center justify-center text-[#6B7280]">
                  <HelpCircle className="w-4 h-4 text-[#6B7280] stroke-[2]" />
                </div>
                <span className="text-[14px] font-medium text-[#1A1A1A]">
                  Ayuda y Soporte
                </span>
              </div>
              <ChevronRight className="w-4 h-4 text-[#6B7280] opacity-60 stroke-[2.5]" />
            </div>
          </div>
        </section>

        {/* Botón Cerrar Sesión */}
        <div className="pt-2 pb-3">
          <button
            id="logout-btn"
            type="button"
            onClick={handleLogoutClick}
            className="w-full py-3.5 px-4 rounded-2xl bg-white border border-[#FEE2E2] hover:bg-[#FEF2F2] active:bg-[#FDE8E8] flex items-center justify-center space-x-2 text-[#E63946] font-semibold text-[14px] transition-all shadow-[0_2px_8px_rgba(0,0,0,0.02)] cursor-pointer"
          >
            <LogOut className="w-4 h-4 text-[#E63946] stroke-[2]" />
            <span>Cerrar sesión</span>
          </button>
          <p className="text-center text-[11px] text-[#6B7280] opacity-70 mt-3 font-medium">
            Thumbi v1.4.2 • Build 2024
          </p>
        </div>
      </div>

      {/* BEGIN: Embedded BottomNavigationBar */}
      <nav
        id="profile-bottom-nav"
        className="w-full bg-white border-t border-gray-100 pt-2 pb-5 px-6 z-30 shrink-0 select-none"
      >
        <div className="flex justify-between items-center max-w-sm mx-auto">
          {/* Tab 1: Buscar */}
          <button
            id="nav-search-btn"
            type="button"
            onClick={onNavigateToSearch}
            className="flex flex-col items-center group cursor-pointer focus:outline-none"
          >
            <div className="relative py-1">
              <Search className="w-5 h-5 transition-colors text-gray-400 group-hover:text-gray-600" />
            </div>
            <span className="text-[11px] font-medium transition-colors text-gray-500 group-hover:text-gray-700">
              Buscar
            </span>
          </button>

          {/* Tab 2: Mis Viajes */}
          <button
            id="nav-trips-btn"
            type="button"
            onClick={onNavigateToTrips}
            className="flex flex-col items-center group cursor-pointer focus:outline-none"
          >
            <div className="relative py-1">
              <Car className="w-5 h-5 transition-colors text-gray-400 group-hover:text-gray-600" />
            </div>
            <span className="text-[11px] font-medium transition-colors text-gray-500 group-hover:text-gray-700">
              Mis Viajes
            </span>
          </button>

          {/* Tab 3: Publicar */}
          <button
            id="nav-publish-btn"
            type="button"
            onClick={onNavigateToPublish}
            className="flex flex-col items-center group cursor-pointer focus:outline-none"
          >
            <div className="relative py-1">
              <PlusCircle className="w-5 h-5 transition-colors text-gray-400 group-hover:text-gray-600" />
            </div>
            <span className="text-[11px] font-medium transition-colors text-gray-500 group-hover:text-gray-700">
              Publicar
            </span>
          </button>

          {/* Tab 4: Perfil (Activo) */}
          <button
            id="nav-profile-active-btn"
            type="button"
            className="flex flex-col items-center group cursor-default focus:outline-none"
          >
            <div className="relative py-1">
              <User className="w-5 h-5 transition-colors text-[#00A896] stroke-[2.4]" />
            </div>
            <span className="text-[11px] font-bold tracking-tight text-[#00A896]">
              Perfil
            </span>
          </button>
        </div>

        {/* iOS Home Indicator */}
        <div className="w-32 h-1 bg-black/80 rounded-full mx-auto mt-3" />
      </nav>
      {/* END: Embedded BottomNavigationBar */}
    </div>

      {/* Modal: + Agregar Vehículo */}
      {isAddVehicleOpen && (
        <div className="fixed inset-0 z-50 bg-black/40 backdrop-blur-xs flex items-end sm:items-center justify-center p-4">
          <div className="bg-white w-full max-w-md rounded-2xl p-5 shadow-xl border border-gray-100 animate-in fade-in slide-in-from-bottom-4 duration-200">
            <div className="flex items-center justify-between pb-3 border-b border-gray-100">
              <div className="flex items-center space-x-2">
                <div className="w-8 h-8 rounded-lg bg-[#E6F7F5] text-[#00A896] flex items-center justify-center">
                  <Car className="w-4 h-4" />
                </div>
                <h3 className="text-base font-bold text-[#1A1A1A]">Agregar Vehículo</h3>
              </div>
              <button
                type="button"
                onClick={() => setIsAddVehicleOpen(false)}
                className="w-8 h-8 rounded-full text-gray-400 hover:bg-gray-100 flex items-center justify-center cursor-pointer"
              >
                <X className="w-4 h-4" />
              </button>
            </div>

            <form onSubmit={handleAddVehicleSubmit} className="space-y-3.5 mt-4">
              <div>
                <label className="block text-xs font-semibold text-gray-700 mb-1">
                  Marca
                </label>
                <input
                  type="text"
                  required
                  value={newBrand}
                  onChange={(e) => setNewBrand(e.target.value)}
                  placeholder="Ej. Toyota, Chevrolet, Ford"
                  className="w-full px-3 py-2 text-sm border border-gray-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-[#00A896]/30 focus:border-[#00A896]"
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-gray-700 mb-1">
                  Modelo
                </label>
                <input
                  type="text"
                  required
                  value={newModel}
                  onChange={(e) => setNewModel(e.target.value)}
                  placeholder="Ej. Corolla, Yaris, Cruze"
                  className="w-full px-3 py-2 text-sm border border-gray-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-[#00A896]/30 focus:border-[#00A896]"
                />
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="block text-xs font-semibold text-gray-700 mb-1">
                    Color
                  </label>
                  <input
                    type="text"
                    value={newColor}
                    onChange={(e) => setNewColor(e.target.value)}
                    placeholder="Ej. Blanco, Gris"
                    className="w-full px-3 py-2 text-sm border border-gray-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-[#00A896]/30 focus:border-[#00A896]"
                  />
                </div>
                <div>
                  <label className="block text-xs font-semibold text-gray-700 mb-1">
                    Patente (Dominio)
                  </label>
                  <input
                    type="text"
                    required
                    value={newPlate}
                    onChange={(e) => setNewPlate(e.target.value.toUpperCase())}
                    placeholder="Ej. AA 123 CD"
                    className="w-full px-3 py-2 text-sm font-mono uppercase border border-gray-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-[#00A896]/30 focus:border-[#00A896]"
                  />
                </div>
              </div>

              <div className="pt-2 flex items-center space-x-2.5">
                <button
                  type="button"
                  onClick={() => setIsAddVehicleOpen(false)}
                  className="w-1/2 py-2.5 text-xs font-semibold text-gray-600 bg-gray-100 hover:bg-gray-200 rounded-xl transition cursor-pointer"
                >
                  Cancelar
                </button>
                <button
                  type="submit"
                  disabled={isUserLoading}
                  className="w-1/2 py-2.5 text-xs font-semibold text-white bg-[#00A896] hover:bg-[#028090] rounded-xl shadow-sm transition cursor-pointer flex items-center justify-center gap-1.5 disabled:opacity-60"
                >
                  {isUserLoading ? (
                    <>
                      <Loader2 className="w-3.5 h-3.5 animate-spin" />
                      <span>Guardando...</span>
                    </>
                  ) : (
                    'Guardar Vehículo'
                  )}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Modales Interactivos para Configuración */}
      {activeConfigModal && (
        <div className="fixed inset-0 z-50 bg-black/40 backdrop-blur-xs flex items-end sm:items-center justify-center p-4">
          <div className="bg-white w-full max-w-md rounded-2xl p-5 shadow-xl border border-gray-100 animate-in fade-in slide-in-from-bottom-4 duration-200">
            {/* Header Modal */}
            <div className="flex items-center justify-between pb-3 border-b border-gray-100">
              <div className="flex items-center space-x-2">
                <div className="w-8 h-8 rounded-lg bg-[#E6F7F5] text-[#00A896] flex items-center justify-center">
                  {activeConfigModal === 'notifications' && <Bell className="w-4 h-4" />}
                  {activeConfigModal === 'payments' && <CreditCard className="w-4 h-4" />}
                  {activeConfigModal === 'security' && <Lock className="w-4 h-4" />}
                  {activeConfigModal === 'help' && <HelpCircle className="w-4 h-4" />}
                </div>
                <h3 className="text-base font-bold text-[#1A1A1A]">
                  {activeConfigModal === 'notifications' && 'Notificaciones'}
                  {activeConfigModal === 'payments' && 'Métodos de Pago y Cobro'}
                  {activeConfigModal === 'security' && 'Seguridad y Privacidad'}
                  {activeConfigModal === 'help' && 'Ayuda y Soporte'}
                </h3>
              </div>
              <button
                type="button"
                onClick={() => setActiveConfigModal(null)}
                className="w-8 h-8 rounded-full text-gray-400 hover:bg-gray-100 flex items-center justify-center cursor-pointer"
              >
                <X className="w-4 h-4" />
              </button>
            </div>

            {/* Contenido según opción seleccionada */}
            <div className="py-4 space-y-3">
              {activeConfigModal === 'notifications' && (
                <div className="space-y-3">
                  <div className="flex items-center justify-between p-3 rounded-xl bg-gray-50 border border-gray-100">
                    <div>
                      <p className="text-xs font-semibold text-gray-800">Alertas de viajes y reservas</p>
                      <p className="text-[11px] text-gray-500">Notificaciones push instantáneas</p>
                    </div>
                    <input
                      type="checkbox"
                      checked={pushEnabled}
                      onChange={(e) => setPushEnabled(e.target.checked)}
                      className="w-4 h-4 accent-[#00A896] cursor-pointer"
                    />
                  </div>
                  <div className="flex items-center justify-between p-3 rounded-xl bg-gray-50 border border-gray-100">
                    <div>
                      <p className="text-xs font-semibold text-gray-800">Notificaciones por WhatsApp</p>
                      <p className="text-[11px] text-gray-500">Alertas directas al celular registrado</p>
                    </div>
                    <input
                      type="checkbox"
                      checked={whatsappAlerts}
                      onChange={(e) => setWhatsappAlerts(e.target.checked)}
                      className="w-4 h-4 accent-[#00A896] cursor-pointer"
                    />
                  </div>
                  <div className="flex items-center justify-between p-3 rounded-xl bg-gray-50 border border-gray-100">
                    <div>
                      <p className="text-xs font-semibold text-gray-800">Avisos de pago y custodia Escrow</p>
                      <p className="text-[11px] text-gray-500">Confirmación de transferencias y depósitos</p>
                    </div>
                    <input
                      type="checkbox"
                      checked={escrowAlerts}
                      onChange={(e) => setEscrowAlerts(e.target.checked)}
                      className="w-4 h-4 accent-[#00A896] cursor-pointer"
                    />
                  </div>
                </div>
              )}

              {activeConfigModal === 'payments' && (
                <div className="space-y-3">
                  <div className="p-3 rounded-xl bg-gray-50 border border-gray-100 flex items-center justify-between">
                    <div className="flex items-center space-x-3">
                      <div className="w-8 h-8 rounded-lg bg-sky-100 text-sky-600 flex items-center justify-center font-bold text-xs">
                        MP
                      </div>
                      <div>
                        <p className="text-xs font-semibold text-gray-800">Mercado Pago (CVU)</p>
                        <p className="text-[11px] text-gray-500 font-mono">00000031000123456789</p>
                      </div>
                    </div>
                    <span className="text-[11px] font-semibold text-[#00A896] bg-[#E6F7F5] px-2 py-0.5 rounded">
                      Principal
                    </span>
                  </div>

                  <div className="p-3 rounded-xl bg-gray-50 border border-gray-100 flex items-center justify-between">
                    <div className="flex items-center space-x-3">
                      <CreditCard className="w-7 h-7 text-gray-400" />
                      <div>
                        <p className="text-xs font-semibold text-gray-800">Tarjeta Visa Débito</p>
                        <p className="text-[11px] text-gray-500 font-mono">•••• 4589</p>
                      </div>
                    </div>
                    <span className="text-[11px] text-gray-500">Vence 08/28</span>
                  </div>

                  <div className="p-3 rounded-xl bg-[#E6F7F5] border border-[#BCEEE6] text-xs text-[#028090]">
                    <p className="font-semibold mb-0.5">Custodia Escrow Activa</p>
                    <p className="text-[11px] text-gray-600">
                      Tus fondos permanecen bloqueados en un contrato de custodia seguro hasta la finalización del trayecto.
                    </p>
                  </div>
                </div>
              )}

              {activeConfigModal === 'security' && (
                <div className="space-y-3">
                  <div className="flex items-center space-x-3 p-3 rounded-xl bg-gray-50 border border-gray-100">
                    <ShieldCheck className="w-5 h-5 text-[#00A896]" />
                    <div className="flex-1">
                      <p className="text-xs font-semibold text-gray-800">Validación de Identidad KYC</p>
                      <p className="text-[11px] text-emerald-600 font-medium">
                        DNI Frente y Dorso aprobados biométricamente
                      </p>
                    </div>
                    <span className="text-[11px] text-gray-400 font-medium">Auditado</span>
                  </div>

                  <div className="flex items-center space-x-3 p-3 rounded-xl bg-gray-50 border border-gray-100">
                    <Lock className="w-5 h-5 text-[#00A896]" />
                    <div className="flex-1">
                      <p className="text-xs font-semibold text-gray-800">Cifrado de Sesión JWT</p>
                      <p className="text-[11px] text-gray-500">
                        Tokens de acceso firmados con expiración estricta de 20 min
                      </p>
                    </div>
                    <span className="text-[11px] text-emerald-600 font-bold">Vigente</span>
                  </div>

                  <div className="flex items-center space-x-3 p-3 rounded-xl bg-gray-50 border border-gray-100">
                    <Smartphone className="w-5 h-5 text-[#00A896]" />
                    <div className="flex-1">
                      <p className="text-xs font-semibold text-gray-800">Autenticación Biométrica</p>
                      <p className="text-[11px] text-gray-500">
                        Habilitada en este dispositivo
                      </p>
                    </div>
                    <span className="text-[11px] text-emerald-600 font-bold">Activa</span>
                  </div>
                </div>
              )}

              {activeConfigModal === 'help' && (
                <div className="space-y-3">
                  <div className="p-3 rounded-xl bg-gray-50 border border-gray-100">
                    <p className="text-xs font-semibold text-gray-800">¿Cómo funciona la reserva?</p>
                    <p className="text-[11px] text-gray-600 mt-0.5">
                      Busca tu trayecto, selecciona el asiento y abona con Escrow. Tu chofer te contactará para coordinar el punto de encuentro.
                    </p>
                  </div>

                  <div className="p-3 rounded-xl bg-gray-50 border border-gray-100">
                    <p className="text-xs font-semibold text-gray-800">¿Cómo resolver una disputa?</p>
                    <p className="text-[11px] text-gray-600 mt-0.5">
                      Si un viaje se canceló o no se completó, puedes abrir una disputa desde 'Mis Viajes' para reembolso inmediato.
                    </p>
                  </div>

                  <a
                    href="https://wa.me/5491100000000?text=Hola%20Thumbi%20necesito%20asistencia"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="flex items-center justify-center space-x-2 py-2.5 px-3 rounded-xl bg-[#25D366] text-white text-xs font-semibold hover:bg-[#1EBE5D] transition cursor-pointer"
                  >
                    <MessageSquare className="w-4 h-4" />
                    <span>Contactar soporte vía WhatsApp</span>
                  </a>
                </div>
              )}
            </div>

            <div className="pt-2">
              <button
                type="button"
                onClick={() => {
                  setActiveConfigModal(null);
                  showFeedbackToast('Configuración guardada correctamente');
                }}
                className="w-full py-2.5 text-xs font-semibold text-white bg-[#00A896] hover:bg-[#028090] rounded-xl shadow-sm transition cursor-pointer"
              >
                Cerrar
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

export default ProfileScreen;
