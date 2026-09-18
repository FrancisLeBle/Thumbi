import React, { useEffect } from 'react';
import { AlertCircle, CheckCircle2, Info, X } from 'lucide-react';

export interface ToastProps {
  id?: string;
  message: string;
  type?: 'error' | 'success' | 'info';
  duration?: number;
  onClose: () => void;
}

export const Toast: React.FC<ToastProps> = ({
  id = 'toast-notification',
  message,
  type = 'success',
  duration = 4000,
  onClose,
}) => {
  useEffect(() => {
    if (duration <= 0) return;
    const timer = setTimeout(() => {
      onClose();
    }, duration);
    return () => clearTimeout(timer);
  }, [duration, onClose]);

  const indicatorColor =
    type === 'error' ? '#E63946' : type === 'info' ? '#05668D' : '#00A896';

  return (
    <div
      id={id}
      role="alert"
      aria-live="assertive"
      className="flex items-center gap-3 w-full rounded-[14px] bg-white p-3.5 text-[#1A1A1A] shadow-[0_8px_24px_rgba(0,0,0,0.12)] border border-slate-100 transition-all select-none"
      style={{
        borderLeft: `4px solid ${indicatorColor}`,
      }}
    >
      <div className="shrink-0 flex items-center justify-center">
        {type === 'error' ? (
          <AlertCircle className="w-5 h-5" style={{ color: '#E63946' }} aria-hidden="true" />
        ) : type === 'info' ? (
          <Info className="w-5 h-5" style={{ color: '#05668D' }} aria-hidden="true" />
        ) : (
          <CheckCircle2 className="w-5 h-5" style={{ color: '#00A896' }} aria-hidden="true" />
        )}
      </div>

      <p className="flex-1 text-[13px] font-medium leading-snug m-0 text-slate-800">{message}</p>

      <button
        id={`${id}-close-btn`}
        type="button"
        onClick={onClose}
        aria-label="Cerrar notificación"
        className="shrink-0 rounded-md p-1 text-[#666666] hover:text-[#1A1A1A] hover:bg-[#F4F4F5] transition-colors focus:outline-none focus:ring-1 focus:ring-black cursor-pointer"
      >
        <X className="w-4 h-4" />
      </button>
    </div>
  );
};

export default Toast;

