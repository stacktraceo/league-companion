package storage

import "errors"

// ErrNotFound — запрошенной строки нет. Хендлеры переводят её в 404 (SPEC.md 3.4).
var ErrNotFound = errors.New("storage: запись не найдена")
