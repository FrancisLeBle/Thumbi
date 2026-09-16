import React, { useState } from 'react';
import {
  Bell,
  Calendar,
  Users,
  Search,
  MapPin,
  Car,
  User,
  PlusCircle,
  Briefcase,
  ChevronDown,
  Loader2,
} from 'lucide-react';
import { tripService } from '../services/tripService';
import { Toast } from '../components/Toast';

export interface SearchQueryParams {
  origin: string;
  destination: string;
  date: string;
  seats: number;
}

export interface HomeScreenProps {
  userName?: string;
  isDriver?: boolean;
  onSearch?: (params: SearchQueryParams) => void;
  onNavigateToPublish?: () => void;
  onNavigateToTrips?: () => void;
  onNavigateToProfile?: () => void;
  onLogout?: () => void;
  onLoginClick?: () => void;
  onRegisterClick?: () => void;
}

export const HomeScreen: React.FC<HomeScreenProps> = ({
  userName = 'Sofía',
  isDriver = false,
  onSearch,
  onNavigateToPublish,
  onNavigateToTrips,
  onNavigateToProfile,
  onLogout,
}) => {
  const [origin, setOrigin] = useState<string>('Palermo, CABA');
  const [destination, setDestination] = useState<string>('Pilar, Buenos Aires');
  const [date, setDate] = useState<string>('2026-09-17');
  const [seats, setSeats] = useState<number>(1);
  const [isLoading, setIsLoading] = useState<boolean>(false);
  const [showDatePicker, setShowDatePicker] = useState<boolean>(false);
  const [showSeatsPicker, setShowSeatsPicker] = useState<boolean>(false);
  const [activeTab, setActiveTab] = useState<'search' | 'trips' | 'publish' | 'profile'>('search');
  const [toast, setToast] = useState<{
    message: string;
    type: 'error' | 'success';
  } | null>(null);

  // Iniciales del usuario
  const initials = userName
    .split(' ')
    .map((n) => n[0])
    .join('')
    .substring(0, 2)
    .toUpperCase() || 'SF';

  // Autocompletado de rutas frecuentes
  const handleFrequentRouteClick = (orig: string, dest: string, label: string) => {
    setOrigin(orig);
    setDestination(dest);
    setToast({
      message: `Ruta frecuente seleccionada: ${label}`,
      type: 'success',
    });
  };

  const handleSearchSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (isLoading) return;

    if (!origin.trim() || !destination.trim()) {
      setToast({
        message: 'Por favor ingresa un origen y destino para buscar.',
        type: 'error',
      });
      return;
    }

    setIsLoading(true);

    try {
      // Invocamos la búsqueda en el servicio para sincronizar resultados
      await tripService.searchTrips({
        origin: origin.trim(),
        destination: destination.trim(),
        date,
        seats,
      });

      if (onSearch) {
        onSearch({
          origin: origin.trim(),
          destination: destination.trim(),
          date,
          seats,
        });
      }
    } catch {
      // Si ocurre un error de red se procede de todas formas a la vista con filtros
      if (onSearch) {
        onSearch({
          origin: origin.trim(),
          destination: destination.trim(),
          date,
          seats,
        });
      }
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className="min-h-screen flex items-center justify-center p-0 md:p-6 bg-slate-900 font-sans">
      {/* Toast de Notificaciones */}
      {toast && (
        <Toast
          message={toast.message}
          type={toast.type}
          onClose={() => setToast(null)}
        />
      )}

      {/* iPhone 15 Pro Frame */}
      <div
        className="w-full max-w-[393px] h-[852px] bg-[#F8FAFC] relative overflow-hidden flex flex-col md:rounded-[54px] shadow-2xl border border-slate-700/50 select-none"
        data-purpose="iphone-mockup"
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

        {/* Scrollable Content Area */}
        <main
          className="flex-1 overflow-y-auto px-5 pt-3 pb-28"
          style={{ scrollbarWidth: 'none', msOverflowStyle: 'none' }}
          data-purpose="main-content"
        >
          {/* BEGIN: HeaderGreeting */}
          <section className="flex items-center justify-between mb-5" data-purpose="user-greeting">
            <div className="flex items-center space-x-3.5">
              {/* Avatar SF with Teal Background */}
              <div className="w-12 h-12 rounded-full bg-[#00A896] flex items-center justify-center text-white font-bold text-lg shadow-sm shrink-0">
                {initials}
              </div>
              {/* Title & Subtitle */}
              <div className="flex flex-col">
                <h1 className="text-xl font-bold text-gray-900 leading-tight tracking-tight">
                  ¡Hola, {userName}!
                </h1>
                <p className="text-sm text-gray-500 mt-0.5">
                  ¿A dónde quieres ir hoy?
                </p>
              </div>
            </div>

            {/* Notification Bell Button */}
            <button
              aria-label="Notificaciones"
              type="button"
              onClick={() =>
                setToast({
                  message: 'No tienes notificaciones pendientes por el momento.',
                  type: 'success',
                })
              }
              className="w-11 h-11 rounded-full bg-white flex items-center justify-center text-gray-700 shadow-sm border border-gray-100 hover:bg-gray-50 active:scale-95 transition-all cursor-pointer"
            >
              <Bell className="w-5 h-5 text-gray-700" />
            </button>
          </section>
          {/* END: HeaderGreeting */}

          {/* BEGIN: SearchFormCard */}
          <section
            className="bg-white rounded-2xl p-4 shadow-[0px_4px_20px_rgba(0,0,0,0.05)] border border-gray-100/70 mb-6"
            data-purpose="search-card"
          >
            <form onSubmit={handleSearchSubmit} className="space-y-3.5">
              {/* Field: Origen */}
              <div className="space-y-1">
                <label className="text-xs font-semibold text-gray-500 tracking-wide ml-0.5 block">
                  Origen
                </label>
                <div className="flex items-center bg-[#F7F9FA] rounded-xl px-3.5 py-3 border border-transparent focus-within:border-[#00A896] focus-within:bg-white transition-all">
                  <div className="w-4 h-4 rounded-full border-2 border-[#00A896] mr-3 shrink-0" />
                  <input
                    aria-label="Origen del viaje"
                    className="w-full bg-transparent border-0 p-0 text-sm font-medium text-gray-800 placeholder-gray-400 focus:outline-none"
                    placeholder="Ej: Palermo, CABA"
                    type="text"
                    value={origin}
                    onChange={(e) => setOrigin(e.target.value)}
                  />
                </div>
              </div>

              {/* Field: Destino */}
              <div className="space-y-1">
                <label className="text-xs font-semibold text-gray-500 tracking-wide ml-0.5 block">
                  Destino
                </label>
                <div className="flex items-center bg-[#F7F9FA] rounded-xl px-3.5 py-3 border border-transparent focus-within:border-[#00A896] focus-within:bg-white transition-all">
                  <MapPin className="w-4 h-4 text-[#00A896] mr-3 shrink-0" />
                  <input
                    aria-label="Destino del viaje"
                    className="w-full bg-transparent border-0 p-0 text-sm font-medium text-gray-800 placeholder-gray-400 focus:outline-none"
                    placeholder="Ej: Pilar, Buenos Aires"
                    type="text"
                    value={destination}
                    onChange={(e) => setDestination(e.target.value)}
                  />
                </div>
              </div>

              {/* 2 Columns Grid: Fecha & Asientos */}
              <div className="grid grid-cols-2 gap-3 pt-0.5">
                {/* Column 1: Fecha */}
                <div className="space-y-1 relative">
                  <label className="text-xs font-semibold text-gray-500 tracking-wide ml-0.5 block">
                    Fecha
                  </label>
                  <button
                    type="button"
                    onClick={() => setShowDatePicker(!showDatePicker)}
                    className="w-full flex items-center justify-between bg-[#F7F9FA] rounded-xl px-3 py-3 cursor-pointer hover:bg-gray-100 transition-colors text-left"
                  >
                    <div className="flex items-center min-w-0">
                      <Calendar className="w-4 h-4 text-gray-500 mr-2 shrink-0" />
                      <span className="text-xs font-medium text-gray-800 truncate">
                        {date === '2026-09-17' ? 'Hoy, 18:30 hs' : date}
                      </span>
                    </div>
                    <ChevronDown className="w-3.5 h-3.5 text-gray-400 shrink-0" />
                  </button>

                  {/* Inline Date Selector */}
                  {showDatePicker && (
                    <div className="absolute top-full left-0 mt-1 z-30 bg-white border border-gray-200 rounded-xl p-3 shadow-lg w-52 space-y-2">
                      <span className="text-[11px] font-semibold text-gray-600">Seleccionar fecha:</span>
                      <input
                        type="date"
                        value={date}
                        onChange={(e) => {
                          setDate(e.target.value);
                          setShowDatePicker(false);
                        }}
                        className="w-full text-xs p-1.5 border border-gray-200 rounded-lg bg-gray-50"
                      />
                      <div className="flex gap-1 pt-1">
                        <button
                          type="button"
                          onClick={() => {
                            setDate('2026-09-17');
                            setShowDatePicker(false);
                          }}
                          className="text-[10px] bg-[#E6F6F4] text-[#00A896] px-2 py-1 rounded font-medium hover:bg-[#00A896] hover:text-white transition"
                        >
                          Hoy
                        </button>
                        <button
                          type="button"
                          onClick={() => {
                            setDate('2026-09-18');
                            setShowDatePicker(false);
                          }}
                          className="text-[10px] bg-gray-100 text-gray-700 px-2 py-1 rounded font-medium hover:bg-gray-200 transition"
                        >
                          Mañana
                        </button>
                      </div>
                    </div>
                  )}
                </div>

                {/* Column 2: Asientos */}
                <div className="space-y-1 relative">
                  <label className="text-xs font-semibold text-gray-500 tracking-wide ml-0.5 block">
                    Asientos
                  </label>
                  <button
                    type="button"
                    onClick={() => setShowSeatsPicker(!showSeatsPicker)}
                    className="w-full flex items-center justify-between bg-[#F7F9FA] rounded-xl px-3 py-3 cursor-pointer hover:bg-gray-100 transition-colors text-left"
                  >
                    <div className="flex items-center min-w-0">
                      <Users className="w-4 h-4 text-gray-500 mr-2 shrink-0" />
                      <span className="text-xs font-medium text-gray-800 truncate">
                        {seats} {seats === 1 ? 'pasajero' : 'pasajeros'}
                      </span>
                    </div>
                    <ChevronDown className="w-3.5 h-3.5 text-gray-400 shrink-0" />
                  </button>

                  {/* Inline Seats Counter Popover */}
                  {showSeatsPicker && (
                    <div className="absolute top-full right-0 mt-1 z-30 bg-white border border-gray-200 rounded-xl p-3 shadow-lg w-44">
                      <span className="text-[11px] font-semibold text-gray-600 block mb-2">
                        Plazas a reservar:
                      </span>
                      <div className="flex items-center justify-between">
                        <button
                          type="button"
                          disabled={seats <= 1}
                          onClick={() => setSeats(Math.max(1, seats - 1))}
                          className="w-7 h-7 rounded-lg bg-gray-100 hover:bg-gray-200 text-gray-800 font-bold disabled:opacity-40 transition"
                        >
                          -
                        </button>
                        <span className="font-bold text-sm text-[#00A896]">{seats}</span>
                        <button
                          type="button"
                          disabled={seats >= 4}
                          onClick={() => setSeats(Math.min(4, seats + 1))}
                          className="w-7 h-7 rounded-lg bg-gray-100 hover:bg-gray-200 text-gray-800 font-bold disabled:opacity-40 transition"
                        >
                          +
                        </button>
                      </div>
                      <button
                        type="button"
                        onClick={() => setShowSeatsPicker(false)}
                        className="w-full mt-2 text-[11px] text-center text-[#00A896] font-semibold hover:underline"
                      >
                        Aceptar
                      </button>
                    </div>
                  )}
                </div>
              </div>

              {/* Primary Action Button */}
              <div className="pt-2">
                <button
                  id="search-trips-submit-btn"
                  type="submit"
                  disabled={isLoading}
                  className="w-full bg-[#00A896] hover:bg-[#009081] active:scale-[0.99] text-white font-semibold text-[15px] py-3.5 rounded-xl shadow-[0px_4px_12px_rgba(0,168,150,0.25)] transition-all duration-150 flex items-center justify-center cursor-pointer disabled:opacity-60"
                >
                  {isLoading ? (
                    <span className="flex items-center gap-2">
                      <Loader2 className="w-5 h-5 animate-spin" />
                      <span>Buscando viajes...</span>
                    </span>
                  ) : (
                    'Buscar viajes'
                  )}
                </button>
              </div>
            </form>
          </section>
          {/* END: SearchFormCard */}

          {/* BEGIN: FrequentRoutesSection */}
          <section className="space-y-3" data-purpose="frequent-routes">
            <h2 className="text-base font-bold text-gray-900 tracking-tight">
              Tus rutas frecuentes
            </h2>

            {/* Horizontal scrollable chips row */}
            <div
              className="flex space-x-2.5 overflow-x-auto pb-1 -mx-5 px-5"
              style={{ scrollbarWidth: 'none', msOverflowStyle: 'none' }}
            >
              {/* Chip 1: Casa -> Trabajo */}
              <button
                type="button"
                onClick={() =>
                  handleFrequentRouteClick(
                    'Palermo (Plaza Italia)',
                    'Pilar (Parque Industrial)',
                    'Casa → Trabajo'
                  )
                }
                className="shrink-0 bg-white border border-gray-200/80 hover:border-[#00A896] px-3.5 py-2.5 rounded-xl flex items-center space-x-2 shadow-xs active:scale-95 transition-all cursor-pointer group"
              >
                <svg
                  className="w-4 h-4 text-[#00A896] -rotate-45 group-hover:scale-110 transition-transform"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                  viewBox="0 0 24 24"
                >
                  <path d="M12 19l9 2-9-18-9 18 9-2zm0 0v-8" strokeLinecap="round" strokeLinejoin="round" />
                </svg>
                <span className="text-xs font-medium text-gray-800 whitespace-nowrap">
                  Casa → Trabajo
                </span>
              </button>

              {/* Chip 2: Trabajo -> Univ. */}
              <button
                type="button"
                onClick={() =>
                  handleFrequentRouteClick(
                    'Microcentro (Catalinas)',
                    'Ciudad Universitaria',
                    'Trabajo → Univ.'
                  )
                }
                className="shrink-0 bg-white border border-gray-200/80 hover:border-[#00A896] px-3.5 py-2.5 rounded-xl flex items-center space-x-2 shadow-xs active:scale-95 transition-all cursor-pointer group"
              >
                <svg
                  className="w-4 h-4 text-[#00A896] -rotate-45 group-hover:scale-110 transition-transform"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                  viewBox="0 0 24 24"
                >
                  <path d="M12 19l9 2-9-18-9 18 9-2zm0 0v-8" strokeLinecap="round" strokeLinejoin="round" />
                </svg>
                <span className="text-xs font-medium text-gray-800 whitespace-nowrap">
                  Trabajo → Univ.
                </span>
              </button>

              {/* Chip 3: Casa -> Gym */}
              <button
                type="button"
                onClick={() =>
                  handleFrequentRouteClick('Palermo', 'Belgrano (Cabildo)', 'Casa → Gym')
                }
                className="shrink-0 bg-white border border-gray-200/80 hover:border-[#00A896] px-3.5 py-2.5 rounded-xl flex items-center space-x-2 shadow-xs active:scale-95 transition-all cursor-pointer group"
              >
                <svg
                  className="w-4 h-4 text-[#00A896] -rotate-45 group-hover:scale-110 transition-transform"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                  viewBox="0 0 24 24"
                >
                  <path d="M12 19l9 2-9-18-9 18 9-2zm0 0v-8" strokeLinecap="round" strokeLinejoin="round" />
                </svg>
                <span className="text-xs font-medium text-gray-800 whitespace-nowrap">
                  Casa → Gym
                </span>
              </button>
            </div>
          </section>
          {/* END: FrequentRoutesSection */}
        </main>

        {/* BEGIN: BottomNavigationBar */}
        <nav
          className="absolute bottom-0 left-0 right-0 bg-white border-t border-gray-200/70 pt-2 pb-5 px-4 z-40"
          data-purpose="bottom-nav"
        >
          <div className="grid grid-cols-4 items-center text-center">
            {/* Tab 1: Buscar (Active) */}
            <button
              type="button"
              onClick={() => setActiveTab('search')}
              className="flex flex-col items-center group cursor-pointer focus:outline-none"
            >
              <div className="relative py-1">
                <Search
                  className={`w-5 h-5 transition-colors ${
                    activeTab === 'search' ? 'text-[#00A896] stroke-[2.4]' : 'text-gray-400 group-hover:text-gray-600'
                  }`}
                />
              </div>
              <span
                className={`text-[11px] font-bold tracking-tight ${
                  activeTab === 'search' ? 'text-[#00A896]' : 'text-gray-500 font-medium'
                }`}
              >
                Buscar
              </span>
            </button>

            {/* Tab 2: Mis Viajes */}
            <button
              type="button"
              onClick={() => {
                setActiveTab('trips');
                if (onNavigateToTrips) onNavigateToTrips();
              }}
              className="flex flex-col items-center group cursor-pointer focus:outline-none"
            >
              <div className="relative py-1">
                <Car
                  className={`w-5 h-5 transition-colors ${
                    activeTab === 'trips' ? 'text-[#00A896] stroke-[2.4]' : 'text-gray-400 group-hover:text-gray-600'
                  }`}
                />
              </div>
              <span
                className={`text-[11px] font-medium transition-colors ${
                  activeTab === 'trips' ? 'text-[#00A896] font-bold' : 'text-gray-500 group-hover:text-gray-700'
                }`}
              >
                Mis Viajes
              </span>
            </button>

            {/* Tab 3: Publicar */}
            <button
              type="button"
              onClick={() => {
                setActiveTab('publish');
                if (onNavigateToPublish) onNavigateToPublish();
              }}
              className="flex flex-col items-center group cursor-pointer focus:outline-none"
            >
              <div className="relative py-1">
                <PlusCircle
                  className={`w-5 h-5 transition-colors ${
                    activeTab === 'publish' ? 'text-[#00A896] stroke-[2.4]' : 'text-gray-400 group-hover:text-gray-600'
                  }`}
                />
              </div>
              <span
                className={`text-[11px] font-medium transition-colors ${
                  activeTab === 'publish' ? 'text-[#00A896] font-bold' : 'text-gray-500 group-hover:text-gray-700'
                }`}
              >
                Publicar
              </span>
            </button>

            {/* Tab 4: Perfil */}
            <button
              type="button"
              onClick={() => {
                setActiveTab('profile');
                if (onNavigateToProfile) onNavigateToProfile();
              }}
              className="flex flex-col items-center group cursor-pointer focus:outline-none"
            >
              <div className="relative py-1">
                <User
                  className={`w-5 h-5 transition-colors ${
                    activeTab === 'profile' ? 'text-[#00A896] stroke-[2.4]' : 'text-gray-400 group-hover:text-gray-600'
                  }`}
                />
              </div>
              <span
                className={`text-[11px] font-medium transition-colors ${
                  activeTab === 'profile' ? 'text-[#00A896] font-bold' : 'text-gray-500 group-hover:text-gray-700'
                }`}
              >
                Perfil
              </span>
            </button>
          </div>

          {/* iOS Home Indicator */}
          <div className="w-32 h-1 bg-black/80 rounded-full mx-auto mt-3" />
        </nav>
        {/* END: BottomNavigationBar */}
      </div>
    </div>
  );
};

export default HomeScreen;
