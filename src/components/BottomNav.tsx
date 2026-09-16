import React from 'react';
import { Home, Search, PlusCircle, Car, User, Clock } from 'lucide-react';

export type BottomNavTab = 'home' | 'search' | 'publish' | 'trips' | 'profile';

export interface BottomNavProps {
  activeTab: BottomNavTab;
  onTabChange: (tab: BottomNavTab) => void;
  isDriver?: boolean;
}

export const BottomNav: React.FC<BottomNavProps> = ({
  activeTab,
  onTabChange,
  isDriver = false,
}) => {
  return (
    <nav
      id="main-bottom-nav"
      aria-label="Navegación principal"
      className="fixed bottom-0 left-0 right-0 z-40 bg-[#FFFFFF] border-t border-[#EAEAEA] shadow-[0_-2px_10px_rgba(0,0,0,0.04)] px-4 py-2"
      style={{
        fontFamily: '-apple-system, BlinkMacSystemFont, "SF Pro Text", "Segoe UI", Roboto, sans-serif',
      }}
    >
      <div className="max-w-md mx-auto flex items-center justify-around">
        {/* Inicio */}
        <button
          id="bottom-nav-home-btn"
          type="button"
          onClick={() => onTabChange('home')}
          className={`flex flex-col items-center justify-center py-1 px-2.5 rounded-lg transition-colors cursor-pointer ${
            activeTab === 'home'
              ? 'text-[#00A896] font-semibold'
              : 'text-[#7A7A7A] hover:text-[#1A1A1A]'
          }`}
        >
          <Home className={`w-5 h-5 ${activeTab === 'home' ? 'stroke-[2.4]' : 'stroke-[1.8]'}`} />
          <span className="text-[11px] mt-1 tracking-tight">Inicio</span>
        </button>

        {/* Buscar */}
        <button
          id="bottom-nav-search-btn"
          type="button"
          onClick={() => onTabChange('search')}
          className={`flex flex-col items-center justify-center py-1 px-2.5 rounded-lg transition-colors cursor-pointer ${
            activeTab === 'search'
              ? 'text-[#00A896] font-semibold'
              : 'text-[#7A7A7A] hover:text-[#1A1A1A]'
          }`}
        >
          <Search className={`w-5 h-5 ${activeTab === 'search' ? 'stroke-[2.4]' : 'stroke-[1.8]'}`} />
          <span className="text-[11px] mt-1 tracking-tight">Buscar</span>
        </button>

        {/* Publicar (Centro Destacado) */}
        <button
          id="bottom-nav-publish-btn"
          type="button"
          onClick={() => onTabChange('publish')}
          className="flex flex-col items-center justify-center -mt-4 group cursor-pointer"
        >
          <div
            className={`w-11 h-11 rounded-full flex items-center justify-center shadow-md transition-transform group-hover:scale-105 ${
              activeTab === 'publish'
                ? 'bg-[#00A896] text-white ring-4 ring-[#E6F6F4]'
                : 'bg-[#00A896] text-white'
            }`}
          >
            <PlusCircle className="w-6 h-6 stroke-[2.2]" />
          </div>
          <span
            className={`text-[11px] mt-1 tracking-tight ${
              activeTab === 'publish'
                ? 'text-[#00A896] font-semibold'
                : 'text-[#7A7A7A] group-hover:text-[#1A1A1A]'
            }`}
          >
            Publicar
          </span>
        </button>

        {/* Mis Viajes */}
        <button
          id="bottom-nav-trips-btn"
          type="button"
          onClick={() => onTabChange('trips')}
          className={`flex flex-col items-center justify-center py-1 px-2.5 rounded-lg transition-colors cursor-pointer ${
            activeTab === 'trips'
              ? 'text-[#00A896] font-semibold'
              : 'text-[#7A7A7A] hover:text-[#1A1A1A]'
          }`}
        >
          <Car className={`w-5 h-5 ${activeTab === 'trips' ? 'stroke-[2.4]' : 'stroke-[1.8]'}`} />
          <span className="text-[11px] mt-1 tracking-tight">Mis Viajes</span>
        </button>

        {/* Perfil */}
        <button
          id="bottom-nav-profile-btn"
          type="button"
          onClick={() => onTabChange('profile')}
          className={`flex flex-col items-center justify-center py-1 px-2.5 rounded-lg transition-colors cursor-pointer ${
            activeTab === 'profile'
              ? 'text-[#00A896] font-semibold'
              : 'text-[#7A7A7A] hover:text-[#1A1A1A]'
          }`}
        >
          <User className={`w-5 h-5 ${activeTab === 'profile' ? 'stroke-[2.4]' : 'stroke-[1.8]'}`} />
          <span className="text-[11px] mt-1 tracking-tight">Perfil</span>
        </button>
      </div>
    </nav>
  );
};

export default BottomNav;
