package browser

import (
	"context"
	"fmt"
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
	Reload() error
	InjectCursor() error
	MoveCursor(deltaX, deltaY float64) error
	ClickAtCursor(button string) error
	ScrollAtCursor(deltaY float64) error
	SendKeyboardInput(keys string) error
	HistoryBack() error
	HistoryForward() error
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

func (b *RealBrowser) Reload() error {
	return chromedp.Run(b.ctx, chromedp.Reload())
}

func (b *RealBrowser) InjectCursor() error {
	script := `
		(function() {
			if (document.getElementById('__virtual_cursor')) return;
			const cursor = document.createElement('div');
			cursor.id = '__virtual_cursor';
			cursor.style.position = 'fixed';
			cursor.style.left = '50%';
			cursor.style.top = '50%';
			cursor.style.width = '15px';
			cursor.style.height = '15px';
			cursor.style.borderRadius = '50%';
			cursor.style.backgroundColor = 'rgba(255, 0, 0, 0.7)';
			cursor.style.border = '2px solid white';
			cursor.style.zIndex = '999999';
			cursor.style.pointerEvents = 'none';
			cursor.style.transform = 'translate(-50%, -50%)';
			document.body.appendChild(cursor);
			window.__cursorX = window.innerWidth / 2;
			window.__cursorY = window.innerHeight / 2;
		})();
	`
	return b.Evaluate(script, nil)
}

func (b *RealBrowser) MoveCursor(deltaX, deltaY float64) error {
	script := fmt.Sprintf(`
		(function(dx, dy) {
			const c = document.getElementById('__virtual_cursor');
			if (!c) return;
			window.__cursorX = Math.max(0, Math.min(window.innerWidth, window.__cursorX + dx));
			window.__cursorY = Math.max(0, Math.min(window.innerHeight, window.__cursorY + dy));
			c.style.left = window.__cursorX + 'px';
			c.style.top = window.__cursorY + 'px';
		})(%f, %f);
	`, deltaX, deltaY)
	return b.Evaluate(script, nil)
}

func (b *RealBrowser) ClickAtCursor(button string) error {
	script := fmt.Sprintf(`
		(function(btn) {
			if (typeof window.__cursorX === 'undefined') return false;
			const el = document.elementFromPoint(window.__cursorX, window.__cursorY);
			if (el) {
				const ev = new MouseEvent(btn === 'right' ? 'contextmenu' : 'click', {
					view: window,
					bubbles: true,
					cancelable: true,
					clientX: window.__cursorX,
					clientY: window.__cursorY,
					button: btn === 'right' ? 2 : (btn === 'middle' ? 1 : 0)
				});
				el.dispatchEvent(ev);
				if (el.tagName === 'INPUT' || el.tagName === 'TEXTAREA') {
					el.focus();
				}
				return true;
			}
			return false;
		})('%s');
	`, button)
	return b.Evaluate(script, nil)
}

func (b *RealBrowser) ScrollAtCursor(deltaY float64) error {
	script := fmt.Sprintf(`
		(function(dy) {
			if (typeof window.__cursorX === 'undefined') {
				window.scrollBy({top: dy, behavior: 'auto'});
				return;
			}
			const el = document.elementFromPoint(window.__cursorX, window.__cursorY);
			if (el) {
				let target = el;
				while (target && target !== document.body) {
					const style = window.getComputedStyle(target);
					if (style.overflowY === 'scroll' || style.overflowY === 'auto') {
						target.scrollBy({top: dy, behavior: 'auto'});
						return;
					}
					target = target.parentElement;
				}
				window.scrollBy({top: dy, behavior: 'auto'});
			}
		})(%f);
	`, deltaY)
	return b.Evaluate(script, nil)
}

func (b *RealBrowser) SendKeyboardInput(keys string) error {
	return chromedp.Run(b.ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		// Use chromedp.KeyEvent to dispatch raw events if possible.
		// SendKeys works but requires a selector. If activeElement exists, we can use JS.
		return chromedp.Evaluate(fmt.Sprintf(`
			(function(keys) {
				if (document.activeElement && (document.activeElement.tagName === 'INPUT' || document.activeElement.tagName === 'TEXTAREA')) {
					// Handling enter key
					if (keys === '\n') {
						const ev = new KeyboardEvent('keydown', {key: 'Enter', keyCode: 13, bubbles: true});
						document.activeElement.dispatchEvent(ev);
						if (document.activeElement.form) {
							// Optionally submit form on enter
							// document.activeElement.form.submit();
						}
					} else if (keys === '\b') {
						document.activeElement.value = document.activeElement.value.slice(0, -1);
						document.activeElement.dispatchEvent(new Event('input', {bubbles: true}));
					} else {
						document.activeElement.value += keys;
						document.activeElement.dispatchEvent(new Event('input', {bubbles: true}));
					}
					document.activeElement.dispatchEvent(new Event('change', {bubbles: true}));
				}
			})(%q);
		`, keys), nil).Do(ctx)
	}))
}

func (b *RealBrowser) HistoryBack() error {
	return b.Evaluate("window.history.back()", nil)
}

func (b *RealBrowser) HistoryForward() error {
	return b.Evaluate("window.history.forward()", nil)
}
