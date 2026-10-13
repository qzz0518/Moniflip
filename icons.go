package main

import "github.com/egoist/mygo/ui"

// 线条图标（Lucide 风格，24 视口），随文字颜色着色。
func icon(paths string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">` + paths + `</svg>`))
}

var (
	iconRefresh = icon(`<path d="M21 12a9 9 0 1 1-9-9c2.52 0 4.93 1 6.74 2.74L21 8"/><path d="M21 3v5h-5"/>`)
	iconGear    = icon(`<path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z"/><circle cx="12" cy="12" r="3"/>`)
	iconBack    = icon(`<path d="m15 18-6-6 6-6"/>`)
	iconArrow   = icon(`<path d="M5 12h14"/><path d="m13 6 6 6-6 6"/>`)
	iconChevron = icon(`<path d="m7 10 5 5 5-5"/>`)
	iconCheck   = icon(`<path d="M20 6 9 17l-5-5"/>`)
	iconPlus    = icon(`<path d="M5 12h14"/><path d="M12 5v14"/>`)
	iconClose   = icon(`<path d="M18 6 6 18"/><path d="m6 6 12 12"/>`)
	iconMinus   = icon(`<circle cx="12" cy="12" r="9"/><path d="M8 12h8"/>`)
	iconAlert   = icon(`<circle cx="12" cy="12" r="9"/><path d="M12 8v4.5"/><path d="M12 16h.01"/>`)
	iconDone    = icon(`<circle cx="12" cy="12" r="9"/><path d="m8.5 12.2 2.4 2.4 4.6-4.9"/>`)
	iconHome    = icon(`<path d="M15 21v-8a1 1 0 0 0-1-1h-4a1 1 0 0 0-1 1v8"/><path d="M3 10a2 2 0 0 1 .709-1.528l7-5.999a2 2 0 0 1 2.582 0l7 5.999A2 2 0 0 1 21 10v9a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/>`)
	iconPower   = icon(`<path d="M12 2v10"/><path d="M18.4 6.6a9 9 0 1 1-12.77.04"/>`)
)

var iconCheckBold = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3.2" stroke-linecap="round" stroke-linejoin="round"><path d="M20 6 9 17l-5-5"/></svg>`))
