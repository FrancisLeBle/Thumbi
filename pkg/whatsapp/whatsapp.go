package whatsapp

import (
	"fmt"
	"net/url"
	"strings"
	"unicode"
)

// CleanPhoneNumber limpia y formatea el número telefónico para WhatsApp.
// Remueve cualquier carácter que no sea dígito (+, espacios, guiones, paréntesis)
// y asegura que incluya el código de país sin signo + ni separadores.
func CleanPhoneNumber(phone string) string {
	var sb strings.Builder
	for _, r := range phone {
		if unicode.IsDigit(r) {
			sb.WriteRune(r)
		}
	}
	cleaned := sb.String()

	// Remover ceros iniciales de prefijos locales
	cleaned = strings.TrimLeft(cleaned, "0")

	// Si es un número local de 10 dígitos (ej: 11XXXXXXXX en Argentina sin código de país 549),
	// se antepone el código de país para cumplir la especificación de wa.me
	if len(cleaned) == 10 {
		cleaned = "549" + cleaned
	}

	return cleaned
}

// GenerateWhatsAppLink construye el Deeplink de WhatsApp (wa.me) para contactar al conductor.
// Formatea el teléfono sin signo + ni separadores, arma el mensaje predeterminado y aplica url.QueryEscape.
func GenerateWhatsAppLink(phone string, bookingCode string, origin string, destination string, departureTime string, passengerName string) string {
	cleanPhone := CleanPhoneNumber(phone)
	msg := fmt.Sprintf("¡Hola! Te escribo por el viaje de hoy a las %s de %s a %s (Reserva #%s en Thumbi).", departureTime, origin, destination, bookingCode)
	encodedText := url.QueryEscape(msg)
	return fmt.Sprintf("https://wa.me/%s?text=%s", cleanPhone, encodedText)
}
