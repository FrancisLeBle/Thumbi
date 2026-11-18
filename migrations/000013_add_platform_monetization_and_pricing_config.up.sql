-- 000013_add_platform_monetization_and_pricing_config.up.sql
-- Modelo de Negocio Fase 1: Passenger Service Fee parametrizable, retención operativa en cancelaciones/reembolsos
-- y desglose contable inmutable en bookings, escrow_transactions y refund_transactions.

-- 1. Tabla de configuración parametrizable de tarifas y políticas de cancelación (gestionada por ADMIN)
CREATE TABLE IF NOT EXISTS platform_pricing_config (
    id UUID PRIMARY KEY,
    currency CHAR(3) NOT NULL DEFAULT 'ARS',
    passenger_service_fee_bps INT NOT NULL DEFAULT 1500,
    passenger_fixed_fee NUMERIC(10, 2) NOT NULL DEFAULT 0.00,
    early_cancel_fee_retention_bps INT NOT NULL DEFAULT 10000,
    late_cancel_window_hours INT NOT NULL DEFAULT 24,
    late_cancel_fee_retention_bps INT NOT NULL DEFAULT 10000,
    late_cancel_passenger_refund_bps INT NOT NULL DEFAULT 5000,
    late_cancel_driver_comp_bps INT NOT NULL DEFAULT 5000,
    late_cancel_platform_retention_bps INT NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    updated_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_ppc_currency_iso4217 CHECK (currency ~ '^[A-Z]{3}$'),
    CONSTRAINT chk_ppc_service_fee_bps CHECK (passenger_service_fee_bps >= 0 AND passenger_service_fee_bps <= 5000),
    CONSTRAINT chk_ppc_fixed_fee_non_negative CHECK (passenger_fixed_fee >= 0),
    CONSTRAINT chk_ppc_early_fee_retention CHECK (early_cancel_fee_retention_bps >= 0 AND early_cancel_fee_retention_bps <= 10000),
    CONSTRAINT chk_ppc_late_window_hours CHECK (late_cancel_window_hours >= 1 AND late_cancel_window_hours <= 168),
    CONSTRAINT chk_ppc_late_fee_retention CHECK (late_cancel_fee_retention_bps >= 0 AND late_cancel_fee_retention_bps <= 10000),
    CONSTRAINT chk_ppc_late_passenger_bps CHECK (late_cancel_passenger_refund_bps >= 0 AND late_cancel_passenger_refund_bps <= 10000),
    CONSTRAINT chk_ppc_late_driver_bps CHECK (late_cancel_driver_comp_bps >= 0 AND late_cancel_driver_comp_bps <= 10000),
    CONSTRAINT chk_ppc_late_platform_bps CHECK (late_cancel_platform_retention_bps >= 0 AND late_cancel_platform_retention_bps <= 10000),
    CONSTRAINT chk_ppc_late_cancel_split_sum CHECK (
        late_cancel_passenger_refund_bps + late_cancel_driver_comp_bps + late_cancel_platform_retention_bps = 10000
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_platform_pricing_config_active
    ON platform_pricing_config (is_active)
    WHERE is_active = TRUE;

-- Semilla inicial por defecto (15% Service Fee al pasajero, retención 100% del fee en cancelaciones de pasajero, 50/50 base en cancelación tardía)
INSERT INTO platform_pricing_config (
    id,
    currency,
    passenger_service_fee_bps,
    passenger_fixed_fee,
    early_cancel_fee_retention_bps,
    late_cancel_window_hours,
    late_cancel_fee_retention_bps,
    late_cancel_passenger_refund_bps,
    late_cancel_driver_comp_bps,
    late_cancel_platform_retention_bps,
    is_active
)
SELECT
    '01960000-0000-7000-8000-000000000001'::uuid,
    'ARS',
    1500,
    0.00,
    10000,
    24,
    10000,
    5000,
    5000,
    0,
    TRUE
WHERE NOT EXISTS (
    SELECT 1 FROM platform_pricing_config WHERE is_active = TRUE
);

-- 2. Evolución de bookings para congelar desglose base_amount + platform_fee = total_price
ALTER TABLE bookings
    DROP CONSTRAINT IF EXISTS chk_booking_total_price;

ALTER TABLE bookings
    ADD COLUMN IF NOT EXISTS base_amount NUMERIC(10, 2),
    ADD COLUMN IF NOT EXISTS platform_fee NUMERIC(10, 2);

UPDATE bookings
SET
    base_amount = COALESCE(base_amount, ROUND(unit_price * seats_booked, 2)),
    platform_fee = COALESCE(platform_fee, GREATEST(total_price - ROUND(unit_price * seats_booked, 2), 0.00))
WHERE base_amount IS NULL OR platform_fee IS NULL;

ALTER TABLE bookings
    ALTER COLUMN base_amount SET DEFAULT 0.00,
    ALTER COLUMN base_amount SET NOT NULL,
    ALTER COLUMN platform_fee SET DEFAULT 0.00,
    ALTER COLUMN platform_fee SET NOT NULL;

ALTER TABLE bookings
    DROP CONSTRAINT IF EXISTS chk_booking_base_amount_non_negative,
    DROP CONSTRAINT IF EXISTS chk_booking_platform_fee_non_negative,
    DROP CONSTRAINT IF EXISTS chk_booking_base_amount;

ALTER TABLE bookings
    ADD CONSTRAINT chk_booking_base_amount_non_negative CHECK (base_amount >= 0),
    ADD CONSTRAINT chk_booking_platform_fee_non_negative CHECK (platform_fee >= 0),
    ADD CONSTRAINT chk_booking_base_amount CHECK (base_amount = ROUND(unit_price * seats_booked, 2)),
    ADD CONSTRAINT chk_booking_total_price CHECK (total_price = base_amount + platform_fee);

-- 3. Evolución de escrow_transactions para separar base_amount (payout conductor) y platform_fee (ingreso Thumbi)
ALTER TABLE escrow_transactions
    ADD COLUMN IF NOT EXISTS base_amount NUMERIC(10, 2),
    ADD COLUMN IF NOT EXISTS platform_fee NUMERIC(10, 2);

UPDATE escrow_transactions
SET
    base_amount = COALESCE(base_amount, amount),
    platform_fee = COALESCE(platform_fee, 0.00)
WHERE base_amount IS NULL OR platform_fee IS NULL;

ALTER TABLE escrow_transactions
    ALTER COLUMN base_amount SET DEFAULT 0.00,
    ALTER COLUMN base_amount SET NOT NULL,
    ALTER COLUMN platform_fee SET DEFAULT 0.00,
    ALTER COLUMN platform_fee SET NOT NULL;

ALTER TABLE escrow_transactions
    DROP CONSTRAINT IF EXISTS chk_escrow_base_amount_non_negative,
    DROP CONSTRAINT IF EXISTS chk_escrow_platform_fee_non_negative,
    DROP CONSTRAINT IF EXISTS chk_escrow_amount_breakdown;

ALTER TABLE escrow_transactions
    ADD CONSTRAINT chk_escrow_base_amount_non_negative CHECK (base_amount >= 0),
    ADD CONSTRAINT chk_escrow_platform_fee_non_negative CHECK (platform_fee >= 0),
    ADD CONSTRAINT chk_escrow_amount_breakdown CHECK (amount = base_amount + platform_fee);

-- 4. Evolución de refund_transactions para registrar retención operativa de la plataforma
ALTER TABLE refund_transactions
    DROP CONSTRAINT IF EXISTS chk_refund_amounts_positive;

ALTER TABLE refund_transactions
    ADD COLUMN IF NOT EXISTS platform_retained_amount NUMERIC(10, 2) NOT NULL DEFAULT 0.00;

ALTER TABLE refund_transactions
    DROP CONSTRAINT IF EXISTS chk_refund_platform_retained_non_negative;

ALTER TABLE refund_transactions
    ADD CONSTRAINT chk_refund_platform_retained_non_negative CHECK (platform_retained_amount >= 0),
    ADD CONSTRAINT chk_refund_amounts_positive CHECK (
        passenger_refund_amount + driver_compensation_amount + platform_retained_amount > 0
    );
