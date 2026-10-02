import React, { useState } from 'react';
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
import { Trip, Booking, TripSearchResult } from './types';
import { UserProvider, useUser } from './context/UserContext';
import { TripProvider } from './context/TripContext';
import { ToastProvider } from './context/ToastContext';

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

function AppContent() {
  const { user, updateProfile } = useUser();
  const [currentAppView, setCurrentAppView] = useState<AppView>('home');
  const [searchParams, setSearchParams] = useState<SearchQueryParams>({
    origin: 'Palermo, CABA',
    destination: 'Pilar, Buenos Aires',
    date: '2026-09-17',
    seats: 1,
  });
  const [selectedTrip, setSelectedTrip] = useState<Trip | null>(null);
  const [isManagingCreatedTrip, setIsManagingCreatedTrip] = useState<boolean>(false);

  return (
    <div className="min-h-screen w-full bg-[#F8FAFC] text-[#1A1A1A] flex flex-col font-sans selection:bg-[#00A896] selection:text-white antialiased overflow-x-hidden">
      {/* Contenedor adaptativo: 100% full-screen en celulares, fluido y elegante en tablets y pantallas grandes */}
      <div className="w-full flex-1 flex flex-col items-center justify-start bg-[#F8FAFC]">
        <div className="w-full min-h-screen flex flex-col bg-[#F8FAFC]">
          
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
              userName={user?.name ? user.name.split(' ')[0] : 'Sofía'}
              isDriver={user?.role === 'DRIVER'}
              onSearch={(params) => {
                setSearchParams(params);
                setCurrentAppView('search-results');
              }}
              onNavigateToPublish={() => setCurrentAppView('publish')}
              onNavigateToTrips={() => setCurrentAppView('trips')}
              onNavigateToProfile={() => setCurrentAppView('profile')}
              onLogout={() => setCurrentAppView('welcome')}
              onLoginClick={() => setCurrentAppView('login')}
              onRegisterClick={() => setCurrentAppView('register')}
            />
          )}

          {currentAppView === 'login' && (
            <LoginScreen
              onNavigateToRegister={() => setCurrentAppView('register')}
              onNavigateToWelcome={() => setCurrentAppView('home')}
              onLoginSuccess={async (authData) => {
                if (authData?.user) {
                  await updateProfile({
                    name: `${authData.user.firstName || ''} ${authData.user.lastName || ''}`.trim() || 'Sofía Martínez',
                    email: authData.user.email,
                    role: authData.user.role === 'DRIVER' ? 'DRIVER' : 'PASSENGER',
                  });
                }
                setCurrentAppView('home');
              }}
            />
          )}

          {currentAppView === 'register' && (
            <RegisterScreen
              onNavigateToLogin={() => setCurrentAppView('login')}
              onNavigateToWelcome={() => setCurrentAppView('home')}
              onRegisterSuccess={async (authData) => {
                if (authData?.user) {
                  await updateProfile({
                    name: `${authData.user.firstName || ''} ${authData.user.lastName || ''}`.trim() || 'Sofía Martínez',
                    email: authData.user.email,
                    role: authData.user.role === 'DRIVER' ? 'DRIVER' : 'PASSENGER',
                  });
                }
                setCurrentAppView('identity-verification');
              }}
            />
          )}

          {currentAppView === 'identity-verification' && (
            <IdentityVerificationScreen
              userEmail={user?.email || 'sofia.martinez@ejemplo.com'}
              userName={user?.name || 'Sofía Martínez'}
              onBack={() => setCurrentAppView('register')}
              onVerificationSuccess={() => {
                setCurrentAppView('home');
              }}
              onSkip={() => {
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
              onBookingSuccess={(_newBooking: Booking) => {
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
                user?.vehicles?.[0]
                  ? { model: `${user.vehicles[0].brand} ${user.vehicles[0].model}`, plate: user.vehicles[0].plate }
                  : { model: 'Toyota Corolla', plate: 'AA 123 CD', color: 'Blanco' }
              }
              onTripCreated={() => {
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
              userName={user?.name || 'Sofía Martínez'}
              userEmail={user?.email || 'sofia.martinez@ejemplo.com'}
              role={user?.role || 'PASSENGER'}
              onNavigateToHome={() => setCurrentAppView('home')}
              onNavigateToSearch={() => setCurrentAppView('search-results')}
              onNavigateToPublish={() => setCurrentAppView('publish')}
              onNavigateToTrips={() => setCurrentAppView('trips')}
              onLogout={() => setCurrentAppView('welcome')}
              onNavigateToWelcome={() => setCurrentAppView('welcome')}
              onToggleRole={async () => {
                const nextRole = user?.role === 'DRIVER' ? 'PASSENGER' : 'DRIVER';
                await updateProfile({ role: nextRole });
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
              onLogout={() => setCurrentAppView('welcome')}
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
                setCurrentAppView('profile');
              }}
            />
          )}

        </div>
      </div>
    </div>
  );
}

export default function App() {
  return (
    <ToastProvider>
      <UserProvider>
        <TripProvider>
          <AppContent />
        </TripProvider>
      </UserProvider>
    </ToastProvider>
  );
}
