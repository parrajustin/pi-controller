package browser

import (
	"context"
	"time"

	"github.com/chromedp/chromedp"
)

// Browser interface abstracts the chromedp calls for easier testing.
type Browser interface {
	Location() (string, error)
	Evaluate(script string, res interface{}) error
	WaitVisible(selector string, byID bool) error
	Click(selector string, byID bool) error
	SendKeys(selector, keys string, byID bool) error
	Sleep(d time.Duration) error
	Navigate(url string) error
	OuterHTML(selector string, res *string) error
	CaptureScreenshot(res *[]byte) error
}

// RealBrowser is a wrapper around chromedp.
type RealBrowser struct {
	ctx context.Context
}

// NewRealBrowser creates a new RealBrowser instance.
func NewRealBrowser(ctx context.Context) *RealBrowser {
	return &RealBrowser{ctx: ctx}
}

func (b *RealBrowser) Location() (string, error) {
	var urlStr string
	err := chromedp.Run(b.ctx, chromedp.Location(&urlStr))
	return urlStr, err
}

func (b *RealBrowser) Evaluate(script string, res interface{}) error {
	return chromedp.Run(b.ctx, chromedp.Evaluate(script, res))
}

func (b *RealBrowser) WaitVisible(selector string, byID bool) error {
	var sel interface{} = chromedp.ByQuery
	if byID {
		sel = chromedp.ByID
	}
	return chromedp.Run(b.ctx, chromedp.WaitVisible(selector, sel.(func(*chromedp.Selector))))
}

func (b *RealBrowser) Click(selector string, byID bool) error {
	var sel interface{} = chromedp.ByQuery
	if byID {
		sel = chromedp.ByID
	}
	return chromedp.Run(b.ctx, chromedp.Click(selector, sel.(func(*chromedp.Selector))))
}

func (b *RealBrowser) SendKeys(selector, keys string, byID bool) error {
	var sel interface{} = chromedp.ByQuery
	if byID {
		sel = chromedp.ByID
	}
	return chromedp.Run(b.ctx, chromedp.SendKeys(selector, keys, sel.(func(*chromedp.Selector))))
}

func (b *RealBrowser) Sleep(d time.Duration) error {
	return chromedp.Run(b.ctx, chromedp.Sleep(d))
}

func (b *RealBrowser) Navigate(url string) error {
	return chromedp.Run(b.ctx, chromedp.Navigate(url))
}

func (b *RealBrowser) OuterHTML(selector string, res *string) error {
	return chromedp.Run(b.ctx, chromedp.OuterHTML(selector, res, chromedp.ByQuery))
}

func (b *RealBrowser) CaptureScreenshot(res *[]byte) error {
	return chromedp.Run(b.ctx, chromedp.CaptureScreenshot(res))
}
