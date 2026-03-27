//go:build js && wasm

package main

const (
	COLS = 8
	ROWS = 16

	CELL   = 36.0
	CENTER = float64(COLS-1) / 2.0

	charPanelW = 260.0 // left panel for the mascot character (2x scale)

	boardX = charPanelW
	boardY = 44.0
	sideX  = charPanelW + COLS*CELL + 10
	sideW  = 264.0

	shipGap   = 32.0
	shipViewH = 88.0

	canvasW = charPanelW + COLS*CELL + 10 + sideW      // 260 + 298 + 264 = 822
	canvasH = ROWS*CELL + boardY + shipGap + shipViewH // 576 + 44 + 32 + 88 = 740

	RedLimit             = 9.0 // seconds in red zone before ship sinks
	ReeferFreezeDuration = 20.0
	TrimReferenceCells   = float64(COLS)
	HazExplosionPenalty  = 300

	MaxLevel = 5
)

// levelConfig defines per-level difficulty parameters.
type levelConfig struct {
	Duration   float64 // seconds to complete the level
	DropSpeed  float64 // base drop interval (lower = faster)
	GreenZone  float64 // heel half-width for green (safe) zone
	YellowZone float64 // heel half-width for yellow (warning) zone
	ReefCount  int     // fixed number of reef pieces per level
	HazCount   int     // fixed number of haz pieces per level
}

var levelConfigs = [MaxLevel]levelConfig{
	{Duration: 150, DropSpeed: 0.85, GreenZone: 0.35, YellowZone: 0.55, ReefCount: 1, HazCount: 10},
	{Duration: 140, DropSpeed: 0.72, GreenZone: 0.30, YellowZone: 0.50, ReefCount: 2, HazCount: 12},
	{Duration: 120, DropSpeed: 0.58, GreenZone: 0.25, YellowZone: 0.42, ReefCount: 2, HazCount: 14},
	{Duration: 105, DropSpeed: 0.46, GreenZone: 0.20, YellowZone: 0.36, ReefCount: 3, HazCount: 13},
	{Duration: 90, DropSpeed: 0.36, GreenZone: 0.16, YellowZone: 0.30, ReefCount: 3, HazCount: 16},
}

func levelDuration(level int) float64 {
	if level >= 1 && level <= MaxLevel {
		return levelConfigs[level-1].Duration
	}
	return 60
}

func levelDropSpeed(level int) float64 {
	if level >= 1 && level <= MaxLevel {
		return levelConfigs[level-1].DropSpeed
	}
	return 0.20
}

var coColor = map[string]string{
	"orange": "#b84a0a", // rdzawo-pomarańczowy, typowy RAL 2009
	"white":  "#7a8e9e", // szaro-niebieski, "białe" kontenery są często brudne
	"reef":   "#0d6e6e", // ciemny morski turkus
	"haz":    "#8b1010", // głęboka czerwień ostrzeżenia
}

var coOutline = map[string]string{
	"orange": "rgba(220,130,50,0.85)",
	"white":  "rgba(170,195,225,0.85)",
	"reef":   "rgba(60,210,220,0.85)",
	"haz":    "rgba(255,120,20,0.85)",
}

// isBlock[co] = true → row containing this type cannot be cleared
var isBlock = map[string]bool{}
