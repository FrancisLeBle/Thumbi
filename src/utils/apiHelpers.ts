/**
 * Convierte una cadena de snake_case a camelCase.
 */
function snakeToCamel(str: string): string {
  return str.replace(/_([a-z0-9])/g, (_, letter) => letter.toUpperCase());
}

/**
 * Función recursiva que transforma todas las claves de un objeto o arreglo
 * de snake_case a camelCase.
 */
export function keysToCamelCase<T = unknown>(obj: unknown): T {
  if (obj === null || typeof obj !== 'object') {
    return obj as T;
  }

  if (Array.isArray(obj)) {
    return obj.map((item) => keysToCamelCase(item)) as unknown as T;
  }

  const result: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(obj as Record<string, unknown>)) {
    const camelKey = snakeToCamel(key);
    result[camelKey] = keysToCamelCase(value);
  }

  return result as T;
}

/**
 * Convierte fechas ISO 8601 a un formato legible en español.
 * Ejemplo: '16 de sep, 14:16 hs'
 */
export function formatDateISOToLocal(isoString: string): string {
  if (!isoString) {
    return '';
  }

  const date = new Date(isoString);
  if (isNaN(date.getTime())) {
    return isoString;
  }

  const day = date.getDate();
  const months = ['ene', 'feb', 'mar', 'abr', 'may', 'jun', 'jul', 'ago', 'sep', 'oct', 'nov', 'dic'];
  const month = months[date.getMonth()];
  const hours = date.getHours().toString().padStart(2, '0');
  const minutes = date.getMinutes().toString().padStart(2, '0');

  return `${day} de ${month}, ${hours}:${minutes} hs`;
}

/**
 * Formatea montos numéricos a moneda local/USD.
 * Ejemplo: '$50.00 USD'
 */
export function formatCurrency(amount: number, currency: string = 'USD'): string {
  const formatted = new Intl.NumberFormat('en-US', {
    style: 'currency',
    currency: currency,
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(amount);

  return `${formatted} ${currency.toUpperCase()}`;
}
