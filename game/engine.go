//go:build js && wasm

package main

import (
	"math/rand"
	"syscall/js"
)

type GameState int

type GameOverReason int

const (
	StateMainMenu GameState = iota
	StatePlaying
	StateGameOver
	StatePaused
	StateLevelEnd
	StateVictory
)

const (
	GameOverReasonNone GameOverReason = iota
	GameOverReasonHoldFull
	GameOverReasonShipSank
)

var replayPrompts = []string{
	"Give it another shot.",
	"One more run.",
	"You were close — try again.",
	"Reset and stack smarter.",
	"New voyage, better balance.",
	"Run it back.",
}

type LevelSummary struct {
	Level      int
	LinesLevel int
	ScoreLevel int
	TotalScore int
	TotalLines int
}

type Cell struct {
	Co   string
	Pid  int
	RibH bool // żebra poziome (kontener stoi pionowo)
}

type PieceBody struct {
	Co    string
	Cells []Vec2
}

type Piece struct {
	Shape []Vec2
	Co    string
	Label string
	R, C  int
}

type FlashMsg struct {
	Text  string
	Color string
	T     float64 // remaining seconds to display
}

type ExplosionFx struct {
	X   float64
	Y   float64
	T   float64
	Dur float64
}

type Engine struct {
	canvas js.Value
	ctx    js.Value

	state GameState

	grid      [ROWS][COLS]*Cell
	rowFreeze [ROWS]float64
	bodies    map[int]*PieceBody // pid → body for shape outline rendering
	pidCount  int

	cur  Piece
	next PieceDef

	score int
	lines int
	level int

	dropTimer  float64
	redTimer   float64 // time spent in red heel zone
	levelTimer float64 // czas w bieżącym poziomie (0 → LevelDuration)

	levelStartScore int // score na początku poziomu
	levelStartLines int // linie na początku poziomu
	levelEndPending bool
	levelPieceCount int
	lastSpawnCo     string
	lastShapeLabel  string // for anti-repeat: label of last spawned piece

	specialSchedule []scheduledSpecial // pre-planned reef/haz timing
	specialIdx      int                // next unserved entry in schedule

	curHeel  float64
	heelAnim float64

	flash             *FlashMsg
	explosions        []ExplosionFx
	comboText         string
	comboTime         float64
	levelSumm         *LevelSummary // dane do ekranu końca poziomu
	retryPrompt       string
	gameOverReason    GameOverReason
	lastResultPending bool
	lastResultScore   int
	lastResultLines   int
	lastResultLevel   int

	completedShipLayers int

	char Character
	keys map[string]bool
}

func (e *Engine) audioScene() string {
	switch e.state {
	case StateMainMenu:
		return "menu"
	default:
		return "game"
	}
}

func (e *Engine) stateName() string {
	switch e.state {
	case StateMainMenu:
		return "menu"
	case StatePlaying:
		return "playing"
	case StateGameOver:
		return "game_over"
	case StatePaused:
		return "paused"
	case StateLevelEnd:
		return "level_end"
	case StateVictory:
		return "victory"
	default:
		return "unknown"
	}
}

func NewEngine(canvas js.Value) *Engine {
	opts := js.Global().Get("Object").New()
	opts.Set("colorSpace", "display-p3")
	hdr := js.Global().Get("hdrDisplay")
	if !hdr.IsUndefined() && hdr.Bool() {
		opts.Set("pixelFormat", "float16") // HDR brightness > 1.0 na wspieranych wyświetlaczach
	}
	ctx := canvas.Call("getContext", "2d", opts)
	if ctx.IsNull() || ctx.IsUndefined() {
		ctx = canvas.Call("getContext", "2d")
	}
	e := &Engine{
		canvas: canvas,
		ctx:    ctx,
		keys:   make(map[string]bool),
	}
	e.enterMainMenu()
	return e
}

func (e *Engine) enterMainMenu() {
	pending := false
	score := 0
	lines := 0
	level := 0
	if (e.state == StateGameOver || e.state == StateVictory) && (e.score > 0 || e.lines > 0) {
		pending = true
		score = e.score
		lines = e.lines
		level = e.level
	}
	e.newGame()
	if pending {
		e.lastResultPending = true
		e.lastResultScore = score
		e.lastResultLines = lines
		e.lastResultLevel = level
	}
	e.state = StateMainMenu
	e.flash = nil
	e.explosions = nil
	e.levelSumm = nil
	e.retryPrompt = ""
	e.gameOverReason = GameOverReasonNone
}

func (e *Engine) newGame() {
	e.grid = [ROWS][COLS]*Cell{}
	e.rowFreeze = [ROWS]float64{}
	e.bodies = make(map[int]*PieceBody)
	e.pidCount = 0

	e.score = 0
	e.lines = 0
	e.level = 1

	e.dropTimer = 0
	e.redTimer = 0
	e.levelTimer = 0
	e.curHeel = 0
	e.heelAnim = 0

	e.levelStartScore = 0
	e.levelStartLines = 0
	e.levelEndPending = false
	e.levelPieceCount = 0
	e.lastSpawnCo = ""
	e.lastShapeLabel = ""
	e.levelSumm = nil
	e.retryPrompt = ""
	e.gameOverReason = GameOverReasonNone
	e.completedShipLayers = 0
	e.char = newCharacter()

	e.flash = nil
	e.explosions = nil
	e.comboText = ""
	e.comboTime = 0
	e.state = StatePlaying
	e.lastResultPending = false

	e.generateSpecialSchedule()
	e.next = e.drawNextPiece()
	e.spawn()
}

func (e *Engine) nextLevel() {
	if e.completedShipLayers < MaxLevel {
		e.completedShipLayers++
	}

	if e.level >= MaxLevel {
		e.grid = [ROWS][COLS]*Cell{}
		e.bodies = make(map[int]*PieceBody)
		e.state = StateVictory
		return
	}

	e.level++

	// Reset board for fresh level
	e.grid = [ROWS][COLS]*Cell{}
	e.rowFreeze = [ROWS]float64{}
	e.bodies = make(map[int]*PieceBody)
	e.pidCount = 0

	e.levelTimer = 0
	e.dropTimer = 0
	e.redTimer = 0
	e.curHeel = 0
	e.heelAnim = 0

	e.levelStartScore = e.score
	e.levelStartLines = e.lines
	e.levelEndPending = false
	e.levelPieceCount = 0
	e.lastSpawnCo = ""
	e.lastShapeLabel = ""
	e.levelSumm = nil
	e.retryPrompt = ""
	e.gameOverReason = GameOverReasonNone

	e.flash = nil
	e.comboText = ""
	e.comboTime = 0

	e.state = StatePlaying
	e.generateSpecialSchedule()
	e.next = e.drawNextPiece()
	e.spawn()
}

func (e *Engine) spawn() {
	d := e.next
	_, w := shapeDims(d.Shape)
	e.cur = Piece{
		Shape: copyShape(d.Shape),
		Co:    d.Co,
		Label: d.Label,
		R:     0,
		C:     (COLS - w) / 2,
	}
	e.levelPieceCount++
	e.lastSpawnCo = d.Co
	e.lastShapeLabel = d.Label
	e.next = e.drawNextPiece()
	if !e.canFit(0, e.cur.C, e.cur.Shape) {
		e.gameOverReason = GameOverReasonHoldFull
		e.state = StateGameOver
	}
}

func (e *Engine) drawNextPiece() PieceDef {
	// Check if a scheduled special cargo piece is due
	if e.specialIdx < len(e.specialSchedule) {
		dur := levelDuration(e.level)
		frac := 0.0
		if dur > 0 {
			frac = e.levelTimer / dur
		}
		sched := e.specialSchedule[e.specialIdx]
		if e.levelEndPending || frac >= sched.Fraction {
			e.specialIdx++
			return makeSpecialPiece(sched.Co)
		}
	}

	// Normal piece with anti-repeat on shape label
	for i := 0; i < 5; i++ {
		d := randNormalDef()
		if d.Label != e.lastShapeLabel {
			return d
		}
	}
	return randNormalDef()
}

// generateSpecialSchedule creates the pre-planned timing for reef/haz pieces.
// Uses stratified sampling: the level timeline is divided into equal segments
// and one special piece is placed randomly within each segment.
func (e *Engine) generateSpecialSchedule() {
	e.specialSchedule = nil
	e.specialIdx = 0

	if e.level < 1 || e.level > MaxLevel {
		return
	}
	cfg := levelConfigs[e.level-1]
	total := cfg.ReefCount + cfg.HazCount
	if total == 0 {
		return
	}

	// Shuffled list of types
	types := make([]string, 0, total)
	for i := 0; i < cfg.ReefCount; i++ {
		types = append(types, "reef")
	}
	for i := 0; i < cfg.HazCount; i++ {
		types = append(types, "haz")
	}
	rand.Shuffle(len(types), func(i, j int) {
		types[i], types[j] = types[j], types[i]
	})

	// Stratified placement across level duration
	e.specialSchedule = make([]scheduledSpecial, total)
	for i, co := range types {
		segStart := float64(i) / float64(total)
		segEnd := float64(i+1) / float64(total)
		margin := (segEnd - segStart) * 0.12
		frac := segStart + margin + rand.Float64()*(segEnd-segStart-2*margin)
		if frac < 0.05 {
			frac = 0.05
		}
		if frac > 0.94 {
			frac = 0.94
		}
		e.specialSchedule[i] = scheduledSpecial{Fraction: frac, Co: co}
	}

	// Insertion sort by fraction (N is small)
	for i := 1; i < len(e.specialSchedule); i++ {
		for j := i; j > 0 && e.specialSchedule[j].Fraction < e.specialSchedule[j-1].Fraction; j-- {
			e.specialSchedule[j], e.specialSchedule[j-1] = e.specialSchedule[j-1], e.specialSchedule[j]
		}
	}
}

func randomReplayPrompt() string {
	if len(replayPrompts) == 0 {
		return "Play again."
	}
	return replayPrompts[rand.Intn(len(replayPrompts))]
}
