import React, { useState } from 'react';
import {
  Car,
  Calendar,
  Clock,
  MapPin,
  ShieldCheck,
  AlertTriangle,
  ChevronRight,
  PlusCircle,
  Receipt,
  Users,
  CheckCircle2,
} from 'lucide-react';
import { BottomNav, BottomNavTab } from '../components/BottomNav';
import { CreateDisputeModal } from '../components/CreateDisputeModal';
import { Toast } from '../components/Toast';
import { Booking } from '../types/api';

export interface MyTripsScreenProps {
  onNavigateToHome: () => void;
  onNavigateToSearch: () => void;
  onNavigateToPublish: () => void;
  onNavigateToProfile: () => void;
  onSelectBookingForDispute?: (bookingId: string) => void;
}

interface SimulatedTripItem {
  id: string;
  bookingId: string;
  driverName: string;
  origin: string;
  destination: string;
  date: string;
  time: string;
  seats: number;
  totalPrice: number;
  status: 'CONFIRMED' | 'IN_ESCROW' | 'COMPLETED' | 'DISPUTED';
  carInfo: string;
}

export const MyTripsScreen: React.FC<MyTripsScreenProps> = ({
  onNavigateToHome,
  onNavigateToSearch,
  onNavigateToPublish,
  onNavigateToProfile,
}) => {
  const [activeSubTab, setActiveSubTab] = useState<'upcoming' | 'past'>('upcoming');
  const [selectedDisputeBookingId, setSelectedDisputeBookingId] = useState<string | null>(null);
  const [isDisputeModalOpen, setIsDisputeModalOpen] = useState<boolean>(false);
  const [toast, setToast] = useState<{
    message: string;
    type: 'error' | 'success';
  } | null>(null);

  const [tripsList, setTripsList] = useState<SimulatedTripItem[]>([
    {
      id: 'trip-101',
      bookingId: 'book-palermo-pilar-01',
      driverName: 'Martín Rodríguez',
      origin: 'Palermo (Plaza Italia)',
      destination: 'Pilar (Parque Industrial)',
      date: 'Mañana, 17 Sep',
      time: '08:00 hs',
      seats: 2,
      totalPrice: 3600,
      status: 'IN_ESCROW',
      carInfo: 'Toyota Corolla • AF 342 TR',
    },
    {
      id: 'trip-102',
      bookingId: 'book-belgrano-tigre-02',
      driverName: 'Carla Méndez',
      origin: 'Belgrano (Cabildo y Juramento)',
      destination: 'Nordelta (Centro Comercial)',
      date: 'Viernes, 19 Sep',
      time: '18:30 hs',
      seats: 1,
      totalPrice: 1950,
      status: 'CONFIRMED',
      carInfo: 'Chevrolet Onix • AC 882 LK',
    },
    {
      id: 'trip-100',
      bookingId: 'book-centro-la-plata-99',
      driverName: 'Mariano Benítez',
      origin: 'Retiro (Torre de los Ingleses)',
      destination: 'La Plata (Plaza Moreno)',
      date: '10 Sep 2026',
      time: '09:00 hs',
      seats: 1,
      totalPrice: 2200,
      status: 'COMPLETED',
      carInfo: 'Volkswagen Gol Trend • AB 551 OP',
    },
  ]);

  const handleOpenDispute = (bookingId: string) => {
    setSelectedDisputeBookingId(bookingId);
    setIsDisputeModalOpen(true);
  };

  const handleDisputeSuccess = () => {
    if (selectedDisputeBookingId) {
      setTripsList((prev) =>
        prev.map((t) =>
          t.bookingId === selectedDisputeBookingId
            ? { ...t, status: 'DISPUTED' as const }
            : t
        )
      );
    }
  };

  const upcomingTrips = tripsList.filter((t) => t.status !== 'COMPLETED');
  const pastTrips = tripsList.filter((t) => t.status === 'COMPLETED');
  const displayTrips = activeSubTab === 'upcoming' ? upcomingTrips : pastTrips;

  const handleBottomNavChange = (tab: BottomNavTab) => {
    if (tab === 'home') onNavigateToHome();
    else if (tab === 'search') onNavigateToSearch();
    else if (tab === 'publish') onNavigateToPublish();
    else if (tab === 'profile') onNavigateToProfile();
  };

  return (
    <div
      id="my-trips-screen"
      className="min-h-screen pb-28 pt-6 px-4 sm:px-6 max-w-xl mx-auto"
      style={{
        backgroundColor: '#F7F9FA',
        fontFamily: '-apple-system, BlinkMacSystemFont, "SF Pro Text", "Segoe UI", Roboto, sans-serif',
      }}
    >
      {/* Encabezado */}
      <div className="mb-5">
        <h1 className="text-2xl font-bold tracking-tight text-[#1A1A1A]">
          Mis Viajes
        </h1>
        <p className="text-xs text-[#666666] mt-1">
          Historial y viajes activos con fondos protegidos por el contrato Escrow.
        </p>
      </div>

      {/* Tabs Sub-Navegación */}
      <div className="flex bg-[#EAEAEA]/80 p-1 rounded-[10px] mb-5">
        <button
          type="button"
          onClick={() => setActiveSubTab('upcoming')}
          className={`flex-1 py-2 text-xs font-semibold rounded-[8px] transition-all cursor-pointer ${
            activeSubTab === 'upcoming'
              ? 'bg-[#FFFFFF] text-[#1A1A1A] shadow-sm'
              : 'text-[#666666] hover:text-[#1A1A1A]'
          }`}
        >
          Próximos ({upcomingTrips.length})
        </button>
        <button
          type="button"
          onClick={() => setActiveSubTab('past')}
          className={`flex-1 py-2 text-xs font-semibold rounded-[8px] transition-all cursor-pointer ${
            activeSubTab === 'past'
              ? 'bg-[#FFFFFF] text-[#1A1A1A] shadow-sm'
              : 'text-[#666666] hover:text-[#1A1A1A]'
          }`}
        >
          Historial ({pastTrips.length})
        </button>
      </div>

      {/* Lista de Viajes */}
      <div className="space-y-4">
        {displayTrips.length === 0 ? (
          <div className="bg-[#FFFFFF] border border-[#EFEFEF] rounded-[12px] p-8 text-center">
            <Car className="w-10 h-10 text-[#CCCCCC] mx-auto mb-3" />
            <p className="text-sm font-semibold text-[#1A1A1A]">No tienes viajes en esta sección</p>
            <p className="text-xs text-[#888888] mt-1">
              Busca trayectos disponibles o publica una ruta para compartir tus gastos.
            </p>
            <button
              type="button"
              onClick={onNavigateToSearch}
              className="mt-4 px-4 py-2 bg-[#00A896] text-white rounded-[10px] text-xs font-semibold hover:bg-[#008f80] transition"
            >
              Explorar viajes
            </button>
          </div>
        ) : (
          displayTrips.map((item) => (
            <div
              key={item.id}
              className="bg-[#FFFFFF] border border-[#EFEFEF] rounded-[12px] p-5 shadow-sm space-y-4"
            >
              {/* Encabezado de la Tarjeta */}
              <div className="flex items-center justify-between pb-3 border-b border-[#F0F0F0]">
                <div className="flex items-center gap-2">
                  <div className="w-8 h-8 rounded-full bg-[#E6F6F4] flex items-center justify-center text-[#00A896] font-bold text-xs">
                    {item.driverName.charAt(0)}
                  </div>
                  <div>
                    <h3 className="text-xs font-bold text-[#1A1A1A]">{item.driverName}</h3>
                    <p className="text-[11px] text-[#777777]">{item.carInfo}</p>
                  </div>
                </div>

                {/* Badge de Estado Escrow */}
                {item.status === 'IN_ESCROW' && (
                  <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-[11px] font-semibold bg-emerald-50 text-emerald-700 border border-emerald-200">
                    <ShieldCheck className="w-3 h-3 text-[#00A896]" />
                    <span>Fondos en Custodia</span>
                  </span>
                )}
                {item.status === 'CONFIRMED' && (
                  <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-[11px] font-semibold bg-blue-50 text-blue-700 border border-blue-200">
                    <CheckCircle2 className="w-3 h-3" />
                    <span>Confirmado</span>
                  </span>
                )}
                {item.status === 'COMPLETED' && (
                  <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-[11px] font-semibold bg-gray-100 text-gray-700">
                    <span>Completado</span>
                  </span>
                )}
                {item.status === 'DISPUTED' && (
                  <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-[11px] font-semibold bg-amber-50 text-amber-700 border border-amber-200">
                    <AlertTriangle className="w-3 h-3" />
                    <span>Disputa en Revisión</span>
                  </span>
                )}
              </div>

              {/* Itinerario */}
              <div className="space-y-2 text-xs">
                <div className="flex items-start gap-2.5">
                  <div className="w-2 h-2 rounded-full bg-emerald-500 mt-1.5 shrink-0" />
                  <div>
                    <span className="text-[10px] text-[#888888] uppercase font-semibold block">Origen</span>
                    <span className="font-semibold text-[#1A1A1A]">{item.origin}</span>
                  </div>
                </div>
                <div className="flex items-start gap-2.5">
                  <div className="w-2 h-2 rounded-full bg-[#00A896] mt-1.5 shrink-0" />
                  <div>
                    <span className="text-[10px] text-[#888888] uppercase font-semibold block">Destino</span>
                    <span className="font-semibold text-[#1A1A1A]">{item.destination}</span>
                  </div>
                </div>
              </div>

              {/* Metadatos y Monto */}
              <div className="flex items-center justify-between pt-3 border-t border-[#F5F5F5] text-xs">
                <div className="flex items-center gap-3 text-[#666666]">
                  <span className="flex items-center gap-1">
                    <Calendar className="w-3.5 h-3.5 text-[#888888]" />
                    {item.date} • {item.time}
                  </span>
                  <span className="flex items-center gap-1">
                    <Users className="w-3.5 h-3.5 text-[#888888]" />
                    {item.seats} asiento(s)
                  </span>
                </div>
                <span className="font-bold text-sm text-[#00A896]">
                  ${item.totalPrice.toLocaleString()} ARS
                </span>
              </div>

              {/* Botón de Abrir Disputa / Reclamo */}
              {item.status !== 'DISPUTED' && (
                <div className="pt-2 flex justify-end">
                  <button
                    type="button"
                    onClick={() => handleOpenDispute(item.bookingId)}
                    className="inline-flex items-center gap-1.5 text-xs text-rose-600 hover:text-rose-700 hover:bg-rose-50 px-2.5 py-1.5 rounded-lg font-medium transition cursor-pointer"
                  >
                    <AlertTriangle className="w-3.5 h-3.5" />
                    <span>Reportar problema o disputa</span>
                  </button>
                </div>
              )}
            </div>
          ))
        )}
      </div>

      {/* Modal de Disputa Flotante */}
      {selectedDisputeBookingId && (
        <CreateDisputeModal
          isOpen={isDisputeModalOpen}
          bookingId={selectedDisputeBookingId}
          onClose={() => {
            setIsDisputeModalOpen(false);
            setSelectedDisputeBookingId(null);
          }}
          onSuccess={handleDisputeSuccess}
          showToast={(msg, type) => setToast({ message: msg, type })}
        />
      )}

      {toast && (
        <Toast
          id="my-trips-toast"
          message={toast.message}
          type={toast.type}
          onClose={() => setToast(null)}
        />
      )}

      {/* Barra de Navegación Inferior Fija */}
      <BottomNav
        activeTab="trips"
        onTabChange={handleBottomNavChange}
      />
    </div>
  );
};

export default MyTripsScreen;
