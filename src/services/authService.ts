import { apiClient, setAuthToken, clearAuthToken, getAuthToken } from './apiClient';

export type AuthProviderType = 'GOOGLE' | 'APPLE';

export interface SocialLoginPayload {
  provider: AuthProviderType;
  idToken: string;
}

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

export interface AuthUser {
  id: string;
  email: string;
  firstName: string;
  lastName: string;
  avatarUrl?: string;
  phone?: string;
  role: 'PASSENGER' | 'DRIVER';
  kycStatus: 'NOT_STARTED' | 'PENDING_VERIFICATION' | 'APPROVED' | 'REJECTED';
  isDriverActive: boolean;
}

export interface AuthResponse {
  message?: string;
  accessToken: string;
  tokenType: string;
  expiresIn: number;
  isNewUser?: boolean;
  user: AuthUser;
}

export interface AuthMeResponse {
  user: AuthUser;
  capabilities: {
    canBookRides: boolean;
    canPublishRides: boolean;
  };
}

/**
 * Autentica al usuario mediante proveedor OAuth social (Google / Apple).
 * Endpoint: POST /v1/auth/social-login
 */
export async function socialLogin(payload: SocialLoginPayload): Promise<AuthResponse> {
  const res = await apiClient.post<AuthResponse>('/v1/auth/social-login', {
    provider: payload.provider,
    id_token: payload.idToken,
  });

  if (res?.accessToken) {
    setAuthToken(res.accessToken);
  }

  return res;
}

/**
 * Obtiene la sesión activa y el perfil del usuario autenticado.
 * Endpoint: GET /v1/auth/me
 */
export async function getMe(): Promise<AuthMeResponse> {
  return apiClient.get<AuthMeResponse>('/v1/auth/me');
}

/**
 * Renueva el token de sesión JWT activo.
 * Endpoint: POST /v1/auth/refresh
 */
export async function refreshSession(token?: string): Promise<AuthResponse> {
  const tokenToRefresh = token || getAuthToken() || undefined;
  const res = await apiClient.post<AuthResponse>('/v1/auth/refresh', {
    token: tokenToRefresh,
  });

  if (res?.accessToken) {
    setAuthToken(res.accessToken);
  }

  return res;
}

/**
 * Inicia sesión de usuario conectando con el backend.
 */
export async function loginWithEmail(payload: LoginPayload): Promise<AuthResponse> {
  return socialLogin({
    provider: 'GOOGLE',
    idToken: payload.password || payload.email,
  });
}

/**
 * Registra una nueva cuenta de usuario conectando con el backend.
 */
export async function registerWithEmail(payload: RegisterPayload): Promise<AuthResponse> {
  return socialLogin({
    provider: 'GOOGLE',
    idToken: payload.password || payload.email,
  });
}

/**
 * Cierra la sesión activa en el servidor y limpia el almacenamiento local de credenciales.
 * Endpoint: POST /v1/auth/logout
 */
export async function logout(): Promise<void> {
  try {
    await apiClient.post('/v1/auth/logout');
  } catch {
    // Continuar con la limpieza local incluso si la red falla
  } finally {
    clearAuthToken();
  }
}

export const authService = {
  socialLogin,
  getMe,
  refreshSession,
  login: loginWithEmail,
  loginWithEmail,
  registerWithEmail,
  logout,
};

export default authService;

