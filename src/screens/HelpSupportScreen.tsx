import React, { useState } from 'react';
import {
  ChevronLeft,
  Search,
  Car,
  CreditCard,
  ShieldCheck,
  ChevronRight,
  ChevronDown,
  Headphones,
  AlertTriangle,
  X,
  MessageSquare,
  CheckCircle2,
  Mail,
  Send,
} from 'lucide-react';

export interface HelpSupportScreenProps {
  onBack: () => void;
  onNavigateToTrips?: () => void;
}

interface FAQItem {
  id: string;
  question: string;
  icon: 'car' | 'card' | 'shield';
  answer: string;
  tags: string[];
}

const FAQ_DATA: FAQItem[] = [
  {
    id: 'faq-publish-cancel',
    question: '¿Cómo publico o cancelo un viaje?',
    icon: 'car',
    answer:
      'Para publicar un viaje, ingresa a la pestaña "Publicar", define origen, destino, horario, asientos disponibles y precio por plaza. Si necesitas cancelarlo, dirígete a "Mis Viajes" > "Como Conductor" y pulsa en "Cancelar viaje". Las cancelaciones con más de 2 horas de anticipación no generan penalizaciones.',
    tags: ['publicar', 'cancelar', 'viaje', 'conductor', 'asientos', 'horario'],
  },
  {
    id: 'faq-payments-refunds',
    question: '¿Cómo funcionan los cobros y reembolsos?',
    icon: 'card',
    answer:
      'Thumbi utiliza un sistema de custodia Escrow. El monto abonado por el pasajero queda retenido de forma segura y se libera al conductor 2 horas después de completado el trayecto. Si el conductor cancela el viaje o se resuelve una disputa a tu favor, el reembolso se acredita de inmediato en tu saldo o medio de pago original.',
    tags: ['cobros', 'reembolsos', 'pagos', 'escrow', 'tarjeta', 'mercado pago', 'disputa'],
  },
  {
    id: 'faq-rules-safety',
    question: 'Normas de convivencia y seguridad',
    icon: 'shield',
    answer:
      'Todos los usuarios deben contar con DNI verificado. Se estipula una tolerancia máxima de espera de 10 minutos en el punto de encuentro acordado. El conductor debe mantener la documentación del vehículo al día (VTV, seguro vigente). Prohibido fumar o transportar equipaje no acordado sin consentimiento previo.',
    tags: ['normas', 'seguridad', 'dni', 'tolerancia', 'convivencia', 'seguro', 'vtv'],
  },
];

export const HelpSupportScreen: React.FC<HelpSupportScreenProps> = ({
  onBack,
  onNavigateToTrips,
}) => {
  const [searchQuery, setSearchQuery] = useState<string>('');
  const [expandedFaqId, setExpandedFaqId] = useState<string | null>(null);

  // Modales de asistencia
  const [isContactModalOpen, setIsContactModalOpen] = useState<boolean>(false);
  const [isReportModalOpen, setIsReportModalOpen] = useState<boolean>(false);

  // Formulario de contacto
  const [contactSubject, setContactSubject] = useState<string>('');
  const [contactMessage, setContactMessage] = useState<string>('');
  const [reportTripReason, setReportTripReason] = useState<string>('conductor_ausente');
  const [reportDetails, setReportDetails] = useState<string>('');

  // Notificación tipo toast
  const [toastMessage, setToastMessage] = useState<string | null>(null);

  const showToast = (msg: string) => {
    setToastMessage(msg);
    setTimeout(() => {
      setToastMessage(null);
    }, 3500);
  };

  // Filtrado de preguntas frecuentes
  const filteredFaqs = FAQ_DATA.filter((item) => {
    if (!searchQuery.trim()) return true;
    const query = searchQuery.toLowerCase();
    return (
      item.question.toLowerCase().includes(query) ||
      item.answer.toLowerCase().includes(query) ||
      item.tags.some((t) => t.includes(query))
    );
  });

  const toggleFaq = (id: string) => {
    setExpandedFaqId((prev) => (prev === id ? null : id));
  };

  const handleSendContact = (e: React.FormEvent) => {
    e.preventDefault();
    if (!contactMessage.trim()) return;

    setIsContactModalOpen(false);
    setContactSubject('');
    setContactMessage('');
    showToast('Tu consulta fue enviada al equipo de soporte. Te responderemos por email en menos de 24 hs.');
  };

  const handleSendReport = (e: React.FormEvent) => {
    e.preventDefault();
    setIsReportModalOpen(false);
    setReportDetails('');
    showToast('Reporte registrado. El equipo de auditoría y mediación se contactará a la brevedad.');
  };

  return (
    <div
      id="help-support-screen"
      className="w-full max-w-md mx-auto min-h-screen bg-[#F7F9FA] px-5 pt-4 pb-28 space-y-4 select-none relative"
      style={{
        fontFamily:
          '-apple-system, BlinkMacSystemFont, "Plus Jakarta Sans", "SF Pro Text", "Segoe UI", Roboto, sans-serif',
      }}
    >
      {/* Toast Notification */}
      {toastMessage && (
        <div className="fixed top-5 left-1/2 -translate-x-1/2 z-50 bg-[#1A1A1A] text-white px-4 py-2.5 rounded-full text-xs font-medium shadow-lg flex items-center space-x-2 animate-in fade-in slide-in-from-top-2 duration-200">
          <CheckCircle2 className="w-4 h-4 text-[#00A896]" />
          <span>{toastMessage}</span>
        </div>
      )}

      {/* Navigation Header */}
      <header className="relative pt-1 pb-1 flex items-center justify-between z-20">
        <button
          id="help-back-btn"
          onClick={onBack}
          className="w-10 h-10 -ml-2 rounded-full flex items-center justify-center text-[#1A1A1A] hover:bg-[#EFF5F2] active:scale-95 transition-all cursor-pointer"
          type="button"
          aria-label="Volver a Perfil"
        >
          <ChevronLeft className="w-6 h-6 stroke-[2.4] text-[#1A1A1A]" />
        </button>
        <h1
          id="help-header-title"
          className="text-[18px] font-bold text-[#1A1A1A] text-center tracking-tight flex-1 mr-8"
        >
          Ayuda y Soporte
        </h1>
      </header>

      {/* Search Input Bar */}
      <div className="relative mt-1">
        <div className="absolute inset-y-0 left-0 pl-3.5 flex items-center pointer-events-none text-[#6B7280]">
          <Search className="w-5 h-5 text-[#6B7280]" />
        </div>
        <input
          id="faq-search-input"
          type="text"
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
          placeholder="Buscar en preguntas frecuentes..."
          className="w-full bg-[#EFF5F2] text-[#1A1A1A] placeholder-[#6B7280] text-[15px] rounded-2xl pl-11 pr-10 py-3.5 border border-transparent focus:border-[#00A896] focus:bg-white focus:outline-none transition-all shadow-sm"
        />
        {searchQuery && (
          <button
            type="button"
            onClick={() => setSearchQuery('')}
            className="absolute inset-y-0 right-0 pr-3.5 flex items-center text-gray-400 hover:text-gray-600 cursor-pointer"
            aria-label="Limpiar búsqueda"
          >
            <X className="w-4 h-4" />
          </button>
        )}
      </div>

      {/* FAQ Section */}
      <div className="space-y-2">
        <div className="flex items-center justify-between px-1">
          <h2 className="text-[12px] font-bold uppercase tracking-wider text-[#6B7280]">
            Preguntas Frecuentes
          </h2>
          {searchQuery && (
            <span className="text-[11px] text-[#00A896] font-medium">
              {filteredFaqs.length} {filteredFaqs.length === 1 ? 'resultado' : 'resultados'}
            </span>
          )}
        </div>

        <div className="bg-white rounded-2xl shadow-[0_2px_10px_rgba(0,0,0,0.03)] border border-[#EDF2F4] overflow-hidden divide-y divide-[#F3F4F6]">
          {filteredFaqs.length === 0 ? (
            <div className="p-6 text-center">
              <p className="text-sm font-semibold text-gray-800">
                No encontramos resultados para "{searchQuery}"
              </p>
              <p className="text-xs text-gray-500 mt-1">
                Intenta con otros términos o comunícate con nuestro equipo de soporte técnico debajo.
              </p>
            </div>
          ) : (
            filteredFaqs.map((faq) => {
              const isExpanded = expandedFaqId === faq.id;
              return (
                <div key={faq.id} className="transition-colors">
                  <button
                    type="button"
                    onClick={() => toggleFaq(faq.id)}
                    className="w-full px-4 py-4 flex items-center justify-between text-left hover:bg-[#F9FAFB] active:bg-[#F3F4F6] transition-colors group cursor-pointer"
                    aria-expanded={isExpanded}
                  >
                    <div className="flex items-center space-x-3.5 pr-2">
                      <div className="w-9 h-9 rounded-xl bg-[#E6F7F5] text-[#00A896] flex items-center justify-center flex-shrink-0">
                        {faq.icon === 'car' && <Car className="w-5 h-5 stroke-[2]" />}
                        {faq.icon === 'card' && <CreditCard className="w-5 h-5 stroke-[2]" />}
                        {faq.icon === 'shield' && <ShieldCheck className="w-5 h-5 stroke-[2]" />}
                      </div>
                      <span className="text-[15px] font-semibold text-[#1A1A1A] leading-snug">
                        {faq.question}
                      </span>
                    </div>
                    {isExpanded ? (
                      <ChevronDown className="w-4 h-4 text-[#00A896] flex-shrink-0 stroke-[2.4] transition-transform" />
                    ) : (
                      <ChevronRight className="w-4 h-4 text-[#9CA3AF] group-hover:text-[#00A896] flex-shrink-0 stroke-[2.2] transition-colors" />
                    )}
                  </button>

                  {isExpanded && (
                    <div className="px-4 pb-4 pt-1 bg-[#F9FBFC] text-sm text-[#475569] leading-relaxed border-t border-[#F1F5F9]">
                      <p className="pl-12 text-[13px]">{faq.answer}</p>
                    </div>
                  )}
                </div>
              );
            })
          )}
        </div>
      </div>

      {/* Direct Contact Section */}
      <div className="space-y-2 pt-1">
        <div className="px-1">
          <h2 className="text-[12px] font-bold uppercase tracking-wider text-[#6B7280]">
            ¿NECESITÁS MÁS AYUDA?
          </h2>
        </div>

        <div className="bg-white rounded-2xl shadow-[0_2px_10px_rgba(0,0,0,0.03)] border border-[#EDF2F4] overflow-hidden divide-y divide-[#F3F4F6]">
          {/* Contact Card 1: Soporte Técnico */}
          <button
            id="contact-support-btn"
            type="button"
            onClick={() => setIsContactModalOpen(true)}
            className="w-full px-4 py-4 flex items-center justify-between text-left hover:bg-[#F9FAFB] active:bg-[#F3F4F6] transition-colors group cursor-pointer"
          >
            <div className="flex items-center space-x-3.5 pr-2">
              <div className="w-10 h-10 rounded-xl bg-[#E6F7F5] text-[#00A896] flex items-center justify-center flex-shrink-0">
                <Headphones className="w-5 h-5 stroke-[2]" />
              </div>
              <div>
                <h3 className="text-[15px] font-bold text-[#1A1A1A] leading-snug">
                  Contactar al soporte técnico
                </h3>
                <p className="text-[13px] text-[#6B7280] font-medium mt-0.5">
                  Respuesta en menos de 24 hs
                </p>
              </div>
            </div>
            <ChevronRight className="w-4 h-4 text-[#9CA3AF] group-hover:text-[#00A896] flex-shrink-0 stroke-[2.2] transition-colors" />
          </button>

          {/* Contact Card 2: Reportar Problema */}
          <button
            id="report-trip-problem-btn"
            type="button"
            onClick={() => setIsReportModalOpen(true)}
            className="w-full px-4 py-4 flex items-center justify-between text-left hover:bg-[#F9FAFB] active:bg-[#F3F4F6] transition-colors group cursor-pointer"
          >
            <div className="flex items-center space-x-3.5 pr-2">
              <div className="w-10 h-10 rounded-xl bg-[#FFF1F2] text-[#E63946] flex items-center justify-center flex-shrink-0">
                <AlertTriangle className="w-5 h-5 stroke-[2]" />
              </div>
              <div className="flex-1">
                <h3 className="text-[15px] font-bold text-[#1A1A1A] leading-snug">
                  Reportar un problema con un viaje
                </h3>
                <p className="text-[13px] text-[#6B7280] font-medium mt-0.5">
                  Inconvenientes, cancelaciones o ausencias
                </p>
              </div>
            </div>
            <ChevronRight className="w-4 h-4 text-[#9CA3AF] group-hover:text-[#00A896] flex-shrink-0 stroke-[2.2] transition-colors" />
          </button>
        </div>
      </div>

      {/* Trust Note Footer */}
      <div className="pt-2 text-center">
        <p className="text-[12px] text-[#9CA3AF] font-medium">
          Thumbi Centro de Seguridad y Asistencia • v1.4.2
        </p>
      </div>

      {/* Modal: Contactar Soporte Técnico */}
      {isContactModalOpen && (
        <div className="fixed inset-0 z-50 bg-black/40 backdrop-blur-xs flex items-end sm:items-center justify-center p-4">
          <div className="bg-white w-full max-w-md rounded-2xl p-5 shadow-xl border border-gray-100 animate-in fade-in slide-in-from-bottom-4 duration-200">
            <div className="flex items-center justify-between pb-3 border-b border-gray-100">
              <div className="flex items-center space-x-2">
                <div className="w-8 h-8 rounded-lg bg-[#E6F7F5] text-[#00A896] flex items-center justify-center">
                  <Headphones className="w-4 h-4" />
                </div>
                <h3 className="text-base font-bold text-[#1A1A1A]">Contactar al Soporte</h3>
              </div>
              <button
                type="button"
                onClick={() => setIsContactModalOpen(false)}
                className="w-8 h-8 rounded-full text-gray-400 hover:bg-gray-100 flex items-center justify-center cursor-pointer"
              >
                <X className="w-4 h-4" />
              </button>
            </div>

            <form onSubmit={handleSendContact} className="space-y-3.5 mt-4">
              <div>
                <label className="block text-xs font-semibold text-gray-700 mb-1">
                  Asunto
                </label>
                <input
                  type="text"
                  required
                  value={contactSubject}
                  onChange={(e) => setContactSubject(e.target.value)}
                  placeholder="Ej. Consulta sobre mi pago / cuenta"
                  className="w-full px-3 py-2 text-sm border border-gray-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-[#00A896]/30 focus:border-[#00A896]"
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-gray-700 mb-1">
                  Mensaje o detalle
                </label>
                <textarea
                  required
                  rows={4}
                  value={contactMessage}
                  onChange={(e) => setContactMessage(e.target.value)}
                  placeholder="Describe detalladamente cómo podemos asistirte..."
                  className="w-full px-3 py-2 text-sm border border-gray-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-[#00A896]/30 focus:border-[#00A896] resize-none"
                />
              </div>

              {/* Botón WhatsApp alternativo */}
              <a
                href="https://wa.me/5491100000000?text=Hola%20Thumbi%20necesito%20ayuda%20con%20la%20app"
                target="_blank"
                rel="noopener noreferrer"
                className="flex items-center justify-center space-x-2 py-2 px-3 rounded-xl bg-[#25D366]/10 text-[#128C7E] text-xs font-semibold hover:bg-[#25D366]/20 transition"
              >
                <MessageSquare className="w-3.5 h-3.5" />
                <span>O chatear por WhatsApp directo</span>
              </a>

              <div className="pt-2 flex items-center space-x-2.5">
                <button
                  type="button"
                  onClick={() => setIsContactModalOpen(false)}
                  className="w-1/2 py-2.5 text-xs font-semibold text-gray-600 bg-gray-100 hover:bg-gray-200 rounded-xl transition cursor-pointer"
                >
                  Cancelar
                </button>
                <button
                  type="submit"
                  className="w-1/2 py-2.5 text-xs font-semibold text-white bg-[#00A896] hover:bg-[#028090] rounded-xl shadow-sm transition flex items-center justify-center space-x-1 cursor-pointer"
                >
                  <Send className="w-3.5 h-3.5" />
                  <span>Enviar mensaje</span>
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Modal: Reportar Problema con un Viaje */}
      {isReportModalOpen && (
        <div className="fixed inset-0 z-50 bg-black/40 backdrop-blur-xs flex items-end sm:items-center justify-center p-4">
          <div className="bg-white w-full max-w-md rounded-2xl p-5 shadow-xl border border-gray-100 animate-in fade-in slide-in-from-bottom-4 duration-200">
            <div className="flex items-center justify-between pb-3 border-b border-gray-100">
              <div className="flex items-center space-x-2">
                <div className="w-8 h-8 rounded-lg bg-rose-100 text-[#E63946] flex items-center justify-center">
                  <AlertTriangle className="w-4 h-4" />
                </div>
                <h3 className="text-base font-bold text-[#1A1A1A]">Reportar Inconveniente</h3>
              </div>
              <button
                type="button"
                onClick={() => setIsReportModalOpen(false)}
                className="w-8 h-8 rounded-full text-gray-400 hover:bg-gray-100 flex items-center justify-center cursor-pointer"
              >
                <X className="w-4 h-4" />
              </button>
            </div>

            <form onSubmit={handleSendReport} className="space-y-3.5 mt-4">
              <div>
                <label className="block text-xs font-semibold text-gray-700 mb-1">
                  Motivo principal
                </label>
                <select
                  value={reportTripReason}
                  onChange={(e) => setReportTripReason(e.target.value)}
                  className="w-full px-3 py-2 text-sm border border-gray-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-[#00A896]/30 focus:border-[#00A896] bg-white"
                >
                  <option value="conductor_ausente">El conductor no se presentó al punto de encuentro</option>
                  <option value="pasajero_ausente">Un pasajero no llegó al horario acordado</option>
                  <option value="desvio_ruta">Desvío no acordado en el itinerario</option>
                  <option value="vehiculo_inadecuado">Vehículo o condiciones de seguridad deficientes</option>
                  <option value="disputa_pago">Inconveniente con cobro o retención Escrow</option>
                </select>
              </div>

              <div>
                <label className="block text-xs font-semibold text-gray-700 mb-1">
                  Descripción del problema
                </label>
                <textarea
                  required
                  rows={3}
                  value={reportDetails}
                  onChange={(e) => setReportDetails(e.target.value)}
                  placeholder="Explica qué sucedió para que mediación aplique las reglas de resolución..."
                  className="w-full px-3 py-2 text-sm border border-gray-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-[#00A896]/30 focus:border-[#00A896] resize-none"
                />
              </div>

              {onNavigateToTrips && (
                <button
                  type="button"
                  onClick={() => {
                    setIsReportModalOpen(false);
                    onNavigateToTrips();
                  }}
                  className="w-full py-2 px-3 rounded-xl bg-gray-50 border border-gray-200 text-gray-700 text-xs font-medium hover:bg-gray-100 transition flex items-center justify-between"
                >
                  <span>¿Prefieres gestionar una disputa desde Mis Viajes?</span>
                  <ChevronRight className="w-3.5 h-3.5 text-gray-400" />
                </button>
              )}

              <div className="pt-2 flex items-center space-x-2.5">
                <button
                  type="button"
                  onClick={() => setIsReportModalOpen(false)}
                  className="w-1/2 py-2.5 text-xs font-semibold text-gray-600 bg-gray-100 hover:bg-gray-200 rounded-xl transition cursor-pointer"
                >
                  Cancelar
                </button>
                <button
                  type="submit"
                  className="w-1/2 py-2.5 text-xs font-semibold text-white bg-[#E63946] hover:bg-[#c92a37] rounded-xl shadow-sm transition cursor-pointer"
                >
                  Enviar Reporte
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
};

export default HelpSupportScreen;
