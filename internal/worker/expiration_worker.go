package worker

import (
	"context"
	"log"
	"time"
)

const (
	// ExpirationBatchSize coincide con el límite de lote en PostgreSQL (LIMIT 500)
	// para detectar si quedaron filas pendientes por drenar tras una ejecución.
	ExpirationBatchSize = 500
	maxBatchesPerTick   = 10
)

// BookingExpirationService define el contrato mínimo requerido por ExpirationWorker (SOLID · ISP).
type BookingExpirationService interface {
	ProcessExpiredBookings(ctx context.Context) (int, error)
}

// ExpirationWorker ejecuta de manera periódica la expiración en lotes de reservas en PENDING_PAYMENT
// que hayan superado su TTL (15 min), restituyendo atómicamente los asientos retenidos.
// Es seguro para ejecutarse en múltiples réplicas concurrentes gracias a FOR UPDATE SKIP LOCKED.
type ExpirationWorker struct {
	expirationService BookingExpirationService
	interval          time.Duration
}

// NewExpirationWorker inicializa el worker de expiración con intervalo configurable
func NewExpirationWorker(expirationService BookingExpirationService, interval time.Duration) *ExpirationWorker {
	if interval <= 0 {
		interval = 1 * time.Minute
	}
	return &ExpirationWorker{
		expirationService: expirationService,
		interval:          interval,
	}
}

// Start inicia el ciclo de vida del worker en un bucle select con soporte de contexto
func (w *ExpirationWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	log.Printf("[INFO] ExpirationWorker iniciado. Frecuencia de chequeo: %v | Tamaño de lote: %d", w.interval, ExpirationBatchSize)

	for {
		select {
		case <-ctx.Done():
			log.Println("[INFO] ExpirationWorker detenido ordenadamente por cancelación de contexto.")
			return

		case <-ticker.C:
			w.runOnce(ctx)
		}
	}
}

// runOnce drena lotes acotados (hasta ExpirationBatchSize por transacción) dentro de un timeout controlado.
func (w *ExpirationWorker) runOnce(ctx context.Context) {
	opCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	totalExpired := 0
	for batch := 0; batch < maxBatchesPerTick; batch++ {
		if opCtx.Err() != nil {
			break
		}

		expiredCount, err := w.expirationService.ProcessExpiredBookings(opCtx)
		if err != nil {
			log.Printf("[ERROR] ExpirationWorker: error al procesar lote de reservas vencidas: %v", err)
			break
		}

		totalExpired += expiredCount
		if expiredCount < ExpirationBatchSize {
			break
		}
	}

	if totalExpired > 0 {
		log.Printf("[INFO] ExpirationWorker: %d reservas pendientes vencidas fueron expiradas y sus asientos restituidos.", totalExpired)
	}
}
