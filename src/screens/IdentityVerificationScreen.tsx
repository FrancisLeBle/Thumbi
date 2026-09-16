import React, { useState } from 'react';
import {
  ShieldCheck,
  Upload,
  Camera,
  CheckCircle2,
  FileCheck,
  AlertCircle,
  Loader2,
  ArrowRight,
  ArrowLeft,
  Lock,
} from 'lucide-react';
import { Toast } from '../components/Toast';

export interface IdentityVerificationScreenProps {
  userEmail?: string;
  userName?: string;
  onVerificationSuccess: () => void;
  onBack?: () => void;
}

export const IdentityVerificationScreen: React.FC<IdentityVerificationScreenProps> = ({
  userEmail = 'sofia.martinez@ejemplo.com',
  userName = 'Sofía',
  onVerificationSuccess,
  onBack,
}) => {
  const [frontUploaded, setFrontUploaded] = useState<boolean>(false);
  const [backUploaded, setBackUploaded] = useState<boolean>(false);
  const [frontFileName, setFrontFileName] = useState<string>('');
  const [backFileName, setBackFileName] = useState<string>('');
  const [isVerifying, setIsVerifying] = useState<boolean>(false);
  const [toast, setToast] = useState<{
    message: string;
    type: 'error' | 'success';
  } | null>(null);

  const canSubmit = frontUploaded && backUploaded;

  const handleSimulateFrontUpload = () => {
    setFrontUploaded(true);
    setFrontFileName('dni_frente_verificado.jpg');
    setToast({
      message: 'Frente del DNI capturado y legible.',
      type: 'success',
    });
  };

  const handleSimulateBackUpload = () => {
    setBackUploaded(true);
    setBackFileName('dni_dorso_verificado.jpg');
    setToast({
      message: 'Dorso del DNI capturado con código PDF417 legible.',
      type: 'success',
    });
  };

  const handleValidateAndLogin = async () => {
    if (!canSubmit || isVerifying) return;

    setIsVerifying(true);

    try {
      // Simula la verificación biométrica y OCR de identidad con tiempo de cómputo seguro
      await new Promise((resolve) => setTimeout(resolve, 1400));

      setToast({
        message: '¡Identidad verificada exitosamente! Tu cuenta está activa con DNI Verificado.',
        type: 'success',
      });

      setTimeout(() => {
        onVerificationSuccess();
      }, 800);
    } catch {
      setToast({
        message: 'Hubo un error al procesar los documentos. Inténtalo de nuevo.',
        type: 'error',
      });
    } finally {
      setIsVerifying(false);
    }
  };

  return (
    <div
      id="identity-verification-screen"
      className="min-h-screen py-8 px-4 sm:px-6 lg:px-8 max-w-md mx-auto flex flex-col justify-between"
      style={{
        backgroundColor: '#F7F9FA',
        fontFamily: '-apple-system, BlinkMacSystemFont, "SF Pro Text", "Segoe UI", Roboto, sans-serif',
      }}
    >
      <div>
        {/* Barra superior con navegación */}
        <div className="flex items-center justify-between mb-6">
          {onBack && (
            <button
              id="back-to-register-btn"
              type="button"
              onClick={onBack}
              className="p-2 -ml-2 rounded-lg text-[#666666] hover:bg-slate-200/60 transition cursor-pointer"
            >
              <ArrowLeft className="w-5 h-5" />
            </button>
          )}
          <span className="text-xs font-semibold text-[#00A896] bg-[#E6F6F4] px-2.5 py-1 rounded-full ml-auto">
            Paso 2 de 2 • Onboarding
          </span>
        </div>

        {/* Encabezado */}
        <div className="text-center mb-6">
          <div className="inline-flex items-center justify-center w-12 h-12 rounded-full bg-[#E6F6F4] text-[#00A896] mb-3">
            <ShieldCheck className="w-6 h-6 stroke-[2]" />
          </div>
          <h1 className="text-2xl font-bold tracking-tight text-[#1A1A1A]">
            Verificación de Identidad
          </h1>
          <p className="mt-2 text-sm text-[#666666] leading-relaxed">
            Para garantizar viajes seguros, todos los miembros de Thumbi deben validar su Documento Nacional de Identidad.
          </p>
        </div>

        {/* Cajas de subida de DNI con border-dashed */}
        <div className="space-y-4 mb-6">
          {/* Frente de DNI */}
          <div
            id="dni-front-upload-box"
            onClick={handleSimulateFrontUpload}
            className={`rounded-[12px] p-5 border-2 border-dashed transition-all cursor-pointer flex flex-col items-center justify-center text-center ${
              frontUploaded
                ? 'border-[#00A896] bg-[#F0FAF8]'
                : 'border-[#D0D7DE] bg-[#FFFFFF] hover:border-[#00A896]/60 hover:bg-[#FAFDFC]'
            }`}
            style={{
              boxShadow: '0px 2px 8px rgba(0, 0, 0, 0.02)',
            }}
          >
            {frontUploaded ? (
              <div className="flex flex-col items-center">
                <div className="w-10 h-10 rounded-full bg-[#00A896] text-white flex items-center justify-center mb-2">
                  <CheckCircle2 className="w-5 h-5" />
                </div>
                <span className="text-xs font-bold text-[#00A896]">Frente del DNI Cargado</span>
                <span className="text-[11px] text-[#666666] mt-0.5">{frontFileName}</span>
                <span className="text-[10px] text-[#00A896] font-semibold mt-2 underline">
                  Toca para volver a capturar
                </span>
              </div>
            ) : (
              <div className="flex flex-col items-center">
                <div className="w-10 h-10 rounded-full bg-[#F3F4F6] text-[#666666] flex items-center justify-center mb-2">
                  <Camera className="w-5 h-5" />
                </div>
                <span className="text-xs font-bold text-[#1A1A1A]">Subir o escanear Frente de DNI</span>
                <span className="text-[11px] text-[#777777] mt-1">
                  Foto nítida del frente con datos y fotografía visibles
                </span>
                <button
                  type="button"
                  className="mt-3 px-3 py-1 rounded-md bg-[#E6F6F4] text-[#00A896] text-xs font-semibold hover:bg-[#D5EFEA] transition"
                >
                  Capturar Frente
                </button>
              </div>
            )}
          </div>

          {/* Dorso de DNI */}
          <div
            id="dni-back-upload-box"
            onClick={handleSimulateBackUpload}
            className={`rounded-[12px] p-5 border-2 border-dashed transition-all cursor-pointer flex flex-col items-center justify-center text-center ${
              backUploaded
                ? 'border-[#00A896] bg-[#F0FAF8]'
                : 'border-[#D0D7DE] bg-[#FFFFFF] hover:border-[#00A896]/60 hover:bg-[#FAFDFC]'
            }`}
            style={{
              boxShadow: '0px 2px 8px rgba(0, 0, 0, 0.02)',
            }}
          >
            {backUploaded ? (
              <div className="flex flex-col items-center">
                <div className="w-10 h-10 rounded-full bg-[#00A896] text-white flex items-center justify-center mb-2">
                  <CheckCircle2 className="w-5 h-5" />
                </div>
                <span className="text-xs font-bold text-[#00A896]">Dorso del DNI Cargado</span>
                <span className="text-[11px] text-[#666666] mt-0.5">{backFileName}</span>
                <span className="text-[10px] text-[#00A896] font-semibold mt-2 underline">
                  Toca para volver a capturar
                </span>
              </div>
            ) : (
              <div className="flex flex-col items-center">
                <div className="w-10 h-10 rounded-full bg-[#F3F4F6] text-[#666666] flex items-center justify-center mb-2">
                  <Upload className="w-5 h-5" />
                </div>
                <span className="text-xs font-bold text-[#1A1A1A]">Subir o escanear Dorso de DNI</span>
                <span className="text-[11px] text-[#777777] mt-1">
                  Asegúrate que el código de barras PDF417 no tenga reflejos
                </span>
                <button
                  type="button"
                  className="mt-3 px-3 py-1 rounded-md bg-[#E6F6F4] text-[#00A896] text-xs font-semibold hover:bg-[#D5EFEA] transition"
                >
                  Capturar Dorso
                </button>
              </div>
            )}
          </div>
        </div>

        {/* Badge de Protección de Datos */}
        <div
          id="kyc-protection-badge"
          className="rounded-[12px] p-4 bg-[#FFFFFF] border border-[#E8EEF2] flex items-start gap-3 shadow-xs"
        >
          <div className="p-1.5 rounded-lg bg-[#E6F6F4] text-[#00A896] shrink-0 mt-0.5">
            <Lock className="w-4 h-4" />
          </div>
          <div>
            <span className="text-xs font-bold text-[#1A1A1A] block">
              Protección de Datos & Cifrado AES-256
            </span>
            <p className="text-[11px] text-[#666666] mt-1 leading-relaxed">
              Tus documentos se cifran y procesan de acuerdo a las leyes de protección de datos personales. Thumbi nunca comparte tus imágenes con otros usuarios.
            </p>
          </div>
        </div>
      </div>

      {/* Botón Principal de Validación */}
      <div className="pt-6">
        <button
          id="validate-and-login-btn"
          type="button"
          disabled={!canSubmit || isVerifying}
          onClick={handleValidateAndLogin}
          className="w-full flex items-center justify-center gap-2 py-3.5 px-4 rounded-[12px] text-white font-semibold text-sm transition-all focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-[#00A896] disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer shadow-sm"
          style={{ backgroundColor: '#00A896' }}
        >
          {isVerifying ? (
            <>
              <Loader2 className="w-4 h-4 animate-spin" />
              <span>Validando biometría y DNI...</span>
            </>
          ) : (
            <>
              <span>Validar e Iniciar Sesión</span>
              <ArrowRight className="w-4 h-4" />
            </>
          )}
        </button>

        {!canSubmit && (
          <p className="text-center text-[11px] text-[#888888] mt-2">
            Debes capturar el frente y dorso del documento para habilitar la validación.
          </p>
        )}
      </div>

      {toast && (
        <Toast
          id="identity-verification-toast"
          message={toast.message}
          type={toast.type}
          onClose={() => setToast(null)}
        />
      )}
    </div>
  );
};

export default IdentityVerificationScreen;
