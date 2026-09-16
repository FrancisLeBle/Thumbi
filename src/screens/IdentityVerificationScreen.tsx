import React, { useState, useRef } from 'react';
import { Loader2, Check, Camera, RefreshCw } from 'lucide-react';
import { Toast } from '../components/Toast';

export interface IdentityVerificationScreenProps {
  userEmail?: string;
  userName?: string;
  onVerificationSuccess: () => void;
  onBack?: () => void;
  onSkip?: () => void;
}

export const IdentityVerificationScreen: React.FC<IdentityVerificationScreenProps> = ({
  userEmail = 'sofia.martinez@ejemplo.com',
  userName = 'Sofía',
  onVerificationSuccess,
  onBack,
  onSkip,
}) => {
  const [frontFile, setFrontFile] = useState<File | null>(null);
  const [frontPreview, setFrontPreview] = useState<string | null>(null);
  const [backFile, setBackFile] = useState<File | null>(null);
  const [backPreview, setBackPreview] = useState<string | null>(null);
  const [isVerifying, setIsVerifying] = useState<boolean>(false);
  const [toast, setToast] = useState<{
    message: string;
    type: 'error' | 'success';
  } | null>(null);

  const frontInputRef = useRef<HTMLInputElement>(null);
  const backInputRef = useRef<HTMLInputElement>(null);

  const handleFrontFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (file) {
      setFrontFile(file);
      const previewUrl = URL.createObjectURL(file);
      setFrontPreview(previewUrl);
      setToast({
        message: 'Frente del DNI cargado correctamente.',
        type: 'success',
      });
    }
  };

  const handleBackFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (file) {
      setBackFile(file);
      const previewUrl = URL.createObjectURL(file);
      setBackPreview(previewUrl);
      setToast({
        message: 'Dorso del DNI cargado correctamente.',
        type: 'success',
      });
    }
  };

  const handleSimulateFront = (e: React.MouseEvent) => {
    e.stopPropagation();
    setFrontPreview('simulated_front');
    setToast({
      message: 'Frente del DNI capturado (simulación).',
      type: 'success',
    });
  };

  const handleSimulateBack = (e: React.MouseEvent) => {
    e.stopPropagation();
    setBackPreview('simulated_back');
    setToast({
      message: 'Dorso del DNI capturado (simulación).',
      type: 'success',
    });
  };

  const handleValidateAndLogin = async () => {
    if (isVerifying) return;

    if (!frontPreview || !backPreview) {
      // Si el usuario presiona validar sin haber cargado ambos, le damos retroalimentación clara
      setToast({
        message: 'Por favor sube o escanea ambas caras de tu DNI (Frente y Dorso).',
        type: 'error',
      });
      return;
    }

    setIsVerifying(true);

    try {
      // Simulación de procesamiento OCR, validación biométrica con Renaper / servicio de verificación
      await new Promise((resolve) => setTimeout(resolve, 1500));

      setToast({
        message: '¡Identidad verificada exitosamente! Tu cuenta ha sido activada.',
        type: 'success',
      });

      setTimeout(() => {
        onVerificationSuccess();
      }, 600);
    } catch {
      setToast({
        message: 'Hubo un error al procesar los documentos. Inténtalo nuevamente.',
        type: 'error',
      });
    } finally {
      setIsVerifying(false);
    }
  };

  const handleSkipLater = () => {
    if (onSkip) {
      onSkip();
    } else {
      onVerificationSuccess();
    }
  };

  return (
    <div className="min-h-screen flex items-center justify-center p-0 md:p-6 bg-slate-900 font-sans">
      {/* Toast Notification */}
      {toast && (
        <Toast
          message={toast.message}
          type={toast.type}
          onClose={() => setToast(null)}
        />
      )}

      {/* Hidden File Inputs for native camera / file picker */}
      <input
        type="file"
        ref={frontInputRef}
        onChange={handleFrontFileChange}
        accept="image/*"
        className="hidden"
      />
      <input
        type="file"
        ref={backInputRef}
        onChange={handleBackFileChange}
        accept="image/*"
        className="hidden"
      />

      {/* BEGIN: PhoneFrame */}
      <main
        className="w-full max-w-[393px] h-[852px] bg-white text-gray-900 md:rounded-[44px] overflow-hidden shadow-2xl relative flex flex-col justify-between select-none border border-gray-200"
        data-purpose="mobile-device-frame"
      >
        {/* Top Scrollable Container */}
        <div className="w-full flex flex-col overflow-y-auto">
          {/* BEGIN: iOSStatusBar */}
          <header
            className="w-full pt-3 px-7 flex justify-between items-center z-20 shrink-0 select-none"
            data-purpose="ios-status-bar"
          >
            {/* Time */}
            <span className="text-[15px] font-semibold tracking-tight text-black">9:41</span>
            {/* System Icons */}
            <div className="flex items-center space-x-1.5 text-black">
              {/* Cellular Signal */}
              <svg aria-label="Señal móvil" className="w-4 h-4 fill-current" viewBox="0 0 24 24">
                <path d="M2 17h3v4H2v-4zm5-4h3v8H7v-8zm5-4h3v12h-3V9zm5-5h3v17h-3V4z" />
              </svg>
              {/* Wi-Fi */}
              <svg aria-label="Wi-Fi" className="w-4 h-4 fill-current" viewBox="0 0 24 24">
                <path d="M12 4C7.31 4 3.07 5.9 0 8.98L12 21 24 8.98C20.93 5.9 16.69 4 12 4zm0 3.5c3.78 0 7.21 1.48 9.77 3.93L12 18.2 2.23 11.43C4.79 8.98 8.22 7.5 12 7.5z" />
              </svg>
              {/* Battery */}
              <div className="w-6 h-3 border border-black rounded-[4px] p-0.5 flex items-center relative">
                <div className="h-full w-full bg-black rounded-[2px]" />
                <div className="absolute -right-1 w-0.5 h-1.5 bg-black rounded-r-sm" />
              </div>
            </div>
          </header>
          {/* END: iOSStatusBar */}

          {/* BEGIN: NavigationAndProgress */}
          <section className="pt-2 px-6 pb-2 shrink-0" data-purpose="navigation-header">
            {/* Top Bar: Back button + Title */}
            <div className="relative flex items-center justify-center h-11">
              {onBack && (
                <button
                  aria-label="Volver atrás"
                  onClick={onBack}
                  className="absolute left-0 p-1 -ml-1 text-gray-900 hover:opacity-70 transition-opacity cursor-pointer"
                  type="button"
                >
                  <svg className="w-6 h-6 stroke-current stroke-[2.2] fill-none" viewBox="0 0 24 24">
                    <path d="M15 19l-7-7 7-7" strokeLinecap="round" strokeLinejoin="round" />
                  </svg>
                </button>
              )}
              <h1 className="text-[17px] font-bold tracking-tight text-gray-900">Crea tu cuenta</h1>
            </div>

            {/* Step Indicator & Category Labels */}
            <div className="flex items-center justify-between text-[13px] mt-3 mb-2">
              <span className="text-[#00A896] font-semibold">Paso 2 de 2</span>
              <span className="text-gray-500 font-normal">Verificación de identidad</span>
            </div>

            {/* Linear Progress Bar: 100% Completed on Step 2 */}
            <div className="w-full h-1 bg-gray-200 rounded-full overflow-hidden">
              <div className="h-full bg-[#00A896] w-full rounded-full transition-all duration-300" />
            </div>
          </section>
          {/* END: NavigationAndProgress */}

          {/* BEGIN: MainContent */}
          <section
            className="flex-1 px-6 pt-4 flex flex-col justify-start"
            data-purpose="verification-content"
          >
            {/* Header Titles */}
            <div className="mb-4">
              <h2 className="text-[22px] font-bold text-gray-900 tracking-tight leading-tight">
                Verifica tu Identidad
              </h2>
              <p className="text-[13.5px] leading-snug text-gray-500 mt-1.5">
                Escanea tu DNI para validar tu perfil y garantizar la seguridad de la comunidad.
              </p>
            </div>

            {/* Upload Slots */}
            <div className="space-y-3.5">
              {/* Card 1: Frente del DNI */}
              <div
                id="dni-front-card"
                onClick={() => frontInputRef.current?.click()}
                className={`w-full min-h-[126px] p-3 rounded-2xl flex flex-col items-center justify-center cursor-pointer shadow-xs transition-all relative ${
                  frontPreview
                    ? 'bg-[#F0F9F8] border-[1.5px] border-[#00A896]'
                    : 'bg-[#F7F9FA] border-[1.5px] border-dashed border-gray-300 hover:bg-gray-100/80 active:scale-[0.99]'
                }`}
                data-purpose="dni-front-upload"
                role="button"
                tabIndex={0}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' || e.key === ' ') frontInputRef.current?.click();
                }}
              >
                {frontPreview ? (
                  <div className="flex flex-col items-center justify-center text-center">
                    <div className="w-9 h-9 rounded-full bg-[#00A896] text-white flex items-center justify-center mb-1.5 shadow-xs">
                      <Check className="w-5 h-5 stroke-[2.5]" />
                    </div>
                    <span className="text-[15px] font-bold text-gray-900 tracking-tight">
                      Frente del DNI cargado
                    </span>
                    <span className="text-[12px] text-[#00A896] font-medium mt-0.5">
                      {frontFile ? frontFile.name : 'Documento escaneado y legible'}
                    </span>
                    <button
                      type="button"
                      onClick={(e) => {
                        e.stopPropagation();
                        setFrontPreview(null);
                        setFrontFile(null);
                      }}
                      className="mt-1 text-[11px] text-gray-500 hover:text-gray-800 underline flex items-center gap-1"
                    >
                      <RefreshCw className="w-3 h-3" /> Reemplazar imagen
                    </button>
                  </div>
                ) : (
                  <>
                    <div className="text-[#00A896] mb-1.5">
                      <svg className="w-7 h-7 stroke-current stroke-[1.8] fill-none" viewBox="0 0 24 24">
                        <path
                          d="M3 9a2 2 0 012-2h.93a2 2 0 001.664-.89l.812-1.22A2 2 0 0110.07 4h3.86a2 2 0 011.664.89l.812 1.22A2 2 0 0018.07 7H19a2 2 0 012 2v9a2 2 0 01-2 2H5a2 2 0 01-2-2V9z"
                          strokeLinecap="round"
                          strokeLinejoin="round"
                        />
                        <circle cx="12" cy="13" r="3.5" strokeLinecap="round" strokeLinejoin="round" />
                      </svg>
                    </div>
                    <span className="text-[15px] font-bold text-gray-900 tracking-tight">
                      Frente del DNI
                    </span>
                    <span className="text-[12.5px] text-gray-500 mt-0.5">
                      Toca para abrir la cámara o seleccionar archivo
                    </span>
                    <button
                      type="button"
                      onClick={handleSimulateFront}
                      className="mt-2 text-[11px] font-medium text-[#00A896] bg-white border border-[#00A896]/30 px-2 py-0.5 rounded-md hover:bg-[#F0F9F8] transition"
                    >
                      Usar captura de prueba
                    </button>
                  </>
                )}
              </div>

              {/* Card 2: Dorso del DNI */}
              <div
                id="dni-back-card"
                onClick={() => backInputRef.current?.click()}
                className={`w-full min-h-[126px] p-3 rounded-2xl flex flex-col items-center justify-center cursor-pointer shadow-xs transition-all relative ${
                  backPreview
                    ? 'bg-[#F0F9F8] border-[1.5px] border-[#00A896]'
                    : 'bg-[#F7F9FA] border-[1.5px] border-dashed border-gray-300 hover:bg-gray-100/80 active:scale-[0.99]'
                }`}
                data-purpose="dni-back-upload"
                role="button"
                tabIndex={0}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' || e.key === ' ') backInputRef.current?.click();
                }}
              >
                {backPreview ? (
                  <div className="flex flex-col items-center justify-center text-center">
                    <div className="w-9 h-9 rounded-full bg-[#00A896] text-white flex items-center justify-center mb-1.5 shadow-xs">
                      <Check className="w-5 h-5 stroke-[2.5]" />
                    </div>
                    <span className="text-[15px] font-bold text-gray-900 tracking-tight">
                      Dorso del DNI cargado
                    </span>
                    <span className="text-[12px] text-[#00A896] font-medium mt-0.5">
                      {backFile ? backFile.name : 'Código PDF417 verificado'}
                    </span>
                    <button
                      type="button"
                      onClick={(e) => {
                        e.stopPropagation();
                        setBackPreview(null);
                        setBackFile(null);
                      }}
                      className="mt-1 text-[11px] text-gray-500 hover:text-gray-800 underline flex items-center gap-1"
                    >
                      <RefreshCw className="w-3 h-3" /> Reemplazar imagen
                    </button>
                  </div>
                ) : (
                  <>
                    <div className="text-[#00A896] mb-1.5">
                      <svg className="w-7 h-7 stroke-current stroke-[1.8] fill-none" viewBox="0 0 24 24">
                        <path
                          d="M3 9a2 2 0 012-2h.93a2 2 0 001.664-.89l.812-1.22A2 2 0 0110.07 4h3.86a2 2 0 011.664.89l.812 1.22A2 2 0 0018.07 7H19a2 2 0 012 2v9a2 2 0 01-2 2H5a2 2 0 01-2-2V9z"
                          strokeLinecap="round"
                          strokeLinejoin="round"
                        />
                        <circle cx="12" cy="13" r="3.5" strokeLinecap="round" strokeLinejoin="round" />
                      </svg>
                    </div>
                    <span className="text-[15px] font-bold text-gray-900 tracking-tight">
                      Dorso del DNI
                    </span>
                    <span className="text-[12.5px] text-gray-500 mt-0.5">
                      Toca para abrir la cámara o seleccionar archivo
                    </span>
                    <button
                      type="button"
                      onClick={handleSimulateBack}
                      className="mt-2 text-[11px] font-medium text-[#00A896] bg-white border border-[#00A896]/30 px-2 py-0.5 rounded-md hover:bg-[#F0F9F8] transition"
                    >
                      Usar captura de prueba
                    </button>
                  </>
                )}
              </div>
            </div>
          </section>
          {/* END: MainContent */}
        </div>

        {/* BEGIN: FooterActions */}
        <footer
          className="px-6 pb-3 pt-2 shrink-0 flex flex-col items-center bg-white border-t border-gray-100"
          data-purpose="actions-and-security"
        >
          {/* Primary Action CTA */}
          <button
            id="validate-dni-submit-btn"
            onClick={handleValidateAndLogin}
            disabled={isVerifying}
            className="w-full h-[52px] bg-[#00A896] hover:bg-[#008F80] active:scale-[0.98] transition-all text-white font-semibold text-[15.5px] rounded-xl flex items-center justify-center shadow-md cursor-pointer disabled:opacity-60"
            data-purpose="submit-verification"
            type="button"
          >
            {isVerifying ? (
              <span className="flex items-center gap-2">
                <Loader2 className="w-5 h-5 animate-spin" />
                <span>Validando identidad...</span>
              </span>
            ) : (
              'Validar e Iniciar Sesión'
            )}
          </button>

          {/* Secondary Action Link: Hacer este paso más tarde */}
          <button
            id="skip-verification-btn"
            onClick={handleSkipLater}
            className="mt-3 mb-3 text-[#00A896] hover:underline text-[14px] font-medium transition cursor-pointer"
            data-purpose="skip-step"
            type="button"
          >
            Hacer este paso más tarde
          </button>

          {/* Security / Trust Badge */}
          <div
            className="flex items-center justify-center space-x-1.5 text-gray-500 mb-1"
            data-purpose="security-note"
          >
            {/* Shield Icon */}
            <svg
              className="w-4 h-4 stroke-gray-400 fill-none shrink-0"
              strokeWidth="2"
              viewBox="0 0 24 24"
            >
              <path
                d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"
                strokeLinecap="round"
                strokeLinejoin="round"
              />
              <path d="M9 12l2 2 4-4" strokeLinecap="round" strokeLinejoin="round" />
            </svg>
            <span className="text-[11.5px] text-gray-500 leading-none">
              Tus datos están protegidos y solo se utilizarán para validación.
            </span>
          </div>

          {/* iOS Home Indicator */}
          <div
            className="w-32 h-1 bg-black/80 rounded-full mt-2 mb-1"
            data-purpose="ios-home-indicator"
          />
        </footer>
        {/* END: FooterActions */}
      </main>
      {/* END: PhoneFrame */}
    </div>
  );
};

export default IdentityVerificationScreen;
