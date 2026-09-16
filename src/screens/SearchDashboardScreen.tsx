import React, { useState } from 'react';
import {
  MapPin,
  Calendar,
  Users,
  Search,
  PlusCircle,
  ShieldCheck,
  ChevronRight,
  Clock,
  Sparkles,
  ArrowRight,
  Car,
  Bell,
  CheckCircle2,
} from 'lucide-react';
import { BottomNav, BottomNavTab } from '../components/BottomNav';

export interface SearchQueryParams {
  origin: string;
  destination: string;
  date: string;
  seats: number;
}

export interface SearchDashboardScreenProps {
  userName?: string;
  isDriver?: boolean;
  onSearch: (params: SearchQueryParams) => void;
  onNavigateToPublish: () => void;
  onNavigateToTrips: () => void;
  onNavigateToProfile: () => void;
  onLogout?: () => void;
}

export const SearchDashboardScreen: React.FC<SearchDashboardScreenProps> = ({
  userName = 'Sofía',
  isDriver = false,
  onSearch,
  onNavigateToPublish,
  onNavigateToTrips,
  onNavigateToProfile,
}) => {
  const [origin, setOrigin] = useState<string>('Palermo');
  const [destination, setDestination] = useState<string>('Pilar');
  const [date, setDate] = useState<string>('2026-09-17');
  const [seats, setSeats] = useState<number>(1);
  const [activeNavTab, setActiveNavTab] = useState<BottomNavTab>('home');

  const frequentRoutes = [
    { id: '1', origin: 'Palermo', destination: 'Pilar', label: 'Palermo → Pilar', subtitle: 'Km 50 / Parque Ind.' },
    { id: '2', origin: 'Córdoba', destination: 'Carlos Paz', label: 'Córdoba → Carlos Paz', subtitle: 'Ruta 20' },
    { id: '3', origin: 'Rosario', destination: 'Funes', label: 'Rosario → Funes', subtitle: 'Ruta 9' },
    { id: '4', origin: 'Belgrano', destination: 'Nordelta', label: 'Belgrano → Nordelta', subtitle: 'Acceso Tigre' },
  ];

  const handleSelectFrequentRoute = (route: { origin: string; destination: string }) => {
    setOrigin(route.origin);
    setDestination(route.destination);
    onSearch({
      origin: route.origin,
      destination: route.destination,
      date,
      seats,
    });
  };

  const handleSearchSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    onSearch({
      origin,
      destination,
      date,
      seats,
    });
  };

  const handleTabChange = (tab: BottomNavTab) => {
    setActiveNavTab(tab);
    if (tab === 'publish') onNavigateToPublish();
    else if (tab === 'trips') onNavigateToTrips();
    else if (tab === 'profile') onNavigateToProfile();
    else if (tab === 'search') {
      onSearch({ origin, destination, date, seats });
    }
  };

  return (
    <div
      id="search-dashboard-screen"
      className="min-h-screen pb-24"
      style={{
        backgroundColor: '#F7F9FA',
        fontFamily: '-apple-system, BlinkMacSystemFont, "SF Pro Text", "SF Pro Display", "Segoe UI", Roboto, sans-serif',
      }}
    >
      {/* Barra Superior de Bienvenida */}
      <header className="bg-[#FFFFFF] border-b border-[#ECECEC] px-4 py-4 sm:px-6 sticky top-0 z-30 shadow-xs">
        <div className="max-w-md mx-auto flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-full bg-[#E6F6F4] text-[#00A896] font-bold text-sm flex items-center justify-center border border-[#00A896]/20">
              {userName.substring(0, 2).toUpperCase()}
            </div>
            <div>
              <div className="flex items-center gap-1.5">
                <h1 className="text-lg font-bold text-[#1A1A1A]">¡Hola, {userName}!</h1>
                <span className="inline-flex items-center px-1.5 py-0.2 rounded-full text-[10px] font-semibold bg-[#E6F6F4] text-[#00A896]">
                  DNI Verificado
                </span>
              </div>
              <p className="text-xs text-[#777777]">¿A dónde viajas hoy?</p>
            </div>
          </div>

          <button
            id="home-notifications-btn"
            type="button"
            aria-label="Notificaciones"
            className="w-9 h-9 rounded-full bg-[#F3F4F6] text-[#444444] flex items-center justify-center hover:bg-slate-200 transition cursor-pointer"
          >
            <Bell className="w-4 h-4" />
          </button>
        </div>
      </header>

      {/* Contenedor Central */}
      <main className="max-w-md mx-auto px-4 pt-5 space-y-6">
        {/* Tarjeta de Búsqueda de Viajes */}
        <div
          id="search-card"
          className="rounded-[16px] p-5 sm:p-6 bg-[#FFFFFF]"
          style={{
            boxShadow: '0px 2px 10px rgba(0, 0, 0, 0.04)',
            border: '1px solid #ECECEC',
          }}
        >
          <div className="flex items-center justify-between mb-4">
            <span className="text-xs font-bold uppercase tracking-wider text-[#00A896]">
              Buscar Viaje Compartido
            </span>
            <span className="text-[11px] text-[#777777] flex items-center gap-1">
              <ShieldCheck className="w-3.5 h-3.5 text-[#00A896]" />
              Tarifas sin lucro
            </span>
          </div>

          <form onSubmit={handleSearchSubmit} className="space-y-3.5">
            {/* Origen */}
            <div className="relative">
              <label htmlFor="search-origin-input" className="sr-only">
                Punto de Origen
              </label>
              <div className="absolute inset-y-0 left-0 pl-3.5 flex items-center pointer-events-none text-[#00A896]">
                <MapPin className="w-4 h-4" />
              </div>
              <input
                id="search-origin-input"
                type="text"
                required
                value={origin}
                onChange={(e) => setOrigin(e.target.value)}
                placeholder="Punto de partida (ej. Palermo)"
                className="w-full pl-10 pr-4 py-2.5 bg-[#F9FAFB] border border-[#E5E7EB] rounded-[10px] text-sm text-[#1A1A1A] placeholder-[#9CA3AF] focus:outline-none focus:border-[#00A896] focus:bg-[#FFFFFF] transition-colors"
              />
            </div>

            {/* Destino */}
            <div className="relative">
              <label htmlFor="search-dest-input" className="sr-only">
                Destino
              </label>
              <div className="absolute inset-y-0 left-0 pl-3.5 flex items-center pointer-events-none text-[#E63946]">
                <MapPin className="w-4 h-4" />
              </div>
              <input
                id="search-dest-input"
                type="text"
                required
                value={destination}
                onChange={(e) => setDestination(e.target.value)}
                placeholder="Destino (ej. Pilar)"
                className="w-full pl-10 pr-4 py-2.5 bg-[#F9FAFB] border border-[#E5E7EB] rounded-[10px] text-sm text-[#1A1A1A] placeholder-[#9CA3AF] focus:outline-none focus:border-[#00A896] focus:bg-[#FFFFFF] transition-colors"
              />
            </div>

            {/* Fecha y Asientos */}
            <div className="grid grid-cols-2 gap-3">
              {/* Fecha */}
              <div className="relative">
                <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none text-[#666666]">
                  <Calendar className="w-4 h-4" />
                </div>
                <input
                  id="search-date-input"
                  type="date"
                  value={date}
                  onChange={(e) => setDate(e.target.value)}
                  className="w-full pl-9 pr-2 py-2 bg-[#F9FAFB] border border-[#E5E7EB] rounded-[10px] text-xs font-medium text-[#1A1A1A] focus:outline-none focus:border-[#00A896] focus:bg-[#FFFFFF] transition-colors"
                />
              </div>

              {/* Asientos */}
              <div className="relative">
                <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none text-[#666666]">
                  <Users className="w-4 h-4" />
                </div>
                <select
                  id="search-seats-select"
                  value={seats}
                  onChange={(e) => setSeats(Number(e.target.value))}
                  className="w-full pl-9 pr-2 py-2 bg-[#F9FAFB] border border-[#E5E7EB] rounded-[10px] text-xs font-medium text-[#1A1A1A] focus:outline-none focus:border-[#00A896] focus:bg-[#FFFFFF] transition-colors appearance-none"
                >
                  <option value={1}>1 Asiento</option>
                  <option value={2}>2 Asientos</option>
                  <option value={3}>3 Asientos</option>
                  <option value={4}>4 Asientos</option>
                </select>
              </div>
            </div>

            {/* Botón Buscar Viajes */}
            <div className="pt-2">
              <button
                id="search-trips-submit-btn"
                type="submit"
                className="w-full flex items-center justify-center gap-2 py-3 px-4 rounded-[10px] text-white font-semibold text-sm transition-all focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-[#00A896] cursor-pointer shadow-sm"
                style={{ backgroundColor: '#00A896' }}
              >
                <Search className="w-4 h-4" />
                <span>Buscar Viajes Disponibles</span>
              </button>
            </div>
          </form>
        </div>

        {/* Sección: Tus rutas frecuentes */}
        <div>
          <div className="flex items-center justify-between mb-2.5">
            <h2 className="text-sm font-bold text-[#1A1A1A]">Tus rutas frecuentes</h2>
            <span className="text-[11px] text-[#00A896] font-medium">Histórico</span>
          </div>

          <div
            id="frequent-routes-container"
            className="flex items-center gap-2.5 overflow-x-auto pb-2 scrollbar-none"
          >
            {frequentRoutes.map((r) => (
              <button
                key={r.id}
                id={`frequent-route-chip-${r.id}`}
                type="button"
                onClick={() => handleSelectFrequentRoute(r)}
                className="shrink-0 text-left px-3.5 py-2.5 rounded-[12px] bg-[#FFFFFF] border border-[#E8EEF2] hover:border-[#00A896] hover:bg-[#F0FAF8] transition-all cursor-pointer shadow-2xs group"
              >
                <div className="flex items-center gap-1.5">
                  <span className="text-xs font-bold text-[#1A1A1A] group-hover:text-[#00A896]">
                    {r.label}
                  </span>
                  <ChevronRight className="w-3.5 h-3.5 text-[#9CA3AF] group-hover:text-[#00A896]" />
                </div>
                <span className="text-[10px] text-[#777777] block mt-0.5">
                  {r.subtitle}
                </span>
              </button>
            ))}
          </div>
        </div>

        {/* Banner para Conductores */}
        <div
          id="publish-prompt-card"
          className="rounded-[16px] p-5 bg-gradient-to-r from-[#00A896] to-[#028090] text-white shadow-sm flex items-center justify-between gap-4"
        >
          <div>
            <div className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full bg-white/20 text-[10px] font-semibold mb-1.5 backdrop-blur-xs">
              <Car className="w-3 h-3" />
              <span>Modo Conductor</span>
            </div>
            <h3 className="text-sm font-bold">¿Tienes asientos libres?</h3>
            <p className="text-xs text-white/85 mt-0.5 max-w-xs">
              Publica tu trayecto y comparte gastos de combustible y peajes con pasajeros verificados.
            </p>
          </div>

          <button
            id="home-publish-trip-btn"
            type="button"
            onClick={onNavigateToPublish}
            className="shrink-0 px-3.5 py-2 rounded-[10px] bg-[#FFFFFF] text-[#00A896] text-xs font-bold hover:bg-white/95 transition shadow-xs cursor-pointer"
          >
            Publicar
          </button>
        </div>

        {/* Garantías y Protección de Custodia Escrow */}
        <div className="rounded-[12px] p-4 bg-[#FFFFFF] border border-[#ECECEC] space-y-3 shadow-xs">
          <div className="flex items-center gap-2 text-xs font-bold text-[#1A1A1A]">
            <Sparkles className="w-4 h-4 text-[#00A896]" />
            <span>Por qué viajar con Thumbi</span>
          </div>

          <div className="grid grid-cols-2 gap-3 text-xs text-[#555555]">
            <div className="flex items-start gap-2">
              <CheckCircle2 className="w-3.5 h-3.5 text-[#00A896] shrink-0 mt-0.5" />
              <span>Pagos en custodia Escrow hasta completar viaje</span>
            </div>
            <div className="flex items-start gap-2">
              <CheckCircle2 className="w-3.5 h-3.5 text-[#00A896] shrink-0 mt-0.5" />
              <span>Conductor y pasajeros con DNI y selfie validada</span>
            </div>
          </div>
        </div>
      </main>

      {/* Barra de navegación inferior fija */}
      <BottomNav
        activeTab={activeNavTab}
        onTabChange={handleTabChange}
        isDriver={isDriver}
      />
    </div>
  );
};

export default SearchDashboardScreen;
