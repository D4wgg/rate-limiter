package limiter

import (
	"context"
	"sync"
	"time"
)

// Limiter описывает общий интерфейс для rate limiter'ов.
type Limiter interface {
	Allow(ctx context.Context, key string, window time.Duration, limit int) (bool, error)
}

// MemoryLimiter — простой fixed-window rate limiter в памяти.
// Подходит для работы "из коробки" без внешних зависимостей.
// В кластере лимиты будут относиться к каждому инстансу отдельно.
type MemoryLimiter struct {
	mu          sync.RWMutex
	windows     map[string]*windowState
	stopCleanup chan struct{}
	cleanupDone sync.WaitGroup
}

type windowState struct {
	mu          sync.Mutex
	windowStart time.Time
	count       int
}

func NewMemoryLimiter() *MemoryLimiter {
	ml := &MemoryLimiter{
		windows:     make(map[string]*windowState),
		stopCleanup: make(chan struct{}),
	}

	// Запускаем фоновую очистку старых окон каждые 5 минут
	ml.cleanupDone.Add(1)
	go ml.cleanupLoop(5 * time.Minute)

	return ml
}

// cleanupLoop периодически удаляет старые окна, которые не использовались долгое время.
func (m *MemoryLimiter) cleanupLoop(interval time.Duration) {
	defer m.cleanupDone.Done()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.cleanup(interval * 2) // Удаляем окна, не использовавшиеся более 2 интервалов
		case <-m.stopCleanup:
			return
		}
	}
}

// cleanup удаляет окна, которые не использовались дольше threshold.
func (m *MemoryLimiter) cleanup(threshold time.Duration) {
	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()

	for key, ws := range m.windows {
		ws.mu.Lock()
		lastUsed := ws.windowStart
		if lastUsed.IsZero() {
			lastUsed = now
		}
		ws.mu.Unlock()

		if now.Sub(lastUsed) > threshold {
			delete(m.windows, key)
		}
	}
}

// Close останавливает фоновую очистку и освобождает ресурсы.
func (m *MemoryLimiter) Close() error {
	close(m.stopCleanup)
	m.cleanupDone.Wait()
	return nil
}

// Allow реализует фиксированное окно:
// - для каждого key храним пару (windowStart, count)
// - если текущее время вышло за пределы window, окно сбрасывается
// - затем инкрементируем count и сравниваем с limit.
func (m *MemoryLimiter) Allow(_ context.Context, key string, window time.Duration, limit int) (bool, error) {
	now := time.Now()

	// Быстрый путь: получить/создать окно для ключа.
	ws := m.getOrCreateWindow(key)

	ws.mu.Lock()
	defer ws.mu.Unlock()

	if ws.windowStart.IsZero() || now.Sub(ws.windowStart) >= window {
		ws.windowStart = now
		ws.count = 0
	}

	ws.count++
	if ws.count > limit {
		return false, nil
	}

	return true, nil
}

func (m *MemoryLimiter) getOrCreateWindow(key string) *windowState {
	m.mu.RLock()
	ws, ok := m.windows[key]
	m.mu.RUnlock()
	if ok {
		return ws
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	ws, ok = m.windows[key]
	if ok {
		return ws
	}

	ws = &windowState{}
	m.windows[key] = ws
	return ws
}

