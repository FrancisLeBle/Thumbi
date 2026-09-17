import React, { useState } from 'react';
import {
  ChevronLeft,
  Info,
  CheckCircle2,
  Bell,
  Mail,
} from 'lucide-react';

export interface NotificationsSettingsScreenProps {
  onBack: () => void;
}

export interface PushNotificationSettings {
  requestsAndBookings: boolean;
  messagesAndAlerts: boolean;
}

export interface EmailNotificationSettings {
  receiptsAndInvoices: boolean;
  newsAndPromotions: boolean;
}

export const NotificationsSettingsScreen: React.FC<NotificationsSettingsScreenProps> = ({
  onBack,
}) => {
  // Estado local para Notificaciones Push
  const [pushSettings, setPushSettings] = useState<PushNotificationSettings>({
    requestsAndBookings: true,
    messagesAndAlerts: true,
  });

  // Estado local para Correo Electrónico
  const [emailSettings, setEmailSettings] = useState<EmailNotificationSettings>({
    receiptsAndInvoices: true,
    newsAndPromotions: false,
  });

  // Feedback toast flotante
  const [toastMessage, setToastMessage] = useState<string | null>(null);

  const showToast = (msg: string) => {
    setToastMessage(msg);
    setTimeout(() => {
      setToastMessage(null);
    }, 2800);
  };

  const handleTogglePush = (key: keyof PushNotificationSettings, label: string) => {
    setPushSettings((prev) => {
      const nextVal = !prev[key];
      showToast(`${label}: ${nextVal ? 'Activado' : 'Desactivado'}`);
      return { ...prev, [key]: nextVal };
    });
  };

  const handleToggleEmail = (key: keyof EmailNotificationSettings, label: string) => {
    setEmailSettings((prev) => {
      const nextVal = !prev[key];
      showToast(`${label}: ${nextVal ? 'Activado' : 'Desactivado'}`);
      return { ...prev, [key]: nextVal };
    });
  };

  return (
    <div
      id="notifications-settings-screen"
      className="w-full max-w-md mx-auto min-h-screen bg-[#F7F9FA] px-5 pt-4 pb-28 space-y-4 select-none relative"
      style={{
        fontFamily:
          '-apple-system, BlinkMacSystemFont, "Plus Jakarta Sans", "Inter", "SF Pro Text", "Segoe UI", Roboto, sans-serif',
      }}
    >
      {/* Toast Notification */}
      {toastMessage && (
        <div className="fixed top-5 left-1/2 -translate-x-1/2 z-50 bg-[#1A1A1A] text-white px-4 py-2.5 rounded-full text-xs font-medium shadow-lg flex items-center space-x-2 animate-in fade-in slide-in-from-top-2 duration-200">
          <CheckCircle2 className="w-4 h-4 text-[#00A896]" />
          <span>{toastMessage}</span>
        </div>
      )}

      {/* Header */}
      <header className="relative pt-1 pb-1 flex items-center justify-between z-20">
        <button
          id="notifications-back-btn"
          type="button"
          onClick={onBack}
          className="w-10 h-10 -ml-2 rounded-full flex items-center justify-center text-[#1A1A1A] hover:bg-[#EFF5F2] active:scale-95 transition-all cursor-pointer"
          aria-label="Volver a Perfil"
        >
          <ChevronLeft className="w-6 h-6 stroke-[2.4] text-[#1A1A1A]" />
        </button>
        <h1
          id="notifications-header-title"
          className="text-[18px] font-bold text-[#1A1A1A] text-center tracking-tight flex-1 mr-8"
        >
          Notificaciones
        </h1>
      </header>

      {/* Main Content Sections */}
      <main className="space-y-6 pt-1">
        {/* Section 1: Push Notifications */}
        <section id="push-notifications-section" aria-labelledby="push-notifications-heading">
          <div className="flex items-center justify-between px-1 mb-2.5">
            <h2
              id="push-notifications-heading"
              className="text-[11px] font-bold tracking-wider text-[#6B7280] uppercase flex items-center space-x-1.5"
            >
              <span>NOTIFICACIONES PUSH</span>
            </h2>
          </div>

          <div className="bg-white rounded-2xl shadow-[0_2px_8px_rgba(0,0,0,0.04)] border border-slate-100/80 overflow-hidden divide-y divide-slate-100">
            {/* Row 1: Solicitudes y Reservas */}
            <div
              id="row-push-requests"
              className="p-4 flex items-center justify-between transition-colors hover:bg-[#F9FBFC]"
            >
              <div className="pr-3 flex-1">
                <p className="text-[15px] font-semibold text-[#1A1A1A] leading-tight mb-1">
                  Solicitudes y Reservas
                </p>
                <p className="text-[13px] text-[#6B7280] leading-snug">
                  Alertas sobre el estado de tus viajes
                </p>
              </div>

              {/* Toggle Switch */}
              <button
                type="button"
                role="switch"
                aria-checked={pushSettings.requestsAndBookings}
                onClick={() =>
                  handleTogglePush('requestsAndBookings', 'Solicitudes y Reservas')
                }
                className={`relative inline-flex h-7 w-12 items-center rounded-full transition-colors duration-200 cursor-pointer focus:outline-none flex-shrink-0 ${
                  pushSettings.requestsAndBookings ? 'bg-[#00A896]' : 'bg-slate-200'
                }`}
              >
                <span
                  className={`inline-block h-5 w-5 transform rounded-full bg-white shadow-md transition-transform duration-200 ${
                    pushSettings.requestsAndBookings ? 'translate-x-6' : 'translate-x-1'
                  }`}
                />
              </button>
            </div>

            {/* Row 2: Mensajes y Avisos */}
            <div
              id="row-push-messages"
              className="p-4 flex items-center justify-between transition-colors hover:bg-[#F9FBFC]"
            >
              <div className="pr-3 flex-1">
                <p className="text-[15px] font-semibold text-[#1A1A1A] leading-tight mb-1">
                  Mensajes y Avisos
                </p>
                <p className="text-[13px] text-[#6B7280] leading-snug">
                  Recordatorios de salida y novedades
                </p>
              </div>

              {/* Toggle Switch */}
              <button
                type="button"
                role="switch"
                aria-checked={pushSettings.messagesAndAlerts}
                onClick={() =>
                  handleTogglePush('messagesAndAlerts', 'Mensajes y Avisos')
                }
                className={`relative inline-flex h-7 w-12 items-center rounded-full transition-colors duration-200 cursor-pointer focus:outline-none flex-shrink-0 ${
                  pushSettings.messagesAndAlerts ? 'bg-[#00A896]' : 'bg-slate-200'
                }`}
              >
                <span
                  className={`inline-block h-5 w-5 transform rounded-full bg-white shadow-md transition-transform duration-200 ${
                    pushSettings.messagesAndAlerts ? 'translate-x-6' : 'translate-x-1'
                  }`}
                />
              </button>
            </div>
          </div>
        </section>

        {/* Section 2: Email Notifications */}
        <section id="email-notifications-section" aria-labelledby="email-notifications-heading">
          <div className="flex items-center justify-between px-1 mb-2.5">
            <h2
              id="email-notifications-heading"
              className="text-[11px] font-bold tracking-wider text-[#6B7280] uppercase flex items-center space-x-1.5"
            >
              <span>CORREO ELECTRÓNICO</span>
            </h2>
          </div>

          <div className="bg-white rounded-2xl shadow-[0_2px_8px_rgba(0,0,0,0.04)] border border-slate-100/80 overflow-hidden divide-y divide-slate-100">
            {/* Row 1: Comprobantes y Recibos */}
            <div
              id="row-email-receipts"
              className="p-4 flex items-center justify-between transition-colors hover:bg-[#F9FBFC]"
            >
              <div className="pr-3 flex-1">
                <p className="text-[15px] font-semibold text-[#1A1A1A] leading-tight mb-1">
                  Comprobantes y Recibos
                </p>
                <p className="text-[13px] text-[#6B7280] leading-snug">
                  Resumen de pago al finalizar cada viaje
                </p>
              </div>

              {/* Toggle Switch */}
              <button
                type="button"
                role="switch"
                aria-checked={emailSettings.receiptsAndInvoices}
                onClick={() =>
                  handleToggleEmail('receiptsAndInvoices', 'Comprobantes y Recibos')
                }
                className={`relative inline-flex h-7 w-12 items-center rounded-full transition-colors duration-200 cursor-pointer focus:outline-none flex-shrink-0 ${
                  emailSettings.receiptsAndInvoices ? 'bg-[#00A896]' : 'bg-slate-200'
                }`}
              >
                <span
                  className={`inline-block h-5 w-5 transform rounded-full bg-white shadow-md transition-transform duration-200 ${
                    emailSettings.receiptsAndInvoices ? 'translate-x-6' : 'translate-x-1'
                  }`}
                />
              </button>
            </div>

            {/* Row 2: Novedades y Promociones */}
            <div
              id="row-email-news"
              className="p-4 flex items-center justify-between transition-colors hover:bg-[#F9FBFC]"
            >
              <div className="pr-3 flex-1">
                <p className="text-[15px] font-semibold text-[#1A1A1A] leading-tight mb-1">
                  Novedades y Promociones
                </p>
                <p className="text-[13px] text-[#6B7280] leading-snug">
                  Ofertas y descuentos especiales
                </p>
              </div>

              {/* Toggle Switch */}
              <button
                type="button"
                role="switch"
                aria-checked={emailSettings.newsAndPromotions}
                onClick={() =>
                  handleToggleEmail('newsAndPromotions', 'Novedades y Promociones')
                }
                className={`relative inline-flex h-7 w-12 items-center rounded-full transition-colors duration-200 cursor-pointer focus:outline-none flex-shrink-0 ${
                  emailSettings.newsAndPromotions ? 'bg-[#00A896]' : 'bg-slate-200'
                }`}
              >
                <span
                  className={`inline-block h-5 w-5 transform rounded-full bg-white shadow-md transition-transform duration-200 ${
                    emailSettings.newsAndPromotions ? 'translate-x-6' : 'translate-x-1'
                  }`}
                />
              </button>
            </div>
          </div>
        </section>

        {/* Section 3: Info Helper Tip Card */}
        <section id="info-helper-section">
          <div className="bg-[#E6F7F5] rounded-2xl p-4 flex items-start space-x-3 border border-[#00A896]/15 shadow-xs">
            <div className="w-6 h-6 rounded-full bg-[#00A896]/15 flex items-center justify-center flex-shrink-0 mt-0.5">
              <Info className="w-4 h-4 text-[#00A896] stroke-[2.2]" />
            </div>
            <p className="text-[13px] text-[#026c60] leading-relaxed">
              Las notificaciones sobre cambios críticos en el punto de encuentro o cancelaciones de último momento se enviarán siempre por seguridad de la comunidad.
            </p>
          </div>
        </section>

        {/* Footer Note */}
        <div className="pt-2 text-center">
          <p className="text-[12px] text-[#9CA3AF] font-medium">
            Thumbi Centro de Notificaciones y Alertas • v1.4.2
          </p>
        </div>
      </main>
    </div>
  );
};

export default NotificationsSettingsScreen;
