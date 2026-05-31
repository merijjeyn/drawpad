// build-webapp-ui writes an Excalidraw scene depicting a hypothetical SaaS
// dashboard UI. Pipe it to `draw --initial -` to open it for review.
package main

import (
	"encoding/json"
	"os"

	"github.com/merijjeyn/drawpad/internal/diagram"
)

func main() {
	s := diagram.NewScene()

	// --- browser frame -----------------------------------------------------
	const (
		frameX, frameY = 40.0, 40.0
		frameW, frameH = 1180.0, 760.0
	)
	frame := diagram.Rectangle(frameX, frameY, frameW, frameH)
	frame["backgroundColor"] = "#ffffff"
	frame["fillStyle"] = "solid"
	frame["strokeWidth"] = 2.0
	s.Elements = append(s.Elements, frame)

	// URL bar
	urlBar := diagram.Rectangle(frameX, frameY, frameW, 44)
	urlBar["backgroundColor"] = "#e5e7eb"
	urlBar["fillStyle"] = "solid"
	s.Elements = append(s.Elements, urlBar)

	for i, c := range []string{"#ef4444", "#f59e0b", "#10b981"} {
		dot := diagram.Ellipse(frameX+14+float64(i)*22, frameY+14, 16, 16)
		dot["backgroundColor"] = c
		dot["fillStyle"] = "solid"
		dot["strokeColor"] = c
		s.Elements = append(s.Elements, dot)
	}
	urlText := diagram.Text(frameX+440, frameY+13, "app.acme.com / dashboard")
	urlText["fontSize"] = 14.0
	urlText["strokeColor"] = "#6b7280"
	s.Elements = append(s.Elements, urlText)

	// --- app header --------------------------------------------------------
	const headerY = frameY + 44
	const headerH = 64.0
	header := diagram.Rectangle(frameX, headerY, frameW, headerH)
	header["backgroundColor"] = "#ffffff"
	header["fillStyle"] = "solid"
	s.Elements = append(s.Elements, header)

	logo := diagram.Text(frameX+24, headerY+20, "◆  Acme")
	logo["fontSize"] = 22.0
	logo["strokeColor"] = "#111827"
	s.Elements = append(s.Elements, logo)

	searchBox := diagram.Rectangle(frameX+360, headerY+14, 460, 36)
	searchBox["backgroundColor"] = "#f3f4f6"
	searchBox["fillStyle"] = "solid"
	searchBox["strokeColor"] = "#d1d5db"
	s.Elements = append(s.Elements, searchBox)
	searchPlaceholder := diagram.Text(frameX+376, headerY+24, "🔍  Search customers, orders, reports…")
	searchPlaceholder["fontSize"] = 13.0
	searchPlaceholder["strokeColor"] = "#9ca3af"
	s.Elements = append(s.Elements, searchPlaceholder)

	bell := diagram.Ellipse(frameX+frameW-120, headerY+18, 28, 28)
	bell["backgroundColor"] = "#f3f4f6"
	bell["fillStyle"] = "solid"
	bellLabel := diagram.Text(frameX+frameW-115, headerY+25, "🔔")
	bellLabel["fontSize"] = 14.0
	s.Elements = append(s.Elements, bell, bellLabel)

	avatar := diagram.Ellipse(frameX+frameW-72, headerY+14, 36, 36)
	avatar["backgroundColor"] = "#4f46e5"
	avatar["fillStyle"] = "solid"
	avatar["strokeColor"] = "#4f46e5"
	avatarText := diagram.Text(frameX+frameW-62, headerY+24, "MU")
	avatarText["fontSize"] = 13.0
	avatarText["strokeColor"] = "#ffffff"
	s.Elements = append(s.Elements, avatar, avatarText)

	// --- sidebar -----------------------------------------------------------
	const sidebarX = frameX
	const sidebarY = headerY + headerH
	const sidebarW = 220.0
	sidebarH := frameH - 44 - headerH
	sidebar := diagram.Rectangle(sidebarX, sidebarY, sidebarW, sidebarH)
	sidebar["backgroundColor"] = "#f9fafb"
	sidebar["fillStyle"] = "solid"
	s.Elements = append(s.Elements, sidebar)

	navItems := []struct {
		label  string
		active bool
	}{
		{"▦  Dashboard", true},
		{"📊  Analytics", false},
		{"👥  Customers", false},
		{"💳  Billing", false},
		{"📦  Products", false},
		{"📑  Reports", false},
		{"⚙   Settings", false},
	}
	for i, it := range navItems {
		y := sidebarY + 20 + float64(i)*44
		bg := diagram.Rectangle(sidebarX+12, y, sidebarW-24, 36)
		if it.active {
			bg["backgroundColor"] = "#eef2ff"
			bg["strokeColor"] = "#c7d2fe"
		} else {
			bg["backgroundColor"] = "#f9fafb"
			bg["strokeColor"] = "#f9fafb"
		}
		bg["fillStyle"] = "solid"
		lbl := diagram.Text(sidebarX+24, y+10, it.label)
		lbl["fontSize"] = 14.0
		if it.active {
			lbl["strokeColor"] = "#4338ca"
		} else {
			lbl["strokeColor"] = "#374151"
		}
		s.Elements = append(s.Elements, bg, lbl)
	}

	// --- main area ---------------------------------------------------------
	const mainX = sidebarX + sidebarW + 24
	const mainY = sidebarY + 16
	mainW := frameW - sidebarW - 48

	title := diagram.Text(mainX, mainY, "Dashboard")
	title["fontSize"] = 26.0
	title["strokeColor"] = "#111827"
	s.Elements = append(s.Elements, title)

	subtitle := diagram.Text(mainX, mainY+36, "Welcome back, Meric. Here's how Acme is doing today.")
	subtitle["fontSize"] = 14.0
	subtitle["strokeColor"] = "#6b7280"
	s.Elements = append(s.Elements, subtitle)

	// KPI cards
	type kpi struct {
		label, value, delta, deltaColor string
	}
	kpis := []kpi{
		{"Active users", "12,847", "▲ 8.4% vs last week", "#059669"},
		{"Monthly revenue", "$84,221", "▲ 12.1% vs last month", "#059669"},
		{"Conversion rate", "3.2%", "▼ 0.3pp vs last week", "#dc2626"},
	}
	cardY := mainY + 80
	cardH := 110.0
	cardW := (mainW - 2*16) / 3
	for i, k := range kpis {
		x := mainX + float64(i)*(cardW+16)
		card := diagram.Rectangle(x, cardY, cardW, cardH)
		card["backgroundColor"] = "#ffffff"
		card["fillStyle"] = "solid"
		card["strokeColor"] = "#e5e7eb"
		// Colored top accent strip
		accent := diagram.Rectangle(x, cardY, cardW, 4)
		accent["backgroundColor"] = "#4f46e5"
		accent["fillStyle"] = "solid"
		accent["strokeColor"] = "#4f46e5"
		// Label
		lbl := diagram.Text(x+20, cardY+18, k.label)
		lbl["fontSize"] = 13.0
		lbl["strokeColor"] = "#6b7280"
		// Value
		val := diagram.Text(x+20, cardY+40, k.value)
		val["fontSize"] = 28.0
		val["strokeColor"] = "#111827"
		// Delta
		delta := diagram.Text(x+20, cardY+82, k.delta)
		delta["fontSize"] = 12.0
		delta["strokeColor"] = k.deltaColor
		s.Elements = append(s.Elements, card, accent, lbl, val, delta)
	}

	// Chart card
	chartY := cardY + cardH + 20
	chartH := 220.0
	chartCard := diagram.Rectangle(mainX, chartY, mainW, chartH)
	chartCard["backgroundColor"] = "#ffffff"
	chartCard["fillStyle"] = "solid"
	chartCard["strokeColor"] = "#e5e7eb"
	s.Elements = append(s.Elements, chartCard)

	chartTitle := diagram.Text(mainX+20, chartY+16, "Revenue — last 30 days")
	chartTitle["fontSize"] = 16.0
	chartTitle["strokeColor"] = "#111827"
	s.Elements = append(s.Elements, chartTitle)

	chartSubtitle := diagram.Text(mainX+20, chartY+40, "Daily gross, $USD")
	chartSubtitle["fontSize"] = 12.0
	chartSubtitle["strokeColor"] = "#6b7280"
	s.Elements = append(s.Elements, chartSubtitle)

	// Sparkline-ish bars
	plotX := mainX + 40
	plotY := chartY + 80
	plotW := mainW - 80
	plotH := 120.0
	axis := diagram.Line(plotX, plotY+plotH, plotX+plotW, plotY+plotH)
	axis["strokeColor"] = "#9ca3af"
	s.Elements = append(s.Elements, axis)

	heights := []float64{40, 52, 48, 65, 60, 72, 80, 70, 88, 84, 92, 78,
		90, 100, 95, 88, 96, 104, 110, 98, 112, 118, 108, 115}
	barW := plotW / float64(len(heights)*2)
	for i, hh := range heights {
		x := plotX + float64(i)*barW*2 + barW/2
		bar := diagram.Rectangle(x, plotY+plotH-hh, barW, hh)
		bar["backgroundColor"] = "#a5b4fc"
		bar["strokeColor"] = "#6366f1"
		bar["fillStyle"] = "solid"
		s.Elements = append(s.Elements, bar)
	}

	// Recent orders table
	tableY := chartY + chartH + 20
	tableH := 220.0
	tableCard := diagram.Rectangle(mainX, tableY, mainW, tableH)
	tableCard["backgroundColor"] = "#ffffff"
	tableCard["fillStyle"] = "solid"
	tableCard["strokeColor"] = "#e5e7eb"
	s.Elements = append(s.Elements, tableCard)

	tableTitle := diagram.Text(mainX+20, tableY+16, "Recent orders")
	tableTitle["fontSize"] = 16.0
	tableTitle["strokeColor"] = "#111827"
	s.Elements = append(s.Elements, tableTitle)

	viewAll := diagram.Text(mainX+mainW-100, tableY+18, "View all →")
	viewAll["fontSize"] = 13.0
	viewAll["strokeColor"] = "#4f46e5"
	s.Elements = append(s.Elements, viewAll)

	// Column headers
	cols := []struct {
		label string
		x     float64
	}{
		{"Order", mainX + 24},
		{"Customer", mainX + 160},
		{"Amount", mainX + 360},
		{"Status", mainX + 500},
		{"Date", mainX + 660},
	}
	hdrY := tableY + 56
	for _, c := range cols {
		t := diagram.Text(c.x, hdrY, c.label)
		t["fontSize"] = 12.0
		t["strokeColor"] = "#6b7280"
		s.Elements = append(s.Elements, t)
	}
	hdrLine := diagram.Line(mainX+16, hdrY+22, mainX+mainW-16, hdrY+22)
	hdrLine["strokeColor"] = "#e5e7eb"
	s.Elements = append(s.Elements, hdrLine)

	// Rows
	rows := [][]string{
		{"#AC-10421", "Lina Park", "$1,240.00", "Paid", "2 min ago"},
		{"#AC-10420", "Diego Alvarez", "$420.00", "Pending", "14 min ago"},
		{"#AC-10419", "Marta König", "$2,790.50", "Paid", "1 h ago"},
		{"#AC-10418", "Yuki Tanaka", "$88.00", "Refunded", "3 h ago"},
	}
	statusColor := map[string]string{
		"Paid":     "#059669",
		"Pending":  "#d97706",
		"Refunded": "#dc2626",
	}
	for i, row := range rows {
		ry := hdrY + 38 + float64(i)*32
		for j, val := range row {
			t := diagram.Text(cols[j].x, ry, val)
			t["fontSize"] = 13.0
			if j == 3 {
				t["strokeColor"] = statusColor[val]
			} else {
				t["strokeColor"] = "#111827"
			}
			s.Elements = append(s.Elements, t)
		}
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(s); err != nil {
		panic(err)
	}
}
