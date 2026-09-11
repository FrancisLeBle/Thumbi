package whatsapp_test

import (
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thumbi/auth-kyc-service/pkg/whatsapp"
)

func TestCleanPhoneNumber(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "formato internacional con mas, espacios y guiones",
			input:    "+54 9 11 1234-5678",
			expected: "5491112345678",
		},
		{
			name:     "formato internacional sin mas",
			input:    "54 9 11 1234-5678",
			expected: "5491112345678",
		},
		{
			name:     "formato ya limpio",
			input:    "5491112345678",
			expected: "5491112345678",
		},
		{
			name:     "formato con parentesis y puntos",
			input:    "+54 (9) 11.1234.5678",
			expected: "5491112345678",
		},
		{
			name:     "numero local argentino de 10 digitos sin prefijo",
			input:    "11 1234 5678",
			expected: "5491112345678",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := whatsapp.CleanPhoneNumber(tt.input)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestGenerateWhatsAppLink_EncodingWithSpecialCharsAndAccents(t *testing.T) {
	phone := "+54 9 11 1234-5678"
	bookingCode := "BK-9021#A"
	origin := "Constitución (Estación Central)"
	destination := "Córdoba Capital"
	departureTime := "18:30"
	passengerName := "Agustín Peña"

	result := whatsapp.GenerateWhatsAppLink(phone, bookingCode, origin, destination, departureTime, passengerName)

	// 1. Debe comenzar con https://wa.me/ con el teléfono limpio
	expectedPrefix := "https://wa.me/5491112345678?text="
	require.True(t, strings.HasPrefix(result, expectedPrefix), "la URL debe iniciar con el dominio wa.me y el numero limpio")

	// 2. Extraer el parametro text de la URL
	parsedURL, err := url.Parse(result)
	require.NoError(t, err)
	assert.Equal(t, "wa.me", parsedURL.Host)
	assert.Equal(t, "/5491112345678", parsedURL.Path)

	encodedText := parsedURL.Query().Get("text")
	require.NotEmpty(t, encodedText)

	// 3. El texto decodificado debe coincidir exactamente con el mensaje predeterminado
	expectedRawMessage := "¡Hola! Te escribo por el viaje de hoy a las 18:30 de Constitución (Estación Central) a Córdoba Capital (Reserva #BK-9021#A en Thumbi)."
	assert.Equal(t, expectedRawMessage, encodedText, "al decodificar el query param, debe coincidir exactamente con el mensaje con acentos")

	// 4. Verificar presencia de codificaciones porcentuales de caracteres especiales y acentos en la URL cruda
	// ¡ -> %C2%A1
	assert.Contains(t, result, "%C2%A1", "debe contener codificado el signo de apertura de exclamacion ¡")
	// ! -> %21
	assert.Contains(t, result, "%21", "debe contener codificado el signo de exclamacion !")
	// ó -> %C3%B3 (Córdoba / Constitución)
	assert.Contains(t, result, "%C3%B3", "debe contener codificada la letra o con tilde ó")
	// # -> %23
	assert.Contains(t, result, "%23", "debe contener codificado el caracter numeral #")
	// ( -> %28 y ) -> %29
	assert.Contains(t, result, "%28", "debe contener codificado el parentesis abierto (")
	assert.Contains(t, result, "%29", "debe contener codificado el parentesis cerrado )")
}
