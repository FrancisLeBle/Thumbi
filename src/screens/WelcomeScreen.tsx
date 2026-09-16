import React, { useState } from 'react';

export interface WelcomeScreenProps {
  onLoginClick?: () => void;
  onRegisterClick?: () => void;
  onNavigateToLogin?: () => void;
  onNavigateToRegister?: () => void;
}

export const WelcomeScreen: React.FC<WelcomeScreenProps> = ({
  onLoginClick,
  onRegisterClick,
  onNavigateToLogin,
  onNavigateToRegister,
}) => {
  const [currentSlideIndex, setCurrentSlideIndex] = useState<number>(0);

  const handleLogin = () => {
    if (onLoginClick) {
      onLoginClick();
    } else if (onNavigateToLogin) {
      onNavigateToLogin();
    }
  };

  const handleRegister = () => {
    if (onRegisterClick) {
      onRegisterClick();
    } else if (onNavigateToRegister) {
      onNavigateToRegister();
    }
  };

  return (
    <div className="min-h-screen flex items-center justify-center p-0 md:p-6 bg-slate-900 font-sans">
      {/* BEGIN: MobileIOSContainer */}
      <main
        className="w-full max-w-[393px] h-[852px] bg-white shadow-2xl relative overflow-hidden flex flex-col justify-between md:rounded-[44px] border-0 md:border-[8px] md:border-slate-800 select-none"
        data-purpose="mobile-viewport"
      >
        {/* BEGIN: iOSStatusBar */}
        <header
          className="w-full pt-3 px-7 flex justify-between items-center z-20 shrink-0 bg-transparent text-neutral-900 select-none"
          data-purpose="ios-status-bar"
        >
          {/* Time */}
          <span className="text-[15px] font-semibold tracking-tight text-neutral-900 pl-1">
            9:41
          </span>

          {/* Dynamic Island Indicator */}
          <div className="w-28 h-6 bg-black rounded-full mx-auto -mr-2 hidden sm:block" />

          {/* Status Icons (Cellular, Wifi, Battery) */}
          <div className="flex items-center space-x-1.5 pr-1 text-neutral-900">
            {/* Cellular Signal Icon */}
            <svg
              className="w-4 h-3.5 fill-neutral-900"
              fill="none"
              viewBox="0 0 17 12"
              xmlns="http://www.w3.org/2000/svg"
            >
              <rect height="3.5" rx="0.75" width="2.5" x="0.5" y="8.5" />
              <rect height="6" rx="0.75" width="2.5" x="4.5" y="6" />
              <rect height="8.5" rx="0.75" width="2.5" x="8.5" y="3.5" />
              <rect height="11.5" rx="0.75" width="2.5" x="12.5" y="0.5" />
            </svg>

            {/* Wi-Fi Icon */}
            <svg
              className="w-4 h-3.5 fill-neutral-900"
              fill="none"
              viewBox="0 0 16 12"
              xmlns="http://www.w3.org/2000/svg"
            >
              <path
                clipRule="evenodd"
                d="M8 1.5C5.16 1.5 2.58 2.65 0.7 4.52L0 3.8C2.07 1.74 4.9 0.5 8 0.5C11.1 0.5 13.93 1.74 16 3.8L15.3 4.52C13.42 2.65 10.84 1.5 8 1.5ZM8 4.5C6.01 4.5 4.19 5.3 2.87 6.61L2.17 5.89C3.67 4.39 5.73 3.5 8 3.5C10.27 3.5 12.33 4.39 13.83 5.89L13.13 6.61C11.81 5.3 9.99 4.5 8 4.5ZM8 7.5C6.9 7.5 5.89 7.95 5.15 8.68L4.44 7.97C5.36 7.06 6.62 6.5 8 6.5C9.38 6.5 10.64 7.06 11.56 7.97L10.85 8.68C10.11 7.95 9.1 7.5 8 7.5ZM9.25 10.75C9.25 11.44 8.69 12 8 12C7.31 12 6.75 11.44 6.75 10.75C6.75 10.06 7.31 9.5 8 9.5C8.69 9.5 9.25 10.06 9.25 10.75Z"
                fillRule="evenodd"
              />
            </svg>

            {/* Battery Icon */}
            <div className="flex items-center">
              <div className="w-5 h-[11px] border border-neutral-900 rounded-[3.5px] p-[1px] flex items-center">
                <div className="h-full w-full bg-neutral-900 rounded-[1.5px]" />
              </div>
              <div className="w-[1.5px] h-[4px] bg-neutral-900 rounded-r-[1px] -ml-[0.5px]" />
            </div>
          </div>
        </header>
        {/* END: iOSStatusBar */}

        {/* BEGIN: HeroIllustrationSection */}
        <section className="w-full px-4 pt-2 shrink-0" data-purpose="hero-illustration">
          {/* Container for isometric style carpooling illustration matching Thumbi 1st screen */}
          <div
            className="w-full h-[360px] rounded-2xl overflow-hidden relative shadow-sm"
            style={{ background: 'linear-gradient(180deg, #D5EDEB 0%, #AEE3DF 100%)' }}
          >
            <svg
              className="w-full h-full object-cover"
              fill="none"
              viewBox="0 0 360 360"
              xmlns="http://www.w3.org/2000/svg"
            >
              {/* Sky and Soft Cloud Shapes */}
              <path
                d="M-20 80 Q 40 50 100 80 Q 160 110 220 70 Q 280 40 380 90 L 380 0 L -20 0 Z"
                fill="#FFFFFF"
                fillOpacity="0.3"
              />
              <path
                d="M40 50 C 60 30, 100 30, 120 50 C 130 50, 150 60, 140 75 C 130 90, 80 90, 60 80 C 40 80, 20 70, 40 50 Z"
                fill="#FFFFFF"
                fillOpacity="0.5"
              />
              <path
                d="M220 40 C 240 25, 270 25, 290 40 C 300 40, 315 50, 305 60 C 295 70, 250 70, 240 65 C 220 65, 210 55, 220 40 Z"
                fill="#FFFFFF"
                fillOpacity="0.4"
              />

              {/* Background Rolling Hills */}
              <path
                d="M-40 260 Q 90 120 280 200 Q 360 230 400 280 L 400 360 L -40 360 Z"
                fill="#78C6A3"
                fillOpacity="0.45"
              />
              <path
                d="M-20 320 Q 160 210 380 290 L 380 360 L -20 360 Z"
                fill="#60BA93"
                fillOpacity="0.5"
              />

              {/* Curving Road Base */}
              <path
                d="M380 180 C 280 160, 140 120, 110 220 C 80 320, 20 350, -20 360 L 50 360 C 110 350, 150 290, 170 230 C 200 150, 300 180, 380 200 Z"
                fill="#4B5868"
              />

              {/* Road Center Dashed Line */}
              <path
                d="M380 190 C 290 170, 155 135, 140 225 C 125 305, 70 345, 10 360"
                fill="none"
                opacity="0.9"
                stroke="#FFFFFF"
                strokeDasharray="12 12"
                strokeWidth="4"
              />

              {/* Solar Panels on the left/green area */}
              {/* Solar Panel 1 */}
              <g transform="translate(15, 120) skewY(-15) skewX(25) scale(0.9)">
                <rect
                  fill="#2E6F8E"
                  height="26"
                  rx="2"
                  stroke="#71A5C1"
                  strokeWidth="1.5"
                  width="34"
                  x="0"
                  y="0"
                />
                <line stroke="#71A5C1" strokeWidth="1" x1="11" x2="11" y1="0" y2="26" />
                <line stroke="#71A5C1" strokeWidth="1" x1="22" x2="22" y1="0" y2="26" />
                <line stroke="#71A5C1" strokeWidth="1" x1="0" x2="34" y1="13" y2="13" />
                <rect fill="#4B6072" height="8" width="6" x="14" y="26" />
              </g>

              {/* Solar Panel 2 */}
              <g transform="translate(55, 125) skewY(-15) skewX(25) scale(0.9)">
                <rect
                  fill="#2E6F8E"
                  height="26"
                  rx="2"
                  stroke="#71A5C1"
                  strokeWidth="1.5"
                  width="34"
                  x="0"
                  y="0"
                />
                <line stroke="#71A5C1" strokeWidth="1" x1="11" x2="11" y1="0" y2="26" />
                <line stroke="#71A5C1" strokeWidth="1" x1="22" x2="22" y1="0" y2="26" />
                <line stroke="#71A5C1" strokeWidth="1" x1="0" x2="34" y1="13" y2="13" />
                <rect fill="#4B6072" height="8" width="6" x="14" y="26" />
              </g>

              {/* Solar Panels on the right foreground */}
              <g transform="translate(260, 220) skewY(-14) skewX(20) scale(0.85)">
                <rect
                  fill="#2E6F8E"
                  height="24"
                  rx="2"
                  stroke="#71A5C1"
                  strokeWidth="1.5"
                  width="32"
                  x="0"
                  y="0"
                />
                <line stroke="#71A5C1" strokeWidth="1" x1="10" x2="10" y1="0" y2="24" />
                <line stroke="#71A5C1" strokeWidth="1" x1="21" x2="21" y1="0" y2="24" />
                <line stroke="#71A5C1" strokeWidth="1" x1="0" x2="32" y1="12" y2="12" />
                <rect fill="#4B6072" height="8" width="6" x="13" y="24" />
              </g>

              <g transform="translate(285, 235) skewY(-14) skewX(20) scale(0.85)">
                <rect
                  fill="#2E6F8E"
                  height="24"
                  rx="2"
                  stroke="#71A5C1"
                  strokeWidth="1.5"
                  width="32"
                  x="0"
                  y="0"
                />
                <line stroke="#71A5C1" strokeWidth="1" x1="10" x2="10" y1="0" y2="24" />
                <line stroke="#71A5C1" strokeWidth="1" x1="21" x2="21" y1="0" y2="24" />
                <line stroke="#71A5C1" strokeWidth="1" x1="0" x2="32" y1="12" y2="12" />
                <rect fill="#4B6072" height="8" width="6" x="13" y="24" />
              </g>

              {/* Isometric Eco Carpooling Minivan */}
              <g transform="translate(42, 115)">
                {/* Car Shadow */}
                <ellipse cx="140" cy="190" fill="#1C3844" fillOpacity="0.3" rx="90" ry="24" />

                {/* Car Wheels */}
                {/* Front-Left Wheel */}
                <ellipse cx="88" cy="180" fill="#29303D" rx="14" ry="17" />
                <ellipse cx="88" cy="180" fill="#9DAAB9" rx="7" ry="9" />

                {/* Front-Right / Mid Wheel */}
                <ellipse cx="195" cy="165" fill="#29303D" rx="14" ry="17" />
                <ellipse cx="195" cy="165" fill="#9DAAB9" rx="7" ry="9" />

                {/* Car Lower Chassis */}
                <path
                  d="M50 152 C 50 152 70 175 95 174 C 120 173 175 160 195 162 C 215 164 225 150 225 140 L 220 115 C 220 115 200 95 150 95 L 80 95 C 60 95 50 115 48 135 Z"
                  fill="#00A896"
                />

                {/* Side Body Highlight */}
                <path d="M55 130 L 218 108 L 222 135 L 53 150 Z" fill="#2EC4B6" opacity="0.4" />

                {/* Front Bumper and Lights */}
                <path
                  d="M48 135 C 48 145 55 155 70 160 L 68 142 C 60 138 52 135 48 135 Z"
                  fill="#028073"
                />
                <ellipse
                  cx="62"
                  cy="142"
                  fill="#FEF08A"
                  rx="7"
                  ry="4"
                  transform="rotate(-15, 62, 142)"
                />
                <rect fill="#FFFFFF" height="6" opacity="0.9" rx="2" width="22" x="68" y="152" />

                {/* Transparent Open Roof Frame & Glass Pillars */}
                <path
                  d="M 65 95 L 75 35 C 75 35 150 30 200 45 L 210 100"
                  fill="none"
                  opacity="0.8"
                  stroke="#FFFFFF"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth="4"
                />
                <path
                  d="M 135 34 L 140 98"
                  fill="none"
                  opacity="0.7"
                  stroke="#FFFFFF"
                  strokeLinecap="round"
                  strokeWidth="3"
                />

                {/* Glass tint fill */}
                <path
                  d="M 75 35 C 75 35 150 30 200 45 L 210 100 L 65 95 Z"
                  fill="#80E5FF"
                  fillOpacity="0.15"
                />

                {/* Happy Passengers & Driver Inside (4 People) */}
                {/* Passenger 1 (Driver, Front-Left, Waving) */}
                <g transform="translate(75, 52)">
                  {/* Torso */}
                  <path d="M 5 28 C 5 24 15 22 24 22 C 32 22 40 24 40 28 L 38 42 L 6 42 Z" fill="#0284C7" />
                  {/* Head */}
                  <circle cx="23" cy="13" fill="#FCD34D" r="9" />
                  {/* Hair */}
                  <path
                    d="M 14 13 C 14 6 19 4 25 4 C 30 4 33 8 32 12 C 30 11 25 11 22 13 Z"
                    fill="#4B382A"
                  />
                  {/* Arm Waving High */}
                  <path
                    d="M 10 24 L 2 12 L -6 4"
                    fill="none"
                    stroke="#FCD34D"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth="4"
                  />
                  {/* Smiling Face Features */}
                  <circle cx="20" cy="13" fill="#1F2937" r="1.2" />
                  <circle cx="25" cy="13" fill="#1F2937" r="1.2" />
                  <path
                    d="M 21 16 Q 23 18 25 16"
                    fill="none"
                    stroke="#1F2937"
                    strokeLinecap="round"
                    strokeWidth="1.2"
                  />
                </g>

                {/* Passenger 2 (Front-Right / Glasses Girl) */}
                <g transform="translate(112, 38)">
                  {/* Hair Back */}
                  <circle cx="20" cy="14" fill="#1E293B" r="12" />
                  {/* Torso */}
                  <path d="M 5 35 C 5 29 14 26 22 26 C 29 26 36 29 36 35 L 34 50 L 8 50 Z" fill="#F43F5E" />
                  {/* Neck & Face */}
                  <circle cx="20" cy="16" fill="#FED7AA" r="9" />
                  {/* Glasses */}
                  <rect
                    fill="none"
                    height="5"
                    rx="1.5"
                    stroke="#1E293B"
                    strokeWidth="1.2"
                    width="6"
                    x="14"
                    y="13"
                  />
                  <rect
                    fill="none"
                    height="5"
                    rx="1.5"
                    stroke="#1E293B"
                    strokeWidth="1.2"
                    width="6"
                    x="22"
                    y="13"
                  />
                  <line stroke="#1E293B" strokeWidth="1.2" x1="20" x2="22" y1="15" y2="15" />
                  {/* Smile */}
                  <path
                    d="M 18 20 Q 20 22 22 20"
                    fill="none"
                    stroke="#9A3412"
                    strokeLinecap="round"
                    strokeWidth="1"
                  />
                  {/* Hair Top / Ponytail */}
                  <ellipse cx="20" cy="7" fill="#1E293B" rx="9" ry="4" />
                  {/* Waving Hand */}
                  <path
                    d="M 10 28 L 2 16 L -2 6"
                    fill="none"
                    stroke="#FED7AA"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth="4"
                  />
                </g>

                {/* Passenger 3 (Back-Seat Guy 1) */}
                <g transform="translate(142, 60)">
                  <path d="M 4 20 C 4 16 12 14 18 14 C 24 14 30 16 30 20 L 29 34 L 5 34 Z" fill="#0D9488" />
                  <circle cx="17" cy="8" fill="#FDBA74" r="7.5" />
                  {/* Dark Hair */}
                  <path
                    d="M 10 7 C 10 2 14 1 18 1 C 23 1 25 3 25 7 C 22 6 18 6 15 8 Z"
                    fill="#334155"
                  />
                  {/* Smiling Face */}
                  <circle cx="15" cy="8" fill="#1F2937" r="1" />
                  <circle cx="19" cy="8" fill="#1F2937" r="1" />
                  <path
                    d="M 15 11 Q 17 13 19 11"
                    fill="none"
                    stroke="#1F2937"
                    strokeLinecap="round"
                    strokeWidth="1"
                  />
                </g>

                {/* Passenger 4 (Back-Seat Guy 2, Far Right, Friendly) */}
                <g transform="translate(170, 68)">
                  <path d="M 2 18 C 2 14 9 12 16 12 C 22 12 28 14 28 18 L 27 30 L 4 30 Z" fill="#3B82F6" />
                  <circle cx="15" cy="6" fill="#FCD34D" r="7.5" />
                  <path d="M 8 5 C 8 0 13 0 16 0 C 20 0 23 2 23 6 Z" fill="#78350F" />
                  {/* Hand on edge */}
                  <circle cx="6" cy="18" fill="#FCD34D" r="3.5" />
                  {/* Smile */}
                  <path
                    d="M 13 8 Q 15 10 17 8"
                    fill="none"
                    stroke="#1F2937"
                    strokeLinecap="round"
                    strokeWidth="1"
                  />
                </g>

                {/* Windshield reflections / Highlights */}
                <line
                  opacity="0.6"
                  stroke="#FFFFFF"
                  strokeLinecap="round"
                  strokeWidth="2.5"
                  x1="72"
                  x2="98"
                  y1="88"
                  y2="48"
                />
                <line
                  opacity="0.4"
                  stroke="#FFFFFF"
                  strokeLinecap="round"
                  strokeWidth="1.5"
                  x1="84"
                  x2="102"
                  y1="92"
                  y2="58"
                />
              </g>
            </svg>
          </div>
        </section>
        {/* END: HeroIllustrationSection */}

        {/* BEGIN: CarouselIndicatorDots */}
        <div
          className="flex justify-center items-center gap-2 pt-5 pb-2"
          data-purpose="carousel-pagination"
        >
          {/* Active Pill Dot (Teal) */}
          <button
            type="button"
            onClick={() => setCurrentSlideIndex(0)}
            aria-current={currentSlideIndex === 0 ? 'true' : undefined}
            aria-label="Slide 1 de 3"
            className={`h-1.5 rounded-full transition-all duration-300 cursor-pointer ${
              currentSlideIndex === 0 ? 'w-6 bg-[#00A896]' : 'w-1.5 bg-[#E5E7EB]'
            }`}
          />
          {/* Dot 2 */}
          <button
            type="button"
            onClick={() => setCurrentSlideIndex(1)}
            aria-current={currentSlideIndex === 1 ? 'true' : undefined}
            aria-label="Slide 2 de 3"
            className={`h-1.5 rounded-full transition-all duration-300 cursor-pointer ${
              currentSlideIndex === 1 ? 'w-6 bg-[#00A896]' : 'w-1.5 bg-[#E5E7EB]'
            }`}
          />
          {/* Dot 3 */}
          <button
            type="button"
            onClick={() => setCurrentSlideIndex(2)}
            aria-current={currentSlideIndex === 2 ? 'true' : undefined}
            aria-label="Slide 3 de 3"
            className={`h-1.5 rounded-full transition-all duration-300 cursor-pointer ${
              currentSlideIndex === 2 ? 'w-6 bg-[#00A896]' : 'w-1.5 bg-[#E5E7EB]'
            }`}
          />
        </div>
        {/* END: CarouselIndicatorDots */}

        {/* BEGIN: ValuePropositionText */}
        <section
          className="px-6 text-center flex flex-col items-center justify-center grow my-auto"
          data-purpose="content-headline"
        >
          <h1 className="text-[25px] font-extrabold text-[#1A1A1A] tracking-tight leading-[1.25] max-w-[320px]">
            Viaja acompañado, ahorra y muévete seguro.
          </h1>
          <p className="text-[#6B7280] text-[14.5px] leading-relaxed mt-3 max-w-[315px] font-normal">
            Únete a la comunidad de auto compartido más confiable. Conecta con conductores calificados y optimiza tus trayectos diarios.
          </p>
        </section>
        {/* END: ValuePropositionText */}

        {/* BEGIN: ActionButtonsGroup */}
        <section
          className="w-full px-6 pb-2 space-y-3 shrink-0"
          data-purpose="bottom-action-buttons"
        >
          {/* Primary Button: Iniciar Sesión */}
          <button
            id="welcome-login-btn"
            onClick={handleLogin}
            className="w-full h-[52px] bg-[#00A896] hover:bg-[#008F80] active:scale-[0.985] text-white font-semibold rounded-xl text-[16px] transition-all flex items-center justify-center shadow-sm cursor-pointer"
            data-purpose="login-button"
            type="button"
          >
            Iniciar Sesión
          </button>

          {/* Secondary Outline Button: Crear cuenta */}
          <button
            id="welcome-register-btn"
            onClick={handleRegister}
            className="w-full h-[52px] bg-white hover:bg-gray-50 active:scale-[0.985] border border-[#E5E7EB] text-[#1A1A1A] font-semibold rounded-xl text-[16px] transition-all flex items-center justify-center shadow-xs cursor-pointer"
            data-purpose="register-button"
            type="button"
          >
            Crear cuenta
          </button>
        </section>
        {/* END: ActionButtonsGroup */}

        {/* BEGIN: iOSHomeIndicator */}
        <footer
          className="w-full pb-2 pt-1 flex justify-center items-center shrink-0"
          data-purpose="ios-home-indicator"
        >
          {/* Standard iOS Home Bar */}
          <div className="w-[138px] h-[4.5px] bg-neutral-300 rounded-full" />
        </footer>
        {/* END: iOSHomeIndicator */}
      </main>
      {/* END: MobileIOSContainer */}
    </div>
  );
};

export default WelcomeScreen;
