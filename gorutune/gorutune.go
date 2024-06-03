package gorutune

import "sync"

// Глобальный атомарный счетчик горутин и карта имен горутин
var GoroutineCounter int32
var GoroutineNames sync.Map
