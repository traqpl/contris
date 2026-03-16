//go:build js && wasm

package main

const (
	COLS = 8
	ROWS = 16

	CELL   = 36.0
	CENTER = float64(COLS-1) / 2.0

	boardX = 0.0
	boardY = 28.0
	sideX  = COLS*CELL + 10
	sideW  = 192.0

	shipViewH = 88.0

	canvasW = COLS*CELL + 10 + sideW         // 298 + 192 = 490
	canvasH = ROWS*CELL + boardY + shipViewH // 576 + 28 + 88 = 692

	RedLimit = 9.0 // seconds in red zone before ship sinks

	MaxLevel = 5
)

// levelConfig defines per-level difficulty parameters.
type levelConfig struct {
	Duration   float64 // seconds to complete the level
	DropSpeed  float64 // base drop interval (lower = faster)
	GreenZone  float64 // heel half-width for green (safe) zone
	YellowZone float64 // heel half-width for yellow (warning) zone
}

var levelConfigs = [MaxLevel]levelConfig{
	{Duration: 120, DropSpeed: 0.70, GreenZone: 0.28, YellowZone: 0.48},
	{Duration: 105, DropSpeed: 0.50, GreenZone: 0.22, YellowZone: 0.40},
	{Duration: 90, DropSpeed: 0.38, GreenZone: 0.16, YellowZone: 0.32},
	{Duration: 75, DropSpeed: 0.28, GreenZone: 0.12, YellowZone: 0.26},
	{Duration: 60, DropSpeed: 0.20, GreenZone: 0.08, YellowZone: 0.20},
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
var isBlock = map[string]bool{
	"haz": true,
}
