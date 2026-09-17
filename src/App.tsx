import React, { useState, useEffect } from 'react';
import { 
  ShieldCheck, 
  Key, 
  UserCheck, 
  Car, 
  Clock, 
  AlertTriangle, 
  CheckCircle2, 
  XCircle, 
  Code2, 
  Layers, 
  FileText, 
  RefreshCw, 
  Terminal, 
  Smartphone,
  ChevronRight,
  Database,
  ArrowRight,
  Activity,
  Server,
  GitBranch,
  Cpu,
  Check,
  MessageCircle
} from 'lucide-react';
import { WhatsAppTester } from './components/WhatsAppTester';
import { WelcomeScreen } from './screens/WelcomeScreen';
import { LoginScreen } from './screens/LoginScreen';
import { RegisterScreen } from './screens/RegisterScreen';
import { IdentityVerificationScreen } from './screens/IdentityVerificationScreen';
import { HomeScreen, SearchQueryParams } from './screens/HomeScreen';
import { SearchResultsScreen } from './screens/SearchResultsScreen';
import { BookingScreen } from './screens/BookingScreen';
import { PublishTripScreen } from './screens/PublishTripScreen';
import { MyTripsScreen } from './screens/MyTripsScreen';
import { ProfileScreen } from './screens/ProfileScreen';
import { HelpSupportScreen } from './screens/HelpSupportScreen';
import { SecurityPrivacyScreen } from './screens/SecurityPrivacyScreen';
import { PaymentMethodsScreen } from './screens/PaymentMethodsScreen';
import { NotificationsSettingsScreen } from './screens/NotificationsSettingsScreen';
import { AddVehicleScreen } from './screens/AddVehicleScreen';
import { BottomNav, BottomNavTab } from './components/BottomNav';
import { TripSearchResult } from './services/tripService';
import { Trip, Booking } from './types/api';

interface SimulatedUser {
  id: string;
  email: string;
  firstName: string;
  lastName: string;
  role: 'PASSENGER' | 'DRIVER';
  kycStatus: 'NOT_STARTED' | 'PENDING_VERIFICATION' | 'APPROVED' | 'REJECTED' | 'PENDING_MANUAL_REVIEW';
  isDriverActive: boolean;
  canBookRides: boolean;
  canPublishRides: boolean;
}

interface SimulatedSession {
  token: string;
  expiresInSeconds: number;
  initialSeconds: number;
  provider: 'GOOGLE' | 'APPLE';
}

interface SimulatedVehicle {
  id: string;
  brand: string;
  model: string;
  plate: string;
  status: 'PENDING_VERIFICATION' | 'APPROVED' | 'REJECTED';
  rejectionReason?: string;
}

export type AppView =
  | 'home'
  | 'welcome'
  | 'login'
  | 'register'
  | 'identity-verification'
  | 'search-results'
  | 'booking'
  | 'publish'
  | 'trips'
  | 'profile'
  | 'help-support'
  | 'security-privacy'
  | 'payment-methods'
  | 'notifications-settings'
  | 'add-vehicle';

export default function App() {
  const [activeTab, setActiveTab] = useState<'app' | 'simulator' | 'whatsapp' | 'architecture' | 'api-docs' | 'deployment'>('app');
  const [currentAppView, setCurrentAppView] = useState<AppView>('home');
  const [searchParams, setSearchParams] = useState<SearchQueryParams>({
    origin: 'Palermo',
    destination: 'Pilar',
    date: '2026-09-17',
    seats: 1,
  });
  const [selectedTrip, setSelectedTrip] = useState<Trip | null>(null);
  const [session, setSession] = useState<SimulatedSession | null>(null);
  const [user, setUser] = useState<SimulatedUser | null>(null);
  const [kycAttempts, setKycAttempts] = useState(0);
  const [vehicle, setVehicle] = useState<SimulatedVehicle | null>(null);
  const [isManagingCreatedTrip, setIsManagingCreatedTrip] = useState<boolean>(false);
  const [logs, setLogs] = useState<Array<{ time: string; type: 'auth' | 'kyc' | 'vehicle' | 'info'; message: string }>>([]);

  // Estados para simulación de Módulo 5 (Observabilidad / Health & Ready)
  const [dbHealth, setDbHealth] = useState<'UP' | 'DOWN'>('UP');
  const [postgisHealth, setPostgisHealth] = useState<'UP' | 'DOWN'>('UP');
  const [probeResult, setProbeResult] = useState<{
    endpoint: string;
    status: number;
    statusText: string;
    body: any;
    durationMs: number;
  } | null>(null);

  const simulateProbe = (endpoint: '/health' | '/ready') => {
    const isHealthy = dbHealth === 'UP';
    let statusCode = 200;
    let statusText = 'OK';
    let body: any = {};

    if (endpoint === '/health') {
      statusCode = isHealthy ? 200 : 503;
      statusText = isHealthy ? 'OK' : 'SERVICE UNAVAILABLE';
      body = {
        status: isHealthy ? 'UP' : 'DOWN',
        service: 'thumbi-api',
        version: '1.0.0',
        timestamp: new Date().toISOString(),
        uptime: '1h 14m 22s',
        checks: {
          database: {
            status: dbHealth,
            ...(isHealthy ? { latency_ms: 1.15 } : { error: 'connection refused to postgres:5432' }),
          },
        },
        metrics: {
          goroutines: 16,
          memory_alloc_mb: '6.40 MB',
          uptime_seconds: 4462,
          db_pool: {
            total_conns: isHealthy ? 5 : 0,
            idle_conns: isHealthy ? 4 : 0,
            acquired_conns: isHealthy ? 1 : 0,
            max_conns: 20,
          },
        },
      };
    } else {
      const isDbOk = dbHealth === 'UP';
      const isGisOk = isDbOk && postgisHealth === 'UP';
      const readyOk = isDbOk && isGisOk;
      statusCode = readyOk ? 200 : 503;
      statusText = readyOk ? 'OK' : 'SERVICE UNAVAILABLE';
      body = {
        status: readyOk ? 'UP' : 'DOWN',
        ready: readyOk,
        service: 'thumbi-api',
        version: '1.0.0',
        timestamp: new Date().toISOString(),
        uptime: '1h 14m 22s',
        checks: {
          database: {
            status: dbHealth,
            ...(isDbOk ? { latency_ms: 1.15 } : { error: 'connection refused to postgres:5432' }),
          },
          postgis: {
            status: isGisOk ? 'UP' : 'DOWN',
            ...(isGisOk
              ? { version: '3.4.2', latency_ms: 1.8 }
              : { error: 'PostGIS extension missing or unavailable: extension "postgis" does not exist' }),
          },
        },
        metrics: {
          goroutines: 16,
          memory_alloc_mb: '6.40 MB',
          uptime_seconds: 4462,
          db_pool: {
            total_conns: isDbOk ? 5 : 0,
            idle_conns: isDbOk ? 4 : 0,
            acquired_conns: isDbOk ? 1 : 0,
            max_conns: 20,
          },
        },
      };
    }

    setProbeResult({
      endpoint,
      status: statusCode,
      statusText,
      body,
      durationMs: 1.8,
    });
    addLog('info', `📡 Probe ${endpoint} -> HTTP ${statusCode} ${statusText} (PostgreSQL: ${dbHealth}, PostGIS: ${postgisHealth})`);
  };

  // Timer para simular el TTL de 20 minutos (RF-04, RF-05, RF-06)
  useEffect(() => {
    if (!session) return;
    const interval = setInterval(() => {
      setSession(prev => {
        if (!prev) return null;
        if (prev.expiresInSeconds <= 1) {
          addLog('auth', '⚠️ Sesión expirada por límite de 20 minutos (RF-06). Redirigiendo a login.');
          setUser(null);
          return null;
        }
        return { ...prev, expiresInSeconds: prev.expiresInSeconds - 1 };
      });
    }, 1000);
    return () => clearInterval(interval);
  }, [session]);

  const addLog = (type: 'auth' | 'kyc' | 'vehicle' | 'info', message: string) => {
    const now = new Date().toLocaleTimeString('es-AR', { hour12: false });
    setLogs(prev => [{ time: now, type, message }, ...prev.slice(0, 19)]);
  };

  // 1. Login Social
  const handleSocialLogin = (provider: 'GOOGLE' | 'APPLE') => {
    const newUser: SimulatedUser = {
      id: 'usr-' + Math.random().toString(36).substring(2, 8),
      email: provider === 'GOOGLE' ? 'juan.perez@gmail.com' : 'juan.perez@privaterelay.appleid.com',
      firstName: 'Juan',
      lastName: 'Pérez',
      role: 'PASSENGER', // RF-02: Perfil base de Pasajero por defecto
      kycStatus: 'NOT_STARTED',
      isDriverActive: false,
      canBookRides: true, // RF-13
      canPublishRides: false,
    };

    // RF-04: TTL de exactamente 20 minutos (1200 segundos)
    setSession({
      token: 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoi' + newUser.id + '...[TTL: 20m]',
      expiresInSeconds: 1200,
      initialSeconds: 1200,
      provider,
    });
    setUser(newUser);
    setKycAttempts(0);
    setVehicle(null);
    addLog('auth', `✅ Autenticado con ${provider}. Perfil asignado: PASSENGER. Token emitido (TTL 20 min).`);
  };

  // 2. Extender / Refrescar Sesión (RF-05)
  const handleRefreshToken = () => {
    if (!session) return;
    setSession(prev => prev ? { ...prev, expiresInSeconds: 1200 } : null);
    addLog('auth', '🔄 Sesión renovada mediante /v1/auth/refresh. TTL reiniciado a 20 minutos (1200s).');
  };

  // 3. Prueba Biométrica KYC (RF-07, RF-08, RF-09)
  const handleKYCAttempt = (forceFail: boolean) => {
    if (!user) return;
    if (kycAttempts >= 3 || user.kycStatus === 'PENDING_MANUAL_REVIEW') {
      addLog('kyc', '⛔ Solicitud bloqueada en PENDING_MANUAL_REVIEW por superar 3 reintentos.');
      return;
    }

    if (forceFail) {
      const nextAttempt = kycAttempts + 1;
      setKycAttempts(nextAttempt);
      if (nextAttempt >= 3) {
        setUser({ ...user, kycStatus: 'PENDING_MANUAL_REVIEW' });
        addLog('kyc', '❌ Intento 3/3 fallido. RF-09: Estado derivado a PENDING_MANUAL_REVIEW para mesa de control.');
      } else {
        setUser({ ...user, kycStatus: 'REJECTED' });
        addLog('kyc', `⚠️ Intento ${nextAttempt}/3 fallido (Liveness score < 0.85). Reintentos disponibles: ${3 - nextAttempt}.`);
      }
    } else {
      setUser({ ...user, kycStatus: 'APPROVED' });
      addLog('kyc', '🎉 Validación biométrica y DNI aprobados (Score 0.94). KYC Status: APPROVED.');
    }
  };

  // 4. Registrar Vehículo (RF-10, RF-11)
  const handleRegisterVehicle = () => {
    if (!user) return;
    if (user.kycStatus !== 'APPROVED') {
      addLog('vehicle', '🚫 Error: Se requiere KYC aprobado previo para registrar un vehículo.');
      return;
    }

    const newVeh: SimulatedVehicle = {
      id: 'veh-' + Math.random().toString(36).substring(2, 6),
      brand: 'Toyota',
      model: 'Corolla 2022',
      plate: 'AF-321-CD',
      status: 'PENDING_VERIFICATION', // RF-11
    };
    setVehicle(newVeh);
    addLog('vehicle', '🚗 Vehículo registrado en estado PENDING_VERIFICATION. Documentación en trámite asíncrono.');
  };

  // 5. Resolución Asíncrona (RF-12, RF-13)
  const handleVehicleResolution = (approve: boolean) => {
    if (!vehicle || !user) return;
    if (approve) {
      setVehicle({ ...vehicle, status: 'APPROVED' });
      setUser({
        ...user,
        role: 'DRIVER',
        isDriverActive: true,
        canPublishRides: true,
      });
      addLog('vehicle', '✅ Trámite aprobado: Rol promovido a DRIVER. Conductor habilitado para publicar viajes.');
    } else {
      setVehicle({
        ...vehicle,
        status: 'REJECTED',
        rejectionReason: 'Cédula del automotor ilegible',
      });
      setUser({
        ...user,
        role: 'PASSENGER',
        isDriverActive: false,
        canBookRides: true, // RF-13
        canPublishRides: false,
      });
      addLog('vehicle', '⚠️ Trámite rechazado. RF-13: Retiene 100% de capacidad como PASAJERO para buscar y reservar.');
    }
  };

  const formatTimer = (seconds: number) => {
    const mins = Math.floor(seconds / 60);
    const secs = seconds % 60;
    return `${mins.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`;
  };

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100 flex flex-col font-sans">
      {/* Header Superior */}
      <header className="border-b border-slate-800 bg-slate-900/70 backdrop-blur px-6 py-4 flex items-center justify-between sticky top-0 z-50">
        <div className="flex items-center space-x-3">
          <div className="w-10 h-10 rounded-xl bg-gradient-to-tr from-emerald-600 to-teal-400 flex items-center justify-center shadow-lg shadow-emerald-500/20">
            <ShieldCheck className="w-6 h-6 text-white" />
          </div>
          <div>
            <div className="flex items-center space-x-2">
              <h1 className="text-xl font-bold tracking-tight text-white">Thumbi Core</h1>
              <span className="text-xs bg-emerald-500/20 text-emerald-300 font-semibold px-2 py-0.5 rounded border border-emerald-500/30">
                Módulos 1 al 5: Core & DevOps
              </span>
            </div>
            <p className="text-xs text-slate-400">Microservicio en Go • Clean Architecture • Docker, PostGIS & CI/CD</p>
          </div>
        </div>

        {/* Pestañas de Navegación */}
        <div className="flex bg-slate-800/80 p-1 rounded-lg border border-slate-700/60 text-sm overflow-x-auto">
          <button
            onClick={() => setActiveTab('app')}
            className={`px-3 py-1.5 rounded-md font-medium transition flex items-center space-x-1.5 whitespace-nowrap cursor-pointer ${
              activeTab === 'app'
                ? 'bg-[#00A896] text-white shadow-sm'
                : 'text-slate-400 hover:text-slate-200'
            }`}
          >
            <Smartphone className="w-4 h-4" />
            <span>📱 App Stitch (Flujo Completo)</span>
          </button>
          <button
            onClick={() => setActiveTab('simulator')}
            className={`px-3 py-1.5 rounded-md font-medium transition whitespace-nowrap cursor-pointer ${
              activeTab === 'simulator'
                ? 'bg-emerald-600 text-white shadow'
                : 'text-slate-400 hover:text-slate-200'
            }`}
          >
            Simulador de Casos de Uso
          </button>
          <button
            onClick={() => setActiveTab('whatsapp')}
            className={`px-3 py-1.5 rounded-md font-medium transition flex items-center space-x-1.5 whitespace-nowrap cursor-pointer ${
              activeTab === 'whatsapp'
                ? 'bg-emerald-600 text-white shadow'
                : 'text-slate-400 hover:text-slate-200'
            }`}
          >
            <MessageCircle className="w-4 h-4" />
            <span>WhatsApp Deeplink</span>
          </button>
          <button
            onClick={() => setActiveTab('architecture')}
            className={`px-3 py-1.5 rounded-md font-medium transition whitespace-nowrap cursor-pointer ${
              activeTab === 'architecture'
                ? 'bg-emerald-600 text-white shadow'
                : 'text-slate-400 hover:text-slate-200'
            }`}
          >
            Clean Architecture (Go)
          </button>
          <button
            onClick={() => setActiveTab('api-docs')}
            className={`px-3 py-1.5 rounded-md font-medium transition whitespace-nowrap cursor-pointer ${
              activeTab === 'api-docs'
                ? 'bg-emerald-600 text-white shadow'
                : 'text-slate-400 hover:text-slate-200'
            }`}
          >
            Contratos REST / Gin
          </button>
          <button
            onClick={() => setActiveTab('deployment')}
            className={`px-3 py-1.5 rounded-md font-medium transition whitespace-nowrap cursor-pointer ${
              activeTab === 'deployment'
                ? 'bg-emerald-600 text-white shadow'
                : 'text-slate-400 hover:text-slate-200'
            }`}
          >
            Módulo 5: Docker, Health & CI/CD
          </button>
        </div>
      </header>

      {/* Contenido Principal */}
      <main className="flex-1 max-w-7xl w-full mx-auto p-4 sm:p-6 space-y-6">
        {activeTab === 'app' && (
          <div className="w-full space-y-4">
            {/* Barra de accesos directos al ciclo de vida de Stitch */}
            <div className="bg-slate-900/90 border border-slate-800 rounded-xl p-3 flex flex-wrap items-center justify-between gap-3 text-xs">
              <div className="flex items-center gap-2">
                <span className="font-semibold text-white">Navegación del Flujo:</span>
                <span className="text-[#00A896] bg-[#00A896]/10 px-2 py-0.5 rounded font-mono font-bold uppercase tracking-wider">
                  {currentAppView}
                </span>
              </div>
              <div className="flex flex-wrap items-center gap-1.5">
                {[
                  { view: 'home' as const, label: '1. Inicio (Buscar)' },
                  { view: 'welcome' as const, label: '2. Bienvenida' },
                  { view: 'login' as const, label: '3. Iniciar Sesión' },
                  { view: 'register' as const, label: '4. Registro' },
                  { view: 'identity-verification' as const, label: '5. Validar DNI' },
                  { view: 'search-results' as const, label: '6. Resultados' },
                  { view: 'booking' as const, label: '7. Reservar' },
                  { view: 'publish' as const, label: '8. Publicar Viaje' },
                  { view: 'trips' as const, label: '9. Mis Viajes' },
                  { view: 'profile' as const, label: '10. Perfil' },
                  { view: 'help-support' as const, label: '11. Ayuda y Soporte' },
                  { view: 'security-privacy' as const, label: '12. Seguridad y Privacidad' },
                  { view: 'payment-methods' as const, label: '13. Métodos de Pago' },
                  { view: 'notifications-settings' as const, label: '14. Notificaciones' },
                  { view: 'add-vehicle' as const, label: '15. Agregar Vehículo' },
                ].map((item) => (
                  <button
                    key={item.view}
                    type="button"
                    onClick={() => setCurrentAppView(item.view)}
                    className={`px-2.5 py-1 rounded-md font-medium transition cursor-pointer ${
                      currentAppView === item.view
                        ? 'bg-[#00A896] text-white shadow-sm font-semibold'
                        : 'bg-slate-800/80 text-slate-300 hover:bg-slate-700 hover:text-white'
                    }`}
                  >
                    {item.label}
                  </button>
                ))}
              </div>
            </div>

            {/* Renderizado de Pantalla Activa según Router */}
            <div className="w-full">
              {currentAppView === 'welcome' && (
                <WelcomeScreen
                  onLoginClick={() => setCurrentAppView('login')}
                  onRegisterClick={() => setCurrentAppView('register')}
                  onNavigateToLogin={() => setCurrentAppView('login')}
                  onNavigateToRegister={() => setCurrentAppView('register')}
                />
              )}

              {currentAppView === 'home' && (
                <HomeScreen
                  userName={user ? user.firstName : 'Sofía'}
                  isDriver={user?.role === 'DRIVER'}
                  onSearch={(params) => {
                    setSearchParams(params);
                    setCurrentAppView('search-results');
                  }}
                  onNavigateToPublish={() => setCurrentAppView('publish')}
                  onNavigateToTrips={() => setCurrentAppView('trips')}
                  onNavigateToProfile={() => setCurrentAppView('profile')}
                  onLogout={() => {
                    setUser(null);
                    setSession(null);
                    setCurrentAppView('welcome');
                  }}
                  onLoginClick={() => setCurrentAppView('login')}
                  onRegisterClick={() => setCurrentAppView('register')}
                />
              )}

              {currentAppView === 'login' && (
                <LoginScreen
                  onNavigateToRegister={() => setCurrentAppView('register')}
                  onNavigateToWelcome={() => setCurrentAppView('home')}
                  onLoginSuccess={(authData) => {
                    setUser({
                      id: authData.user.id,
                      email: authData.user.email,
                      firstName: authData.user.firstName,
                      lastName: authData.user.lastName,
                      role: (authData.user.role as 'PASSENGER' | 'DRIVER') || 'PASSENGER',
                      kycStatus: 'APPROVED',
                      isDriverActive: authData.user.isDriverActive,
                      canBookRides: true,
                      canPublishRides: authData.user.role === 'DRIVER',
                    });
                    setSession({
                      token: authData.accessToken,
                      expiresInSeconds: authData.expiresIn || 1200,
                      initialSeconds: authData.expiresIn || 1200,
                      provider: 'GOOGLE',
                    });
                    addLog('auth', `Sesión iniciada con éxito para ${authData.user.email}`);
                    setCurrentAppView('home');
                  }}
                />
              )}

              {currentAppView === 'register' && (
                <RegisterScreen
                  onNavigateToLogin={() => setCurrentAppView('login')}
                  onNavigateToWelcome={() => setCurrentAppView('home')}
                  onRegisterSuccess={(authData) => {
                    setUser({
                      id: authData.user.id,
                      email: authData.user.email,
                      firstName: authData.user.firstName,
                      lastName: authData.user.lastName,
                      role: (authData.user.role as 'PASSENGER' | 'DRIVER') || 'PASSENGER',
                      kycStatus: 'PENDING_VERIFICATION',
                      isDriverActive: authData.user.isDriverActive,
                      canBookRides: true,
                      canPublishRides: authData.user.role === 'DRIVER',
                    });
                    setSession({
                      token: authData.accessToken,
                      expiresInSeconds: authData.expiresIn || 1200,
                      initialSeconds: authData.expiresIn || 1200,
                      provider: 'GOOGLE',
                    });
                    addLog('auth', `Registro completado para ${authData.user.email}. Pasando a validación de identidad.`);
                    setCurrentAppView('identity-verification');
                  }}
                />
              )}

              {currentAppView === 'identity-verification' && (
                <IdentityVerificationScreen
                  userEmail={user?.email || 'sofia.martinez@ejemplo.com'}
                  userName={user ? `${user.firstName} ${user.lastName}` : 'Sofía Martínez'}
                  onBack={() => setCurrentAppView('register')}
                  onVerificationSuccess={() => {
                    if (user) {
                      setUser({ ...user, kycStatus: 'APPROVED' });
                    }
                    addLog('kyc', 'DNI Frente y Dorso aprobados exitosamente con validación biométrica.');
                    setCurrentAppView('home');
                  }}
                  onSkip={() => {
                    addLog('kyc', 'Validación de identidad postergada. Accediendo con verificación pendiente.');
                    setCurrentAppView('home');
                  }}
                />
              )}

              {currentAppView === 'search-results' && (
                <SearchResultsScreen
                  initialOrigin={searchParams.origin}
                  initialDestination={searchParams.destination}
                  initialDate={searchParams.date}
                  initialSeats={searchParams.seats}
                  onBackToHome={() => setCurrentAppView('home')}
                  onSelectTripToBook={(trip: TripSearchResult) => {
                    setSelectedTrip(trip as unknown as Trip);
                    setCurrentAppView('booking');
                  }}
                  onNavigateToPublish={() => setCurrentAppView('publish')}
                  onNavigateToTrips={() => setCurrentAppView('trips')}
                  onNavigateToProfile={() => setCurrentAppView('profile')}
                />
              )}

              {currentAppView === 'booking' && (
                <BookingScreen
                  trip={selectedTrip || undefined}
                  onBack={() => setCurrentAppView('search-results')}
                  onNavigateToTrips={() => setCurrentAppView('trips')}
                  onNavigateToHome={() => setCurrentAppView('home')}
                  onBookingSuccess={(newBooking: Booking) => {
                    addLog('info', `Reserva ${newBooking.id} creada exitosamente con fondos en Escrow.`);
                    setCurrentAppView('trips');
                  }}
                />
              )}

              {currentAppView === 'publish' && (
                <PublishTripScreen
                  onBack={() => setCurrentAppView('home')}
                  onNavigateToHome={() => setCurrentAppView('home')}
                  onNavigateToSearch={() => setCurrentAppView('home')}
                  onNavigateToTrips={() => setCurrentAppView('trips')}
                  onNavigateToProfile={() => setCurrentAppView('profile')}
                  userVehicle={
                    vehicle
                      ? { model: `${vehicle.brand} ${vehicle.model}`, plate: vehicle.plate }
                      : { model: 'Toyota Corolla', plate: 'AA 123 CD', color: 'Blanco' }
                  }
                  onTripCreated={() => {
                    addLog('info', 'Nuevo viaje publicado en la red.');
                    setIsManagingCreatedTrip(true);
                    setCurrentAppView('trips');
                  }}
                />
              )}

              {currentAppView === 'trips' && (
                <MyTripsScreen
                  initialManageTrip={isManagingCreatedTrip}
                  onNavigateToHome={() => {
                    setIsManagingCreatedTrip(false);
                    setCurrentAppView('home');
                  }}
                  onNavigateToSearch={() => {
                    setIsManagingCreatedTrip(false);
                    setCurrentAppView('search-results');
                  }}
                  onNavigateToPublish={() => {
                    setIsManagingCreatedTrip(false);
                    setCurrentAppView('publish');
                  }}
                  onNavigateToProfile={() => {
                    setIsManagingCreatedTrip(false);
                    setCurrentAppView('profile');
                  }}
                />
              )}

              {currentAppView === 'profile' && (
                <ProfileScreen
                  userName={user ? `${user.firstName} ${user.lastName}` : 'Sofía Martínez'}
                  userEmail={user ? user.email : 'sofia.martinez@ejemplo.com'}
                  role={user?.role || 'PASSENGER'}
                  onNavigateToHome={() => setCurrentAppView('home')}
                  onNavigateToSearch={() => setCurrentAppView('search-results')}
                  onNavigateToPublish={() => setCurrentAppView('publish')}
                  onNavigateToTrips={() => setCurrentAppView('trips')}
                  onLogout={() => {
                    setUser(null);
                    setSession(null);
                    setCurrentAppView('welcome');
                  }}
                  onNavigateToWelcome={() => {
                    setUser(null);
                    setSession(null);
                    setCurrentAppView('welcome');
                  }}
                  onToggleRole={() => {
                    if (user) {
                      const nextRole = user.role === 'DRIVER' ? 'PASSENGER' : 'DRIVER';
                      setUser({
                        ...user,
                        role: nextRole,
                        canPublishRides: nextRole === 'DRIVER',
                      });
                      addLog('info', `Rol alternado a ${nextRole}`);
                    }
                  }}
                  onNavigateToHelpSupport={() => setCurrentAppView('help-support')}
                  onNavigateToSecurityPrivacy={() => setCurrentAppView('security-privacy')}
                  onNavigateToPaymentMethods={() => setCurrentAppView('payment-methods')}
                  onNavigateToNotifications={() => setCurrentAppView('notifications-settings')}
                  onNavigateToAddVehicle={() => setCurrentAppView('add-vehicle')}
                />
              )}

            {currentAppView === 'help-support' && (
              <HelpSupportScreen
                onBack={() => setCurrentAppView('profile')}
                onNavigateToTrips={() => setCurrentAppView('trips')}
              />
            )}

            {currentAppView === 'security-privacy' && (
              <SecurityPrivacyScreen
                onBack={() => setCurrentAppView('profile')}
                onLogout={() => {
                  setUser(null);
                  setSession(null);
                  setCurrentAppView('welcome');
                }}
              />
            )}

            {currentAppView === 'payment-methods' && (
              <PaymentMethodsScreen
                onBack={() => setCurrentAppView('profile')}
              />
            )}

            {currentAppView === 'notifications-settings' && (
              <NotificationsSettingsScreen
                onBack={() => setCurrentAppView('profile')}
              />
            )}

            {currentAppView === 'add-vehicle' && (
              <AddVehicleScreen
                onBack={() => setCurrentAppView('profile')}
                onVehicleAdded={() => {
                  addLog('info', 'Nuevo vehículo registrado con éxito.');
                }}
              />
            )}
            </div>
          </div>
        )}

        {activeTab === 'simulator' && (
          <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
            {/* Columna Izquierda: Controles Interactivos */}
            <div className="lg:col-span-2 space-y-6">
              
              {/* Paso 1: Autenticación Social (RF-01, RF-02, RF-03, RF-04) */}
              <section className="bg-slate-900 border border-slate-800 rounded-xl p-5 shadow-sm">
                <div className="flex items-center justify-between pb-3 border-b border-slate-800 mb-4">
                  <div className="flex items-center space-x-2">
                    <span className="w-6 h-6 rounded-full bg-blue-500/20 text-blue-400 text-xs font-bold flex items-center justify-center">1</span>
                    <h2 className="font-semibold text-white">Autenticación Social & JWT (RF-01 al RF-06)</h2>
                  </div>
                  <span className="text-xs text-slate-400">TTL Estricto: 20 Minutos</span>
                </div>

                {!user ? (
                  <div className="space-y-3">
                    <p className="text-sm text-slate-400">
                      Inicia sesión con un proveedor social para crear síncronamente una cuenta con rol base de <b>Pasajero</b>.
                    </p>
                    <div className="flex flex-wrap gap-3">
                      <button
                        onClick={() => handleSocialLogin('GOOGLE')}
                        className="flex items-center space-x-2 bg-slate-800 hover:bg-slate-700 text-white px-4 py-2.5 rounded-lg border border-slate-700 text-sm font-medium transition"
                      >
                        <Key className="w-4 h-4 text-red-400" />
                        <span>Continuar con Google Sign-In</span>
                      </button>
                      <button
                        onClick={() => handleSocialLogin('APPLE')}
                        className="flex items-center space-x-2 bg-slate-800 hover:bg-slate-700 text-white px-4 py-2.5 rounded-lg border border-slate-700 text-sm font-medium transition"
                      >
                        <Key className="w-4 h-4 text-slate-200" />
                        <span>Continuar con Apple ID</span>
                      </button>
                    </div>
                  </div>
                ) : (
                  <div className="bg-slate-950/60 rounded-lg p-4 border border-slate-800 space-y-3">
                    <div className="flex items-center justify-between">
                      <div className="flex items-center space-x-3">
                        <div className="w-10 h-10 rounded-full bg-emerald-500/20 text-emerald-400 flex items-center justify-center font-bold">
                          {user.firstName[0]}{user.lastName[0]}
                        </div>
                        <div>
                          <p className="font-medium text-white">{user.firstName} {user.lastName}</p>
                          <p className="text-xs text-slate-400">{user.email}</p>
                        </div>
                      </div>
                      <div className="flex items-center space-x-2">
                        <span className="px-2 py-1 rounded text-xs font-semibold bg-emerald-500/20 text-emerald-300 border border-emerald-500/30">
                          {user.role}
                        </span>
                        <button
                          onClick={() => { setUser(null); setSession(null); addLog('auth', 'Sesión cerrada manualmente.'); }}
                          className="text-xs text-rose-400 hover:underline ml-2"
                        >
                          Cerrar Sesión
                        </button>
                      </div>
                    </div>

                    {/* Barra de Tiempo de Sesión (RF-04, RF-05) */}
                    {session && (
                      <div className="pt-2 border-t border-slate-800/80">
                        <div className="flex items-center justify-between text-xs mb-1">
                          <span className="flex items-center text-slate-300">
                            <Clock className="w-3.5 h-3.5 mr-1 text-emerald-400" />
                            Expiración de Token JWT (RF-04):
                          </span>
                          <span className={`font-mono font-bold ${session.expiresInSeconds < 300 ? 'text-amber-400 animate-pulse' : 'text-emerald-400'}`}>
                            {formatTimer(session.expiresInSeconds)} restantes
                          </span>
                        </div>
                        <div className="w-full bg-slate-800 h-1.5 rounded-full overflow-hidden">
                          <div
                            className="bg-emerald-500 h-full transition-all duration-1000"
                            style={{ width: `${(session.expiresInSeconds / session.initialSeconds) * 100}%` }}
                          />
                        </div>
                        {session.expiresInSeconds <= 300 && (
                          <div className="mt-2 flex items-center justify-between bg-amber-500/10 border border-amber-500/30 px-3 py-1.5 rounded text-xs text-amber-300">
                            <span>Faltan menos de 5 min para la expiración (RF-05).</span>
                            <button
                              onClick={handleRefreshToken}
                              className="font-semibold underline hover:text-white"
                            >
                              Extender Sesión
                            </button>
                          </div>
                        )}
                      </div>
                    )}
                  </div>
                )}
              </section>

              {/* Paso 2: Verificación KYC y Biometría (RF-07, RF-08, RF-09) */}
              <section className="bg-slate-900 border border-slate-800 rounded-xl p-5 shadow-sm">
                <div className="flex items-center justify-between pb-3 border-b border-slate-800 mb-4">
                  <div className="flex items-center space-x-2">
                    <span className="w-6 h-6 rounded-full bg-purple-500/20 text-purple-400 text-xs font-bold flex items-center justify-center">2</span>
                    <h2 className="font-semibold text-white">Verificación KYC: DNI y Selfie 3D (RF-07, RF-08, RF-09)</h2>
                  </div>
                  <span className="text-xs bg-slate-800 px-2.5 py-1 rounded text-slate-300">
                    Máx 3 Reintentos
                  </span>
                </div>

                {!user ? (
                  <p className="text-sm text-slate-500">Inicia sesión para acceder al módulo de verificación.</p>
                ) : (
                  <div className="space-y-4">
                    <div className="flex items-center justify-between text-sm bg-slate-950/60 p-3 rounded-lg border border-slate-800">
                      <div>
                        <span className="text-slate-400 text-xs block">Estado Actual de Identidad</span>
                        <span className="font-semibold text-white">
                          {user.kycStatus === 'NOT_STARTED' && '⚪ Sin Iniciar'}
                          {user.kycStatus === 'REJECTED' && '🔴 Rechazado (Reintentable)'}
                          {user.kycStatus === 'PENDING_MANUAL_REVIEW' && '🟡 Bloqueado en Mesa de Control (RF-09)'}
                          {user.kycStatus === 'APPROVED' && '🟢 Aprobado'}
                        </span>
                      </div>
                      <div className="text-right">
                        <span className="text-slate-400 text-xs block">Intentos Realizados</span>
                        <span className="font-mono text-xs font-bold text-slate-200">{kycAttempts} / 3</span>
                      </div>
                    </div>

                    {user.kycStatus !== 'APPROVED' && user.kycStatus !== 'PENDING_MANUAL_REVIEW' && (
                      <div className="flex flex-wrap gap-3">
                        <button
                          onClick={() => handleKYCAttempt(false)}
                          className="flex items-center space-x-2 bg-emerald-600 hover:bg-emerald-500 text-white px-4 py-2 rounded-lg text-sm font-medium transition"
                        >
                          <CheckCircle2 className="w-4 h-4" />
                          <span>Simular Captura Exitosa (Liveness 0.94)</span>
                        </button>
                        <button
                          onClick={() => handleKYCAttempt(true)}
                          className="flex items-center space-x-2 bg-rose-600/80 hover:bg-rose-600 text-white px-4 py-2 rounded-lg text-sm font-medium transition"
                        >
                          <XCircle className="w-4 h-4" />
                          <span>Simular Fallo Biométrico (RF-08)</span>
                        </button>
                      </div>
                    )}

                    {user.kycStatus === 'PENDING_MANUAL_REVIEW' && (
                      <div className="bg-amber-500/10 border border-amber-500/30 p-3 rounded-lg text-xs text-amber-300">
                        <b>RF-09 Aplicado:</b> Al superar 3 reintentos fallidos, la solicitud se congela en <code>PENDING_MANUAL_REVIEW</code> para resolución de mesa de control humana.
                      </div>
                    )}
                  </div>
                )}
              </section>

              {/* Paso 3: Registro de Vehículo & Retención de Pasajero (RF-10 al RF-13) */}
              <section className="bg-slate-900 border border-slate-800 rounded-xl p-5 shadow-sm">
                <div className="flex items-center justify-between pb-3 border-b border-slate-800 mb-4">
                  <div className="flex items-center space-x-2">
                    <span className="w-6 h-6 rounded-full bg-emerald-500/20 text-emerald-400 text-xs font-bold flex items-center justify-center">3</span>
                    <h2 className="font-semibold text-white">Registro de Vehículo & Trámite Asíncrono (RF-10 a RF-13)</h2>
                  </div>
                  <span className="text-xs text-slate-400">Trámite Asíncrono</span>
                </div>

                {!user ? (
                  <p className="text-sm text-slate-500">Inicia sesión para tramitar tu vehículo.</p>
                ) : user.kycStatus !== 'APPROVED' ? (
                  <div className="bg-slate-950/40 p-4 rounded-lg border border-slate-800 text-sm text-slate-400">
                    <p className="flex items-center text-amber-400 font-medium mb-1">
                      <AlertTriangle className="w-4 h-4 mr-1.5" />
                      Requiere Verificación Biométrica Aprobada
                    </p>
                    Debes completar la verificación de DNI y selfie 3D antes de poder registrar un automóvil.
                  </div>
                ) : !vehicle ? (
                  <div className="space-y-3">
                    <p className="text-sm text-slate-400">
                      Identidad verificada. Ahora puedes presentar Licencia de Conducir, Cédula del Automotor y datos del vehículo.
                    </p>
                    <button
                      onClick={handleRegisterVehicle}
                      className="flex items-center space-x-2 bg-indigo-600 hover:bg-indigo-500 text-white px-4 py-2 rounded-lg text-sm font-medium transition"
                    >
                      <Car className="w-4 h-4" />
                      <span>Registrar Toyota Corolla (RF-10, RF-11)</span>
                    </button>
                  </div>
                ) : (
                  <div className="bg-slate-950/60 p-4 rounded-lg border border-slate-800 space-y-3">
                    <div className="flex items-center justify-between">
                      <div>
                        <span className="text-xs text-slate-400">Automóvil Presentado</span>
                        <p className="font-medium text-white">{vehicle.brand} {vehicle.model} • Patente: {vehicle.plate}</p>
                      </div>
                      <span className={`text-xs px-2.5 py-1 rounded font-semibold ${
                        vehicle.status === 'APPROVED' ? 'bg-emerald-500/20 text-emerald-300 border border-emerald-500/30' :
                        vehicle.status === 'REJECTED' ? 'bg-rose-500/20 text-rose-300 border border-rose-500/30' :
                        'bg-amber-500/20 text-amber-300 border border-amber-500/30'
                      }`}>
                        {vehicle.status}
                      </span>
                    </div>

                    {vehicle.status === 'PENDING_VERIFICATION' && (
                      <div className="pt-2 border-t border-slate-800 flex items-center space-x-3">
                        <span className="text-xs text-slate-400">Simular Webhook de Resolución (RF-12):</span>
                        <button
                          onClick={() => handleVehicleResolution(true)}
                          className="text-xs bg-emerald-600 hover:bg-emerald-500 px-3 py-1 rounded text-white font-medium"
                        >
                          Aprobar Trámite
                        </button>
                        <button
                          onClick={() => handleVehicleResolution(false)}
                          className="text-xs bg-rose-600 hover:bg-rose-500 px-3 py-1 rounded text-white font-medium"
                        >
                          Rechazar Trámite
                        </button>
                      </div>
                    )}

                    {/* RF-13: Retención de Capacidad como Pasajero */}
                    <div className="bg-slate-900 p-3 rounded border border-slate-800/80 text-xs space-y-1">
                      <p className="font-semibold text-slate-200">Capacidades Operativas del Usuario (RF-13):</p>
                      <div className="flex items-center space-x-4">
                        <span className="text-emerald-400 flex items-center">
                          <CheckCircle2 className="w-3.5 h-3.5 mr-1" />
                          Buscar y Reservar Viajes: 100% Habilitado
                        </span>
                        <span className={user.canPublishRides ? 'text-emerald-400 flex items-center' : 'text-slate-500 flex items-center'}>
                          {user.canPublishRides ? <CheckCircle2 className="w-3.5 h-3.5 mr-1" /> : <XCircle className="w-3.5 h-3.5 mr-1" />}
                          Publicar Viajes: {user.canPublishRides ? 'Habilitado' : 'Bloqueado'}
                        </span>
                      </div>
                    </div>
                  </div>
                )}
              </section>
            </div>

            {/* Columna Derecha: Consola de Auditoría en Tiempo Real */}
            <div className="space-y-6">
              <section className="bg-slate-900 border border-slate-800 rounded-xl p-5 shadow-sm h-full flex flex-col">
                <div className="flex items-center justify-between pb-3 border-b border-slate-800 mb-3">
                  <div className="flex items-center space-x-2">
                    <Terminal className="w-4 h-4 text-emerald-400" />
                    <h3 className="font-semibold text-sm text-white">Event Log del Microservicio</h3>
                  </div>
                  <span className="text-xs text-slate-500 font-mono">live telemetry</span>
                </div>

                <div className="flex-1 bg-slate-950 rounded-lg p-3 border border-slate-800 font-mono text-xs space-y-2 overflow-y-auto max-h-[500px]">
                  {logs.length === 0 ? (
                    <p className="text-slate-600 text-center py-8">No hay eventos registrados. Interactúa con los controles para ver el flujo.</p>
                  ) : (
                    logs.map((log, idx) => (
                      <div key={idx} className="leading-relaxed border-b border-slate-900 pb-1.5 last:border-0">
                        <span className="text-slate-500 mr-2">[{log.time}]</span>
                        <span className={
                          log.type === 'auth' ? 'text-cyan-400' :
                          log.type === 'kyc' ? 'text-purple-400' :
                          log.type === 'vehicle' ? 'text-emerald-400' : 'text-slate-300'
                        }>
                          {log.message}
                        </span>
                      </div>
                    ))
                  )}
                </div>
              </section>
            </div>
          </div>
        )}

        {/* Tab: WhatsApp Deeplink Generator & Tester */}
        {activeTab === 'whatsapp' && <WhatsAppTester />}

        {/* Tab: Clean Architecture */}
        {activeTab === 'architecture' && (
          <div className="bg-slate-900 border border-slate-800 rounded-xl p-6 space-y-6">
            <div>
              <h2 className="text-lg font-bold text-white mb-1">Estructura Clean Architecture Implementada</h2>
              <p className="text-sm text-slate-400">Total desacoplamiento del núcleo del negocio respecto de frameworks, bases de datos y APIs externas.</p>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
              <div className="bg-slate-950 p-4 rounded-lg border border-slate-800 space-y-2">
                <div className="flex items-center space-x-2 text-emerald-400 font-semibold text-sm">
                  <Layers className="w-4 h-4" />
                  <span>1. Domain (Core)</span>
                </div>
                <p className="text-xs text-slate-400">Entidades puras y reglas de negocio sin dependencias externas.</p>
                <ul className="text-xs text-slate-300 space-y-1 font-mono">
                  <li>• user.go (Passenger default)</li>
                  <li>• session.go (20 min TTL)</li>
                  <li>• kyc.go (3 retries max)</li>
                  <li>• vehicle.go (Async flow)</li>
                  <li>• errors.go (Typed errors)</li>
                </ul>
              </div>

              <div className="bg-slate-950 p-4 rounded-lg border border-slate-800 space-y-2">
                <div className="flex items-center space-x-2 text-blue-400 font-semibold text-sm">
                  <FileText className="w-4 h-4" />
                  <span>2. Ports (Contracts)</span>
                </div>
                <p className="text-xs text-slate-400">Interfaces de entrada y salida que aíslan la lógica.</p>
                <ul className="text-xs text-slate-300 space-y-1 font-mono">
                  <li>• repositories.go (User, KYC, Veh)</li>
                  <li>• services.go (Use cases DTO)</li>
                  <li>• external.go (OAuth, Bio, Notif)</li>
                </ul>
              </div>

              <div className="bg-slate-950 p-4 rounded-lg border border-slate-800 space-y-2">
                <div className="flex items-center space-x-2 text-purple-400 font-semibold text-sm">
                  <Code2 className="w-4 h-4" />
                  <span>3. Services (Use Cases)</span>
                </div>
                <p className="text-xs text-slate-400">Orquestación de casos de uso con inyección de dependencias.</p>
                <ul className="text-xs text-slate-300 space-y-1 font-mono">
                  <li>• auth_service.go</li>
                  <li>• kyc_service.go</li>
                  <li>• vehicle_service.go</li>
                  <li>• *_test.go (Unit mocks)</li>
                </ul>
              </div>

              <div className="bg-slate-950 p-4 rounded-lg border border-slate-800 space-y-2">
                <div className="flex items-center space-x-2 text-amber-400 font-semibold text-sm">
                  <Database className="w-4 h-4" />
                  <span>4. Adapters (Infra)</span>
                </div>
                <p className="text-xs text-slate-400">PostgreSQL (pgx/v5), Router Gin, Mock Biometrics y Notifiers.</p>
                <ul className="text-xs text-slate-300 space-y-1 font-mono">
                  <li>• postgres/user_repo.go</li>
                  <li>• postgres/kyc_repo.go</li>
                  <li>• handlers/http/router.go</li>
                  <li>• middleware/auth.go</li>
                </ul>
              </div>
            </div>
          </div>
        )}

        {/* Tab: Contratos REST / Gin */}
        {activeTab === 'api-docs' && (
          <div className="bg-slate-900 border border-slate-800 rounded-xl p-6 space-y-6">
            <div>
              <h2 className="text-lg font-bold text-white mb-1">Endpoints REST del Microservicio (Gin Framework)</h2>
              <p className="text-sm text-slate-400">Rutas HTTP registradas bajo <code>/v1</code> con control de autorización Bearer JWT.</p>
            </div>

            <div className="space-y-4">
              <div className="bg-slate-950 p-4 rounded-lg border border-slate-800 space-y-2">
                <div className="flex items-center space-x-3">
                  <span className="px-2.5 py-0.5 rounded text-xs font-bold bg-blue-500/20 text-blue-400">POST</span>
                  <code className="text-sm text-white font-mono">/v1/auth/social-login</code>
                  <span className="text-xs text-slate-400">• RF-01, RF-02, RF-03, RF-04</span>
                </div>
                <p className="text-xs text-slate-300">Autentica token Google/Apple, crea usuario como Pasajero y retorna JWT con TTL de 20 min (1200s).</p>
              </div>

              <div className="bg-slate-950 p-4 rounded-lg border border-slate-800 space-y-2">
                <div className="flex items-center space-x-3">
                  <span className="px-2.5 py-0.5 rounded text-xs font-bold bg-blue-500/20 text-blue-400">POST</span>
                  <code className="text-sm text-white font-mono">/v1/auth/refresh</code>
                  <span className="text-xs text-slate-400">• RF-05</span>
                </div>
                <p className="text-xs text-slate-300">Renueva o extiende la sesión si no ha expirado el TTL de 20 minutos.</p>
              </div>

              <div className="bg-slate-950 p-4 rounded-lg border border-slate-800 space-y-2">
                <div className="flex items-center space-x-3">
                  <span className="px-2.5 py-0.5 rounded text-xs font-bold bg-emerald-500/20 text-emerald-400">GET</span>
                  <code className="text-sm text-white font-mono">/v1/auth/me</code>
                  <span className="text-xs text-slate-400">• RF-13 (AuthMiddleware)</span>
                </div>
                <p className="text-xs text-slate-300">Retorna perfil, rol y confirma que <code>can_book_rides: true</code> siempre está garantizado.</p>
              </div>

              <div className="bg-slate-950 p-4 rounded-lg border border-slate-800 space-y-2">
                <div className="flex items-center space-x-3">
                  <span className="px-2.5 py-0.5 rounded text-xs font-bold bg-purple-500/20 text-purple-400">POST</span>
                  <code className="text-sm text-white font-mono">/v1/kyc/submit</code>
                  <span className="text-xs text-slate-400">• RF-07, RF-08, RF-09</span>
                </div>
                <p className="text-xs text-slate-300">Carga de DNI frente/dorso y selfie 3D. Controla reintentos ($&le; 3$) y deriva a <code>PENDING_MANUAL_REVIEW</code>.</p>
              </div>

              <div className="bg-slate-950 p-4 rounded-lg border border-slate-800 space-y-2">
                <div className="flex items-center space-x-3">
                  <span className="px-2.5 py-0.5 rounded text-xs font-bold bg-amber-500/20 text-amber-400">POST</span>
                  <code className="text-sm text-white font-mono">/v1/vehicles/register</code>
                  <span className="text-xs text-slate-400">• RF-10, RF-11</span>
                </div>
                <p className="text-xs text-slate-300">Exige KYC aprobado. Registra el automóvil en estado <code>PENDING_VERIFICATION</code> para revisión asíncrona.</p>
              </div>

              {/* Endpoint: WhatsApp Deeplink para Pasajeros */}
              <div className="bg-slate-950 p-4 rounded-lg border border-slate-800 space-y-2">
                <div className="flex items-center space-x-3">
                  <span className="px-2.5 py-0.5 rounded text-xs font-bold bg-emerald-500/20 text-emerald-400">GET</span>
                  <code className="text-sm text-white font-mono">/api/v1/bookings/:id/contact-link</code>
                  <span className="text-xs text-slate-400">• Deeplink WhatsApp (wa.me) Pasajero-Conductor</span>
                </div>
                <p className="text-xs text-slate-300">Genera enlace directo seguro wa.me con número normalizado y mensaje predeterminado codificado para coordinar el viaje.</p>
              </div>

              {/* Módulo 5: Endpoints de Salud y Observabilidad */}
              <div className="bg-slate-950 p-4 rounded-lg border border-slate-800 space-y-2">
                <div className="flex items-center space-x-3">
                  <span className="px-2.5 py-0.5 rounded text-xs font-bold bg-emerald-500/20 text-emerald-400">GET</span>
                  <code className="text-sm text-white font-mono">/health</code>
                  <span className="text-xs text-slate-400">• Liveness Probe & DB Ping (Módulo 5)</span>
                </div>
                <p className="text-xs text-slate-300">Verifica vitalidad del servicio, latencia de base de datos, memoria allocada y métricas activas del pool de conexiones.</p>
              </div>

              <div className="bg-slate-950 p-4 rounded-lg border border-slate-800 space-y-2">
                <div className="flex items-center space-x-3">
                  <span className="px-2.5 py-0.5 rounded text-xs font-bold bg-teal-500/20 text-teal-400">GET</span>
                  <code className="text-sm text-white font-mono">/ready</code>
                  <span className="text-xs text-slate-400">• Readiness Probe & PostGIS (Módulo 5)</span>
                </div>
                <p className="text-xs text-slate-300">Garantiza disponibilidad para tráfico validando conectividad activa a PostgreSQL y comprobación funcional de la extensión geoespacial PostGIS.</p>
              </div>
            </div>
          </div>
        )}

        {/* Tab: Módulo 5: Observabilidad, Docker & CI/CD */}
        {activeTab === 'deployment' && (
          <div className="bg-slate-900 border border-slate-800 rounded-xl p-6 space-y-6">
            <div>
              <div className="flex items-center justify-between mb-1">
                <h2 className="text-lg font-bold text-white flex items-center space-x-2">
                  <Activity className="w-5 h-5 text-emerald-400" />
                  <span>Módulo 5: Observabilidad, Contenerización (Docker/PostGIS) & CI/CD Pipeline</span>
                </h2>
                <span className="text-xs font-semibold px-2.5 py-1 rounded bg-emerald-500/20 text-emerald-300 border border-emerald-500/30">
                  Producción Lista
                </span>
              </div>
              <p className="text-sm text-slate-400">
                Pruebas activas de salud (Liveness / Readiness), métricas de pool pgxpool, empaquetado multi-stage (Go 1.24 / Alpine) y pipeline de GitHub Actions con cobertura &ge; 80%.
              </p>
            </div>

            {/* 1. Consola Interactiva de Probes /health y /ready */}
            <div className="bg-slate-950 p-5 rounded-lg border border-slate-800 space-y-4">
              <div className="flex flex-wrap items-center justify-between gap-3 border-b border-slate-800/80 pb-3">
                <div className="flex items-center space-x-2 text-emerald-400 font-semibold text-sm">
                  <Server className="w-4 h-4" />
                  <span>Consola de Verificación Activa: Liveness & Readiness</span>
                </div>
                <div className="flex items-center space-x-3 text-xs">
                  <span className="text-slate-400">Simulador de Fallas en DB:</span>
                  <button
                    onClick={() => setDbHealth(prev => prev === 'UP' ? 'DOWN' : 'UP')}
                    className={`px-2.5 py-1 rounded font-medium border transition ${
                      dbHealth === 'UP'
                        ? 'bg-emerald-500/20 text-emerald-300 border-emerald-500/30 hover:bg-emerald-500/30'
                        : 'bg-rose-500/20 text-rose-300 border-rose-500/30 hover:bg-rose-500/30'
                    }`}
                  >
                    PostgreSQL: {dbHealth}
                  </button>
                  <button
                    onClick={() => setPostgisHealth(prev => prev === 'UP' ? 'DOWN' : 'UP')}
                    className={`px-2.5 py-1 rounded font-medium border transition ${
                      postgisHealth === 'UP'
                        ? 'bg-emerald-500/20 text-emerald-300 border-emerald-500/30 hover:bg-emerald-500/30'
                        : 'bg-amber-500/20 text-amber-300 border-amber-500/30 hover:bg-amber-500/30'
                    }`}
                  >
                    PostGIS: {postgisHealth}
                  </button>
                </div>
              </div>

              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                {/* Botones de Disparo de Probes */}
                <div className="space-y-3">
                  <div className="p-3.5 rounded-lg bg-slate-900 border border-slate-800 space-y-2">
                    <div className="flex items-center justify-between">
                      <span className="text-xs font-mono font-bold text-cyan-400">GET /health (Liveness)</span>
                      <button
                        onClick={() => simulateProbe('/health')}
                        className="bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-medium px-3 py-1.5 rounded transition flex items-center space-x-1.5"
                      >
                        <Activity className="w-3.5 h-3.5" />
                        <span>Ejecutar Liveness</span>
                      </button>
                    </div>
                    <p className="text-xs text-slate-400 leading-relaxed">
                      Verifica que el servicio HTTP esté vivo, uptime acumulado, memoria allocada, conteo de goroutines y ping hacia PostgreSQL.
                    </p>
                  </div>

                  <div className="p-3.5 rounded-lg bg-slate-900 border border-slate-800 space-y-2">
                    <div className="flex items-center justify-between">
                      <span className="text-xs font-mono font-bold text-teal-400">GET /ready (Readiness)</span>
                      <button
                        onClick={() => simulateProbe('/ready')}
                        className="bg-teal-600 hover:bg-teal-500 text-white text-xs font-medium px-3 py-1.5 rounded transition flex items-center space-x-1.5"
                      >
                        <Check className="w-3.5 h-3.5" />
                        <span>Ejecutar Readiness</span>
                      </button>
                    </div>
                    <p className="text-xs text-slate-400 leading-relaxed">
                      Comprobación estricta de tráfico: valida socket activo a PostgreSQL y consulta la función/versión de la extensión <b>PostGIS 3.4</b>. Si falla, emite HTTP 503.
                    </p>
                  </div>

                  {/* Telemetría del Connection Pool pgxpool */}
                  <div className="p-3 rounded-lg bg-slate-900/90 border border-slate-800 text-xs font-mono space-y-1.5">
                    <span className="text-slate-400 text-[11px] block font-semibold">Métricas de Conexión pgxpool (configs/config.go):</span>
                    <div className="grid grid-cols-4 gap-2 text-center pt-1">
                      <div className="bg-slate-950 p-1.5 rounded border border-slate-800">
                        <span className="text-slate-500 block text-[10px]">Total</span>
                        <span className="text-emerald-400 font-bold">{dbHealth === 'UP' ? '5' : '0'}</span>
                      </div>
                      <div className="bg-slate-950 p-1.5 rounded border border-slate-800">
                        <span className="text-slate-500 block text-[10px]">Inactivas</span>
                        <span className="text-cyan-400 font-bold">{dbHealth === 'UP' ? '4' : '0'}</span>
                      </div>
                      <div className="bg-slate-950 p-1.5 rounded border border-slate-800">
                        <span className="text-slate-500 block text-[10px]">Adquiridas</span>
                        <span className="text-purple-400 font-bold">{dbHealth === 'UP' ? '1' : '0'}</span>
                      </div>
                      <div className="bg-slate-950 p-1.5 rounded border border-slate-800">
                        <span className="text-slate-500 block text-[10px]">Máx Pool</span>
                        <span className="text-amber-400 font-bold">20</span>
                      </div>
                    </div>
                  </div>
                </div>

                {/* Salida JSON en Vivo del Endpoint */}
                <div className="bg-slate-900 rounded-lg p-3.5 border border-slate-800 flex flex-col font-mono text-xs">
                  <div className="flex items-center justify-between pb-2 border-b border-slate-800 mb-2">
                    <span className="text-slate-400 text-[11px]">Respuesta HTTP en Vivo:</span>
                    {probeResult ? (
                      <div className="flex items-center space-x-2">
                        <span className={`px-2 py-0.5 rounded text-[10px] font-bold ${
                          probeResult.status === 200 ? 'bg-emerald-500/20 text-emerald-300' : 'bg-rose-500/20 text-rose-300'
                        }`}>
                          HTTP {probeResult.status} {probeResult.statusText}
                        </span>
                        <span className="text-slate-500 text-[10px]">{probeResult.durationMs}ms</span>
                      </div>
                    ) : (
                      <span className="text-slate-600 text-[10px]">esperando ejecución...</span>
                    )}
                  </div>

                  <pre className="flex-1 bg-slate-950 p-3 rounded border border-slate-800/80 text-[11px] text-emerald-300 overflow-x-auto leading-relaxed">
                    {probeResult
                      ? JSON.stringify(probeResult.body, null, 2)
                      : '// Haz clic en "Ejecutar Liveness" o "Ejecutar Readiness" para inspeccionar el payload'}
                  </pre>
                </div>
              </div>
            </div>

            {/* 2. Contenerización: Dockerfile & docker-compose.yml */}
            <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
              {/* Dockerfile Multi-Stage */}
              <div className="bg-slate-950 p-5 rounded-lg border border-slate-800 space-y-3">
                <div className="flex items-center justify-between">
                  <div className="flex items-center space-x-2 text-purple-400 font-semibold text-sm">
                    <Layers className="w-4 h-4" />
                    <span>Dockerfile Multi-Stage (golang:1.24-alpine)</span>
                  </div>
                  <span className="text-[10px] font-mono bg-purple-500/20 text-purple-300 px-2 py-0.5 rounded border border-purple-500/30">
                    scratch / alpine:latest
                  </span>
                </div>
                <div className="space-y-2 text-xs text-slate-300 font-mono">
                  <div className="bg-slate-900 p-2.5 rounded border border-slate-800 space-y-1">
                    <span className="text-slate-500 block text-[10px]">PATRÓN DE COMPILACIÓN SEGURO</span>
                    <p className="text-purple-300">1. Builder: <span className="text-slate-300">FROM golang:1.24-alpine</span></p>
                    <p className="text-purple-300">2. CGO_ENABLED=0 <span className="text-slate-400">-ldflags="-w -s" (binario estático)</span></p>
                    <p className="text-purple-300">3. Runner: <span className="text-slate-300">FROM alpine:latest</span></p>
                    <p className="text-purple-300">4. Certificados CA: <span className="text-slate-400">apk add ca-certificates tzdata</span></p>
                    <p className="text-purple-300">5. Hardening: <span className="text-slate-400">USER appuser:appgroup (non-root)</span></p>
                    <p className="text-purple-300">6. Healthcheck: <span className="text-slate-400">wget --spider http://localhost:8080/health</span></p>
                  </div>
                  <p className="text-slate-400 text-xs font-sans">
                    El binario final pesa menos de 20 MB, no requiere runtime de Go y cuenta con certificados raíz para validaciones de tokens OAuth de Google y Apple.
                  </p>
                </div>
              </div>

              {/* docker-compose.yml con PostGIS 16 */}
              <div className="bg-slate-950 p-5 rounded-lg border border-slate-800 space-y-3">
                <div className="flex items-center justify-between">
                  <div className="flex items-center space-x-2 text-blue-400 font-semibold text-sm">
                    <Database className="w-4 h-4" />
                    <span>docker-compose.yml (postgis/postgis:16-3.4-alpine)</span>
                  </div>
                  <span className="text-[10px] font-mono bg-blue-500/20 text-blue-300 px-2 py-0.5 rounded border border-blue-500/30">
                    Servicios: api & postgres
                  </span>
                </div>
                <div className="space-y-2 text-xs text-slate-300 font-mono">
                  <div className="bg-slate-900 p-2.5 rounded border border-slate-800 space-y-1">
                    <span className="text-slate-500 block text-[10px]">TOPOLOGÍA Y HEALTHCHECKS</span>
                    <p><span className="text-cyan-400">api:</span> puerto 8080, dependiente de postgres healthy</p>
                    <p><span className="text-cyan-400">postgres:</span> imagen postgis/postgis:16-3.4-alpine</p>
                    <p><span className="text-cyan-400">healthcheck:</span> pg_isready -U thumbi -d thumbi_carpooling</p>
                    <p><span className="text-cyan-400">volúmenes:</span> postgres_data:/var/lib/postgresql/data</p>
                    <p><span className="text-cyan-400">migrations:</span> montadas en /docker-entrypoint-initdb.d</p>
                  </div>
                  <p className="text-slate-400 text-xs font-sans">
                    Arranque coordinado garantizado: la API espera a que PostgreSQL y las extensiones espaciales PostGIS hayan completado el bootstrapping.
                  </p>
                </div>
              </div>
            </div>

            {/* 3. GitHub Actions CI/CD Pipeline (.github/workflows/ci.yml) */}
            <div className="bg-slate-950 p-5 rounded-lg border border-slate-800 space-y-3">
              <div className="flex items-center justify-between">
                <div className="flex items-center space-x-2 text-emerald-400 font-semibold text-sm">
                  <GitBranch className="w-4 h-4" />
                  <span>Pipeline CI/CD Automatizado (.github/workflows/ci.yml)</span>
                </div>
                <span className="text-xs text-slate-400 font-mono">Trigger: push & PR a rama main</span>
              </div>

              <div className="grid grid-cols-1 md:grid-cols-3 gap-3 text-xs">
                <div className="bg-slate-900 p-3.5 rounded border border-slate-800 space-y-1.5">
                  <div className="flex items-center justify-between">
                    <span className="font-semibold text-white">1. Job: lint</span>
                    <span className="text-[10px] bg-cyan-500/20 text-cyan-300 px-1.5 py-0.5 rounded font-mono">golangci-lint</span>
                  </div>
                  <p className="text-slate-400 text-[11px]">
                    Ejecuta <code>go vet</code> y <code>golangci-lint-action@v6</code> para asegurar formato estándar y ausencia de code smells.
                  </p>
                </div>

                <div className="bg-slate-900 p-3.5 rounded border border-slate-800 space-y-1.5">
                  <div className="flex items-center justify-between">
                    <span className="font-semibold text-white">2. Job: test</span>
                    <span className="text-[10px] bg-emerald-500/20 text-emerald-300 px-1.5 py-0.5 rounded font-mono">&ge; 80% Cobertura</span>
                  </div>
                  <p className="text-slate-400 text-[11px]">
                    Lanza un contenedor PostGIS de servicio y corre <code>go test -v -race -cover ./...</code> verificando umbral de calidad.
                  </p>
                </div>

                <div className="bg-slate-900 p-3.5 rounded border border-slate-800 space-y-1.5">
                  <div className="flex items-center justify-between">
                    <span className="font-semibold text-white">3. Job: build</span>
                    <span className="text-[10px] bg-purple-500/20 text-purple-300 px-1.5 py-0.5 rounded font-mono">Docker Buildx</span>
                  </div>
                  <p className="text-slate-400 text-[11px]">
                    Compila el binario estático <code>thumbi-api</code> y valida la construcción exitosa de la imagen Docker en GitHub Runner.
                  </p>
                </div>
              </div>
            </div>
          </div>
        )}
      </main>
    </div>
  );
}
