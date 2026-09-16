import React from 'react';

export interface ContactDriverModalProps {
  isOpen: boolean;
  onClose: () => void;
  driverName?: string;
  driverPhoneNumber?: string;
  destinationCity?: string;
}

export const ContactDriverModal: React.FC<ContactDriverModalProps> = ({
  isOpen,
  onClose,
  driverName = 'Carlos M.',
  driverPhoneNumber = '+5491148291123',
  destinationCity = 'Pilar',
}) => {
  if (!isOpen) return null;

  // Extraer primer nombre del conductor
  const driverFirstName = driverName.trim().split(' ')[0] || 'conductor';

  // Sanitizar número telefónico para wa.me (solo dígitos sin +, espacios o guiones)
  const cleanPhoneDigits = driverPhoneNumber.replace(/\D/g, '') || '5491148291123';

  const handleWhatsAppClick = () => {
    const message = `Hola ${driverFirstName}, soy tu pasajero en Thumbi para el viaje a ${destinationCity}`;
    const url = `https://wa.me/${cleanPhoneDigits}?text=${encodeURIComponent(message)}`;
    window.open(url, '_blank');
    onClose();
  };

  const handlePhoneCallClick = () => {
    window.location.href = `tel:${driverPhoneNumber}`;
    onClose();
  };

  return (
    <div className="absolute inset-0 z-50 flex flex-col justify-end select-none animate-fadeIn">
      {/* BEGIN: Backdrop Dimmer Overlay */}
      <div
        className="absolute inset-0 bg-black/45 backdrop-blur-[1.5px] z-10 transition-opacity"
        onClick={onClose}
        data-purpose="backdrop-overlay"
      />
      {/* END: Backdrop Dimmer Overlay */}

      {/* BEGIN: Contact Options Bottom Sheet */}
      <div
        className="relative z-20 bg-white rounded-t-[24px] shadow-[0_-8px_30px_rgba(0,0,0,0.12)] pt-2.5 pb-2 px-5 flex flex-col"
        data-purpose="bottom-sheet-modal"
      >
        {/* Top Grabber Pill */}
        <div
          className="w-9 h-1 bg-slate-300 rounded-full mx-auto mb-4 cursor-pointer"
          onClick={onClose}
          data-purpose="drag-handle"
        />

        {/* Modal Header */}
        <header className="mb-4" data-purpose="modal-header">
          <h2 className="text-lg font-bold text-[#1A1A1A] tracking-tight">
            Contactar a {driverName}
          </h2>
          <p className="text-sm text-[#6B7280] mt-0.5">
            Selecciona el medio de contacto
          </p>
        </header>

        {/* Options Container */}
        <section className="space-y-3 mb-4" data-purpose="contact-options-list">
          {/* Option 1: WhatsApp */}
          <button
            id="contact-whatsapp-btn"
            onClick={handleWhatsAppClick}
            className="w-full bg-[#F7F9FA] hover:bg-slate-100 active:bg-slate-200 transition-colors border border-[#E5E7EB] rounded-xl p-3.5 flex items-center justify-between text-left group cursor-pointer"
            data-purpose="option-whatsapp"
            type="button"
          >
            <div className="flex items-center space-x-3.5">
              {/* WhatsApp Brand Icon Container */}
              <div className="w-11 h-11 rounded-full bg-[#25D366] flex items-center justify-center text-white shrink-0 shadow-sm">
                <svg className="w-6 h-6 fill-current" viewBox="0 0 24 24">
                  <path d="M12.04 2c-5.46 0-9.91 4.45-9.91 9.91 0 1.75.46 3.45 1.32 4.95L2.05 22l5.25-1.38c1.45.79 3.08 1.21 4.74 1.21 5.46 0 9.91-4.45 9.91-9.91 0-2.65-1.03-5.14-2.9-7.01A9.82 9.82 0 0 0 12.04 2zm.01 1.67c4.54 0 8.24 3.7 8.24 8.24 0 2.2-.86 4.27-2.42 5.82a8.19 8.19 0 0 1-5.82 2.42c-1.42 0-2.81-.37-4.04-1.1l-.29-.17-3 .79.8-2.93-.19-.3a8.178 8.178 0 0 1-1.26-4.43c0-4.54 3.7-8.24 8.24-8.24zm4.52 11.66c-.25-.13-1.47-.73-1.7-.81-.23-.09-.39-.13-.56.13-.17.25-.64.81-.79.98-.14.17-.29.19-.54.06-.25-.13-1.06-.39-2.02-1.25-.75-.67-1.26-1.5-1.4-1.75-.15-.25-.02-.39.11-.51.11-.11.25-.29.37-.44.13-.15.17-.25.25-.42.09-.17.04-.32-.02-.45-.06-.13-.56-1.35-.77-1.85-.2-.49-.41-.42-.56-.43h-.48c-.17 0-.44.06-.67.31-.23.25-.87.85-.87 2.08 0 1.22.89 2.41 1.02 2.58.13.17 1.75 2.67 4.24 3.74.59.26 1.05.41 1.41.52.59.19 1.13.16 1.56.1.47-.07 1.47-.6 1.68-1.18.21-.58.21-1.07.15-1.18-.06-.11-.23-.17-.48-.3z" />
                </svg>
              </div>
              {/* Labels */}
              <div>
                <p className="font-bold text-[15px] text-[#1A1A1A] leading-tight">
                  Enviar WhatsApp
                </p>
                <p className="text-xs text-[#6B7280] mt-0.5">
                  Abre un chat con mensaje predefinido
                </p>
              </div>
            </div>
            {/* Action Indicator Chevron */}
            <svg
              className="w-5 h-5 text-gray-400 group-hover:translate-x-0.5 transition-transform"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                d="M9 5l7 7-7 7"
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth="2"
              />
            </svg>
          </button>

          {/* Option 2: Phone Call */}
          <button
            id="contact-call-btn"
            onClick={handlePhoneCallClick}
            className="w-full bg-[#F7F9FA] hover:bg-slate-100 active:bg-slate-200 transition-colors border border-[#E5E7EB] rounded-xl p-3.5 flex items-center justify-between text-left group cursor-pointer"
            data-purpose="option-phone-call"
            type="button"
          >
            <div className="flex items-center space-x-3.5">
              {/* Phone Icon Container */}
              <div className="w-11 h-11 rounded-full bg-[#E6F7F5] flex items-center justify-center text-[#00A896] shrink-0 shadow-sm">
                <svg className="w-5 h-5 fill-current" viewBox="0 0 24 24">
                  <path d="M6.62 10.79c1.44 2.83 3.76 5.15 6.59 6.59l2.2-2.2c.28-.28.67-.36 1.02-.25 1.12.37 2.32.57 3.57.57.55 0 1 .45 1 1V20c0 .55-.45 1-1 1-9.39 0-17-7.61-17-17 0-.55.45-1 1-1h3.5c.55 0 1 .45 1 1 0 1.25.2 2.45.57 3.57.11.35.03.74-.25 1.02l-2.2 2.2z" />
                </svg>
              </div>
              {/* Labels */}
              <div>
                <p className="font-bold text-[15px] text-[#1A1A1A] leading-tight">
                  Llamar por teléfono
                </p>
                <p className="text-xs text-[#6B7280] mt-0.5">
                  Para urgencias en el punto de encuentro
                </p>
              </div>
            </div>
            {/* Action Indicator Chevron */}
            <svg
              className="w-5 h-5 text-gray-400 group-hover:translate-x-0.5 transition-transform"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                d="M9 5l7 7-7 7"
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth="2"
              />
            </svg>
          </button>
        </section>

        {/* Dismiss Button */}
        <footer className="mt-1" data-purpose="modal-footer">
          <button
            id="close-contact-modal-btn"
            onClick={onClose}
            className="w-full bg-[#F1F5F9] hover:bg-gray-200 active:bg-gray-300 text-[#1A1A1A] font-semibold text-[15px] py-3.5 rounded-xl transition-colors cursor-pointer"
            type="button"
          >
            Cancelar
          </button>
        </footer>

        {/* iOS Home Indicator */}
        <div className="pt-4 pb-1 flex justify-center" data-purpose="home-indicator-area">
          <div className="w-36 h-1 bg-gray-300 rounded-full" />
        </div>
      </div>
      {/* END: Contact Options Bottom Sheet */}
    </div>
  );
};

export default ContactDriverModal;
