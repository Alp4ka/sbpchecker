// Package sbpchecker
//
// Предоставляет извлечение статуса оплаты по веб-ссылке СБП (Система быстрых платежей)
// через автоматизацию Chromium в безголовом режиме с эмуляцией мобильного устройства.
//
// Перед первым запуском необходимо установить браузеры и драйвер Playwright, совместимые с версией
// модуля github.com/playwright-community/playwright-go в вашем go.mod, например:
//
//	go run github.com/playwright-community/playwright-go/cmd/playwright@v0.5700.1 install --with-deps
package sbpchecker
