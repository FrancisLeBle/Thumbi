package worker

import (
	"context"
	"log"
	"time"

	"github.com/thumbi/auth-kyc-service/internal/core/ports"
)

// ExpirationWorker ejecuta de manera periódica la expiración de reservas en PENDING_PAYMENT
// que hayan superado su TTL (15 min), restituyendo atómicamente los asientos retenidos.
type ExpirationWorker struct {
	bookingService ports.BookingService
	interval       time.Duration
}

// NewExpirationWorker inicializa el worker de expiración con intervalo configurable
func NewExpirationWorker(bookingService ports.BookingService, interval time.Duration) *ExpirationWorker {
	if interval <= 0 {
		interval = 1 * time.Minute
	}
	return &ExpirationWorker{
		bookingService: bookingService,
		interval:       interval,
	}
}

// Start inicia el ciclo de vida del worker en un bucle select con soporte de contexto
func (w *ExpirationWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	log.Printf("[INFO] ExpirationWorker iniciado. Frecuencia de chequeo: %v", w.interval)

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

// runOnce ejecuta un ciclo único de expiración con timeout controlado
func (w *ExpirationWorker) runOnce(ctx context.Context) {
	opCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	expiredCount, err := w.bookingService.ProcessExpiredBookings(opCtx)
	if err != nil {
		log.Printf("[ERROR] ExpirationWorker: error al procesar reservas vencidas: %v", err)
		return
	}

	if expiredCount > 0 {
		log.Printf("[INFO] ExpirationWorker: %d reservas pendientes vencidas fueron expiradas y sus asientos restituidos.", expiredCount)
	}
}
