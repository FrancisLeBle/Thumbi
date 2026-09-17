import React, { useState } from 'react';
import {
  ChevronLeft,
  Landmark,
  CreditCard,
  Plus,
  Check,
  Lock,
  X,
  CheckCircle2,
  ChevronRight,
  ShieldCheck,
  AlertCircle,
} from 'lucide-react';

export interface PaymentMethodsScreenProps {
  onBack: () => void;
}

interface PayoutAccount {
  id: string;
  name: string;
  alias: string;
  cvuMasked: string;
  bankName: string;
  isPrimary: boolean;
  isVerified: boolean;
}

interface SavedCard {
  id: string;
  brand: string;
  type: string;
  lastFour: string;
  expiry: string;
  holder: string;
  isDefault: boolean;
}

export const PaymentMethodsScreen: React.FC<PaymentMethodsScreenProps> = ({ onBack }) => {
  // Cuentas para cobro (Conductor)
  const [payoutAccounts, setPayoutAccounts] = useState<PayoutAccount[]>([
    {
      id: 'payout-1',
      name: 'Mercado Pago / CBU',
      alias: 'thumbi.conductor.mp',
      cvuMasked: '•••• 9281',
      bankName: 'Banco Industrial',
      isPrimary: true,
      isVerified: true,
    },
  ]);

  // Tarjetas para pagar reservas (Pasajero)
  const [savedCards, setSavedCards] = useState<SavedCard[]>([
    {
      id: 'card-1',
      brand: 'VISA',
      type: 'Visa Débito',
      lastFour: '4821',
      expiry: '09/27',
      holder: 'JUAN F LORUSSO',
      isDefault: true,
    },
  ]);

  // Estados de Modales
  const [isAddPayoutOpen, setIsAddPayoutOpen] = useState<boolean>(false);
  const [isAddCardOpen, setIsAddCardOpen] = useState<boolean>(false);
  const [selectedCardDetail, setSelectedCardDetail] = useState<SavedCard | null>(null);

  // Formulario nuevo CBU/CVU
  const [newAlias, setNewAlias] = useState<string>('');
  const [newCbu, setNewCbu] = useState<string>('');
  const [newBank, setNewBank] = useState<string>('Mercado Pago');

  // Formulario nueva tarjeta
  const [newCardNumber, setNewCardNumber] = useState<string>('');
  const [newCardHolder, setNewCardHolder] = useState<string>('');
  const [newCardExpiry, setNewCardExpiry] = useState<string>('');
  const [newCardCvc, setNewCardCvc] = useState<string>('');

  // Toast feedback
  const [toastMessage, setToastMessage] = useState<string | null>(null);

  const showToast = (msg: string) => {
    setToastMessage(msg);
    setTimeout(() => {
      setToastMessage(null);
    }, 3200);
  };

  const handleAddPayoutSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!newCbu.trim()) return;

    const lastFour = newCbu.trim().slice(-4) || '1234';
    const newAccount: PayoutAccount = {
      id: `payout-${Date.now()}`,
      name: `${newBank} / Cuenta`,
      alias: newAlias.trim().toLowerCase() || 'thumbi.cobro.nuevo',
      cvuMasked: `•••• ${lastFour}`,
      bankName: newBank,
      isPrimary: false,
      isVerified: true,
    };

    setPayoutAccounts((prev) => [...prev, newAccount]);
    setIsAddPayoutOpen(false);
    setNewAlias('');
    setNewCbu('');
    showToast('Cuenta de cobro vinculada y verificada');
  };

  const handleAddCardSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!newCardNumber.trim()) return;

    const cleanNum = newCardNumber.replace(/\s+/g, '');
    const lastFour = cleanNum.slice(-4) || '7890';
    const isMaster = cleanNum.startsWith('5');
    const brand = isMaster ? 'MASTERCARD' : 'VISA';
    const type = `${isMaster ? 'Mastercard' : 'Visa'} Crédito`;

    const newCard: SavedCard = {
      id: `card-${Date.now()}`,
      brand,
      type,
      lastFour,
      expiry: newCardExpiry || '12/28',
      holder: newCardHolder.toUpperCase() || 'USUARIO THUMBI',
      isDefault: false,
    };

    setSavedCards((prev) => [...prev, newCard]);
    setIsAddCardOpen(false);
    setNewCardNumber('');
    setNewCardHolder('');
    setNewCardExpiry('');
    setNewCardCvc('');
    showToast('Tarjeta de pago registrada con éxito');
  };

  return (
    <div
      id="payment-methods-screen"
      className="w-full max-w-md mx-auto min-h-screen bg-[#F7F9FA] px-5 pt-4 pb-28 space-y-4 select-none relative"
      style={{
        fontFamily:
          '-apple-system, BlinkMacSystemFont, "Plus Jakarta Sans", "SF Pro Text", "Inter", sans-serif',
      }}
    >
      {/* Toast flotante */}
      {toastMessage && (
        <div className="fixed top-5 left-1/2 -translate-x-1/2 z-50 bg-[#1A1A1A] text-white px-4 py-2.5 rounded-full text-xs font-medium shadow-lg flex items-center space-x-2 animate-in fade-in slide-in-from-top-2 duration-200">
          <CheckCircle2 className="w-4 h-4 text-[#00A896]" />
          <span>{toastMessage}</span>
        </div>
      )}

      {/* Navigation Header */}
      <header className="relative pt-1 pb-1 flex items-center justify-between z-20">
        <button
          id="payments-back-btn"
          type="button"
          onClick={onBack}
          className="w-10 h-10 -ml-2 rounded-full flex items-center justify-center text-[#1A1A1A] hover:bg-[#EFF5F2] active:scale-95 transition-all cursor-pointer"
          aria-label="Volver a Perfil"
        >
          <ChevronLeft className="w-6 h-6 stroke-[2.4] text-[#1A1A1A]" />
        </button>
        <h1
          id="payments-header-title"
          className="text-[18px] font-bold text-[#1A1A1A] text-center tracking-tight flex-1 mr-8"
        >
          Métodos de pago y cobro
        </h1>
      </header>

      {/* Contenido Principal */}
      <main className="space-y-6 pt-1">
        {/* SECCIÓN 1: PARA RECIBIR COBROS (CONDUCTOR) */}
        <section id="payout-methods-section" aria-labelledby="payout-heading" className="space-y-2.5">
          <div className="px-1">
            <h2
              id="payout-heading"
              className="text-[12px] font-semibold tracking-wider text-[#6B7280] uppercase"
            >
              PARA RECIBIR COBROS (CONDUCTOR)
            </h2>
          </div>

          <div className="space-y-3">
            {payoutAccounts.map((account) => (
              <div
                key={account.id}
                className="bg-white rounded-2xl p-4 border border-slate-100 shadow-[0_2px_8px_rgba(0,0,0,0.03)] flex flex-col space-y-3 transition-all"
              >
                <div className="flex items-start justify-between">
                  <div className="flex items-center space-x-3.5">
                    {/* Bank Icon Badge */}
                    <div className="w-11 h-11 rounded-xl bg-[#E6F7F5] flex items-center justify-center text-[#00A896] flex-shrink-0">
                      <Landmark className="w-5 h-5 stroke-[2]" />
                    </div>
                    <div className="space-y-0.5">
                      <h3 className="text-[15px] font-bold text-[#1A1A1A] leading-snug">
                        {account.name}
                      </h3>
                      <p className="text-[13px] text-[#6B7280] font-normal leading-tight">
                        Alias: <span className="font-medium text-slate-700">{account.alias}</span>
                      </p>
                    </div>
                  </div>

                  {/* Principal Badge */}
                  {account.isPrimary && (
                    <span className="inline-flex items-center px-2.5 py-1 rounded-full text-[11px] font-semibold bg-[#E6F7F5] text-[#00A896]">
                      Principal
                    </span>
                  )}
                </div>

                <div className="border-t border-slate-100 pt-2.5 flex items-center justify-between">
                  <span className="text-[12px] text-[#6B7280]">
                    CVU: {account.cvuMasked} • {account.bankName}
                  </span>
                  {account.isVerified && (
                    <span className="inline-flex items-center text-[12px] font-semibold text-[#00A896]">
                      Verificado
                      <Check className="w-3.5 h-3.5 ml-1 stroke-[3]" />
                    </span>
                  )}
                </div>
              </div>
            ))}
          </div>

          {/* Botón: + Agregar CBU/CVU para cobro */}
          <button
            id="add-payout-btn"
            type="button"
            onClick={() => setIsAddPayoutOpen(true)}
            className="w-full flex items-center justify-center py-2 px-3 text-[#00A896] hover:text-[#028090] font-semibold text-[14px] active:opacity-75 transition-opacity cursor-pointer"
          >
            <Plus className="w-4 h-4 mr-1 stroke-[2.5]" />
            <span>Agregar CBU/CVU para cobro</span>
          </button>
        </section>

        {/* SECCIÓN 2: PARA PAGAR RESERVAS (PASAJERO) */}
        <section id="payment-methods-section" aria-labelledby="payment-heading" className="space-y-2.5">
          <div className="px-1">
            <h2
              id="payment-heading"
              className="text-[12px] font-semibold tracking-wider text-[#6B7280] uppercase"
            >
              PARA PAGAR RESERVAS (PASAJERO)
            </h2>
          </div>

          <div className="space-y-3">
            {savedCards.map((card) => (
              <div
                key={card.id}
                onClick={() => setSelectedCardDetail(card)}
                className="bg-white rounded-2xl p-4 border border-slate-100 shadow-[0_2px_8px_rgba(0,0,0,0.03)] flex items-center justify-between active:bg-slate-50 hover:bg-[#F9FBFC] transition-colors cursor-pointer group"
              >
                <div className="flex items-center space-x-3.5">
                  {/* Card Icon Badge */}
                  <div className="w-11 h-11 rounded-xl bg-slate-50 border border-slate-100 flex items-center justify-center text-[#03658C] flex-shrink-0">
                    <CreditCard className="w-5 h-5 stroke-[2]" />
                  </div>
                  <div className="space-y-0.5">
                    <div className="flex items-center space-x-2">
                      <h3 className="text-[15px] font-bold text-[#1A1A1A] leading-snug">
                        {card.type}
                      </h3>
                      <span className="text-[10px] font-bold text-slate-500 bg-slate-100 px-1.5 py-0.5 rounded">
                        {card.brand}
                      </span>
                    </div>
                    <p className="text-[13px] text-[#6B7280] font-normal leading-tight">
                      •••• {card.lastFour}{' '}
                      <span className="text-[11px] text-slate-400 ml-1.5">
                        Vence {card.expiry}
                      </span>
                    </p>
                  </div>
                </div>

                {/* Chevron Right */}
                <div className="w-8 h-8 rounded-full bg-slate-50/80 flex items-center justify-center text-slate-400 group-hover:text-[#00A896] transition-colors">
                  <ChevronRight className="w-4 h-4 stroke-[2.3]" />
                </div>
              </div>
            ))}
          </div>

          {/* Botón: + Agregar tarjeta de débito/crédito */}
          <button
            id="add-card-btn"
            type="button"
            onClick={() => setIsAddCardOpen(true)}
            className="w-full flex items-center justify-center py-2 px-3 text-[#00A896] hover:text-[#028090] font-semibold text-[14px] active:opacity-75 transition-opacity cursor-pointer"
          >
            <Plus className="w-4 h-4 mr-1 stroke-[2.5]" />
            <span>Agregar tarjeta de débito/crédito</span>
          </button>
        </section>

        {/* SECCIÓN 3: BANNER DE SEGURIDAD */}
        <section id="security-banner" className="pt-2 flex flex-col items-center justify-center text-center px-2 space-y-2">
          <div className="inline-flex items-center justify-center space-x-2 text-[12.5px] font-medium text-[#475569] bg-white py-2 px-4 rounded-full border border-slate-200/80 shadow-xs">
            <Lock className="w-3.5 h-3.5 text-[#00A896] stroke-[2.5]" />
            <span>Tus transacciones están protegidas con cifrado bancario</span>
          </div>
          <p className="text-[11px] text-[#6B7280] max-w-xs leading-relaxed">
            Thumbi no almacena los datos sensibles de tus tarjetas. Toda operación se gestiona mediante pasarelas auditadas con protocolo PCI-DSS.
          </p>
        </section>
      </main>

      {/* MODAL 1: Agregar CBU/CVU para Cobro */}
      {isAddPayoutOpen && (
        <div className="fixed inset-0 z-50 bg-black/40 backdrop-blur-xs flex items-end sm:items-center justify-center p-4">
          <div className="bg-white w-full max-w-md rounded-2xl p-5 shadow-xl border border-gray-100 animate-in fade-in slide-in-from-bottom-4 duration-200">
            <div className="flex items-center justify-between pb-3 border-b border-gray-100">
              <div className="flex items-center space-x-2">
                <div className="w-8 h-8 rounded-lg bg-[#E6F7F5] text-[#00A896] flex items-center justify-center">
                  <Landmark className="w-4 h-4" />
                </div>
                <h3 className="text-base font-bold text-[#1A1A1A]">Nueva Cuenta de Cobro</h3>
              </div>
              <button
                type="button"
                onClick={() => setIsAddPayoutOpen(false)}
                className="w-8 h-8 rounded-full text-gray-400 hover:bg-gray-100 flex items-center justify-center cursor-pointer"
              >
                <X className="w-4 h-4" />
              </button>
            </div>

            <form onSubmit={handleAddPayoutSubmit} className="space-y-3.5 mt-4">
              <div>
                <label className="block text-xs font-semibold text-gray-700 mb-1">
                  Entidad o Billetera
                </label>
                <select
                  value={newBank}
                  onChange={(e) => setNewBank(e.target.value)}
                  className="w-full px-3 py-2 text-sm border border-gray-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-[#00A896]/30 focus:border-[#00A896] bg-white"
                >
                  <option value="Mercado Pago">Mercado Pago (CVU)</option>
                  <option value="Banco Santander">Banco Santander</option>
                  <option value="Banco Galicia">Banco Galicia</option>
                  <option value="Banco BBVA">Banco BBVA</option>
                  <option value="Banco Nación">Banco Nación</option>
                  <option value="Brubank">Brubank</option>
                  <option value="Ualá">Ualá</option>
                </select>
              </div>

              <div>
                <label className="block text-xs font-semibold text-gray-700 mb-1">
                  Alias de cobro
                </label>
                <input
                  type="text"
                  required
                  value={newAlias}
                  onChange={(e) => setNewAlias(e.target.value)}
                  placeholder="ej. juan.chofer.mp"
                  className="w-full px-3 py-2 text-sm border border-gray-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-[#00A896]/30 focus:border-[#00A896]"
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-gray-700 mb-1">
                  CBU o CVU (22 dígitos)
                </label>
                <input
                  type="text"
                  required
                  maxLength={22}
                  value={newCbu}
                  onChange={(e) => setNewCbu(e.target.value.replace(/\D/g, ''))}
                  placeholder="0000003100012345678901"
                  className="w-full px-3 py-2 text-sm font-mono border border-gray-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-[#00A896]/30 focus:border-[#00A896]"
                />
              </div>

              <div className="pt-2 flex items-center space-x-2.5">
                <button
                  type="button"
                  onClick={() => setIsAddPayoutOpen(false)}
                  className="w-1/2 py-2.5 text-xs font-semibold text-gray-600 bg-gray-100 hover:bg-gray-200 rounded-xl transition cursor-pointer"
                >
                  Cancelar
                </button>
                <button
                  type="submit"
                  className="w-1/2 py-2.5 text-xs font-semibold text-white bg-[#00A896] hover:bg-[#028090] rounded-xl shadow-sm transition cursor-pointer"
                >
                  Guardar Cuenta
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* MODAL 2: Agregar Tarjeta de Pago */}
      {isAddCardOpen && (
        <div className="fixed inset-0 z-50 bg-black/40 backdrop-blur-xs flex items-end sm:items-center justify-center p-4">
          <div className="bg-white w-full max-w-md rounded-2xl p-5 shadow-xl border border-gray-100 animate-in fade-in slide-in-from-bottom-4 duration-200">
            <div className="flex items-center justify-between pb-3 border-b border-gray-100">
              <div className="flex items-center space-x-2">
                <div className="w-8 h-8 rounded-lg bg-blue-50 text-[#03658C] flex items-center justify-center">
                  <CreditCard className="w-4 h-4" />
                </div>
                <h3 className="text-base font-bold text-[#1A1A1A]">Nueva Tarjeta de Pago</h3>
              </div>
              <button
                type="button"
                onClick={() => setIsAddCardOpen(false)}
                className="w-8 h-8 rounded-full text-gray-400 hover:bg-gray-100 flex items-center justify-center cursor-pointer"
              >
                <X className="w-4 h-4" />
              </button>
            </div>

            <form onSubmit={handleAddCardSubmit} className="space-y-3.5 mt-4">
              <div>
                <label className="block text-xs font-semibold text-gray-700 mb-1">
                  Número de tarjeta
                </label>
                <input
                  type="text"
                  required
                  maxLength={19}
                  value={newCardNumber}
                  onChange={(e) => {
                    const val = e.target.value.replace(/\D/g, '').substring(0, 16);
                    const formatted = val.match(/.{1,4}/g)?.join(' ') || val;
                    setNewCardNumber(formatted);
                  }}
                  placeholder="4500 0000 0000 0000"
                  className="w-full px-3 py-2 text-sm font-mono border border-gray-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-[#00A896]/30 focus:border-[#00A896]"
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-gray-700 mb-1">
                  Nombre y Apellido (como figura en la tarjeta)
                </label>
                <input
                  type="text"
                  required
                  value={newCardHolder}
                  onChange={(e) => setNewCardHolder(e.target.value.toUpperCase())}
                  placeholder="JUAN F LORUSSO"
                  className="w-full px-3 py-2 text-sm uppercase border border-gray-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-[#00A896]/30 focus:border-[#00A896]"
                />
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="block text-xs font-semibold text-gray-700 mb-1">
                    Vencimiento (MM/AA)
                  </label>
                  <input
                    type="text"
                    required
                    maxLength={5}
                    value={newCardExpiry}
                    onChange={(e) => {
                      let val = e.target.value.replace(/\D/g, '').substring(0, 4);
                      if (val.length >= 3) {
                        val = `${val.slice(0, 2)}/${val.slice(2)}`;
                      }
                      setNewCardExpiry(val);
                    }}
                    placeholder="09/27"
                    className="w-full px-3 py-2 text-sm font-mono border border-gray-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-[#00A896]/30 focus:border-[#00A896]"
                  />
                </div>
                <div>
                  <label className="block text-xs font-semibold text-gray-700 mb-1">
                    Cód. Seguridad (CVV)
                  </label>
                  <input
                    type="password"
                    required
                    maxLength={4}
                    value={newCardCvc}
                    onChange={(e) => setNewCardCvc(e.target.value.replace(/\D/g, ''))}
                    placeholder="123"
                    className="w-full px-3 py-2 text-sm font-mono border border-gray-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-[#00A896]/30 focus:border-[#00A896]"
                  />
                </div>
              </div>

              <div className="pt-2 flex items-center space-x-2.5">
                <button
                  type="button"
                  onClick={() => setIsAddCardOpen(false)}
                  className="w-1/2 py-2.5 text-xs font-semibold text-gray-600 bg-gray-100 hover:bg-gray-200 rounded-xl transition cursor-pointer"
                >
                  Cancelar
                </button>
                <button
                  type="submit"
                  className="w-1/2 py-2.5 text-xs font-semibold text-white bg-[#00A896] hover:bg-[#028090] rounded-xl shadow-sm transition cursor-pointer"
                >
                  Guardar Tarjeta
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* MODAL 3: Detalle de Tarjeta */}
      {selectedCardDetail && (
        <div className="fixed inset-0 z-50 bg-black/40 backdrop-blur-xs flex items-end sm:items-center justify-center p-4">
          <div className="bg-white w-full max-w-md rounded-2xl p-5 shadow-xl border border-gray-100 animate-in fade-in slide-in-from-bottom-4 duration-200">
            <div className="flex items-center justify-between pb-3 border-b border-gray-100">
              <div className="flex items-center space-x-2">
                <CreditCard className="w-5 h-5 text-[#00A896]" />
                <h3 className="text-base font-bold text-[#1A1A1A]">Detalle de Tarjeta</h3>
              </div>
              <button
                type="button"
                onClick={() => setSelectedCardDetail(null)}
                className="w-8 h-8 rounded-full text-gray-400 hover:bg-gray-100 flex items-center justify-center cursor-pointer"
              >
                <X className="w-4 h-4" />
              </button>
            </div>

            <div className="py-4 space-y-3 text-sm">
              <div className="p-4 rounded-xl bg-gradient-to-tr from-[#05668D] to-[#028090] text-white space-y-4">
                <div className="flex justify-between items-center">
                  <span className="font-mono text-xs opacity-80">{selectedCardDetail.brand}</span>
                  <span className="text-[10px] bg-white/20 px-2 py-0.5 rounded font-semibold">
                    {selectedCardDetail.isDefault ? 'Predeterminada' : 'Secundaria'}
                  </span>
                </div>
                <p className="text-lg font-mono tracking-widest">
                  •••• •••• •••• {selectedCardDetail.lastFour}
                </p>
                <div className="flex justify-between text-xs opacity-90">
                  <span>{selectedCardDetail.holder}</span>
                  <span>VENCE: {selectedCardDetail.expiry}</span>
                </div>
              </div>

              <div className="p-3 bg-[#E6F7F5] border border-[#BCEEE6] rounded-xl flex items-start space-x-2 text-xs text-[#028090]">
                <ShieldCheck className="w-4 h-4 flex-shrink-0 mt-0.5" />
                <p>
                  Tarjeta habilitada para depósitos Escrow y pagos instantáneos sin cargos extra.
                </p>
              </div>
            </div>

            <div className="pt-2">
              <button
                type="button"
                onClick={() => setSelectedCardDetail(null)}
                className="w-full py-2.5 text-xs font-semibold text-gray-700 bg-gray-100 hover:bg-gray-200 rounded-xl transition cursor-pointer"
              >
                Cerrar
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

export default PaymentMethodsScreen;
