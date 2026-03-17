//go:build js && wasm

package main

import "math"

func (e *Engine) Update(dt float64) {
	if e.state != StatePlaying {
		return
	}

	for r := 0; r < ROWS; r++ {
		e.rowFreeze[r] = math.Max(0, e.rowFreeze[r]-dt)
	}

	levelDurationLimit := levelDuration(e.level)
	if !e.levelEndPending {
		e.levelTimer += dt
		if e.levelTimer >= levelDurationLimit {
			e.levelTimer = levelDurationLimit
			e.levelEndPending = true
		}
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
			e.gameOverReason = GameOverReasonShipSank
			e.retryPrompt = randomReplayPrompt()
			e.state = StateGameOver
			e.flash = &FlashMsg{Text: "⛵", Color: "#4af", T: 99}
		}
	} else {
		e.redTimer = math.Max(0, e.redTimer-dt*0.5)
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
	freezeRows := map[int]bool{}
	for i, v := range e.cur.Shape {
		r, c := e.cur.R+v.R, e.cur.C+v.C
		if r >= 0 && r < ROWS {
			cell := &Cell{Co: e.cur.Co, Pid: pid, Wear: e.cur.Wear[i], RibH: ribH}
			if e.cur.Co == "reef" {
				freezeRows[r] = true
			}
			e.grid[r][c] = cell
		}
	}
	if e.cur.Co == "reef" {
		for r := range freezeRows {
			e.rowFreeze[r] = ReeferFreezeDuration
		}
		e.flash = &FlashMsg{
			Text:  "❄  ROW FROZEN",
			Color: "#44ddff",
			T:     2.2,
		}
	}
	e.processBoard()
	if e.levelEndPending {
		e.levelSumm = &LevelSummary{
			Level:      e.level,
			LinesLevel: e.lines - e.levelStartLines,
			ScoreLevel: e.score - e.levelStartScore,
			TotalScore: e.score,
			TotalLines: e.lines,
		}
		e.levelEndPending = false
		e.state = StateLevelEnd
		return
	}
	e.spawn()
}

// ── board processing ──────────────────────────────────────────────────────────

func (e *Engine) processBoard() {
	e.rebuildBodies()
	if e.activateHaz() {
		e.rebuildBodies()
		e.applyGravity()
	}
	if e.clearRows() > 0 {
		e.rebuildBodies()
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
	zone string
}

func (e *Engine) rowFrozenTime(r int) float64 {
	return e.rowFreeze[r]
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
	if e.rowFrozenTime(r) > 0 {
		return nil
	}
	zone := heelZone(heel, e.level)
	if zone == "red" {
		return nil
	}
	pts := 100
	if zone == "green" {
		pts = 200
	}
	return &rowResult{pts: pts, zone: zone}
}

func (e *Engine) clearRows() int {
	heel := e.gridHeel()
	cleared, totalPts := 0, 0

	for r := ROWS - 1; r >= 0; r-- {
		res := e.evalRow(r, heel)
		if res == nil {
			continue
		}
		pts := res.pts
		e.score += pts
		totalPts += pts
		e.lines++
		cleared++

		// Remove row, prepend empty
		copy(e.grid[1:r+1], e.grid[0:r])
		e.grid[0] = [COLS]*Cell{}
		r++ // re-check same index
	}

	if cleared > 0 {
		if cleared > 1 {
			e.comboText = itoa(cleared) + " ROWS  +" + itoa(totalPts)
		} else {
			e.comboText = "+" + itoa(totalPts)
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
					Text:  "⚠  EXPLOSION  −50 pts",
					Color: "#ff8822",
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
	for {
		e.rebuildBodies()
		falling := e.fallingBodies()
		if len(falling) == 0 {
			return
		}

		next := [ROWS][COLS]*Cell{}
		for r := 0; r < ROWS; r++ {
			for c := 0; c < COLS; c++ {
				cell := e.grid[r][c]
				if cell == nil || falling[cell.Pid] {
					continue
				}
				next[r][c] = cell
			}
		}
		for pid, body := range e.bodies {
			for _, cellPos := range body.Cells {
				cell := e.grid[cellPos.R][cellPos.C]
				if cell == nil {
					continue
				}
				targetR := cellPos.R
				if falling[pid] {
					targetR++
				}
				next[targetR][cellPos.C] = cell
			}
		}
		e.grid = next
	}
}

func (e *Engine) fallingBodies() map[int]bool {
	falling := make(map[int]bool, len(e.bodies))
	for pid := range e.bodies {
		falling[pid] = true
	}

	changed := true
	for changed {
		changed = false
		for pid, body := range e.bodies {
			if !falling[pid] {
				continue
			}
			for _, cellPos := range body.Cells {
				belowR := cellPos.R + 1
				if belowR >= ROWS {
					delete(falling, pid)
					changed = true
					break
				}
				below := e.grid[belowR][cellPos.C]
				if below == nil || below.Pid == pid {
					continue
				}
				if !falling[below.Pid] {
					delete(falling, pid)
					changed = true
					break
				}
			}
		}
	}

	return falling
}

// ── bodies map for shape outline rendering ───────────────────────────────────

func (e *Engine) rebuildBodies() {
	visited := [ROWS][COLS]bool{}
	e.bodies = make(map[int]*PieceBody)
	nextPid := 0
	d4 := [][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}}

	for r := 0; r < ROWS; r++ {
		for c := 0; c < COLS; c++ {
			cell := e.grid[r][c]
			if cell == nil || visited[r][c] {
				continue
			}

			originalPid := cell.Pid
			nextPid++
			body := &PieceBody{Co: cell.Co}
			queue := []Vec2{{R: r, C: c}}
			visited[r][c] = true

			for len(queue) > 0 {
				cur := queue[0]
				queue = queue[1:]
				curCell := e.grid[cur.R][cur.C]
				if curCell == nil {
					continue
				}
				curCell.Pid = nextPid
				body.Cells = append(body.Cells, Vec2{R: cur.R, C: cur.C})

				for _, d := range d4 {
					nr, nc := cur.R+d[0], cur.C+d[1]
					if nr < 0 || nr >= ROWS || nc < 0 || nc >= COLS || visited[nr][nc] {
						continue
					}
					nextCell := e.grid[nr][nc]
					if nextCell == nil || nextCell.Pid != originalPid {
						continue
					}
					visited[nr][nc] = true
					queue = append(queue, Vec2{R: nr, C: nc})
				}
			}

			e.bodies[nextPid] = body
		}
	}
	e.pidCount = nextPid
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
