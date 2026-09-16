import { ApiErrorResponse } from '../types/api';
import { keysToCamelCase } from '../utils/apiHelpers';
import { ERROR_MESSAGES, getErrorMessage } from '../utils/errorHelpers';

const BASE_URL = '/api';
const TOKEN_KEY = 'thumbi_auth_token';

/**
 * Error estandarizado de la API que incluye código, mensaje procesado y detalles opcionales.
 */
export class ApiClientError extends Error {
  public code: string;
  public status: number;
  public details?: unknown;

  constructor(code: string, message: string, status: number, details?: unknown) {
    super(message);
    this.name = 'ApiClientError';
    this.code = code;
    this.status = status;
    this.details = details;
  }
}

/**
 * Obtiene el token JWT actual desde el almacenamiento local.
 */
export function getAuthToken(): string | null {
  return localStorage.getItem(TOKEN_KEY);
}

/**
 * Guarda o actualiza el token JWT en el almacenamiento local.
 */
export function setAuthToken(token: string): void {
  localStorage.setItem(TOKEN_KEY, token);
}

/**
 * Elimina el token JWT del almacenamiento local.
 */
export function clearAuthToken(): void {
  localStorage.removeItem(TOKEN_KEY);
}

/**
 * Dispara el cierre de sesión y redirección ante un error de autorización (401).
 */
function handleUnauthorized(): void {
  clearAuthToken();
  // Evitar bucles si ya nos encontramos en la pantalla de autenticación
  if (!window.location.pathname.includes('/login')) {
    window.location.href = '/login';
  }
}

export interface RequestOptions extends Omit<RequestInit, 'body'> {
  body?: unknown;
  params?: Record<string, string | number | boolean | undefined | null>;
}

/**
 * Realiza una petición HTTP con gestión centralizada de cabeceras, tokens,
 * transformación snake_case a camelCase y parseo de errores.
 */
async function request<T = unknown>(endpoint: string, options: RequestOptions = {}): Promise<T> {
  const { body, params, headers = {}, ...customConfig } = options;

  let url = endpoint.startsWith('http') ? endpoint : `${BASE_URL}${endpoint.startsWith('/') ? '' : '/'}${endpoint}`;

  if (params) {
    const searchParams = new URLSearchParams();
    for (const [key, value] of Object.entries(params)) {
      if (value !== undefined && value !== null) {
        searchParams.append(key, String(value));
      }
    }
    const queryString = searchParams.toString();
    if (queryString) {
      url += (url.includes('?') ? '&' : '?') + queryString;
    }
  }

  const defaultHeaders: Record<string, string> = {
    'Content-Type': 'application/json',
    Accept: 'application/json',
  };

  const token = getAuthToken();
  if (token) {
    defaultHeaders['Authorization'] = `Bearer ${token}`;
  }

  const config: RequestInit = {
    ...customConfig,
    headers: {
      ...defaultHeaders,
      ...(headers as Record<string, string>),
    },
  };

  if (body !== undefined) {
    config.body = typeof body === 'string' ? body : JSON.stringify(body);
  }

  let response: Response;
  try {
    response = await fetch(url, config);
  } catch (error) {
    throw new ApiClientError(
      'NETWORK_ERROR',
      'Error de conexión. No se pudo conectar con el servidor.',
      0,
      error
    );
  }

  // Manejo de respuestas no exitosas (4xx, 5xx)
  if (!response.ok) {
    let errorData: Partial<ApiErrorResponse> = {};
    try {
      errorData = await response.json();
    } catch {
      // Si la respuesta no es un JSON válido
      errorData = {
        code: response.status === 401 ? 'UNAUTHORIZED' : 'INTERNAL_SERVER_ERROR',
        message: response.statusText,
      };
    }

    if (response.status === 401 || errorData.code === 'UNAUTHORIZED') {
      handleUnauthorized();
    }

    const errorCode = errorData.code || (response.status === 401 ? 'UNAUTHORIZED' : 'UNKNOWN_ERROR');
    const localizedMessage = errorData.message && errorData.code && errorData.code in ERROR_MESSAGES
      ? getErrorMessage(errorData.code)
      : errorData.message || getErrorMessage(errorCode);

    throw new ApiClientError(
      errorCode,
      localizedMessage,
      response.status,
      errorData.details
    );
  }

  // Si la respuesta no tiene contenido (204 No Content)
  if (response.status === 204) {
    return null as unknown as T;
  }

  // Parseo exitoso y transformación snake_case -> camelCase
  const rawData = await response.json();
  return keysToCamelCase<T>(rawData);
}

/**
 * Métodos limpios exportados del cliente API
 */
export const apiClient = {
  get<T = unknown>(endpoint: string, options?: RequestOptions): Promise<T> {
    return request<T>(endpoint, { ...options, method: 'GET' });
  },

  post<T = unknown>(endpoint: string, body?: unknown, options?: RequestOptions): Promise<T> {
    return request<T>(endpoint, { ...options, method: 'POST', body });
  },

  put<T = unknown>(endpoint: string, body?: unknown, options?: RequestOptions): Promise<T> {
    return request<T>(endpoint, { ...options, method: 'PUT', body });
  },

  patch<T = unknown>(endpoint: string, body?: unknown, options?: RequestOptions): Promise<T> {
    return request<T>(endpoint, { ...options, method: 'PATCH', body });
  },

  delete<T = unknown>(endpoint: string, options?: RequestOptions): Promise<T> {
    return request<T>(endpoint, { ...options, method: 'DELETE' });
  },
};

export default apiClient;
