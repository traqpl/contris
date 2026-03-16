//go:build js && wasm

package main

import "math"

func (e *Engine) Update(dt float64) {
	if e.state != StatePlaying {
		return
	}

	// Drop timer
	speed := levelDropSpeed(e.level)
	e.dropTimer += dt
	if e.dropTimer >= speed {
		e.dropTimer = 0
		e.drop()
	}

	// Red zone timer → ship sinks
	if heelZone(e.curHeel, e.level) == "red" {
		e.redTimer = math.Min(e.redTimer+dt, RedLimit)
		if e.redTimer >= RedLimit {
			e.state = StateGameOver
			e.flash = &FlashMsg{Text: "⛵  STATEK ZATONĄŁ", Color: "#4af", T: 99}
		}
	} else {
		e.redTimer = math.Max(0, e.redTimer-dt*0.5)
	}

	// Poziom czasowy
	if e.state == StatePlaying {
		e.levelTimer += dt
		if e.levelTimer >= levelDuration(e.level) {
			e.levelSumm = &LevelSummary{
				Level:      e.level,
				LinesLevel: e.lines - e.levelStartLines,
				ScoreLevel: e.score - e.levelStartScore,
				TotalScore: e.score,
				TotalLines: e.lines,
			}
			e.state = StateLevelEnd
		}
	}

	// Decay combo text
	if e.comboTime > 0 {
		e.comboTime -= dt
		if e.comboTime <= 0 {
			e.comboText = ""
		}
	}
	// Decay flash
	if e.flash != nil {
		e.flash.T -= dt
	}
}

// ── movement ─────────────────────────────────────────────────────────────────

func (e *Engine) canFit(r, c int, shape []Vec2) bool {
	for _, v := range shape {
		nr, nc := r+v.R, c+v.C
		if nr >= ROWS || nc < 0 || nc >= COLS {
			return false
		}
		if nr >= 0 && e.grid[nr][nc] != nil {
			return false
		}
	}
	return true
}

func (e *Engine) moveH(d int) {
	if e.canFit(e.cur.R, e.cur.C+d, e.cur.Shape) {
		e.cur.C += d
	}
}

func (e *Engine) rotPiece() {
	if len(e.cur.Shape) == 1 {
		return
	}
	ns := rotate90(e.cur.Shape)
	for _, dc := range []int{0, -1, 1, -2, 2} {
		if e.canFit(e.cur.R, e.cur.C+dc, ns) {
			e.cur.Shape = ns
			e.cur.C += dc
			return
		}
	}
}

func (e *Engine) drop() {
	if e.canFit(e.cur.R+1, e.cur.C, e.cur.Shape) {
		e.cur.R++
	} else {
		e.lock()
	}
}

func (e *Engine) hardDrop() {
	for e.canFit(e.cur.R+1, e.cur.C, e.cur.Shape) {
		e.cur.R++
	}
	e.lock()
}

func (e *Engine) lock() {
	pid := e.pidCount + 1
	e.pidCount++
	h, w := shapeDims(e.cur.Shape)
	ribH := h > w // więcej wierszy niż kolumn → kontener pionowy → żebra poziome
	for i, v := range e.cur.Shape {
		r, c := e.cur.R+v.R, e.cur.C+v.C
		if r >= 0 && r < ROWS {
			e.grid[r][c] = &Cell{Co: e.cur.Co, Pid: pid, Wear: e.cur.Wear[i], RibH: ribH}
		}
	}
	e.processBoard()
	e.spawn()
}

// ── board processing ──────────────────────────────────────────────────────────

func (e *Engine) processBoard() {
	if e.activateHaz() {
		e.applyGravity()
	}
	if e.activateReef() {
		e.applyGravity()
	}
	if e.clearRows() > 0 {
		e.applyGravity()
	}
	e.curHeel = e.gridHeel()
	e.rebuildBodies()
}

// ── balance ───────────────────────────────────────────────────────────────────

func (e *Engine) heelFromExtra(extra []int) float64 {
	moment, total := 0.0, 0.0
	for r := 0; r < ROWS; r++ {
		for c := 0; c < COLS; c++ {
			if e.grid[r][c] == nil {
				continue
			}
			moment += float64(c) - CENTER
			total++
		}
	}
	for _, c := range extra {
		moment += float64(c) - CENTER
		total++
	}
	if total == 0 {
		return 0
	}
	return moment / (total * CENTER)
}

func (e *Engine) gridHeel() float64 { return e.heelFromExtra(nil) }

func (e *Engine) predictedHeel() float64 {
	gr := e.cur.R
	for e.canFit(gr+1, e.cur.C, e.cur.Shape) {
		gr++
	}
	extra := make([]int, len(e.cur.Shape))
	for i, v := range e.cur.Shape {
		extra[i] = e.cur.C + v.C
	}
	return e.heelFromExtra(extra)
}

func zones(level int) (g, y float64) {
	if level >= 1 && level <= MaxLevel {
		cfg := levelConfigs[level-1]
		return cfg.GreenZone, cfg.YellowZone
	}
	return 0.08, 0.20
}

func heelZone(h float64, level int) string {
	g, y := zones(level)
	a := math.Abs(h)
	if a <= g {
		return "green"
	}
	if a <= y {
		return "yellow"
	}
	return "red"
}

// ── row logic ─────────────────────────────────────────────────────────────────

type rowResult struct {
	pts  int
	mono bool
	zone string
}

func (e *Engine) evalRow(r int, heel float64) *rowResult {
	for _, cell := range e.grid[r] {
		if cell == nil {
			return nil
		}
	}
	for _, cell := range e.grid[r] {
		if isBlock[cell.Co] {
			return nil
		}
	}
	zone := heelZone(heel, e.level)
	if zone == "red" {
		return nil
	}
	cos := map[string]bool{}
	for _, cell := range e.grid[r] {
		if cell.Co == "orange" || cell.Co == "white" {
			cos[cell.Co] = true
		}
	}
	mono := len(cos) <= 1
	pts := 0
	switch {
	case mono && zone == "green":
		pts = 400
	case mono && zone == "yellow":
		pts = 200
	case !mono && zone == "green":
		pts = 150
	default:
		pts = 80
	}
	return &rowResult{pts: pts, mono: mono, zone: zone}
}

func (e *Engine) clearRows() int {
	heel := e.gridHeel()
	cleared, totalPts, lastZone := 0, 0, ""

	for r := ROWS - 1; r >= 0; r-- {
		res := e.evalRow(r, heel)
		if res == nil {
			continue
		}
		pts := res.pts * (cleared + 1)
		e.score += pts
		totalPts += pts
		e.lines++
		cleared++
		lastZone = res.zone

		// Remove row, prepend empty
		copy(e.grid[1:r+1], e.grid[0:r])
		e.grid[0] = [COLS]*Cell{}
		r++ // re-check same index
	}

	if cleared > 0 {
		if lastZone == "green" {
			bonus := totalPts / 2
			e.score += bonus
			totalPts += bonus
		}
		bonus := ""
		if lastZone == "green" {
			bonus = " ⚓+50%"
		}
		if cleared > 1 {
			e.comboText = "×" + itoa(cleared) + " COMBO  +" + itoa(totalPts) + bonus
		} else {
			e.comboText = "+" + itoa(totalPts) + bonus
		}
		e.comboTime = 2.5
	}
	return cleared
}

// ── special cargo ─────────────────────────────────────────────────────────────

var d8 = [8][2]int{{-1, -1}, {-1, 0}, {-1, 1}, {0, -1}, {0, 1}, {1, -1}, {1, 0}, {1, 1}}

func (e *Engine) bfs8(sr, sc int, co string, vis map[[2]int]bool) [][2]int {
	grp := [][2]int{}
	q := [][2]int{{sr, sc}}
	for len(q) > 0 {
		cur := q[0]
		q = q[1:]
		cr, cc := cur[0], cur[1]
		if cr < 0 || cr >= ROWS || cc < 0 || cc >= COLS {
			continue
		}
		if vis[cur] {
			continue
		}
		cell := e.grid[cr][cc]
		if cell == nil || cell.Co != co {
			continue
		}
		vis[cur] = true
		grp = append(grp, cur)
		for _, d := range d8 {
			q = append(q, [2]int{cr + d[0], cc + d[1]})
		}
	}
	return grp
}

func (e *Engine) activateHaz() bool {
	vis := map[[2]int]bool{}
	any := false
	for r := 0; r < ROWS; r++ {
		for c := 0; c < COLS; c++ {
			k := [2]int{r, c}
			if vis[k] || e.grid[r][c] == nil || e.grid[r][c].Co != "haz" {
				continue
			}
			grp := e.bfs8(r, c, "haz", vis)
			if len(grp) >= 2 {
				rows := map[int]bool{}
				for _, pos := range grp {
					rows[pos[0]] = true
				}
				for row := range rows {
					e.grid[row] = [COLS]*Cell{}
				}
				e.score = max0(e.score - 50)
				e.flash = &FlashMsg{
					Text:  "⚠  WYBUCH  −50 pkt",
					Color: "#ff8822",
					T:     2.2,
				}
				any = true
			}
		}
	}
	return any
}

func (e *Engine) activateReef() bool {
	vis := map[[2]int]bool{}
	any := false
	for r := 0; r < ROWS; r++ {
		for c := 0; c < COLS; c++ {
			k := [2]int{r, c}
			if vis[k] || e.grid[r][c] == nil || e.grid[r][c].Co != "reef" {
				continue
			}
			grp := e.bfs8(r, c, "reef", vis)
			if len(grp) >= 3 {
				pts := len(grp) * 100
				e.score += pts
				for _, pos := range grp {
					e.grid[pos[0]][pos[1]] = nil
				}
				e.flash = &FlashMsg{
					Text:  "❄  ŁAŃCUCH  +" + itoa(pts) + " pkt",
					Color: "#44ddff",
					T:     2.2,
				}
				any = true
			}
		}
	}
	return any
}

// ── gravity ───────────────────────────────────────────────────────────────────

func (e *Engine) applyGravity() {
	for c := 0; c < COLS; c++ {
		col := []*Cell{}
		for r := ROWS - 1; r >= 0; r-- {
			if e.grid[r][c] != nil {
				col = append(col, e.grid[r][c])
			}
		}
		for r := ROWS - 1; r >= 0; r-- {
			if len(col) > 0 {
				e.grid[r][c] = col[0]
				col = col[1:]
			} else {
				e.grid[r][c] = nil
			}
		}
	}
}

// ── bodies map for shape outline rendering ───────────────────────────────────

func (e *Engine) rebuildBodies() {
	e.bodies = make(map[int]*PieceBody)
	for r := 0; r < ROWS; r++ {
		for c := 0; c < COLS; c++ {
			cell := e.grid[r][c]
			if cell == nil {
				continue
			}
			b, ok := e.bodies[cell.Pid]
			if !ok {
				b = &PieceBody{Co: cell.Co}
				e.bodies[cell.Pid] = b
			}
			b.Cells = append(b.Cells, Vec2{r, c})
		}
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	buf := [20]byte{}
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}

func max0(n int) int {
	if n < 0 {
		return 0
	}
	return n
}
