package cache

import (
	"context"
	"sync"
	"time"
)

// minSweepSize — раньше этого числа записей чистить нечего.
const minSweepSize = 64

// Memory — кэш в памяти процесса. Используется, когда Redis не настроен или
// недоступен (SPEC.md 3.1 допускает «sync.Map + TTL» как альтернативу Redis).
//
// Отличие от Redis по смыслу одно: кэш перестаёт быть общим между инстансами
// и обнуляется при рестарте.
type Memory struct {
	mu    sync.RWMutex
	items map[string]memoryItem

	// nextSweep — размер, при котором пора выбрасывать протухшие записи.
	// Без этого ключи с истёкшим TTL копились бы вечно: ленивое вытеснение
	// на Get освобождает только то, что перечитывают.
	nextSweep int

	now func() time.Time
}

type memoryItem struct {
	value   []byte
	expires time.Time
}

// NewMemory создаёт пустой кэш в памяти.
func NewMemory() *Memory {
	return &Memory{
		items:     make(map[string]memoryItem),
		nextSweep: minSweepSize,
		now:       time.Now,
	}
}

func (m *Memory) Get(_ context.Context, key string) ([]byte, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	item, ok := m.items[key]
	if !ok || !item.expires.After(m.now()) {
		return nil, false, nil
	}

	// Своя копия: Redis на каждый Get отдаёт свежий срез, и вызывающий код вправе
	// рассчитывать на такое же поведение здесь.
	return append([]byte(nil), item.value...), true, nil
}

func (m *Memory) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	if ttl <= 0 {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.items) >= m.nextSweep {
		m.sweep()
	}

	m.items[key] = memoryItem{
		value:   append([]byte(nil), value...),
		expires: m.now().Add(ttl),
	}

	return nil
}

func (m *Memory) Close() error { return nil }

// sweep выбрасывает протухшие записи. Вызывается под уже взятым m.mu.
func (m *Memory) sweep() {
	now := m.now()

	for key, item := range m.items {
		if !item.expires.After(now) {
			delete(m.items, key)
		}
	}

	if next := 2 * len(m.items); next > minSweepSize {
		m.nextSweep = next
	} else {
		m.nextSweep = minSweepSize
	}
}
