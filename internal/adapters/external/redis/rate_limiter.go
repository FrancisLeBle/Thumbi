package redis

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/thumbi/auth-kyc-service/internal/core/ports"
)

// RedisRateLimiter implementa ports.RateLimiter utilizando Redis (mediante protocolo RESP nativo)
// con fallback automático en memoria para alta disponibilidad y resiliencia ante caídas de red.
type RedisRateLimiter struct {
	addr        string
	dialTimeout time.Duration
	fallback    *memoryRateLimiter
}

// NewRateLimiter crea una nueva instancia del limitador de tasa respaldado por Redis.
// Acepta una dirección en formato "host:port" o URL "redis://host:port".
func NewRateLimiter(rawAddr string) ports.RateLimiter {
	cleanAddr := strings.TrimSpace(rawAddr)
	cleanAddr = strings.TrimPrefix(cleanAddr, "redis://")
	cleanAddr = strings.TrimPrefix(cleanAddr, "tcp://")
	if cleanAddr == "" {
		cleanAddr = "localhost:6379"
	}
	if !strings.Contains(cleanAddr, ":") {
		cleanAddr = cleanAddr + ":6379"
	}

	return &RedisRateLimiter{
		addr:        cleanAddr,
		dialTimeout: 250 * time.Millisecond,
		fallback:    newMemoryRateLimiter(),
	}
}

// Allow evalúa atómicamente si una clave puede realizar una solicitud dentro de la ventana de tiempo.
// Utiliza comandos nativos de Redis (INCR, EXPIRE, TTL). Si Redis no responde, recurre al limitador en memoria.
func (r *RedisRateLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, int, time.Duration, error) {
	if limit <= 0 {
		return false, 0, window, nil
	}

	windowSeconds := int(window.Seconds())
	if windowSeconds <= 0 {
		windowSeconds = 1
	}

	// 1. Intentar ejecutar el incremento atómico en Redis
	count, ttlSec, err := r.executeRedisRateLimit(key, windowSeconds)
	if err != nil {
		// Redis no disponible o error de red: fallback transparente a memoria local
		return r.fallback.Allow(ctx, key, limit, window)
	}

	resetDur := time.Duration(ttlSec) * time.Second
	if resetDur <= 0 {
		resetDur = window
	}

	if count > limit {
		return false, 0, resetDur, nil
	}

	remaining := limit - count
	if remaining < 0 {
		remaining = 0
	}

	return true, remaining, resetDur, nil
}

// executeRedisRateLimit ejecuta los comandos INCR, EXPIRE y TTL mediante protocolo RESP
func (r *RedisRateLimiter) executeRedisRateLimit(key string, windowSeconds int) (int, int, error) {
	conn, err := net.DialTimeout("tcp", r.addr, r.dialTimeout)
	if err != nil {
		return 0, 0, err
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(500 * time.Millisecond))

	reader := bufio.NewReader(conn)

	// Pipeline de comandos: INCR key seguido de TTL key
	// Formato RESP: *2\r\n$4\r\nINCR\r\n$<len>\r\n<key>\r\n
	incrCmd := fmt.Sprintf("*2\r\n$4\r\nINCR\r\n$%d\r\n%s\r\n", len(key), key)
	if _, err := conn.Write([]byte(incrCmd)); err != nil {
		return 0, 0, err
	}

	count, err := readIntResponse(reader)
	if err != nil {
		return 0, 0, err
	}

	// Si es la primera solicitud en la ventana (count == 1), establecer el TTL
	if count == 1 {
		expireCmd := fmt.Sprintf("*3\r\n$6\r\nEXPIRE\r\n$%d\r\n%s\r\n$%d\r\n%d\r\n", len(key), key, len(strconv.Itoa(windowSeconds)), windowSeconds)
		if _, err := conn.Write([]byte(expireCmd)); err != nil {
			return count, windowSeconds, nil
		}
		_, _ = readIntResponse(reader)
		return count, windowSeconds, nil
	}

	// Consultar TTL restante
	ttlCmd := fmt.Sprintf("*2\r\n$3\r\nTTL\r\n$%d\r\n%s\r\n", len(key), key)
	if _, err := conn.Write([]byte(ttlCmd)); err != nil {
		return count, windowSeconds, nil
	}

	ttl, err := readIntResponse(reader)
	if err != nil || ttl <= 0 {
		ttl = windowSeconds
	}

	return count, ttl, nil
}

// readIntResponse parsea una respuesta numérica de tipo entero en RESP (ej: ":42\r\n")
func readIntResponse(reader *bufio.Reader) (int, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return 0, err
	}
	line = strings.TrimRight(line, "\r\n")
	if len(line) == 0 {
		return 0, fmt.Errorf("empty redis response")
	}

	if line[0] == ':' {
		return strconv.Atoi(line[1:])
	}
	if line[0] == '-' {
		return 0, fmt.Errorf("redis error: %s", line[1:])
	}

	return 0, fmt.Errorf("unexpected redis response type: %s", line)
}

// memoryRateLimiter estructura de respaldo local en memoria cuando Redis no está accesible
type memoryRateLimiter struct {
	mu      sync.Mutex
	entries map[string]*rateLimitEntry
}

type rateLimitEntry struct {
	count     int
	expiresAt time.Time
}

func newMemoryRateLimiter() *memoryRateLimiter {
	return &memoryRateLimiter{
		entries: make(map[string]*rateLimitEntry),
	}
}

func (m *memoryRateLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, int, time.Duration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now().UTC()
	entry, exists := m.entries[key]

	if !exists || now.After(entry.expiresAt) {
		m.entries[key] = &rateLimitEntry{
			count:     1,
			expiresAt: now.Add(window),
		}
		remaining := limit - 1
		if remaining < 0 {
			remaining = 0
		}
		return true, remaining, window, nil
	}

	entry.count++
	resetDur := entry.expiresAt.Sub(now)
	if resetDur <= 0 {
		resetDur = window
	}

	if entry.count > limit {
		return false, 0, resetDur, nil
	}

	return true, limit - entry.count, resetDur, nil
}
