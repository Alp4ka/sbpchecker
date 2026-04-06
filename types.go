package sbpchecker

import (
	"fmt"
	"regexp"
	"time"
)

const (
	errPrefix = "sbpstatus"

	nspkBaseURL            = "https://qr.nspk.ru/"
	nspkPaymentLinkBaseURL = "https://upc.cbrpay.ru/v1/payment-link/"

	orderIDPattern = `^[A-Z0-9]{32}$`
)

var _reOrderIDPattern = regexp.MustCompile(orderIDPattern)

const DefaultNavigationTimeout = 25 * time.Second

// PaymentStatus описывает грубую классификацию состояния платежа по содержимому страницы.
type PaymentStatus string

const (
	// PaymentStatusUnknown означает, что по странице нельзя однозначно вывести статус. Так, заявки с протекшим ТТЛ -
	// однозначно определить нельзя.
	PaymentStatusUnknown PaymentStatus = "unknown"
	// PaymentStatusPending соответствует ожиданию оплаты или обработке.
	PaymentStatusPending PaymentStatus = "pending"
	// PaymentStatusSuccess соответствует успешному завершению.
	PaymentStatusSuccess PaymentStatus = "success"
	// PaymentStatusFailed соответствует отказу, ошибке.
	PaymentStatusFailed PaymentStatus = "failed"
)

// Result агрегирует итог обхода ссылки и распознавания статуса.
type Result struct {
	// Status — нормализованный статус платежа.
	Status PaymentStatus
	// Detail — краткое пояснение (например, найденная фраза или сообщение детектора).
	Details        string
	RemoteResponse *NSPKPaymentLinkResponseBody
}

// Options настраивает браузер, сеть и распознавание статуса.
type Options struct {
	// Headless включает безголовый режим Chromium (по умолчанию false).
	Headless bool

	// EntityPoolSize задаёт количество параллельно обрабатываемых запросов.
	// Каждая "сущность" может обрабатывать только один запрос одновременно.
	// Если 0, используется 1.
	EntityPoolSize int

	// NavigationTimeout — лимит навигации. Ноль означает взять значение из context.Deadline,
	// или, в случае его отсутствия, использовать DefaultNavigationTimeout.
	NavigationTimeout time.Duration

	// PostNavigateDelay — дополнительная пауза после навигации для отрисовки SPA (0 — не ждать).
	PostNavigateDelay time.Duration

	// ExtraHTTPHeaders добавляется ко всем запросам контекста.
	ExtraHTTPHeaders map[string]string

	// NSPKBaseURL – начало ссылки на платежный сервис НСПК.
	// По-умолчанию: https://qr.nspk.ru/
	NSPKBaseURL string

	// NSPKPaymentLinkBaseURL – начало ссылки на АПИ получения информации о платеже.
	// По-умолчанию: https://upc.cbrpay.ru/v1/payment-link/
	NSPKPaymentLinkBaseURL string
}

var (
	// ErrInvalidURL возвращается, если ссылка не является допустимым HTTPS URL.
	ErrInvalidURL = fmt.Errorf("%s: invalid url", errPrefix)
	// ErrNoPlaywright возвращается, если не удалось запустить Playwright (часто из-за отсутствия install).
	ErrNoPlaywright = fmt.Errorf("%s: no playwright", errPrefix)
	// ErrClosedClient возвращается, когда клиент закрыт или не инициализирован.
	ErrClosedClient = fmt.Errorf("%s: client is closed", errPrefix)
	// ErrInvalidOrderID возвращается, когда передан неверный ID заказа.
	ErrInvalidOrderID = fmt.Errorf("%s: invalid order ID", errPrefix)
)

type NSPKPaymentLinkResponseBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}
