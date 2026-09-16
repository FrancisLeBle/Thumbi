/**
 * Diccionario centralizado de códigos de error de la API y sus mensajes descriptivos en español.
 */
export const ERROR_MESSAGES: Record<string, string> = {
  PRICE_EXCEEDS_CAP_PRICE: 'El precio por asiento supera el límite máximo permitido.',
  INVALID_SEATS_REQUESTED: 'La cantidad de asientos solicitados no es válida o excede la capacidad.',
  SEATS_EXCEED_CAPACITY: 'La cantidad de asientos solicitados supera los asientos disponibles.',
  ESCROW_NOT_FOUND: 'No se encontró la transacción de custodia/pago para este viaje.',
  BOOKING_NOT_FOUND: 'La reserva indicada no existe o fue cancelada.',
  DISPUTE_ALREADY_EXISTS: 'Ya existe una disputa abierta para esta reserva.',
  UNAUTHORIZED: 'Tu sesión ha expirado. Por favor, vuelve a iniciar sesión.',
  INTERNAL_SERVER_ERROR: 'Ocurrió un problema en nuestros servidores. Inténtalo de nuevo.',
};

/**
 * Obtiene el mensaje legible para un código de error de la API.
 * Si el código no está mapeado, retorna el fallback provisto o un mensaje estándar.
 */
export function getErrorMessage(
  code: string,
  fallback: string = 'Ha ocurrido un error inesperado. Por favor, inténtalo nuevamente.'
): string {
  if (!code) {
    return fallback;
  }
  return ERROR_MESSAGES[code] || fallback;
}
