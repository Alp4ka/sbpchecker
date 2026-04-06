package sbpchecker

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/playwright-community/playwright-go"
)

// Client переиспользует один процесс Chromium; отдельный BrowserContext создаётся на каждый вызов FetchPaymentStatus.
// Параллельные вызовы допустимы в пределах потокобезопасности playwright-go.
type Client struct {
	pw      *playwright.Playwright
	browser playwright.Browser
	opts    Options
}

// NewClient запускает Playwright и поднимает Chromium с учётом Options.
func NewClient(opt Options) (*Client, error) {
	pw, err := playwright.Run()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrNoPlaywright, err)
	}

	launch := playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(opt.Headless),
		Args: []string{
			"--disable-dev-shm-usage",
			"--disable-blink-features=AutomationControlled",
		},
	}

	browser, err := pw.Chromium.Launch(launch)
	if err != nil {
		_ = pw.Stop()
		return nil, fmt.Errorf("%w: %w", ErrNoPlaywright, err)
	}

	return &Client{pw: pw, browser: browser, opts: opt}, nil
}

// Close завершает браузер и драйвер.
func (c *Client) Close() error {
	if c == nil {
		return nil
	}
	var err error
	if c.browser != nil {
		err = errors.Join(err, c.browser.Close())
		c.browser = nil
	}
	if c.pw != nil {
		err = errors.Join(err, c.pw.Stop())
		c.pw = nil
	}
	return err
}

// FetchPaymentStatus открывает ссылку в новом мобильном контексте и возвращает Result.
func (c *Client) FetchPaymentStatus(ctx context.Context, orderID string) (*Result, error) {
	if c == nil || c.browser == nil || c.pw == nil {
		return nil, ErrClosedClient
	} else if !_reOrderIDPattern.MatchString(orderID) {
		return nil, ErrInvalidOrderID
	} else if err := ctx.Err(); err != nil {
		return nil, err
	}

	payURL, err := getPayURL(orderID, c.opts)
	if err != nil {
		return nil, err
	}

	paymentLinkURL, err := getPaymentLinkURL(orderID, c.opts)
	if err != nil {
		return nil, err
	}

	browserContext, err := newBrowserContext(c.browser, c.pw, c.opts)
	if err != nil {
		return nil, err
	}
	defer func() { _ = browserContext.Close() }()

	page, err := browserContext.NewPage()
	if err != nil {
		return nil, err
	}
	applyPlaywrightTimeouts(browserContext, page, ctx, c.opts)

	navDoneCh := navDoneFuture(payURL, page)
	select {
	case <-ctx.Done():
		_ = browserContext.Close()
		<-navDoneCh
		return nil, ctx.Err()
	case navErr := <-navDoneCh:
		if navErr != nil {
			return nil, navErr
		}
	}

	postNavigateDelay := calculatePostNavigateDelay(ctx, c.opts)
	if postNavigateDelay > 0 {
		page.WaitForTimeout(float64(postNavigateDelay.Milliseconds()))
	}

	paymentLinkResponseFutureCh := paymentLinkResponseFuture(paymentLinkURL, page)
	var paymentLinkResponse nspkPaymentLinkResponse
	select {
	case <-ctx.Done():
		_ = browserContext.Close()
		// Возможно, потечет paymentLinkResponseFutureCh.
		return nil, ctx.Err()
	case paymentLinkResponse = <-paymentLinkResponseFutureCh:
	}

	if paymentLinkResponse.Error != nil {
		return nil, paymentLinkResponse.Error
	}

	status := PaymentStatusUnknown
	if paymentLinkResponse.NSPKResp != nil {
		switch code := paymentLinkResponse.NSPKResp.Code; code {
		case "RQ05301":
			status = PaymentStatusSuccess
		case "RQ00000":
			status = PaymentStatusPending
		}
	}
	return &Result{
		Status:         status,
		RemoteResponse: paymentLinkResponse.NSPKResp,
	}, nil
}

// FetchPaymentStatus поднимает временный браузер, открывает ссылку и сразу завершает процесс.
// Для серии запросов используйте NewClient и Client.FetchPaymentStatus.
func FetchPaymentStatus(ctx context.Context, orderID string, opt Options) (*Result, error) {
	c, err := NewClient(opt)
	if err != nil {
		return nil, err
	}
	defer func() { _ = c.Close() }()
	return c.FetchPaymentStatus(ctx, orderID)
}

func getPayURL(orderID string, opts Options) (*url.URL, error) {
	baseURL := nspkBaseURL
	if opts.NSPKBaseURL != "" {
		baseURL = opts.NSPKBaseURL
	}
	rawURL, _ := url.JoinPath(baseURL, orderID)

	return parseURL(rawURL)
}

func getPaymentLinkURL(orderID string, opts Options) (*url.URL, error) {
	paymentLinkBaseURL := nspkPaymentLinkBaseURL
	if opts.NSPKPaymentLinkBaseURL != "" {
		paymentLinkBaseURL = opts.NSPKPaymentLinkBaseURL
	}

	rawURL, _ := url.JoinPath(paymentLinkBaseURL, orderID)
	return parseURL(rawURL)
}

func calculatePostNavigateDelay(ctx context.Context, opts Options) time.Duration {
	postNavigateDelay := time.Duration(0)

	if opts.PostNavigateDelay > 0 {
		postNavigateDelay = opts.PostNavigateDelay
		if deadline, ok := ctx.Deadline(); ok {
			if timeRemains := time.Until(deadline); timeRemains < postNavigateDelay {
				postNavigateDelay = timeRemains
			}
		}
	}

	return postNavigateDelay
}

func navDoneFuture(url *url.URL, page playwright.Page) chan error {
	future := make(chan error, 1)

	go func() {
		defer close(future)
		_, e := page.Goto(url.String(), playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateCommit})
		future <- e
	}()

	return future
}

func paymentLinkResponseFuture(url *url.URL, page playwright.Page) chan nspkPaymentLinkResponse {
	future := make(chan nspkPaymentLinkResponse, 1)

	go func() {
		err := page.Route(
			url.String(),
			func(r playwright.Route) {
				defer close(future)

				resp, _ := r.Fetch()

				var dst NSPKPaymentLinkResponseBody
				err := resp.JSON(&dst)
				if err != nil {
					future <- nspkPaymentLinkResponse{Error: err}
				}

				future <- nspkPaymentLinkResponse{NSPKResp: &dst}
			},
		)
		if err != nil {
			future <- nspkPaymentLinkResponse{Error: err}
			close(future)
		}
	}()

	return future
}

func parseURL(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return nil, fmt.Errorf("%w: %s", ErrInvalidURL, raw)
	}

	return u, nil
}

func newBrowserContext(
	browser playwright.Browser,
	pw *playwright.Playwright,
	opt Options,
) (playwright.BrowserContext, error) {
	browserOptions := playwright.BrowserNewContextOptions{
		Locale:     playwright.String("ru-RU"),
		TimezoneId: playwright.String("Europe/Moscow"),
		ExtraHttpHeaders: map[string]string{
			"Accept-Language": "ru-RU,ru;q=0.9,en;q=0.4",
		},
	}

	if pwDeviceDesc, ok := pw.Devices["Pixel 5"]; ok {
		browserOptions.Viewport = pwDeviceDesc.Viewport
		browserOptions.UserAgent = playwright.String(pwDeviceDesc.UserAgent)
		browserOptions.DeviceScaleFactor = playwright.Float(pwDeviceDesc.DeviceScaleFactor)
		browserOptions.IsMobile = playwright.Bool(pwDeviceDesc.IsMobile)
		browserOptions.HasTouch = playwright.Bool(pwDeviceDesc.HasTouch)
	} else {
		browserOptions.Viewport = &playwright.Size{Width: 390, Height: 844}
		browserOptions.UserAgent = playwright.String("Mozilla/5.0 (Linux; Android 13; Pixel 7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Mobile Safari/537.36")
		browserOptions.DeviceScaleFactor = playwright.Float(2.75)
		browserOptions.IsMobile = playwright.Bool(true)
		browserOptions.HasTouch = playwright.Bool(true)
	}
	if len(opt.ExtraHTTPHeaders) > 0 {
		for k, v := range opt.ExtraHTTPHeaders {
			browserOptions.ExtraHttpHeaders[k] = v
		}
	}

	return browser.NewContext(browserOptions)
}

func applyPlaywrightTimeouts(
	browserCtx playwright.BrowserContext,
	page playwright.Page,
	ctx context.Context,
	opt Options,
) {
	ms := navigationTimeoutMs(ctx, opt)
	browserCtx.SetDefaultTimeout(ms)
	browserCtx.SetDefaultNavigationTimeout(ms)
	page.SetDefaultTimeout(ms)
	page.SetDefaultNavigationTimeout(ms)
}

func navigationTimeoutMs(ctx context.Context, opt Options) float64 {
	if opt.NavigationTimeout > 0 {
		return float64(opt.NavigationTimeout.Milliseconds())
	}
	if t, ok := ctx.Deadline(); ok {
		if rem := time.Until(t); rem > 0 {
			return float64(rem.Milliseconds())
		}
	}

	return float64((DefaultNavigationTimeout).Milliseconds())
}

type nspkPaymentLinkResponse struct {
	Error    error
	NSPKResp *NSPKPaymentLinkResponseBody
}
