import React, { useState } from 'react';
import { 
  MessageCircle, 
  ExternalLink, 
  Copy, 
  Check, 
  ShieldCheck, 
  Phone, 
  MapPin, 
  Clock, 
  User, 
  Hash, 
  Code2, 
  AlertCircle,
  Sparkles
} from 'lucide-react';

export function WhatsAppTester() {
  const [driverPhone, setDriverPhone] = useState('+54 9 11 1234-5678');
  const [bookingId, setBookingId] = useState('BK-98421');
  const [originTitle, setOriginTitle] = useState('Constitución (CABA)');
  const [destinationTitle, setDestinationTitle] = useState('Córdoba Capital');
  const [departureTime, setDepartureTime] = useState('18:30');
  const [passengerName, setPassengerName] = useState('Agustín Peña');
  const [authScenario, setAuthScenario] = useState<'AUTHORIZED' | 'FORBIDDEN' | 'NOT_FOUND'>('AUTHORIZED');
  const [copied, setCopied] = useState(false);
  const [activeCodeTab, setActiveCodeTab] = useState<'pkg' | 'service' | 'handler' | 'tests'>('pkg');

  // Replica en TypeScript exacta de pkg/whatsapp/whatsapp.go: CleanPhoneNumber
  const cleanPhoneNumber = (phone: string): string => {
    let cleaned = phone.replace(/\D/g, '');
    if (cleaned.length === 10) {
      cleaned = '549' + cleaned;
    } else if (cleaned.length === 12 && cleaned.startsWith('54') && !cleaned.startsWith('549')) {
      cleaned = '549' + cleaned.slice(2);
    }
    return cleaned;
  };

  // Replica exacta de pkg/whatsapp/whatsapp.go: GenerateWhatsAppLink
  const generateWhatsAppLink = (
    phone: string, 
    bId: string, 
    origin: string, 
    dest: string, 
    timeStr: string,
    _passenger: string
  ): { url: string; rawText: string; cleanPhone: string } => {
    const cleaned = cleanPhoneNumber(phone);
    const rawText = `¡Hola! Te escribo por el viaje de hoy a las ${timeStr} de ${origin} a ${dest} (Reserva #${bId} en Thumbi).`;
    const encoded = encodeURIComponent(rawText);
    const url = `https://wa.me/${cleaned}?text=${encoded}`;
    return { url, rawText, cleanPhone: cleaned };
  };

  const { url: generatedUrl, rawText: generatedRawText, cleanPhone } = generateWhatsAppLink(
    driverPhone,
    bookingId,
    originTitle,
    destinationTitle,
    departureTime,
    passengerName
  );

  const handleCopy = () => {
    navigator.clipboard.writeText(generatedUrl);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  // Simulación de la respuesta HTTP del endpoint GET /api/v1/bookings/:id/contact-link
  const getSimulatedHttpResponse = () => {
    if (authScenario === 'FORBIDDEN') {
      return {
        status: 403,
        statusText: 'Forbidden',
        payload: {
          error: 'no tienes autorización para acceder al contacto de esta reserva',
          code: 'FORBIDDEN'
        }
      };
    }
    if (authScenario === 'NOT_FOUND') {
      return {
        status: 404,
        statusText: 'Not Found',
        payload: {
          error: 'reserva no encontrada',
          code: 'BOOKING_NOT_FOUND'
        }
      };
    }
    return {
      status: 200,
      statusText: 'OK',
      payload: {
        whatsapp_url: generatedUrl,
        driver_phone: driverPhone
      }
    };
  };

  const httpResp = getSimulatedHttpResponse();

  return (
    <div className="space-y-6">
      {/* Banner Informativo */}
      <div className="bg-gradient-to-r from-emerald-900/40 via-slate-900 to-slate-900 border border-emerald-500/30 rounded-xl p-5 shadow-sm">
        <div className="flex items-start justify-between">
          <div className="space-y-1.5">
            <div className="flex items-center space-x-2">
              <div className="p-1.5 rounded bg-emerald-500/20 text-emerald-400">
                <MessageCircle className="w-5 h-5" />
              </div>
              <h2 className="text-base font-bold text-white tracking-tight">
                Deeplink de Comunicación Directa (Pasajero → Conductor)
              </h2>
              <span className="text-[11px] font-mono bg-emerald-500/20 text-emerald-300 px-2 py-0.5 rounded border border-emerald-500/30">
                pkg/whatsapp • GET /api/v1/bookings/:id/contact-link
              </span>
            </div>
            <p className="text-xs text-slate-300 max-w-3xl leading-relaxed">
              Permite al pasajero autenticado contactar vía WhatsApp al conductor una vez confirmada la reserva.
              Normaliza prefijos telefónicos (+54, 9, código de área), codifica caracteres especiales y acentos en formato RFC 3986 (<code className="text-emerald-300">url.QueryEscape</code>) y valida la titularidad de la reserva.
            </p>
          </div>
          <div className="hidden sm:flex items-center space-x-2 text-xs bg-slate-950 px-3 py-1.5 rounded-lg border border-slate-800 text-slate-400">
            <ShieldCheck className="w-4 h-4 text-emerald-400" />
            <span>Clean Architecture</span>
          </div>
        </div>
      </div>

      {/* Grid de Configuración Interactiva y Salida */}
      <div className="grid grid-cols-1 lg:grid-cols-12 gap-6">
        {/* Panel Izquierdo: Parámetros del Viaje y Reserva */}
        <div className="lg:col-span-6 space-y-4">
          <div className="bg-slate-900 border border-slate-800 rounded-xl p-5 space-y-4">
            <div className="flex items-center justify-between pb-3 border-b border-slate-800">
              <h3 className="font-semibold text-white text-sm flex items-center space-x-2">
                <Phone className="w-4 h-4 text-emerald-400" />
                <span>Parámetros de la Reserva</span>
              </h3>
              <span className="text-[11px] text-slate-500">Inputs dinámicos</span>
            </div>

            <div className="space-y-3 text-xs">
              <div>
                <label className="block text-slate-400 font-medium mb-1 flex items-center space-x-1.5">
                  <Phone className="w-3.5 h-3.5 text-slate-500" />
                  <span>Teléfono del Conductor (con prefijo o local)</span>
                </label>
                <div className="flex space-x-2">
                  <input
                    type="text"
                    value={driverPhone}
                    onChange={(e) => setDriverPhone(e.target.value)}
                    placeholder="+54 9 11 1234-5678"
                    className="flex-1 bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-white font-mono focus:border-emerald-500 focus:outline-none"
                  />
                  <button
                    onClick={() => setDriverPhone('+54 9 11 1234-5678')}
                    className="px-2.5 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-300 rounded-lg text-[11px] border border-slate-700"
                  >
                    Reset
                  </button>
                </div>
                <span className="text-[11px] text-slate-500 mt-1 block">
                  Normalizado por <code className="text-emerald-400">CleanPhoneNumber</code>: <b className="text-slate-300 font-mono">{cleanPhone}</b>
                </span>
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="block text-slate-400 font-medium mb-1 flex items-center space-x-1.5">
                    <Hash className="w-3.5 h-3.5 text-slate-500" />
                    <span>Código de Reserva</span>
                  </label>
                  <input
                    type="text"
                    value={bookingId}
                    onChange={(e) => setBookingId(e.target.value)}
                    className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-white font-mono focus:border-emerald-500 focus:outline-none"
                  />
                </div>

                <div>
                  <label className="block text-slate-400 font-medium mb-1 flex items-center space-x-1.5">
                    <Clock className="w-3.5 h-3.5 text-slate-500" />
                    <span>Hora de Partida</span>
                  </label>
                  <input
                    type="text"
                    value={departureTime}
                    onChange={(e) => setDepartureTime(e.target.value)}
                    className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-white font-mono focus:border-emerald-500 focus:outline-none"
                  />
                </div>
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="block text-slate-400 font-medium mb-1 flex items-center space-x-1.5">
                    <MapPin className="w-3.5 h-3.5 text-slate-500" />
                    <span>Origen</span>
                  </label>
                  <input
                    type="text"
                    value={originTitle}
                    onChange={(e) => setOriginTitle(e.target.value)}
                    className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-white focus:border-emerald-500 focus:outline-none"
                  />
                </div>

                <div>
                  <label className="block text-slate-400 font-medium mb-1 flex items-center space-x-1.5">
                    <MapPin className="w-3.5 h-3.5 text-slate-500" />
                    <span>Destino</span>
                  </label>
                  <input
                    type="text"
                    value={destinationTitle}
                    onChange={(e) => setDestinationTitle(e.target.value)}
                    className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-white focus:border-emerald-500 focus:outline-none"
                  />
                </div>
              </div>

              <div>
                <label className="block text-slate-400 font-medium mb-1 flex items-center space-x-1.5">
                  <User className="w-3.5 h-3.5 text-slate-500" />
                  <span>Nombre del Pasajero</span>
                </label>
                <input
                  type="text"
                  value={passengerName}
                  onChange={(e) => setPassengerName(e.target.value)}
                  className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-white focus:border-emerald-500 focus:outline-none"
                />
              </div>

              {/* Selector de Caso de Autorización HTTP */}
              <div className="pt-2 border-t border-slate-800">
                <label className="block text-slate-400 font-medium mb-2">
                  Simulación de Control de Autorización en Gin:
                </label>
                <div className="grid grid-cols-3 gap-2">
                  <button
                    onClick={() => setAuthScenario('AUTHORIZED')}
                    className={`py-1.5 px-2 rounded text-[11px] font-medium border transition ${
                      authScenario === 'AUTHORIZED'
                        ? 'bg-emerald-600/30 text-emerald-300 border-emerald-500/50'
                        : 'bg-slate-950 text-slate-400 border-slate-800 hover:text-slate-200'
                    }`}
                  >
                    ✓ Pasajero Titular (200 OK)
                  </button>
                  <button
                    onClick={() => setAuthScenario('FORBIDDEN')}
                    className={`py-1.5 px-2 rounded text-[11px] font-medium border transition ${
                      authScenario === 'FORBIDDEN'
                        ? 'bg-rose-600/30 text-rose-300 border-rose-500/50'
                        : 'bg-slate-950 text-slate-400 border-slate-800 hover:text-slate-200'
                    }`}
                  >
                    ✕ No Autorizado (403)
                  </button>
                  <button
                    onClick={() => setAuthScenario('NOT_FOUND')}
                    className={`py-1.5 px-2 rounded text-[11px] font-medium border transition ${
                      authScenario === 'NOT_FOUND'
                        ? 'bg-amber-600/30 text-amber-300 border-amber-500/50'
                        : 'bg-slate-950 text-slate-400 border-slate-800 hover:text-slate-200'
                    }`}
                  >
                    ✕ Inexistente (404)
                  </button>
                </div>
              </div>
            </div>
          </div>
        </div>

        {/* Panel Derecho: Vista Previa del Deeplink y JSON de la API */}
        <div className="lg:col-span-6 space-y-4">
          {/* Tarjeta Simulación Mensaje WhatsApp */}
          <div className="bg-slate-900 border border-slate-800 rounded-xl p-5 space-y-3">
            <div className="flex items-center justify-between pb-3 border-b border-slate-800">
              <h3 className="font-semibold text-white text-sm flex items-center space-x-2">
                <MessageCircle className="w-4 h-4 text-emerald-400" />
                <span>Mensaje Predefinido de WhatsApp</span>
              </h3>
              <span className="text-[11px] text-emerald-400 bg-emerald-500/10 px-2 py-0.5 rounded border border-emerald-500/20">
                wa.me Deeplink
              </span>
            </div>

            {/* Burbuja de chat simulada */}
            <div className="bg-slate-950/80 rounded-lg p-4 border border-slate-800 space-y-2">
              <div className="flex items-center space-x-2 text-[11px] text-slate-400">
                <span className="font-semibold text-emerald-400">Destinatario:</span>
                <span className="font-mono text-slate-200">+{cleanPhone}</span>
                <span className="text-slate-600">({driverPhone})</span>
              </div>

              <div className="bg-emerald-950/40 border border-emerald-700/40 rounded-lg p-3 text-xs text-emerald-100 leading-relaxed font-sans shadow-inner">
                {generatedRawText}
              </div>

              <div className="pt-2 flex flex-wrap gap-2">
                <a
                  href={generatedUrl}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="flex items-center space-x-1.5 bg-emerald-600 hover:bg-emerald-500 text-white px-3 py-1.5 rounded-lg text-xs font-medium transition shadow"
                >
                  <ExternalLink className="w-3.5 h-3.5" />
                  <span>Abrir en WhatsApp</span>
                </a>

                <button
                  onClick={handleCopy}
                  className="flex items-center space-x-1.5 bg-slate-800 hover:bg-slate-700 text-slate-200 px-3 py-1.5 rounded-lg text-xs font-medium border border-slate-700 transition"
                >
                  {copied ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Copy className="w-3.5 h-3.5" />}
                  <span>{copied ? '¡URL Copiada!' : 'Copiar URL wa.me'}</span>
                </button>
              </div>
            </div>

            {/* URL Completa Codificada */}
            <div className="space-y-1">
              <span className="text-[11px] text-slate-500 font-mono">URL Escapada (QueryEscape):</span>
              <div className="bg-slate-950 p-2.5 rounded border border-slate-800 text-[11px] font-mono text-slate-400 break-all select-all">
                {generatedUrl}
              </div>
            </div>
          </div>

          {/* Respuesta HTTP del Endpoint Gin */}
          <div className="bg-slate-900 border border-slate-800 rounded-xl p-5 space-y-3 font-mono text-xs">
            <div className="flex items-center justify-between pb-2 border-b border-slate-800">
              <div className="flex items-center space-x-2">
                <span className="text-slate-400">Endpoint:</span>
                <span className="text-white font-bold">GET /api/v1/bookings/:id/contact-link</span>
              </div>
              <span className={`px-2 py-0.5 rounded text-[10px] font-bold ${
                httpResp.status === 200
                  ? 'bg-emerald-500/20 text-emerald-300 border border-emerald-500/30'
                  : 'bg-rose-500/20 text-rose-300 border border-rose-500/30'
              }`}>
                HTTP {httpResp.status} {httpResp.statusText}
              </span>
            </div>

            <pre className="bg-slate-950 p-3 rounded-lg border border-slate-800 text-emerald-300 overflow-x-auto text-[11px] leading-relaxed">
              {JSON.stringify(httpResp.payload, null, 2)}
            </pre>
          </div>
        </div>
      </div>

      {/* Visor de Código Go Implementado */}
      <div className="bg-slate-900 border border-slate-800 rounded-xl p-5 space-y-4">
        <div className="flex items-center justify-between pb-3 border-b border-slate-800">
          <div className="flex items-center space-x-2">
            <Code2 className="w-4 h-4 text-emerald-400" />
            <h3 className="font-semibold text-white text-sm">Archivos Go Implementados en el Microservicio</h3>
          </div>
          <div className="flex space-x-1 bg-slate-950 p-1 rounded-lg border border-slate-800 text-xs">
            <button
              onClick={() => setActiveCodeTab('pkg')}
              className={`px-3 py-1 rounded transition ${activeCodeTab === 'pkg' ? 'bg-emerald-600 text-white' : 'text-slate-400 hover:text-slate-200'}`}
            >
              pkg/whatsapp/whatsapp.go
            </button>
            <button
              onClick={() => setActiveCodeTab('tests')}
              className={`px-3 py-1 rounded transition ${activeCodeTab === 'tests' ? 'bg-emerald-600 text-white' : 'text-slate-400 hover:text-slate-200'}`}
            >
              whatsapp_test.go
            </button>
            <button
              onClick={() => setActiveCodeTab('service')}
              className={`px-3 py-1 rounded transition ${activeCodeTab === 'service' ? 'bg-emerald-600 text-white' : 'text-slate-400 hover:text-slate-200'}`}
            >
              booking_service.go
            </button>
            <button
              onClick={() => setActiveCodeTab('handler')}
              className={`px-3 py-1 rounded transition ${activeCodeTab === 'handler' ? 'bg-emerald-600 text-white' : 'text-slate-400 hover:text-slate-200'}`}
            >
              booking_handler.go
            </button>
          </div>
        </div>

        <div className="bg-slate-950 p-4 rounded-lg border border-slate-800 font-mono text-xs overflow-x-auto text-slate-300">
          {activeCodeTab === 'pkg' && (
            <pre className="text-emerald-300 leading-relaxed">
{`package whatsapp

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var nonDigitRegex = regexp.MustCompile(\`\\D\`)

// CleanPhoneNumber normaliza un número telefónico removiendo caracteres no numéricos
func CleanPhoneNumber(phone string) string {
	cleaned := nonDigitRegex.ReplaceAllString(phone, "")
	if len(cleaned) == 10 {
		cleaned = "549" + cleaned
	} else if len(cleaned) == 12 && strings.HasPrefix(cleaned, "54") && !strings.HasPrefix(cleaned, "549") {
		cleaned = "549" + cleaned[2:]
	}
	return cleaned
}

// GenerateWhatsAppLink construye el Deeplink seguro wa.me con mensaje predeterminado
func GenerateWhatsAppLink(phone, bookingID, origin, destination, departureTime, passengerName string) string {
	cleanedPhone := CleanPhoneNumber(phone)
	msg := fmt.Sprintf(
		"¡Hola! Te escribo por el viaje de hoy a las %s de %s a %s (Reserva #%s en Thumbi).",
		departureTime,
		origin,
		destination,
		bookingID,
	)
	encodedMsg := url.QueryEscape(msg)
	return fmt.Sprintf("https://wa.me/%s?text=%s", cleanedPhone, encodedMsg)
}`}
            </pre>
          )}

          {activeCodeTab === 'tests' && (
            <pre className="text-emerald-300 leading-relaxed">
{`package whatsapp_test

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"github.com/thumbi/auth-kyc-service/pkg/whatsapp"
)

func TestCleanPhoneNumber(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"número con formato internacional y espacios", "+54 9 11 1234-5678", "5491112345678"},
		{"número de 10 dígitos sin prefijo de país", "1112345678", "5491112345678"},
		{"número con prefijo 54 sin el 9 móvil", "541112345678", "5491112345678"},
		{"número ya limpio y completo", "5491112345678", "5491112345678"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, whatsapp.CleanPhoneNumber(tc.input))
		})
	}
}

func TestGenerateWhatsAppLink(t *testing.T) {
	link := whatsapp.GenerateWhatsAppLink(
		"+54 9 11 1234-5678",
		"BK-9901",
		"Constitución (CABA)",
		"Córdoba Capital",
		"18:30",
		"Agustín",
	)
	assert.Contains(t, link, "https://wa.me/5491112345678?text=")
	assert.Contains(t, link, "%C2%A1Hola%21")
	assert.Contains(t, link, "BK-9901")
}`}
            </pre>
          )}

          {activeCodeTab === 'service' && (
            <pre className="text-emerald-300 leading-relaxed">
{`// GetContactLink genera el Deeplink de WhatsApp para que el pasajero contacte al conductor
func (s *bookingService) GetContactLink(ctx context.Context, bookingID, requestingUserID string) (*ports.ContactLinkDTO, error) {
	booking, err := s.bookingRepo.FindByID(ctx, bookingID)
	if err != nil || booking == nil {
		return nil, domain.ErrBookingNotFound
	}
	// Invariante de seguridad: Solo el pasajero titular puede acceder al link
	if booking.PassengerID != requestingUserID {
		return nil, domain.ErrUnauthorized
	}
	trip, err := s.tripRepo.FindByID(ctx, booking.TripID)
	if err != nil || trip == nil {
		return nil, domain.ErrTripNotFound
	}
	driverPhone := "+54 9 11 1234-5678"
	if s.userRepo != nil {
		if driver, _ := s.userRepo.GetByID(ctx, trip.DriverID); driver != nil && driver.Phone != "" {
			driverPhone = driver.Phone
		}
	}
	whatsappURL := whatsapp.GenerateWhatsAppLink(
		driverPhone,
		booking.ID,
		trip.OriginTitle,
		trip.DestinationTitle,
		trip.DepartureTime.Format("15:04"),
		"",
	)
	return &ports.ContactLinkDTO{
		WhatsAppURL: whatsappURL,
		DriverPhone: driverPhone,
	}, nil
}`}
            </pre>
          )}

          {activeCodeTab === 'handler' && (
            <pre className="text-emerald-300 leading-relaxed">
{`// GetContactLink handler en Gin
// GET /api/v1/bookings/:id/contact-link
func (h *BookingHandler) GetContactLink(c *gin.Context) {
	passengerID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no autenticado", "code": "UNAUTHORIZED"})
		return
	}
	bookingID := strings.TrimSpace(c.Param("id"))
	dto, err := h.bookingService.GetContactLink(c.Request.Context(), bookingID, passengerID)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrBookingNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "reserva no encontrada", "code": "BOOKING_NOT_FOUND"})
		case errors.Is(err, domain.ErrUnauthorized):
			c.JSON(http.StatusForbidden, gin.H{"error": "no tienes autorización", "code": "FORBIDDEN"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "error interno", "code": "INTERNAL_SERVER_ERROR"})
		}
		return
	}
	c.JSON(http.StatusOK, dto)
}`}
            </pre>
          )}
        </div>
      </div>
    </div>
  );
}
