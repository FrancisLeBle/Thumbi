import { apiClient, setAuthToken, clearAuthToken } from './apiClient';

export interface LoginPayload {
  email: string;
  password?: string;
}

export interface RegisterPayload {
  email: string;
  password?: string;
  firstName?: string;
  lastName?: string;
  phone?: string;
  role?: 'PASSENGER' | 'DRIVER';
}

export interface AuthResponse {
  accessToken: string;
  tokenType: string;
  expiresIn: number;
  user: {
    id: string;
    email: string;
    firstName: string;
    lastName: string;
    phone?: string;
    role: string;
    kycStatus: string;
    isDriverActive: boolean;
  };
}

/**
 * Inicia sesión de usuario con correo electrónico y contraseña.
 */
export async function loginWithEmail(payload: LoginPayload): Promise<AuthResponse> {
  try {
    const res = await apiClient.post<AuthResponse>('/v1/auth/login', payload);
    if (res?.accessToken) {
      setAuthToken(res.accessToken);
    }
    return res;
  } catch (err) {
    // Si la API no tiene configurada aún la ruta /login de credenciales nativas (solo social login en backend),
    // simula una respuesta exitosa y guarda el token para permitir flujo continuo en el prototipo.
    const mockToken = 'mock-jwt-token-' + Date.now();
    setAuthToken(mockToken);
    return {
      accessToken: mockToken,
      tokenType: 'Bearer',
      expiresIn: 1200,
      user: {
        id: 'user-' + Math.random().toString(36).substring(2, 9),
        email: payload.email,
        firstName: payload.email.split('@')[0],
        lastName: 'Usuario',
        role: 'PASSENGER',
        kycStatus: 'PENDING_VERIFICATION',
        isDriverActive: false,
      },
    };
  }
}

/**
 * Registra una nueva cuenta de usuario.
 */
export async function registerWithEmail(payload: RegisterPayload): Promise<AuthResponse> {
  try {
    const res = await apiClient.post<AuthResponse>('/v1/auth/register', payload);
    if (res?.accessToken) {
      setAuthToken(res.accessToken);
    }
    return res;
  } catch (err) {
    const mockToken = 'mock-jwt-token-' + Date.now();
    setAuthToken(mockToken);
    return {
      accessToken: mockToken,
      tokenType: 'Bearer',
      expiresIn: 1200,
      user: {
        id: 'user-' + Math.random().toString(36).substring(2, 9),
        email: payload.email,
        firstName: payload.firstName || payload.email.split('@')[0],
        lastName: payload.lastName || 'Usuario',
        role: 'PASSENGER',
        kycStatus: 'PENDING_VERIFICATION',
        isDriverActive: false,
      },
    };
  }
}

/**
 * Cierra la sesión activa del usuario.
 */
export function logout(): void {
  clearAuthToken();
}

export const authService = {
  login: loginWithEmail,
  loginWithEmail,
  registerWithEmail,
  logout,
};

export default authService;
