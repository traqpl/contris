//go:build js && wasm

package main

import (
	"fmt"
	"math"
	"strings"
	"syscall/js"
)

const (
	uiFontBody    = `"IBM Plex Sans", Arial, sans-serif`
	uiFontDisplay = `"IBM Plex Sans Condensed", "Arial Narrow", Arial, sans-serif`
)

// ── helpers ───────────────────────────────────────────────────────────────────

func (e *Engine) hdrMode() bool {
	v := js.Global().Get("hdrMode")
	return !v.IsUndefined() && v.Bool()
}

func (e *Engine) crtColor() string {
	theme := js.Global().Get("crtTheme").String()
	if e.hdrMode() {
		switch theme {
		case "theme-green":
			return "color(display-p3 0.30 1.70 0.30)"
		case "theme-cyan":
			return "color(display-p3 0.22 1.30 1.90)"
		default: // amber
			return "color(display-p3 1.60 0.95 0.15)"
		}
	}
	switch theme {
	case "theme-green":
		return "color(display-p3 0.50 1.00 0.50)"
	case "theme-cyan":
		return "color(display-p3 0.48 0.92 1.00)"
	default:
		return "color(display-p3 1.00 0.78 0.38)"
	}
}

func (e *Engine) glowColor() string {
	theme := js.Global().Get("crtTheme").String()
	if e.hdrMode() {
		switch theme {
		case "theme-green":
			return "color(display-p3 0.00 2.20 0.00)"
		case "theme-cyan":
			return "color(display-p3 0.00 1.40 2.50)"
		default: // amber
			return "color(display-p3 2.00 0.90 0.00)"
		}
	}
	switch theme {
	case "theme-green":
		return "color(display-p3 0.10 1.00 0.10)"
	case "theme-cyan":
		return "color(display-p3 0.00 0.82 1.00)"
	default:
		return "color(display-p3 1.00 0.72 0.00)"
	}
}

func (e *Engine) glow(blur float64) {
	e.ctx.Set("shadowBlur", blur)
	e.ctx.Set("shadowColor", e.glowColor())
}

func (e *Engine) noGlow() {
	e.ctx.Set("shadowBlur", 0)
	e.ctx.Set("shadowColor", "transparent")
}

func (e *Engine) text(str string, x, y, size float64, align string) {
	e.ctx.Set("font", fmt.Sprintf("%s %.0fpx %s", "500", size, uiFontBody))
	e.ctx.Set("textAlign", align)
	e.ctx.Set("textBaseline", "alphabetic")
	e.ctx.Call("fillText", str, x, y)
}

// crispGlow rysuje tekst dwukrotnie: najpierw z małym rozmyciem (halo),
// potem ostro na wierzchu — efekt fosforowy CRT bez mazania.
func (e *Engine) crispGlow(str string, x, y, size float64, align, col string) {
	e.ctx.Set("textAlign", align)
	e.ctx.Set("textBaseline", "alphabetic")
	e.ctx.Set("font", fmt.Sprintf("%s %.0fpx %s", "600", size, uiFontDisplay))
	// halo — mały blur, kolor CRT
	e.ctx.Set("shadowBlur", 6)
	e.ctx.Set("shadowColor", e.glowColor())
	e.ctx.Set("fillStyle", col)
	e.ctx.Call("fillText", str, x, y)
	// drugi pass — ostry, bez cienia
	e.ctx.Set("shadowBlur", 0)
	e.ctx.Set("shadowColor", "transparent")
	e.ctx.Call("fillText", str, x, y)
}

func (e *Engine) wrappedText(str string, x, y, maxWidth, size, lineHeight float64, align string, color string) float64 {
	words := strings.Fields(str)
	if len(words) == 0 {
		return y
	}

	e.ctx.Set("font", fmt.Sprintf("%s %.0fpx %s", "500", size, uiFontBody))
	e.ctx.Set("textAlign", align)
	e.ctx.Set("textBaseline", "alphabetic")
	e.ctx.Set("fillStyle", color)

	line := words[0]
	for _, word := range words[1:] {
		candidate := line + " " + word
		width := e.ctx.Call("measureText", candidate).Get("width").Float()
		if width <= maxWidth {
			line = candidate
			continue
		}
		e.ctx.Call("fillText", line, x, y)
		y += lineHeight
		line = word
	}
	e.ctx.Call("fillText", line, x, y)
	return y + lineHeight
}

func (e *Engine) clear() {
	e.ctx.Set("fillStyle", "#0b0c0b")
	e.ctx.Call("fillRect", 0, 0, canvasW, canvasH)
}

// ── main render ───────────────────────────────────────────────────────────────

func (e *Engine) Render() {
	e.clear()
	if e.state == StateMainMenu {
		e.renderMainMenu()
		return
	}
	e.renderHeader()
	e.renderBoard()
	e.renderSidebar()
	e.renderShip(0, boardY+ROWS*CELL+shipGap, canvasW, shipViewH, e.heelAnim, true)
	if e.state == StateGameOver {
		e.renderGameOver()
	}
	if e.state == StatePaused {
		e.renderPaused()
	}
	if e.state == StateLevelEnd {
		e.renderLevelEnd()
	}
	if e.state == StateVictory {
		e.renderVictory()
	}
}

func (e *Engine) renderMainMenu() {
	color := e.crtColor()
	dim := "#5a7888"
	soft := "#3a5060"
	accent := "#8aaa7a"
	panelStroke := "#22364a"

	e.crispGlow("CARGO SHIFT", canvasW/2, 30, 28, "center", color)
	e.ctx.Set("fillStyle", soft)
	e.noGlow()
	e.text("stack smart · keep trim · load all five decks", canvasW/2, 54, 14, "center")

	panelX := 26.0
	panelY := 76.0
	panelW := canvasW - 52.0
	panelH := 246.0
	gap := 18.0
	colW := (panelW - gap - 32.0) / 2
	leftX := panelX + 16.0
	rightX := leftX + colW + gap

	e.ctx.Set("fillStyle", "rgba(0,0,0,0.72)")
	e.ctx.Call("fillRect", panelX, panelY, panelW, panelH)
	e.ctx.Set("strokeStyle", panelStroke)
	e.ctx.Set("lineWidth", 2)
	e.ctx.Call("strokeRect", panelX+1, panelY+1, panelW-2, panelH-2)
	dividerX := leftX + colW + gap/2
	e.ctx.Set("strokeStyle", "rgba(58,80,96,0.7)")
	e.ctx.Set("lineWidth", 1)
	e.ctx.Call("beginPath")
	e.ctx.Call("moveTo", dividerX, panelY+14)
	e.ctx.Call("lineTo", dividerX, panelY+panelH-14)
	e.ctx.Call("stroke")

	leftY := panelY + 28.0
	e.crispGlow("START SHIFT", leftX, leftY, 18, "left", color)
	leftY += 28
	leftY = e.wrappedText("Keep the ship level while stacking mixed cargo.", leftX, leftY, colW, 14, 18, "left", dim)
	leftY = e.wrappedText("Each finished level seals one deck in the hull.", leftX, leftY, colW, 14, 18, "left", dim)
	leftY += 8

	e.ctx.Set("fillStyle", "rgba(18,30,42,0.85)")
	e.ctx.Call("fillRect", leftX, leftY, colW, 56)
	e.ctx.Set("strokeStyle", "rgba(108,144,170,0.45)")
	e.ctx.Set("lineWidth", 1)
	e.ctx.Call("strokeRect", leftX+0.5, leftY+0.5, colW-1, 55)
	e.crispGlow("SPACE / ENTER", leftX+colW/2, leftY+25, 18, "center", accent)
	e.ctx.Set("fillStyle", dim)
	e.text("start a new run", leftX+colW/2, leftY+45, 13, "center")
	leftY += 78

	e.ctx.Set("fillStyle", dim)
	e.text("CONTROLS", leftX, leftY, 15, "left")
	leftY += 22
	controls := []string{
		"← / →  move cargo",
		"↑  rotate   ↓  soft drop",
		"P / ESC  pause / resume",
		"Q  menu   M  mute   T  theme",
	}
	for _, line := range controls {
		leftY = e.wrappedText(line, leftX, leftY, colW, 13, 17, "left", soft)
	}

	rightY := panelY + 28.0
	e.ctx.Set("fillStyle", dim)
	e.text("SHIFT BRIEF", rightX, rightY, 15, "left")
	rightY += 22
	brief := []string{
		"Green trim pays best.",
		"Yellow trim is risky but still scores.",
		"Red trim blocks row clears and sinks runs.",
		"Hazmat pairs explode. Reefers chain for bonus points.",
	}
	for _, line := range brief {
		rightY = e.wrappedText(line, rightX, rightY, colW, 13, 17, "left", soft)
		if rightY < panelY+panelH-74 {
			rightY += 2
		}
	}

	rightY = panelY + panelH - 74
	e.ctx.Set("strokeStyle", "rgba(58,80,96,0.7)")
	e.ctx.Set("lineWidth", 1)
	e.ctx.Call("beginPath")
	e.ctx.Call("moveTo", rightX, rightY)
	e.ctx.Call("lineTo", rightX+colW, rightY)
	e.ctx.Call("stroke")
	rightY += 20
	e.ctx.Set("fillStyle", accent)
	e.text("GOAL", rightX, rightY, 15, "left")
	rightY += 22
	rightY = e.wrappedText("Complete five levels to fully load the ship.", rightX, rightY, colW, 13, 17, "left", soft)
	e.wrappedText("One level completed = one sealed deck.", rightX, rightY, colW, 13, 17, "left", soft)

	e.ctx.Set("fillStyle", "#314656")
	e.text("Press SPACE or ENTER to begin.", canvasW/2, 348, 13, "center")
	e.renderShip(0, boardY+ROWS*CELL+shipGap, canvasW, shipViewH, 0, false)
}

func (e *Engine) renderHeader() {
	color := e.crtColor()
	e.crispGlow("CARGO SHIFT", canvasW/2, 20, 22, "center", color)
	e.ctx.Set("fillStyle", "#3a5060")
	e.noGlow()
	e.text(fmt.Sprintf("LVL %d/%d", e.level, MaxLevel), canvasW-4, 20, 18, "right")
	e.text(fmt.Sprintf("%d", e.score), 4, 20, 18, "left")
}

// ── board ─────────────────────────────────────────────────────────────────────

var zoneTint = map[string]string{
	"green":  "rgba(40,120,60,0.18)",
	"yellow": "rgba(160,120,20,0.18)",
	"red":    "rgba(140,20,10,0.18)",
}

var zoneBorder = map[string]string{
	"green":  "#2a4a3a",
	"yellow": "#5a5010",
	"red":    "#6a1010",
}

var zoneSide = map[string]string{
	"green":  "#2a6a3a",
	"yellow": "#8a7010",
	"red":    "#7a1010",
}

func (e *Engine) renderBoard() {
	predH := e.curHeel
	if e.state == StatePlaying {
		predH = e.predictedHeel()
		// smooth anim
		e.heelAnim += (e.curHeel - e.heelAnim) * 0.08
	}

	heel := predH

	// Frozen-row overlay
	for r := 0; r < ROWS; r++ {
		frozenFor := e.rowFrozenTime(r)
		if frozenFor <= 0 {
			continue
		}
		rowY := boardY + float64(r)*CELL
		e.ctx.Set("fillStyle", "rgba(86, 188, 220, 0.14)")
		e.ctx.Call("fillRect", boardX, rowY, float64(COLS)*CELL, CELL)
		e.ctx.Set("strokeStyle", "rgba(120, 230, 255, 0.42)")
		e.ctx.Set("lineWidth", 1)
		e.ctx.Call("beginPath")
		e.ctx.Call("moveTo", boardX, rowY+1)
		e.ctx.Call("lineTo", boardX+float64(COLS)*CELL, rowY+1)
		e.ctx.Call("moveTo", boardX, rowY+CELL-1)
		e.ctx.Call("lineTo", boardX+float64(COLS)*CELL, rowY+CELL-1)
		e.ctx.Call("stroke")
	}

	// Full-row tints
	for r := 0; r < ROWS; r++ {
		full := true
		for _, cell := range e.grid[r] {
			if cell == nil {
				full = false
				break
			}
		}
		if !full {
			continue
		}
		res := e.evalRow(r, heel)
		zone := "red"
		frozen := e.rowFrozenTime(r) > 0
		if res != nil {
			zone = res.zone
		}
		if frozen {
			e.ctx.Set("fillStyle", "rgba(70, 150, 190, 0.18)")
		} else {
			e.ctx.Set("fillStyle", zoneTint[zone])
		}
		e.ctx.Call("fillRect", boardX, boardY+float64(r)*CELL, float64(COLS)*CELL, CELL)
	}

	// Grid lines - only where adjacent to truly empty cell (not covered by active piece)
	type rc struct{ r, c int }
	occupied := make(map[rc]bool, ROWS*COLS)
	for r := 0; r < ROWS; r++ {
		for c := 0; c < COLS; c++ {
			if e.grid[r][c] != nil {
				occupied[rc{r, c}] = true
			}
		}
	}
	if e.state == StatePlaying {
		// active piece
		for _, v := range e.cur.Shape {
			occupied[rc{e.cur.R + v.R, e.cur.C + v.C}] = true
		}
		// ghost piece
		gr := e.cur.R
		for e.canFit(gr+1, e.cur.C, e.cur.Shape) {
			gr++
		}
		for _, v := range e.cur.Shape {
			occupied[rc{gr + v.R, e.cur.C + v.C}] = true
		}
	}
	isEmpty := func(r, c int) bool {
		if r < 0 || r >= ROWS || c < 0 || c >= COLS {
			return true
		}
		return !occupied[rc{r, c}]
	}

	e.ctx.Set("strokeStyle", "#111c28")
	e.ctx.Set("lineWidth", 1)
	e.ctx.Call("beginPath")
	for r := 0; r <= ROWS; r++ {
		for c := 0; c < COLS; c++ {
			above := isEmpty(r-1, c)
			below := isEmpty(r, c)
			if above || below {
				e.ctx.Call("moveTo", boardX+float64(c)*CELL, boardY+float64(r)*CELL)
				e.ctx.Call("lineTo", boardX+float64(c+1)*CELL, boardY+float64(r)*CELL)
			}
		}
	}
	for c := 0; c <= COLS; c++ {
		for r := 0; r < ROWS; r++ {
			left := isEmpty(r, c-1)
			right := isEmpty(r, c)
			if left || right {
				e.ctx.Call("moveTo", boardX+float64(c)*CELL, boardY+float64(r)*CELL)
				e.ctx.Call("lineTo", boardX+float64(c)*CELL, boardY+float64(r+1)*CELL)
			}
		}
	}
	e.ctx.Call("stroke")

	// Centre divider
	e.ctx.Set("strokeStyle", "rgba(255,255,255,0.05)")
	e.ctx.Set("lineWidth", 1)
	e.ctx.Call("setLineDash", js.ValueOf([]interface{}{4, 4}))
	e.ctx.Call("beginPath")
	e.ctx.Call("moveTo", boardX+float64(COLS)/2*CELL, boardY)
	e.ctx.Call("lineTo", boardX+float64(COLS)/2*CELL, boardY+float64(ROWS)*CELL)
	e.ctx.Call("stroke")
	e.ctx.Call("setLineDash", js.ValueOf([]interface{}{}))

	// Placed cells
	for r := 0; r < ROWS; r++ {
		for c := 0; c < COLS; c++ {
			cell := e.grid[r][c]
			if cell == nil {
				continue
			}
			e.drawCell(boardX+float64(c)*CELL, boardY+float64(r)*CELL, CELL, cell.Co, 1.0, cell.Wear, c, r, cell.RibH)
		}
	}

	// Row score labels
	e.ctx.Set("font", fmt.Sprintf("%s %.0fpx %s", "600", 9.0, uiFontBody))
	for r := 0; r < ROWS; r++ {
		full := true
		for _, cell := range e.grid[r] {
			if cell == nil {
				full = false
				break
			}
		}
		if !full {
			continue
		}
		res := e.evalRow(r, heel)
		x := boardX + float64(COLS)*CELL - 2
		y := boardY + float64(r)*CELL + CELL - 4
		frozenFor := e.rowFrozenTime(r)
		if frozenFor > 0 {
			e.ctx.Set("fillStyle", "#6fd8ff")
			e.ctx.Set("textAlign", "right")
			e.ctx.Call("fillText", fmt.Sprintf("❄%.0f", math.Ceil(frozenFor)), x, y)
		} else if res != nil {
			e.ctx.Set("fillStyle", zoneSide[res.zone])
			e.ctx.Set("textAlign", "right")
			e.ctx.Call("fillText", fmt.Sprintf("▶%d", res.pts), x, y)
		} else {
			e.ctx.Set("fillStyle", "#6a2020")
			e.ctx.Set("textAlign", "right")
			e.ctx.Call("fillText", "✗", x, y)
		}
	}

	if e.state != StatePlaying {
		return
	}

	// Ghost piece
	gr := e.cur.R
	for e.canFit(gr+1, e.cur.C, e.cur.Shape) {
		gr++
	}
	curH, curW := shapeDims(e.cur.Shape)
	curRibH := curH > curW
	if gr != e.cur.R {
		for i, v := range e.cur.Shape {
			e.drawCell(
				boardX+float64(e.cur.C+v.C)*CELL,
				boardY+float64(gr+v.R)*CELL,
				CELL, e.cur.Co, 0.18, e.cur.Wear[i], e.cur.C+v.C, v.R, curRibH,
			)
		}
	}

	// Active piece
	for i, v := range e.cur.Shape {
		e.drawCell(
			boardX+float64(e.cur.C+v.C)*CELL,
			boardY+float64(e.cur.R+v.R)*CELL,
			CELL, e.cur.Co, 1.0, e.cur.Wear[i], e.cur.C+v.C, v.R, curRibH,
		)
	}

	// Active piece outline
	e.drawShapeOutline(e.cur.Shape, e.cur.R, e.cur.C, "rgba(255,255,255,0.7)", 1.5)

	// Board border = predicted zone colour
	bz := heelZone(predH, e.level)
	e.ctx.Set("strokeStyle", zoneBorder[bz])
	e.ctx.Set("lineWidth", 3)
	e.ctx.Call("strokeRect", boardX+1, boardY+1, float64(COLS)*CELL-2, float64(ROWS)*CELL-2)

	// Flash overlay
	if e.flash != nil && e.flash.T > 0 {
		alpha := math.Min(1, e.flash.T*1.5)
		e.ctx.Call("save")
		e.ctx.Set("globalAlpha", alpha)
		e.ctx.Set("fillStyle", "rgba(0,0,0,0.72)")
		cy := boardY + float64(ROWS)*CELL/2
		e.ctx.Call("fillRect", boardX, cy-26, float64(COLS)*CELL, 44)
		e.ctx.Set("fillStyle", e.flash.Color)
		e.ctx.Set("shadowBlur", 12)
		e.ctx.Set("shadowColor", e.flash.Color)
		e.text(e.flash.Text, boardX+float64(COLS)*CELL/2, cy+10, 15, "center")
		e.ctx.Call("restore")
	}
}

// ── cell drawing ──────────────────────────────────────────────────────────────

func (e *Engine) drawCell(x, y, sz float64, co string, alpha float64, wear, seedC, seedR int, ribH bool) {
	e.ctx.Call("save")
	if alpha < 1.0 {
		e.ctx.Set("globalAlpha", alpha)
	}

	// ── baza ─────────────────────────────────────────────────────────────────
	e.ctx.Set("fillStyle", coColor[co])
	e.ctx.Call("fillRect", x, y, sz, sz)

	switch co {
	case "haz", "reef":
		// Specjalne: ramka + ikona
		if co == "haz" {
			e.ctx.Set("strokeStyle", "#ff8800")
			e.ctx.Set("fillStyle", "#ffcc00")
		} else {
			e.ctx.Set("strokeStyle", "#72ecff")
			e.ctx.Set("fillStyle", "#d9fbff")
		}
		e.ctx.Set("lineWidth", 1.5)
		e.ctx.Call("strokeRect", x+2, y+2, sz-4, sz-4)
		icon := "⚠"
		if co == "reef" {
			icon = "❄"
		}
		e.ctx.Set("font", fmt.Sprintf("%s %.0fpx %s", "600", sz*0.56, uiFontBody))
		e.ctx.Set("textAlign", "center")
		e.ctx.Set("textBaseline", "middle")
		e.ctx.Call("fillText", icon, x+sz/2, y+sz/2)

	default:
		// ── Kontener właściwy ─────────────────────────────────────────────

		// Deterministyczny hash — stabilne współrzędne siatki zamiast pikseli,
		// żeby ślady nie "chodziły" gdy klocek opada.
		h := wear*2654435761 + seedC*40503 + seedR*12345
		if h < 0 {
			h = -h
		}

		// ── ~1% kontenerów lekko odgiętych (warp) ────────────────────────
		if h%100 < 1 {
			warpPx := float64(1 + (h>>10)%2)
			if (h>>12)%2 == 0 {
				warpPx = -warpPx
			}
			shear := warpPx / sz
			centerY := y + sz/2
			e.ctx.Call("transform", 1, 0, shear, 1, -shear*centerY, 0)
		}

		// ── Żebra karbowania — kierunek zależny od orientacji klocka ────────
		// Spójna faza globalna: żebra sąsiednich cel nie tworzą podwójnej szczeliny.
		ribSpacing := 7.0
		ribPhase := 4.0
		ribLines := make([]float64, 0, int(sz/ribSpacing)+2)
		if !ribH {
			// pionowe żebra (kontener leży poziomo)
			start := math.Floor((x-ribPhase)/ribSpacing)*ribSpacing + ribPhase
			for rx := start; rx < x+sz; rx += ribSpacing {
				if rx < x || rx >= x+sz {
					continue
				}
				ribLines = append(ribLines, rx)
				e.ctx.Set("fillStyle", "rgba(0,0,0,0.22)")
				e.ctx.Call("fillRect", rx, y, 1, sz)
				e.ctx.Set("fillStyle", "rgba(255,255,255,0.09)")
				e.ctx.Call("fillRect", rx+1, y, 1, sz)
			}
		} else {
			// poziome żebra (kontener stoi pionowo)
			start := math.Floor((y-ribPhase)/ribSpacing)*ribSpacing + ribPhase
			for ry := start; ry < y+sz; ry += ribSpacing {
				if ry < y || ry >= y+sz {
					continue
				}
				ribLines = append(ribLines, ry)
				e.ctx.Set("fillStyle", "rgba(0,0,0,0.22)")
				e.ctx.Call("fillRect", x, ry, sz, 1)
				e.ctx.Set("fillStyle", "rgba(255,255,255,0.09)")
				e.ctx.Call("fillRect", x, ry+1, sz, 1)
			}
		}

		// ── Ślady zużycia — tylko wear == 3 ──────────────────────────────
		if wear == 3 {
			dirtColor := "rgba(145,58,8,0.45)"
			dirtW := 2.0
			if co == "orange" {
				dirtColor = "rgba(10,5,2,0.42)"
				dirtW = 3.0
			}
			for i, rl := range ribLines {
				mod := 3
				if co == "orange" {
					mod = 4
				}
				if (h+i)%mod == 0 {
					e.ctx.Set("fillStyle", dirtColor)
					if !ribH {
						sH := sz*0.28 + float64((h+i*7)%int(sz*35/100))
						sY := y + float64((h*3+i*11)%int(sz*55/100))
						e.ctx.Call("fillRect", rl-1, sY, dirtW, sH)
					} else {
						sW := sz*0.28 + float64((h+i*7)%int(sz*35/100))
						sX := x + float64((h*3+i*11)%int(sz*55/100))
						e.ctx.Call("fillRect", sX, rl-1, sW, dirtW)
					}
				}
			}
		}
	}

	e.ctx.Call("restore")
}

func (e *Engine) drawShapeOutline(cells []Vec2, offR, offC int, color string, lw float64) {
	type key struct{ r, c int }
	set := make(map[key]bool, len(cells))
	for _, v := range cells {
		set[key{v.R + offR, v.C + offC}] = true
	}
	e.ctx.Set("strokeStyle", color)
	e.ctx.Set("lineWidth", lw)
	e.ctx.Call("beginPath")
	for _, v := range cells {
		r, c := v.R+offR, v.C+offC
		x := boardX + float64(c)*CELL
		y := boardY + float64(r)*CELL
		if !set[key{r - 1, c}] {
			e.ctx.Call("moveTo", x, y)
			e.ctx.Call("lineTo", x+CELL, y)
		}
		if !set[key{r + 1, c}] {
			e.ctx.Call("moveTo", x, y+CELL)
			e.ctx.Call("lineTo", x+CELL, y+CELL)
		}
		if !set[key{r, c - 1}] {
			e.ctx.Call("moveTo", x, y)
			e.ctx.Call("lineTo", x, y+CELL)
		}
		if !set[key{r, c + 1}] {
			e.ctx.Call("moveTo", x+CELL, y)
			e.ctx.Call("lineTo", x+CELL, y+CELL)
		}
	}
	e.ctx.Call("stroke")
}

// ── sidebar ───────────────────────────────────────────────────────────────────

func (e *Engine) renderSidebar() {
	color := e.crtColor()
	dim := "#5a7888"
	x := sideX
	y := boardY + 6.0

	sepLine := func(after float64) {
		e.ctx.Set("strokeStyle", "#2e404e")
		e.ctx.Set("lineWidth", 1)
		e.ctx.Call("beginPath")
		e.ctx.Call("moveTo", x+4, y)
		e.ctx.Call("lineTo", x+sideW-4, y)
		e.ctx.Call("stroke")
		y += after
	}

	// ── Next piece ────────────────────────────────────────────────────────
	e.ctx.Set("fillStyle", dim)
	e.noGlow()
	e.text("NEXT", x+sideW/2, y+16, 18, "center")
	y += 24

	if e.next.Shape != nil {
		nh, nw := shapeDims(e.next.Shape)
		s := math.Min(math.Floor(60/float64(nw)), math.Floor(60/float64(nh)))
		s = math.Min(s, 30)
		ox := x + (sideW-float64(nw)*s)/2
		oy := y + (68-float64(nh)*s)/2
		nextRibH := nh > nw
		for i, v := range e.next.Shape {
			e.drawCell(ox+float64(v.C)*s, oy+float64(v.R)*s, s, e.next.Co, 1.0, e.next.Wear[i], v.C, v.R, nextRibH)
		}
		e.ctx.Set("strokeStyle", coOutline[e.next.Co])
		e.ctx.Set("lineWidth", 1.5)
		e.ctx.Call("beginPath")
		type key2 struct{ r, c int }
		set := map[key2]bool{}
		for _, v := range e.next.Shape {
			set[key2{v.R, v.C}] = true
		}
		for _, v := range e.next.Shape {
			nx2 := ox + float64(v.C)*s
			ny2 := oy + float64(v.R)*s
			if !set[key2{v.R - 1, v.C}] {
				e.ctx.Call("moveTo", nx2, ny2)
				e.ctx.Call("lineTo", nx2+s, ny2)
			}
			if !set[key2{v.R + 1, v.C}] {
				e.ctx.Call("moveTo", nx2, ny2+s)
				e.ctx.Call("lineTo", nx2+s, ny2+s)
			}
			if !set[key2{v.R, v.C - 1}] {
				e.ctx.Call("moveTo", nx2, ny2)
				e.ctx.Call("lineTo", nx2, ny2+s)
			}
			if !set[key2{v.R, v.C + 1}] {
				e.ctx.Call("moveTo", nx2+s, ny2)
				e.ctx.Call("lineTo", nx2+s, ny2+s)
			}
		}
		e.ctx.Call("stroke")
	}
	y += 70

	if e.next.Label != "" {
		e.ctx.Set("fillStyle", dim)
		e.text(e.next.Label, x+sideW/2, y+14, 16, "center")
		y += 20
	}

	sepLine(12)

	// ── Stats ──────────────────────────────────────────────────────────────
	statRow := func(label, val, valCol string) {
		e.ctx.Set("fillStyle", dim)
		e.text(label, x+6, y+17, 16, "left")
		e.crispGlow(val, x+sideW-6, y+17, 22, "right", valCol)
		y += 26
	}

	statRow("SCORE", itoa(e.score), color)
	statRow("LINES", itoa(e.lines), color)
	statRow("LEVEL", itoa(e.level), color)

	remaining := math.Max(0, levelDuration(e.level)-e.levelTimer)
	mins := int(remaining) / 60
	secs := int(remaining) % 60
	timerStr := itoa(mins) + ":"
	if secs < 10 {
		timerStr += "0"
	}
	timerStr += itoa(secs)
	timerColor := color
	if remaining < 30 {
		timerColor = "#dd4444"
	} else if remaining < 60 {
		timerColor = "#dd8844"
	}
	statRow("TIME LEFT", timerStr, timerColor)

	if e.comboText != "" && e.comboTime > 0 {
		alpha := math.Min(1.0, e.comboTime)
		e.ctx.Call("save")
		e.ctx.Set("globalAlpha", alpha)
		e.crispGlow(e.comboText, x+sideW/2, y+14, 16, "center", "#8aaa7a")
		e.ctx.Call("restore")
		y += 20
	}

	sepLine(10)

	// ── Trym ───────────────────────────────────────────────────────────────
	predH := e.curHeel
	if e.state == StatePlaying || e.state == StatePaused {
		predH = e.predictedHeel()
	}
	zone := heelZone(predH, e.level)

	zoneColors := map[string]string{"green": color, "yellow": "#d09030", "red": "#dd3030"}
	zoneLabels := map[string]string{"green": "TRIM ✓ GOOD", "yellow": "TRIM ⚠ WARNING", "red": "TRIM ✗ DANGER"}

	e.noGlow()
	e.ctx.Set("fillStyle", zoneColors[zone])
	e.text(zoneLabels[zone], x+sideW/2, y+17, 20, "center")
	y += 24

	e.ctx.Set("fillStyle", dim)
	e.text("lateral trim", x+sideW/2, y+14, 15, "center")
	y += 18

	e.renderGauge(x, y, sideW, 28, e.heelAnim, predH)
	y += 32

	e.renderDangerBar(x, y, sideW, 9)
	y += 14

	sepLine(8)

	// ── Rules + shortcuts — przypięte do dołu ───────────────────────────
	rules := []string{
		"GREEN  clear row = 200",
		"YELLOW clear row = 100",
		"RED    no row clear",
		"DG pair explodes = -50",
		"REEFER 2TU freezes row = 20s",
	}
	shortcuts := []struct{ key, action string }{
		{"ARROWS", "move"},
		{"UP", "rotate"},
		{"DOWN", "drop"},
		{"SPACE", "hard drop"},
		{"P / ESC", "pause"},
		{"Q", "menu"},
		{"M / T", "mute / theme"},
	}

	ruleLineH := 16.0
	rulesPanelH := 28.0 + float64(len(rules))*ruleLineH
	keyH := 18.0
	shortcutsPanelH := 30.0 + float64(len(shortcuts))*keyH
	blockGap := 8.0
	blockH := rulesPanelH + blockGap + shortcutsPanelH
	sideBottom := boardY + float64(ROWS)*CELL
	ry := sideBottom - blockH
	if ry < y {
		ry = y
	}
	y = ry

	panelX := x + 2
	panelW := sideW - 4
	keyColW := 78.0
	actionX := panelX + keyColW + 14

	e.ctx.Set("fillStyle", "rgba(8,14,20,0.82)")
	e.ctx.Call("fillRect", panelX, y, panelW, rulesPanelH)
	e.ctx.Set("strokeStyle", "#22364a")
	e.ctx.Set("lineWidth", 1)
	e.ctx.Call("strokeRect", panelX+0.5, y+0.5, panelW-1, rulesPanelH-1)

	e.ctx.Set("fillStyle", dim)
	e.text("RULES", x+sideW/2, y+18, 14, "center")
	y += 30

	for _, line := range rules {
		parts := strings.SplitN(line, "=", 2)
		left := strings.TrimSpace(parts[0])
		right := ""
		if len(parts) > 1 {
			right = strings.TrimSpace(parts[1])
		}
		e.crispGlow(left, panelX+8, y+12, 11, "left", color)
		if right != "" {
			e.ctx.Set("fillStyle", "#607888")
			e.text(right, panelX+panelW-8, y+12, 11, "right")
		}
		y += ruleLineH
	}

	y += blockGap

	e.ctx.Set("fillStyle", "rgba(8,14,20,0.82)")
	e.ctx.Call("fillRect", panelX, y, panelW, shortcutsPanelH)
	e.ctx.Set("strokeStyle", "#22364a")
	e.ctx.Set("lineWidth", 1)
	e.ctx.Call("strokeRect", panelX+0.5, y+0.5, panelW-1, shortcutsPanelH-1)

	e.ctx.Set("fillStyle", dim)
	e.text("SHORTCUTS", x+sideW/2, y+18, 14, "center")
	e.ctx.Set("strokeStyle", "rgba(58,80,96,0.6)")
	e.ctx.Set("lineWidth", 1)
	e.ctx.Call("beginPath")
	e.ctx.Call("moveTo", panelX+keyColW, y+24)
	e.ctx.Call("lineTo", panelX+keyColW, y+shortcutsPanelH-10)
	e.ctx.Call("stroke")
	y += 32

	for _, item := range shortcuts {
		e.crispGlow(item.key, panelX+10, y+13, 12, "left", color)
		e.ctx.Set("fillStyle", "#607888")
		e.text(item.action, actionX, y+13, 12, "left")
		y += keyH
	}
}

// ── gauge ─────────────────────────────────────────────────────────────────────

func (e *Engine) renderGauge(x, y, w, h, curH, predH float64) {
	g, yy := zones(e.level)
	cx := x + w/2
	toX := func(v float64) float64 { return cx + v*(w/2) }

	bands := [][3]float64{{-1, -yy, 0}, {-yy, -g, 1}, {-g, g, 2}, {g, yy, 1}, {yy, 1, 0}}
	bandColors := []string{"#8a1010", "#907010", "#1a7832", "#907010", "#8a1010"}
	for i, b := range bands {
		e.ctx.Set("fillStyle", bandColors[i])
		e.ctx.Call("fillRect", toX(b[0]), y+4, toX(b[1])-toX(b[0]), h-8)
	}

	e.ctx.Set("strokeStyle", "#1a2a3a")
	e.ctx.Set("lineWidth", 1)
	e.ctx.Call("strokeRect", x, y+4, w, h-8)

	// Centre mark
	e.ctx.Set("strokeStyle", "rgba(255,255,255,0.08)")
	e.ctx.Call("beginPath")
	e.ctx.Call("moveTo", cx, y+2)
	e.ctx.Call("lineTo", cx, y+h-2)
	e.ctx.Call("stroke")

	// Predicted (ghost)
	px := toX(math.Max(-1, math.Min(1, predH)))
	e.ctx.Set("fillStyle", "rgba(255,255,255,0.22)")
	e.ctx.Call("fillRect", px-5, y+2, 10, h-4)

	// Current (dot)
	ax := toX(math.Max(-1, math.Min(1, curH)))
	e.ctx.Set("fillStyle", "#fff")
	e.ctx.Call("beginPath")
	e.ctx.Call("arc", ax, y+h/2, 5, 0, math.Pi*2)
	e.ctx.Call("fill")
}

// ── danger bar ────────────────────────────────────────────────────────────────

func (e *Engine) renderDangerBar(x, y, w, h float64) {
	if e.redTimer <= 0 {
		return
	}
	ratio := e.redTimer / RedLimit

	e.ctx.Set("fillStyle", "#1a1010")
	e.ctx.Call("fillRect", x, y+1, w, h-2)

	// Pulse when urgent
	pulse := 1.0
	if ratio > 0.7 {
		pulse = 0.65 + 0.35*math.Sin(float64(js.Global().Get("performance").Call("now").Float())/120)
	}
	e.ctx.Call("save")
	e.ctx.Set("globalAlpha", pulse)
	col := "#8a4010"
	if ratio >= 0.8 {
		col = "#ff2000"
	} else if ratio >= 0.5 {
		col = "#c03010"
	}
	e.ctx.Set("fillStyle", col)
	e.ctx.Call("fillRect", x, y+1, w*ratio, h-2)
	e.ctx.Call("restore")

	e.ctx.Set("fillStyle", "rgba(255,255,255,0.55)")
	e.ctx.Set("font", fmt.Sprintf("%s %.0fpx %s", "500", 8.0, uiFontBody))
	e.ctx.Set("textAlign", "center")
	e.ctx.Set("textBaseline", "middle")
	e.ctx.Call("fillText",
		fmt.Sprintf("⛵ %.0fs", RedLimit-e.redTimer),
		x+w/2, y+h/2,
	)
}

// ── ship side view ────────────────────────────────────────────────────────────

func (e *Engine) renderShip(x, y, w, h, heel float64, showHUD bool) {
	e.ctx.Call("save")
	e.ctx.Call("beginPath")
	e.ctx.Call("rect", x, y, w, h)
	e.ctx.Call("clip")

	// Background
	e.ctx.Set("fillStyle", "#060c12")
	e.ctx.Call("fillRect", x, y, w, h)

	waterY := y + h*0.64
	angle := heel * 0.35

	e.ctx.Call("translate", x+w/2, waterY)
	e.ctx.Call("rotate", angle)

	sa := w * 0.45
	deckH := h * 0.23
	hullD := h * 0.155
	bowS := deckH * 1.05
	sternT := 3.0
	maxCargoH := h * 0.45 // cały obszar od pokładu w górę
	stackTop := -deckH - maxCargoH
	stackBottom := hullD
	stackH := stackBottom - stackTop

	shipLeft := -sa + sternT
	shipRight := sa - bowS*0.2
	shipW := shipRight - shipLeft
	cellW := shipW / COLS
	layerH := stackH / float64(MaxLevel)

	// ── Completed campaign cargo layers ───────────────────────────────────
	for layer := 0; layer < e.completedShipLayers; layer++ {
		layerY := stackBottom - float64(layer+1)*layerH

		e.ctx.Set("fillStyle", "#a94710")
		e.ctx.Call("fillRect", shipLeft+0.5, layerY+0.5, shipW-1, layerH-1)

		e.ctx.Set("fillStyle", "rgba(255,190,120,0.16)")
		e.ctx.Call("fillRect", shipLeft+0.5, layerY+0.5, shipW-1, math.Max(1, layerH*0.12))

		e.ctx.Set("strokeStyle", "rgba(55,18,5,0.65)")
		e.ctx.Set("lineWidth", 0.8)
		e.ctx.Call("strokeRect", shipLeft+0.5, layerY+0.5, shipW-1, layerH-1)

		e.ctx.Set("strokeStyle", "rgba(70,24,8,0.36)")
		e.ctx.Set("lineWidth", 0.5)
		for c := 0; c < COLS; c++ {
			cx := shipLeft + float64(c)*cellW
			e.ctx.Call("strokeRect", cx+0.5, layerY+0.5, cellW-1, layerH-1)

			ribInsetX := math.Max(1, cellW*0.18)
			e.ctx.Set("fillStyle", "rgba(35,10,2,0.22)")
			e.ctx.Call("fillRect", cx+ribInsetX, layerY+1, 0.6, layerH-2)
			e.ctx.Call("fillRect", cx+cellW-ribInsetX, layerY+1, 0.6, layerH-2)
			e.ctx.Set("fillStyle", "rgba(255,210,170,0.08)")
			e.ctx.Call("fillRect", cx+ribInsetX+0.8, layerY+1, 0.5, layerH-2)
			e.ctx.Call("fillRect", cx+cellW-ribInsetX+0.8, layerY+1, 0.5, layerH-2)
		}
	}

	if e.completedShipLayers > 0 {
		e.ctx.Set("strokeStyle", "rgba(255,184,96,0.42)")
		e.ctx.Set("lineWidth", 1)
		for layer := 1; layer < e.completedShipLayers; layer++ {
			sepY := stackBottom - float64(layer)*layerH
			e.ctx.Call("beginPath")
			e.ctx.Call("moveTo", shipLeft, sepY)
			e.ctx.Call("lineTo", shipLeft+shipW, sepY)
			e.ctx.Call("stroke")
		}
	}

	// ── Obrys kadłuba — tylko linie, bez wypełnienia ───────────────────────
	hullGlow := e.glowColor()
	e.ctx.Set("shadowBlur", 10)
	e.ctx.Set("shadowColor", hullGlow)

	// Boczna ściana powyżej wody (pokład)
	e.ctx.Set("strokeStyle", "#68a8da")
	e.ctx.Set("lineWidth", 1.8)
	e.ctx.Call("beginPath")
	e.ctx.Call("moveTo", -sa+sternT, 0)
	e.ctx.Call("lineTo", -sa, -deckH)
	e.ctx.Call("lineTo", sa, -deckH)
	e.ctx.Call("lineTo", sa-bowS*0.25, 0)
	e.ctx.Call("stroke")

	// Obrys kadłuba poniżej wody — czerwona linia
	e.ctx.Set("strokeStyle", "#ff3b14")
	e.ctx.Set("lineWidth", 2.1)
	e.ctx.Call("beginPath")
	e.ctx.Call("moveTo", -sa+sternT, 0)
	e.ctx.Call("lineTo", -sa, hullD)
	e.ctx.Call("lineTo", sa-bowS*0.35, hullD)
	e.ctx.Call("lineTo", sa-bowS*0.05, hullD*0.5)
	e.ctx.Call("lineTo", sa-bowS*0.25, 0)
	e.ctx.Call("stroke")

	// Linia wodna (niebieska)
	e.ctx.Set("strokeStyle", "#9ce8ff")
	e.ctx.Set("lineWidth", 1.3)
	e.ctx.Call("beginPath")
	e.ctx.Call("moveTo", -sa+sternT, 0)
	e.ctx.Call("lineTo", sa-bowS*0.25, 0)
	e.ctx.Call("stroke")
	e.ctx.Set("shadowBlur", 0)
	e.ctx.Set("shadowColor", "transparent")

	// Pionowe wręgi kadłuba (szkielet)
	e.ctx.Set("strokeStyle", "rgba(58,96,144,0.35)")
	e.ctx.Set("lineWidth", 0.5)
	e.ctx.Call("beginPath")
	for c := 1; c < COLS; c++ {
		cx := shipLeft + float64(c)*cellW
		e.ctx.Call("moveTo", cx, -deckH-maxCargoH)
		e.ctx.Call("lineTo", cx, -deckH)
	}
	e.ctx.Call("stroke")

	// ── Nadbudówka 1: mostek ──────────────────────────────────────────────
	bw := w * 0.085
	bx := sa*0.30 - bw/2
	bh := deckH * 1.85
	bridgeY := -deckH - bh
	e.ctx.Set("fillStyle", "#b8c3ca")
	e.ctx.Call("fillRect", bx, -deckH-bh, bw, bh)
	e.ctx.Set("fillStyle", "rgba(255,255,255,0.14)")
	e.ctx.Call("fillRect", bx+1, bridgeY+1, bw-2, bh*0.16)
	e.ctx.Set("fillStyle", "#284f76")
	e.ctx.Call("fillRect", bx+1.5, bridgeY+2, bw-3, bh*0.30)
	e.ctx.Set("strokeStyle", "#8ea2b4")
	e.ctx.Set("lineWidth", 1)
	e.ctx.Call("strokeRect", bx+0.5, bridgeY+0.5, bw-1, bh-1)
	e.ctx.Set("strokeStyle", "rgba(36,58,78,0.65)")
	e.ctx.Call("beginPath")
	e.ctx.Call("moveTo", bx+2, bridgeY+bh*0.46)
	e.ctx.Call("lineTo", bx+bw-2, bridgeY+bh*0.46)
	e.ctx.Call("moveTo", bx+bw*0.5, bridgeY+bh*0.46)
	e.ctx.Call("lineTo", bx+bw*0.5, bridgeY+bh-2)
	e.ctx.Call("stroke")

	e.ctx.Set("fillStyle", "#aeb8bf")
	e.ctx.Call("fillRect", bx+bw*0.18, bridgeY-4, bw*0.64, 4)
	e.ctx.Set("strokeStyle", "#9fb8c8")
	e.ctx.Call("beginPath")
	e.ctx.Call("moveTo", bx+bw/2, bridgeY-4)
	e.ctx.Call("lineTo", bx+bw/2, bridgeY-16)
	e.ctx.Call("stroke")
	e.ctx.Set("fillStyle", "#9ce8ff")
	e.ctx.Call("fillRect", bx+bw/2-1, bridgeY-18, 2, 3)
	e.ctx.Set("fillStyle", "#d8edf8")
	for i := 0; i < 3; i++ {
		wx := bx + bw*0.14 + float64(i)*bw*0.24
		e.ctx.Call("fillRect", wx, bridgeY+bh*0.10, bw*0.14, bh*0.12)
	}

	// ── Nadbudówka 2: komin ───────────────────────────────────────────────
	fw := w * 0.06
	fx := -sa*0.50 - fw/2
	fh := deckH * 1.45
	fy := -deckH - fh
	e.ctx.Set("fillStyle", "#c6a21d")
	e.ctx.Call("fillRect", fx, fy, fw, fh)
	e.ctx.Set("fillStyle", "rgba(255,255,255,0.12)")
	e.ctx.Call("fillRect", fx+1, fy+1, fw-2, fh*0.16)
	e.ctx.Set("strokeStyle", "#6a5a20")
	e.ctx.Set("lineWidth", 1)
	e.ctx.Call("strokeRect", fx+0.5, fy+0.5, fw-1, fh-1)
	e.ctx.Set("strokeStyle", "rgba(78,56,18,0.55)")
	e.ctx.Call("beginPath")
	e.ctx.Call("moveTo", fx+fw*0.5, fy+2)
	e.ctx.Call("lineTo", fx+fw*0.5, fy+fh-2)
	e.ctx.Call("stroke")
	e.ctx.Set("fillStyle", "#1a1a1a")
	e.ctx.Call("fillRect", fx+fw*0.22, fy-10, fw*0.56, 12)
	e.ctx.Set("fillStyle", "#c03010")
	e.ctx.Call("fillRect", fx+fw*0.22, fy-4, fw*0.56, 3)
	e.ctx.Set("fillStyle", "#f4d46f")
	e.ctx.Call("fillRect", fx+fw*0.18, fy+fh*0.24, fw*0.64, 2)

	e.ctx.Call("restore")

	// ── Woda ──────────────────────────────────────────────────────────────
	e.ctx.Set("fillStyle", "rgba(8,28,55,0.72)")
	e.ctx.Call("fillRect", x, waterY, w, y+h-waterY)

	e.ctx.Call("save")
	e.ctx.Call("translate", x+w/2, waterY)
	e.ctx.Call("rotate", angle)
	e.ctx.Set("strokeStyle", "rgba(80,160,220,0.45)")
	e.ctx.Set("lineWidth", 1.5)
	e.ctx.Call("beginPath")
	e.ctx.Call("moveTo", -w, 0)
	e.ctx.Call("lineTo", w, 0)
	e.ctx.Call("stroke")
	e.ctx.Call("restore")

	if !showHUD {
		return
	}

	// ── Labels ────────────────────────────────────────────────────────────
	deg := angle * 180 / math.Pi
	sign := ""
	if deg >= 0 {
		sign = "+"
	}
	col := "#4a4"
	if math.Abs(heel) >= 0.35 {
		col = "#c44"
	} else if math.Abs(heel) >= 0.15 {
		col = "#ca4"
	}
	e.ctx.Set("font", fmt.Sprintf("%s %.0fpx %s", "500", 9.0, uiFontBody))
	e.ctx.Set("textBaseline", "alphabetic")
	e.ctx.Set("fillStyle", "#2a4050")
	e.ctx.Set("textAlign", "left")
	e.ctx.Call("fillText", "P", x+4, y+h-4)
	e.ctx.Set("textAlign", "right")
	e.ctx.Call("fillText", "S", x+w-4, y+h-4)
	e.ctx.Set("fillStyle", col)
	e.ctx.Set("textAlign", "center")
	e.ctx.Call("fillText", fmt.Sprintf("%s%.1f°", sign, deg), x+w/2, y+h-4)

	pct := e.completedShipLayers * 100 / MaxLevel
	e.ctx.Set("fillStyle", "#2a4050")
	e.ctx.Call("fillText", fmt.Sprintf("loaded %d%%", pct), x+w/2, y+h-14)
}

// ── level end ─────────────────────────────────────────────────────────────────

func (e *Engine) renderLevelEnd() {
	if e.levelSumm == nil {
		return
	}
	s := e.levelSumm
	color := e.crtColor()
	dim := "#5a7888"

	// Overlay na planszy
	e.ctx.Set("fillStyle", "rgba(0,0,0,0.82)")
	e.ctx.Call("fillRect", boardX, boardY, float64(COLS)*CELL, float64(ROWS)*CELL)

	cx := boardX + float64(COLS)*CELL/2
	y := boardY + 28.0

	e.crispGlow(fmt.Sprintf("LEVEL %d", s.Level), cx, y, 24, "center", color)
	y += 10

	// Separator
	e.ctx.Set("strokeStyle", "#2e404e")
	e.ctx.Set("lineWidth", 1)
	e.ctx.Call("beginPath")
	e.ctx.Call("moveTo", boardX+16, y+6)
	e.ctx.Call("lineTo", boardX+float64(COLS)*CELL-16, y+6)
	e.ctx.Call("stroke")
	y += 22

	// Stats poziomu
	row := func(label, val string) {
		e.ctx.Set("fillStyle", dim)
		e.text(label, boardX+12, y, 16, "left")
		e.crispGlow(val, boardX+float64(COLS)*CELL-12, y, 18, "right", color)
		y += 24
	}
	e.ctx.Set("fillStyle", "#4a6070")
	e.text("— this level —", cx, y, 14, "center")
	y += 20
	row("LINES", itoa(s.LinesLevel))
	row("SCORE", itoa(s.ScoreLevel))

	// Separator
	e.ctx.Set("strokeStyle", "#2e404e")
	e.ctx.Call("beginPath")
	e.ctx.Call("moveTo", boardX+16, y)
	e.ctx.Call("lineTo", boardX+float64(COLS)*CELL-16, y)
	e.ctx.Call("stroke")
	y += 14

	e.ctx.Set("fillStyle", "#4a6070")
	e.text("— total —", cx, y, 14, "center")
	y += 20
	row("LINES", itoa(s.TotalLines))
	row("SCORE", itoa(s.TotalScore))

	// Separator
	e.ctx.Set("strokeStyle", "#2e404e")
	e.ctx.Call("beginPath")
	e.ctx.Call("moveTo", boardX+16, y+4)
	e.ctx.Call("lineTo", boardX+float64(COLS)*CELL-16, y+4)
	e.ctx.Call("stroke")
	y += 24

	// Przyciski
	e.ctx.Set("fillStyle", color)
	e.text("SPACE / ENTER", cx, y, 16, "center")
	y += 20
	e.ctx.Set("fillStyle", "#4a7a8a")
	if s.Level >= MaxLevel {
		e.text("end mission", cx, y, 15, "center")
	} else {
		e.text("next level", cx, y, 15, "center")
	}
	y += 26

	e.ctx.Set("fillStyle", "#6a4040")
	e.text("ESC", cx, y, 16, "center")
	y += 20
	e.ctx.Set("fillStyle", "#4a4040")
	e.text("main menu", cx, y, 15, "center")
}

// ── victory ───────────────────────────────────────────────────────────────

func (e *Engine) renderVictory() {
	color := e.crtColor()

	e.ctx.Set("fillStyle", "rgba(0,0,0,0.85)")
	e.ctx.Call("fillRect", boardX, boardY, float64(COLS)*CELL, float64(ROWS)*CELL)

	cx := boardX + float64(COLS)*CELL/2
	y := boardY + float64(ROWS)*CELL/2 - 60

	e.crispGlow("⚓  MISSION COMPLETE", cx, y, 22, "center", color)
	y += 36

	e.crispGlow(fmt.Sprintf("SCORE: %d", e.score), cx, y, 20, "center", color)
	y += 28

	e.ctx.Set("fillStyle", "#5a7888")
	e.text(fmt.Sprintf("LINES: %d", e.lines), cx, y, 16, "center")
	y += 20
	e.text(fmt.Sprintf("LEVELS: %d/%d", MaxLevel, MaxLevel), cx, y, 16, "center")
	y += 36

	e.ctx.Set("fillStyle", color)
	e.text("SPACE / ENTER = main menu", cx, y, 16, "center")
	y += 20
	e.ctx.Set("fillStyle", "#4a4040")
	e.text("ESC = main menu", cx, y, 14, "center")
}

// ── paused ────────────────────────────────────────────────────────────────────

func (e *Engine) renderPaused() {
	e.ctx.Set("fillStyle", "rgba(0,0,0,0.65)")
	e.ctx.Call("fillRect", boardX, boardY, float64(COLS)*CELL, float64(ROWS)*CELL)
	cy := boardY + float64(ROWS)*CELL/2
	e.glow(14)
	e.ctx.Set("fillStyle", e.crtColor())
	e.text("PAUSED", boardX+float64(COLS)*CELL/2, cy, 28, "center")
	e.noGlow()
	e.ctx.Set("fillStyle", "#3a5060")
	e.text("P / ESC = resume", boardX+float64(COLS)*CELL/2, cy+20, 13, "center")
	e.text("Q = main menu", boardX+float64(COLS)*CELL/2, cy+36, 13, "center")
}

// ── game over ─────────────────────────────────────────────────────────────────

func (e *Engine) renderGameOver() {
	e.ctx.Set("fillStyle", "rgba(0,0,0,0.78)")
	e.ctx.Call("fillRect", boardX, boardY, float64(COLS)*CELL, float64(ROWS)*CELL)

	titleCol := "#f84"
	titleText := "CARGO HOLD FULL"
	titleSize := 22.0
	if e.gameOverReason == GameOverReasonShipSank {
		titleCol = "#4af"
		titleText = e.retryPrompt
		if titleText == "" {
			titleText = "Play again."
		}
		titleSize = 18
	}

	cy := boardY + float64(ROWS)*CELL/2
	e.glow(16)
	e.ctx.Set("fillStyle", titleCol)
	e.text(titleText, boardX+float64(COLS)*CELL/2, cy-8, titleSize, "center")
	e.noGlow()
	e.ctx.Set("fillStyle", "#3a5060")
	e.text(fmt.Sprintf("score: %d", e.score), boardX+float64(COLS)*CELL/2, cy+14, 16, "center")
	e.ctx.Set("fillStyle", "#2a4050")
	e.text("SPACE / ENTER = main menu", boardX+float64(COLS)*CELL/2, cy+30, 14, "center")
	e.ctx.Set("fillStyle", "#4a4040")
	e.text("ESC = main menu", boardX+float64(COLS)*CELL/2, cy+46, 14, "center")
}
