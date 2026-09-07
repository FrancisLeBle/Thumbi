package notifier

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/thumbi/auth-kyc-service/internal/core/ports"
)

type DispatchedNotification struct {
	Payload   ports.NotificationPayload
	Timestamp time.Time
}

// PushNotifier implementa ports.Notifier simulando el envío de notificaciones push / email / SNS
type PushNotifier struct {
	mu           sync.RWMutex
	dispatched   []DispatchedNotification
	OnNotifyHook func(payload ports.NotificationPayload)
}

// NewPushNotifier inicializa el notificador
func NewPushNotifier() *PushNotifier {
	return &PushNotifier{
		dispatched: make([]DispatchedNotification, 0),
	}
}

func (n *PushNotifier) SendResolutionNotification(ctx context.Context, payload ports.NotificationPayload) error {
	n.mu.Lock()
	n.dispatched = append(n.dispatched, DispatchedNotification{
		Payload:   payload,
		Timestamp: time.Now().UTC(),
	})
	hook := n.OnNotifyHook
	n.mu.Unlock()

	log.Printf("[NOTIFIER] [%s] Destinatario: %s (%s) | Título: %q | Mensaje: %q",
		payload.Type, payload.Email, payload.UserID, payload.Title, payload.Message)

	if hook != nil {
		hook(payload)
	}

	return nil
}

// GetHistory retorna una copia del historial de notificaciones emitidas (útil para auditoría y testing)
func (n *PushNotifier) GetHistory() []DispatchedNotification {
	n.mu.RLock()
	defer n.mu.RUnlock()

	copyList := make([]DispatchedNotification, len(n.dispatched))
	copy(copyList, n.dispatched)
	return copyList
}

var _ ports.Notifier = (*PushNotifier)(nil)
