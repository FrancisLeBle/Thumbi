import React, { useEffect } from 'react';
import { AlertCircle, CheckCircle2, X } from 'lucide-react';

export interface ToastProps {
  id?: string;
  message: string;
  type?: 'error' | 'success';
  duration?: number;
  onClose: () => void;
}

export const Toast: React.FC<ToastProps> = ({
  id = 'toast-notification',
  message,
  type = 'error',
  duration = 5000,
  onClose,
}) => {
  useEffect(() => {
    if (duration <= 0) return;
    const timer = setTimeout(() => {
      onClose();
    }, duration);
    return () => clearTimeout(timer);
  }, [duration, onClose]);

  const isError = type === 'error';
  const indicatorColor = isError ? '#E63946' : '#2A9D8F';

  return (
    <div
      id={id}
      role="alert"
      aria-live="assertive"
      className="fixed bottom-6 right-6 z-50 flex items-center gap-3 w-full max-w-sm rounded-[12px] bg-[#FFFFFF] p-4 text-[#1A1A1A] transition-all"
      style={{
        boxShadow: '0px 2px 8px rgba(0, 0, 0, 0.04)',
        borderLeft: `4px solid ${indicatorColor}`,
        borderTop: '1px solid #F0F0F0',
        borderRight: '1px solid #F0F0F0',
        borderBottom: '1px solid #F0F0F0',
      }}
    >
      <div className="shrink-0 flex items-center justify-center">
        {isError ? (
          <AlertCircle className="w-5 h-5" style={{ color: '#E63946' }} aria-hidden="true" />
        ) : (
          <CheckCircle2 className="w-5 h-5" style={{ color: '#2A9D8F' }} aria-hidden="true" />
        )}
      </div>

      <p className="flex-1 text-sm font-medium leading-relaxed m-0">{message}</p>

      <button
        id={`${id}-close-btn`}
        type="button"
        onClick={onClose}
        aria-label="Cerrar notificación"
        className="shrink-0 rounded-md p-1 text-[#666666] hover:text-[#1A1A1A] hover:bg-[#F4F4F5] transition-colors focus:outline-none focus:ring-2 focus:ring-offset-1 focus:ring-black"
      >
        <X className="w-4 h-4" />
      </button>
    </div>
  );
};

export default Toast;
