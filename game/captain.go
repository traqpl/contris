//go:build js && wasm

package main

import (
	"fmt"
	"math"
	"math/rand"
)

// Captain AOP — upper character that watches each piece and gives placement tips.

type Captain struct {
	AnimTime   float64
	BlinkCD    float64
	Blinking   bool
	BubbleText string
	BubbleDur  float64
	BubbleTime float64
	LastBubble float64
	LastLabel  string  // label of last piece we commented on
	TipCD      float64 // cooldown between tips
	Mood       CharacterMood
	MoodTimer  float64
	LookX      float64 // smoothed horizontal gaze offset (pupils follow piece)
	PrevLines  int
}

func newCaptain() Captain {
	return Captain{
		BlinkCD: 1.5 + rand.Float64()*2.0,
	}
}

func (c *Captain) setMood(mood CharacterMood, dur float64) {
	c.Mood = mood
	c.MoodTimer = dur
}

// ── tip message pools ─────────────────────────────────────────────────────────

var captainTips = map[string][]string{
	"2TU": {
		"Place centrally!",
		"Compact piece~",
		"Fills the gaps!",
		"Small but useful!",
		"Stack it tight~",
	},
	"4TU": {
		"Wide load! ↔",
		"Needs 4 columns!",
		"Level the deck~",
		"Keep it flat!",
		"Long piece ahead!",
	},
	"O": {
		"2×2 block!",
		"Stack it high~",
		"Compact square!",
		"Solid block ★",
		"Nice and square!",
	},
	"L": {
		"Corner piece! ↙",
		"Fit the edge~",
		"Rotate for gaps!",
		"L fills corners!",
		"Right-side edge~",
	},
	"J": {
		"Mirror of L~",
		"Left-side edge!",
		"Other corner ↘",
		"Flip it right~",
		"J for junction!",
	},
	"T": {
		"T for trim~ ★",
		"Fill the midgap!",
		"Center it well~",
		"T-bone stacker!",
		"T keeps balance!",
	},
	"S": {
		"Tricky piece~!",
		"Stagger stack!",
		"Fits steps~ ↗",
		"Watch the gaps!",
		"Rotate wisely~",
	},
	"Z": {
		"Fits steps~ ↘",
		"Zigzag stacker!",
		"Z is tricky too!",
		"Fit into steps~",
		"Watch alignment!",
	},
	"REEFER 2TU": {
		"Cold cargo! ❄",
		"Handle with care~",
		"Refrigerated unit!",
		"Keep it cool~ ❄",
		"Special cargo! ❄",
	},
	"HAZMAT": {
		"HAZMAT! Caution!",
		"Isolate this one!",
		"Danger cargo! ☠",
		"Don't cluster it!",
		"Hazmat — careful!",
	},
}

var captainBalanceTips = map[string][]string{
	"left": {
		"Shift right! →",
		"Too heavy port!",
		"Load starboard!",
		"Balance right~",
	},
	"right": {
		"Shift left! ←",
		"Too heavy star!",
		"Load port side!",
		"Balance left~",
	},
}

var captainGameOverAdvice = []string{
	"Keep the center flatter next run.",
	"Leave fewer roof gaps up top.",
	"Build low first, then climb.",
	"Save room for the long pieces.",
	"Don't rush yellow trim too long.",
}

var captainSinkAdvice = []string{
	"Correct trim earlier, not in red.",
	"Counterweight the heavy side sooner.",
	"Keep port and starboard closer.",
	"Flatten the deck before it tilts.",
}

var captainHoldFullAdvice = []string{
	"Clear shelves before stacking higher.",
	"Keep one side open for awkward cargo.",
	"Avoid sealing narrow pockets early.",
	"Use the wide loads to level the deck.",
}

var captainLevelClearCheers = []string{
	"Deck secured! Fine work!",
	"Level cleared. Excellent trim!",
	"Ha! Clean loading there!",
	"Good discipline. Next deck!",
	"Solid work, chief. Carry on!",
}

func (c *Captain) chooseTip(e *Engine) string {
	// Heel-based override when noticeably unbalanced
	heel := e.curHeel
	if math.Abs(heel) > 0.12 {
		if heel < 0 {
			return pickMsg(captainBalanceTips["left"])
		}
		return pickMsg(captainBalanceTips["right"])
	}
	// Piece-specific tip
	if pool, ok := captainTips[e.cur.Label]; ok {
		return pickMsg(pool)
	}
	return "Looking good! ★"
}

func (c *Captain) chooseGameOverAdvice(e *Engine) string {
	switch e.gameOverReason {
	case GameOverReasonShipSank:
		return pickMsg(captainSinkAdvice)
	case GameOverReasonHoldFull:
		return pickMsg(captainHoldFullAdvice)
	default:
		return pickMsg(captainGameOverAdvice)
	}
}

func (c *Captain) showBubble(text string, dur float64) {
	if c.LastBubble > 0 && c.BubbleTime > 0.8 {
		return
	}
	c.BubbleText = text
	c.BubbleDur = dur
	c.BubbleTime = dur
	c.LastBubble = dur * 0.8
}

func (c *Captain) update(dt float64, e *Engine) {
	c.AnimTime += dt

	// blink cycle
	c.BlinkCD -= dt
	if c.BlinkCD <= 0 {
		if c.Blinking {
			c.Blinking = false
			c.BlinkCD = 2.0 + rand.Float64()*3.5
		} else {
			c.Blinking = true
			c.BlinkCD = 0.12
		}
	}

	if c.BubbleTime > 0 {
		c.BubbleTime -= dt
	}
	if c.LastBubble > 0 {
		c.LastBubble -= dt
	}
	if c.TipCD > 0 {
		c.TipCD -= dt
	}

	// Mood decay
	if c.MoodTimer > 0 {
		c.MoodTimer -= dt
		if c.MoodTimer <= 0 && c.Mood != MoodSad {
			c.Mood = MoodIdle
		}
	}

	// Game-over: brief disappointment, then practical coaching
	if e.state == StateGameOver {
		if c.Mood != MoodSad && c.Mood != MoodComfort {
			c.setMood(MoodSad, 1.8)
			c.showBubble("We'll fix it next run.", 2.4)
		} else if c.Mood == MoodSad && c.MoodTimer <= 0 {
			c.setMood(MoodComfort, 99)
			c.showBubble(c.chooseGameOverAdvice(e), 4.0)
		} else if c.Mood == MoodComfort && c.BubbleTime <= 0 && c.LastBubble <= 0 {
			c.showBubble(c.chooseGameOverAdvice(e), 4.0)
		}
		c.LookX *= math.Max(0, 1-dt*2)
		c.PrevLines = e.lines
		return
	}

	if e.state == StateLevelEnd {
		if c.Mood != MoodExcited {
			c.setMood(MoodExcited, 99)
			c.showBubble(pickMsg(captainLevelClearCheers), 4.0)
		} else if c.BubbleTime <= 0 && c.LastBubble <= 0 {
			c.showBubble(pickMsg(captainLevelClearCheers), 4.0)
		}
		c.LookX *= math.Max(0, 1-dt*3)
		c.PrevLines = e.lines
		return
	}

	if e.state != StatePlaying {
		c.LookX *= math.Max(0, 1-dt*3)
		c.PrevLines = e.lines
		return
	}

	// ── smooth gaze toward falling piece ──────────────────────────────────
	if len(e.cur.Shape) > 0 {
		sumC := 0.0
		for _, v := range e.cur.Shape {
			sumC += float64(e.cur.C + v.C)
		}
		avgCol := sumC/float64(len(e.cur.Shape)) + 0.5
		target := (avgCol - float64(COLS)/2.0) / (float64(COLS) / 2.0) * 9.0
		c.LookX += (target - c.LookX) * math.Min(dt*6, 1)
	}

	// ── mood driven by game events ─────────────────────────────────────────
	// row cleared → happy
	if e.lines > c.PrevLines {
		c.setMood(MoodHappy, 2.2)
	}

	// heel zone
	zone := heelZone(e.curHeel, e.level)
	if zone == "red" && c.Mood != MoodScared {
		c.setMood(MoodScared, 99)
	} else if zone == "yellow" && c.Mood != MoodScared && c.Mood != MoodHappy {
		c.setMood(MoodWorried, 99)
	} else if zone == "green" && (c.Mood == MoodScared || c.Mood == MoodWorried) {
		c.setMood(MoodIdle, 0)
	}

	// New piece — show tip
	if e.cur.Label != c.LastLabel {
		c.LastLabel = e.cur.Label
		if c.TipCD <= 0 {
			c.showBubble(c.chooseTip(e), 3.5)
			c.TipCD = 4.5
		}
	}

	// Occasional balance nudge
	if c.BubbleTime <= 0 && c.LastBubble <= 0 {
		heel := e.curHeel
		if math.Abs(heel) > 0.12 && rand.Float64() < 0.007 {
			if heel < 0 {
				c.showBubble(pickMsg(captainBalanceTips["left"]), 3.0)
			} else {
				c.showBubble(pickMsg(captainBalanceTips["right"]), 3.0)
			}
		}
	}

	c.PrevLines = e.lines
}

// ── render ────────────────────────────────────────────────────────────────────

func (e *Engine) renderCaptain() {
	c := &e.captain
	ctx := e.ctx

	// Show in all states except the main menu
	if e.state == StateMainMenu {
		return
	}

	const captainScale = 1.8

	cx := charPanelW / 2.0
	drawBaseY := boardY + float64(ROWS)*CELL*0.31
	captainMidY := drawBaseY - 55.0

	t := c.AnimTime
	levelCelebration := e.state == StateLevelEnd

	// Animation — more subdued on game over
	bobAmp := 1.0
	if c.Mood == MoodSad {
		bobAmp = 0.4
	} else if levelCelebration && c.Mood == MoodExcited {
		bobAmp = 2.8
	}
	bobFreq := 1.1
	swayAmp := 0.4
	if levelCelebration && c.Mood == MoodExcited {
		bobFreq = 3.1
		swayAmp = 1.1
	}
	bob := math.Sin(t*bobFreq) * bobAmp
	sway := math.Sin(t*1.6) * swayAmp
	// Head turns toward the piece: tilt + horizontal shift driven by LookX
	headTilt := c.LookX*0.048 + math.Sin(t*0.7+0.3)*0.010
	if levelCelebration && c.Mood == MoodExcited {
		headTilt += math.Sin(t*2.8) * 0.05
	}

	y := drawBaseY + bob

	ctx.Call("save")
	ctx.Call("translate", cx, captainMidY)
	ctx.Call("scale", captainScale, captainScale)
	ctx.Call("translate", -cx, -captainMidY)

	s := 1.0

	// 3/4 view: body shifted right toward the board
	const poseShift = 6.0

	// ── legs ──────────────────────────────────────────────────────────────
	legY := y
	legW := 7.0 * s
	legH := 19.0 * s
	bootH := 5.0 * s
	legCx := cx + poseShift

	ctx.Set("fillStyle", "#1a2a4a")
	ctx.Call("fillRect", legCx-13*s+sway*0.3, legY-legH, legW, legH)
	ctx.Call("fillRect", legCx+5*s+sway*0.3, legY-legH, legW+1, legH)
	ctx.Set("fillStyle", "#1a1820")
	ctx.Call("fillRect", legCx-14*s+sway*0.3, legY-bootH, legW+2, bootH)
	ctx.Call("fillRect", legCx+4*s+sway*0.3, legY-bootH, legW+3, bootH)

	// ── body (merchant officer jacket, 3/4 view) ──────────────────────────
	bodyTop := legY - legH - 28*s
	bodyH := 31.0 * s
	bodyW := 34.0 * s
	bodyCx := cx + poseShift + sway*0.4

	leftW := bodyW * 0.44
	rightW := bodyW * 0.56

	// Navy jacket silhouette
	ctx.Set("fillStyle", "#152040")
	ctx.Call("beginPath")
	ctx.Call("moveTo", bodyCx-leftW, bodyTop+bodyH)
	ctx.Call("lineTo", bodyCx-leftW+1, bodyTop+2)
	ctx.Call("lineTo", bodyCx+rightW-1, bodyTop+2)
	ctx.Call("lineTo", bodyCx+rightW, bodyTop+bodyH)
	ctx.Call("closePath")
	ctx.Call("fill")

	// Narrow white shirt placket for a closed jacket
	ctx.Set("fillStyle", "#f1f3f6")
	ctx.Call("beginPath")
	ctx.Call("moveTo", bodyCx-3, bodyTop+6)
	ctx.Call("lineTo", bodyCx+3, bodyTop+6)
	ctx.Call("lineTo", bodyCx+4.5, bodyTop+bodyH-2)
	ctx.Call("lineTo", bodyCx-4.5, bodyTop+bodyH-2)
	ctx.Call("closePath")
	ctx.Call("fill")

	// Shirt collar
	ctx.Set("fillStyle", "#fafbfd")
	ctx.Call("beginPath")
	ctx.Call("moveTo", bodyCx-6, bodyTop+2)
	ctx.Call("lineTo", bodyCx-1, bodyTop+8.5)
	ctx.Call("lineTo", bodyCx-7.5, bodyTop+8.5)
	ctx.Call("closePath")
	ctx.Call("fill")
	ctx.Call("beginPath")
	ctx.Call("moveTo", bodyCx+5, bodyTop+2)
	ctx.Call("lineTo", bodyCx+1, bodyTop+8.5)
	ctx.Call("lineTo", bodyCx+8.5, bodyTop+8.5)
	ctx.Call("closePath")
	ctx.Call("fill")

	// Tie + closed front seam
	ctx.Set("fillStyle", "#10161f")
	ctx.Call("beginPath")
	ctx.Call("moveTo", bodyCx-1, bodyTop+8)
	ctx.Call("lineTo", bodyCx+3, bodyTop+8)
	ctx.Call("lineTo", bodyCx+3.5, bodyTop+bodyH-3)
	ctx.Call("lineTo", bodyCx-1.5, bodyTop+bodyH-3)
	ctx.Call("closePath")
	ctx.Call("fill")
	ctx.Set("strokeStyle", "rgba(220,220,230,0.35)")
	ctx.Set("lineWidth", 1.0)
	ctx.Call("beginPath")
	ctx.Call("moveTo", bodyCx+1, bodyTop+3)
	ctx.Call("lineTo", bodyCx+1, bodyTop+bodyH-1.5)
	ctx.Call("stroke")

	// Small front pockets
	ctx.Set("strokeStyle", "rgba(225,230,238,0.35)")
	ctx.Set("lineWidth", 1.0)
	ctx.Call("strokeRect", bodyCx-leftW+5, bodyTop+15, 7, 5)
	ctx.Call("strokeRect", bodyCx+rightW-14, bodyTop+15, 7, 5)

	// Few metal buttons, not parade-style
	ctx.Set("fillStyle", "#c8961a")
	for _, btn := range []struct{ x, y float64 }{
		{bodyCx + 3.5, bodyTop + 13},
		{bodyCx + 3.5, bodyTop + 21},
	} {
		ctx.Call("beginPath")
		ctx.Call("arc", btn.x, btn.y, 1.5*s, 0, math.Pi*2)
		ctx.Call("fill")
	}

	// Merchant epaulettes
	ctx.Set("fillStyle", "#111826")
	ctx.Call("fillRect", bodyCx-leftW-4, bodyTop+2, 10, 4)
	ctx.Call("fillRect", bodyCx+rightW-6, bodyTop+2, 12, 4)
	ctx.Set("fillStyle", "#c8961a")
	ctx.Call("fillRect", bodyCx-leftW-1, bodyTop+2.6, 2, 2.8)
	ctx.Call("fillRect", bodyCx+rightW-3, bodyTop+2.6, 2, 2.8)

	// ── arms ──────────────────────────────────────────────────────────────
	armW := 6.0 * s
	armLen := 24.0 * s
	armY := bodyTop + 5*s
	armBob := math.Sin(t*1.1+0.5) * 0.8

	if c.Mood == MoodExcited || c.Mood == MoodCheering {
		// Distinguished celebration: right arm raised at ~60°, left at side
		// Left arm (far side) — hanging normally
		ctx.Set("fillStyle", "#152040")
		ctx.Call("fillRect", bodyCx-leftW-armW+1, armY+armBob, armW-1, armLen)
		ctx.Set("fillStyle", "#f0f0f0")
		ctx.Call("fillRect", bodyCx-leftW-armW+0.5, armY+armBob+armLen-6.5, armW-0.5, 3.5)
		ctx.Set("fillStyle", "#e8c8a0")
		ctx.Call("beginPath")
		ctx.Call("arc", bodyCx-leftW-armW/2+1, armY+armBob+armLen, 3.5*s, 0, math.Pi*2)
		ctx.Call("fill")
		// Right arm raised — upper sleeve going up-right, then forearm up
		liftAnim := math.Sin(t*2.0) * 3.0
		if levelCelebration && c.Mood == MoodExcited {
			liftAnim = math.Sin(t*5.2) * 5.5
		}
		ctx.Set("fillStyle", "#152040")
		rx := bodyCx + rightW - 1
		// upper arm angled up-right
		ctx.Call("beginPath")
		ctx.Call("moveTo", rx, armY)
		ctx.Call("lineTo", rx+armW, armY)
		ctx.Call("lineTo", rx+armW+10-liftAnim, armY-armLen*0.55)
		ctx.Call("lineTo", rx+4-liftAnim, armY-armLen*0.55)
		ctx.Call("closePath")
		ctx.Call("fill")
		// forearm continuing upward
		ctx.Call("beginPath")
		ctx.Call("moveTo", rx+4-liftAnim, armY-armLen*0.55)
		ctx.Call("lineTo", rx+armW+10-liftAnim, armY-armLen*0.55)
		ctx.Call("lineTo", rx+armW+8-liftAnim, armY-armLen*1.05)
		ctx.Call("lineTo", rx+2-liftAnim, armY-armLen*1.05)
		ctx.Call("closePath")
		ctx.Call("fill")
		ctx.Set("fillStyle", "#f0f0f0")
		ctx.Call("beginPath")
		ctx.Call("moveTo", rx+2-liftAnim, armY-armLen*1.05)
		ctx.Call("lineTo", rx+armW+8-liftAnim, armY-armLen*1.05)
		ctx.Call("lineTo", rx+armW+7-liftAnim, armY-armLen*0.94)
		ctx.Call("lineTo", rx+3-liftAnim, armY-armLen*0.94)
		ctx.Call("closePath")
		ctx.Call("fill")
		// hand
		ctx.Set("fillStyle", "#f5d0b0")
		ctx.Call("beginPath")
		ctx.Call("arc", rx+armW/2+6-liftAnim, armY-armLen*1.05, 4*s, 0, math.Pi*2)
		ctx.Call("fill")
	} else {
		// Normal: both arms hanging at sides
		ctx.Set("fillStyle", "#152040")
		ctx.Call("fillRect", bodyCx-leftW-armW+1, armY+armBob, armW-1, armLen)
		ctx.Set("fillStyle", "#f0f0f0")
		ctx.Call("fillRect", bodyCx-leftW-armW+0.5, armY+armBob+armLen-6.5, armW-0.5, 3.5)
		ctx.Set("fillStyle", "#e8c8a0")
		ctx.Call("beginPath")
		ctx.Call("arc", bodyCx-leftW-armW/2+1, armY+armBob+armLen, 3.5*s, 0, math.Pi*2)
		ctx.Call("fill")
		ctx.Set("fillStyle", "#152040")
		ctx.Call("fillRect", bodyCx+rightW-1, armY-armBob, armW, armLen)
		ctx.Set("fillStyle", "#f0f0f0")
		ctx.Call("fillRect", bodyCx+rightW-0.5, armY-armBob+armLen-6.5, armW, 3.5)
		ctx.Set("fillStyle", "#f5d0b0")
		ctx.Call("beginPath")
		ctx.Call("arc", bodyCx+rightW+armW/2-1, armY-armBob+armLen, 3.5*s, 0, math.Pi*2)
		ctx.Call("fill")
	}

	// ── head (3/4 turned right) ────────────────────────────────────────────
	headR := 20.0 * s
	// Head physically shifts left/right with piece position (whole head turns)
	headCx := cx + poseShift*0.7 + sway*0.5 + c.LookX*1.9
	headY := bodyTop - headR + 4*s
	capBaseY := headY - headR*0.15

	ctx.Call("save")
	ctx.Call("translate", headCx, headY)
	ctx.Call("rotate", headTilt)
	ctx.Call("translate", -headCx, -headY)

	// Hair back
	ctx.Set("fillStyle", "#18101e")
	ctx.Call("beginPath")
	ctx.Call("arc", headCx, headY, headR+3, 0, math.Pi*2)
	ctx.Call("fill")

	// Face (slightly oval — 3/4 look)
	ctx.Call("save")
	ctx.Call("translate", headCx, headY)
	ctx.Call("scale", 0.94, 1.0)
	ctx.Call("translate", -headCx, -headY)
	ctx.Set("fillStyle", "#fce4c8")
	ctx.Call("beginPath")
	ctx.Call("arc", headCx, headY, headR, 0, math.Pi*2)
	ctx.Call("fill")
	ctx.Call("restore")

	// Side hair
	hairFlow := math.Sin(t*1.1+0.5) * 2.0
	ctx.Set("fillStyle", "#18101e")
	sideH := 22.0 * s
	ctx.Call("beginPath")
	ctx.Call("moveTo", headCx-headR+3, capBaseY)
	ctx.Call("quadraticCurveTo", headCx-headR-5-hairFlow, capBaseY+sideH*0.32, headCx-headR+5, capBaseY+sideH*0.92)
	ctx.Call("lineTo", headCx-headR+10, capBaseY+2)
	ctx.Call("closePath")
	ctx.Call("fill")
	ctx.Call("beginPath")
	ctx.Call("moveTo", headCx+headR-1, capBaseY)
	ctx.Call("quadraticCurveTo", headCx+headR+9+hairFlow, capBaseY+sideH*0.34, headCx+headR-2, capBaseY+sideH)
	ctx.Call("lineTo", headCx+headR-7, capBaseY+2)
	ctx.Call("closePath")
	ctx.Call("fill")

	// Bangs
	ctx.Set("fillStyle", "#18101e")
	ctx.Call("beginPath")
	ctx.Call("arc", headCx, headY-headR*0.3, headR+2, math.Pi*1.05, math.Pi*1.95)
	ctx.Call("lineTo", headCx+headR-2, headY-2)
	ctx.Call("quadraticCurveTo", headCx+headR*0.3, headY+4, headCx, headY-2)
	ctx.Call("quadraticCurveTo", headCx-headR*0.3, headY+4, headCx-headR+2, headY-2)
	ctx.Call("closePath")
	ctx.Call("fill")

	// ── eyes + brows + mouth — full mood set ─────────────────────────────
	// 3/4 view: features shifted slightly right
	eyeY := headY + 2*s
	eyeSpread := 10.0 * s
	eyeCx := headCx + 2*s
	eyeR := 6.0 * s
	lx := c.LookX // pupil offset follows piece

	mood := c.Mood

	switch mood {
	case MoodHappy:
		// ^-^ happy closed arcs
		ctx.Set("strokeStyle", "#2a1830")
		ctx.Set("lineWidth", 2.5)
		ctx.Call("beginPath")
		ctx.Call("arc", eyeCx-eyeSpread, eyeY+2, 5*s, math.Pi*1.1, math.Pi*1.9)
		ctx.Call("stroke")
		ctx.Call("beginPath")
		ctx.Call("arc", eyeCx+eyeSpread, eyeY+2, 5*s, math.Pi*1.1, math.Pi*1.9)
		ctx.Call("stroke")
		// raised brows
		ctx.Set("strokeStyle", "#2a1830")
		ctx.Set("lineWidth", 1.8)
		ctx.Call("beginPath")
		ctx.Call("moveTo", eyeCx-eyeSpread-5, eyeY-10)
		ctx.Call("lineTo", eyeCx-eyeSpread+5, eyeY-11)
		ctx.Call("stroke")
		ctx.Call("beginPath")
		ctx.Call("moveTo", eyeCx+eyeSpread-5, eyeY-11)
		ctx.Call("lineTo", eyeCx+eyeSpread+5, eyeY-10)
		ctx.Call("stroke")

	case MoodExcited, MoodCheering:
		// sparkle eyes — arcs + small stars
		ctx.Set("strokeStyle", "#2a1830")
		ctx.Set("lineWidth", 2.5)
		ctx.Call("beginPath")
		ctx.Call("arc", eyeCx-eyeSpread, eyeY+2, 5*s, math.Pi*1.1, math.Pi*1.9)
		ctx.Call("stroke")
		ctx.Call("beginPath")
		ctx.Call("arc", eyeCx+eyeSpread, eyeY+2, 5*s, math.Pi*1.1, math.Pi*1.9)
		ctx.Call("stroke")
		sparkle := math.Abs(math.Sin(t * 4.0))
		ctx.Set("fillStyle", fmt.Sprintf("rgba(255,220,80,%.2f)", sparkle))
		e.drawStar(eyeCx-eyeSpread-9, eyeY-6, 3, 4)
		e.drawStar(eyeCx+eyeSpread+9, eyeY-6, 3, 4)
		// excited brows — raised high
		ctx.Set("strokeStyle", "#2a1830")
		ctx.Set("lineWidth", 2)
		ctx.Call("beginPath")
		ctx.Call("moveTo", eyeCx-eyeSpread-5, eyeY-12)
		ctx.Call("lineTo", eyeCx-eyeSpread+5, eyeY-13)
		ctx.Call("stroke")
		ctx.Call("beginPath")
		ctx.Call("moveTo", eyeCx+eyeSpread-5, eyeY-13)
		ctx.Call("lineTo", eyeCx+eyeSpread+5, eyeY-12)
		ctx.Call("stroke")

	case MoodScared:
		// wide open eyes, tiny pupils frozen in fear
		ctx.Set("fillStyle", "#ffffff")
		ctx.Call("beginPath")
		ctx.Call("arc", eyeCx-eyeSpread, eyeY, eyeR+1, 0, math.Pi*2)
		ctx.Call("fill")
		ctx.Call("beginPath")
		ctx.Call("arc", eyeCx+eyeSpread, eyeY, eyeR+1, 0, math.Pi*2)
		ctx.Call("fill")
		ctx.Set("fillStyle", "#4a2a8a")
		ctx.Call("beginPath")
		ctx.Call("arc", eyeCx-eyeSpread, eyeY, 3*s, 0, math.Pi*2)
		ctx.Call("fill")
		ctx.Call("beginPath")
		ctx.Call("arc", eyeCx+eyeSpread, eyeY, 3*s, 0, math.Pi*2)
		ctx.Call("fill")
		ctx.Set("fillStyle", "#1a1020")
		ctx.Call("beginPath")
		ctx.Call("arc", eyeCx-eyeSpread, eyeY, 1.5*s, 0, math.Pi*2)
		ctx.Call("fill")
		ctx.Call("beginPath")
		ctx.Call("arc", eyeCx+eyeSpread, eyeY, 1.5*s, 0, math.Pi*2)
		ctx.Call("fill")
		ctx.Set("strokeStyle", "#2a1830")
		ctx.Set("lineWidth", 1.5)
		ctx.Call("beginPath")
		ctx.Call("arc", eyeCx-eyeSpread, eyeY, eyeR+1, 0, math.Pi*2)
		ctx.Call("stroke")
		ctx.Call("beginPath")
		ctx.Call("arc", eyeCx+eyeSpread, eyeY, eyeR+1, 0, math.Pi*2)
		ctx.Call("stroke")
		// raised scared brows
		ctx.Set("strokeStyle", "#2a1830")
		ctx.Set("lineWidth", 2)
		ctx.Call("beginPath")
		ctx.Call("moveTo", eyeCx-eyeSpread-5, eyeY-11)
		ctx.Call("lineTo", eyeCx-eyeSpread+5, eyeY-9)
		ctx.Call("stroke")
		ctx.Call("beginPath")
		ctx.Call("moveTo", eyeCx+eyeSpread-5, eyeY-9)
		ctx.Call("lineTo", eyeCx+eyeSpread+5, eyeY-11)
		ctx.Call("stroke")

	case MoodWorried:
		// normal eyes with gaze tracking + worried brows
		for _, side := range []float64{-1, 1} {
			ex := eyeCx + side*eyeSpread
			ctx.Set("fillStyle", "#ffffff")
			ctx.Call("beginPath")
			ctx.Call("arc", ex, eyeY, eyeR, 0, math.Pi*2)
			ctx.Call("fill")
			ctx.Set("fillStyle", "#4a2a8a")
			ctx.Call("beginPath")
			ctx.Call("arc", ex+lx*0.6, eyeY+1, eyeR*0.65, 0, math.Pi*2)
			ctx.Call("fill")
			ctx.Set("fillStyle", "#1a1020")
			ctx.Call("beginPath")
			ctx.Call("arc", ex+lx*0.6, eyeY+1.5, eyeR*0.35, 0, math.Pi*2)
			ctx.Call("fill")
			ctx.Set("fillStyle", "#ffffff")
			ctx.Call("beginPath")
			ctx.Call("arc", ex+lx*0.6+2, eyeY-1.5, 2.5*s, 0, math.Pi*2)
			ctx.Call("fill")
			ctx.Set("strokeStyle", "#2a1830")
			ctx.Set("lineWidth", 1.5)
			ctx.Call("beginPath")
			ctx.Call("arc", ex, eyeY, eyeR, 0, math.Pi*2)
			ctx.Call("stroke")
		}
		// worried brows — inner ends lower
		ctx.Set("strokeStyle", "#2a1830")
		ctx.Set("lineWidth", 2.2)
		ctx.Call("beginPath")
		ctx.Call("moveTo", eyeCx-eyeSpread-6, eyeY-9)
		ctx.Call("lineTo", eyeCx-eyeSpread+4, eyeY-7)
		ctx.Call("stroke")
		ctx.Call("beginPath")
		ctx.Call("moveTo", eyeCx+eyeSpread+6, eyeY-9)
		ctx.Call("lineTo", eyeCx+eyeSpread-4, eyeY-7)
		ctx.Call("stroke")

	case MoodSad: // game over — sour squint
		// half-lidded squinting eyes
		for _, side := range []float64{-1, 1} {
			ex := eyeCx + side*eyeSpread
			ctx.Set("fillStyle", "#ffffff")
			ctx.Call("beginPath")
			ctx.Call("arc", ex, eyeY, eyeR, 0, math.Pi*2)
			ctx.Call("fill")
			ctx.Set("fillStyle", "#4a2a8a")
			ctx.Call("beginPath")
			ctx.Call("arc", ex, eyeY+1, eyeR*0.60, 0, math.Pi*2)
			ctx.Call("fill")
			ctx.Set("fillStyle", "#1a1020")
			ctx.Call("beginPath")
			ctx.Call("arc", ex, eyeY+1.5, eyeR*0.30, 0, math.Pi*2)
			ctx.Call("fill")
			// skin-coloured lid covering top half
			ctx.Set("fillStyle", "#fce4c8")
			ctx.Call("fillRect", ex-eyeR-1, eyeY-eyeR-1, eyeR*2+2, eyeR*0.65)
			ctx.Set("strokeStyle", "#2a1830")
			ctx.Set("lineWidth", 1.5)
			ctx.Call("beginPath")
			ctx.Call("arc", ex, eyeY, eyeR, 0, math.Pi*2)
			ctx.Call("stroke")
		}
		// angry furrowed brows
		ctx.Set("strokeStyle", "#2a1830")
		ctx.Set("lineWidth", 2.4)
		ctx.Call("beginPath")
		ctx.Call("moveTo", eyeCx-eyeSpread-6, eyeY-8)
		ctx.Call("lineTo", eyeCx-eyeSpread+5, eyeY-6)
		ctx.Call("stroke")
		ctx.Call("beginPath")
		ctx.Call("moveTo", eyeCx+eyeSpread+6, eyeY-8)
		ctx.Call("lineTo", eyeCx+eyeSpread-5, eyeY-6)
		ctx.Call("stroke")

	default: // MoodIdle — normal eyes, pupils track piece
		for _, side := range []float64{-1, 1} {
			ex := eyeCx + side*eyeSpread
			ctx.Set("fillStyle", "#ffffff")
			ctx.Call("beginPath")
			ctx.Call("arc", ex, eyeY, eyeR, 0, math.Pi*2)
			ctx.Call("fill")
			ctx.Set("fillStyle", "#4a2a8a")
			ctx.Call("beginPath")
			ctx.Call("arc", ex+lx*0.7, eyeY+1, eyeR*0.65, 0, math.Pi*2)
			ctx.Call("fill")
			ctx.Set("fillStyle", "#1a1020")
			ctx.Call("beginPath")
			ctx.Call("arc", ex+lx*0.7, eyeY+1.5, eyeR*0.35, 0, math.Pi*2)
			ctx.Call("fill")
			ctx.Set("fillStyle", "#ffffff")
			ctx.Call("beginPath")
			ctx.Call("arc", ex+lx*0.7+2, eyeY-1.5, 2.5*s, 0, math.Pi*2)
			ctx.Call("fill")
			ctx.Set("strokeStyle", "#2a1830")
			ctx.Set("lineWidth", 1.5)
			ctx.Call("beginPath")
			ctx.Call("arc", ex, eyeY, eyeR, 0, math.Pi*2)
			ctx.Call("stroke")
		}
		// neutral brows — flat, slightly arched
		ctx.Set("strokeStyle", "#2a1830")
		ctx.Set("lineWidth", 1.8)
		ctx.Call("beginPath")
		ctx.Call("moveTo", eyeCx-eyeSpread-5, eyeY-9)
		ctx.Call("quadraticCurveTo", eyeCx-eyeSpread, eyeY-10, eyeCx-eyeSpread+5, eyeY-9)
		ctx.Call("stroke")
		ctx.Call("beginPath")
		ctx.Call("moveTo", eyeCx+eyeSpread-5, eyeY-9)
		ctx.Call("quadraticCurveTo", eyeCx+eyeSpread, eyeY-10, eyeCx+eyeSpread+5, eyeY-9)
		ctx.Call("stroke")
	}

	// Blush cheeks
	blushAlpha := 0.25
	if mood == MoodSad {
		blushAlpha = 0.10
	} else if mood == MoodHappy || mood == MoodExcited || mood == MoodCheering {
		blushAlpha = 0.45
	}
	ctx.Set("fillStyle", fmt.Sprintf("rgba(255,130,130,%.2f)", blushAlpha))
	ctx.Call("beginPath")
	ctx.Call("arc", eyeCx-eyeSpread-3, eyeY+9, 5*s, 0, math.Pi*2)
	ctx.Call("fill")
	ctx.Call("beginPath")
	ctx.Call("arc", eyeCx+eyeSpread+3, eyeY+9, 5*s, 0, math.Pi*2)
	ctx.Call("fill")

	// Mouth — varies by mood
	mouthY := headY + 13*s
	mouthCx := headCx + 2*s
	ctx.Set("strokeStyle", "#8a3a2a")
	ctx.Set("lineWidth", 1.8)
	switch mood {
	case MoodSad:
		// downturned frown
		ctx.Call("beginPath")
		ctx.Call("arc", mouthCx, mouthY+5, 5*s, math.Pi*1.15, math.Pi*1.85)
		ctx.Call("stroke")
	case MoodHappy:
		// wide smile
		ctx.Call("beginPath")
		ctx.Call("arc", mouthCx, mouthY-4, 6*s, math.Pi*0.12, math.Pi*0.88)
		ctx.Call("stroke")
	case MoodExcited, MoodCheering:
		// open mouth — filled arc
		ctx.Set("fillStyle", "#8a3a2a")
		ctx.Call("beginPath")
		ctx.Call("arc", mouthCx, mouthY-2, 5*s, math.Pi*0.1, math.Pi*0.9)
		ctx.Call("fill")
	case MoodScared:
		// small open circle
		ctx.Set("fillStyle", "#8a3a2a")
		ctx.Call("beginPath")
		ctx.Call("arc", mouthCx, mouthY+1, 3*s, 0, math.Pi*2)
		ctx.Call("fill")
	case MoodWorried:
		// wavy uncertain line
		ctx.Call("beginPath")
		ctx.Call("moveTo", mouthCx-6, mouthY)
		ctx.Call("quadraticCurveTo", mouthCx-3, mouthY+3, mouthCx, mouthY)
		ctx.Call("quadraticCurveTo", mouthCx+3, mouthY-3, mouthCx+6, mouthY)
		ctx.Call("stroke")
	default:
		// relaxed smile
		ctx.Call("beginPath")
		ctx.Call("arc", mouthCx, mouthY-3, 5*s, math.Pi*0.15, math.Pi*0.85)
		ctx.Call("stroke")
	}

	ctx.Call("restore") // head tilt

	// ── captain's peaked cap ───────────────────────────────────────────────
	ctx.Call("save")
	ctx.Call("translate", headCx, headY)
	ctx.Call("rotate", headTilt)
	ctx.Call("translate", -headCx, -headY)

	// Band sits just above the eyes, crown covers all hair above
	bandY := headY - headR*0.7
	hatW := headR * 1.9
	crownH := headR * 0.55 // white part height

	// White crown — slightly narrower than band
	ctx.Set("fillStyle", "#f4f4f6")
	ctx.Call("fillRect", headCx-hatW*0.38, bandY-crownH, hatW*0.76, crownH)
	// Subtle highlight on top
	ctx.Set("fillStyle", "#ffffff")
	ctx.Call("fillRect", headCx-hatW*0.32, bandY-crownH, hatW*0.64, 2.0)

	// Navy band — wider than crown
	ctx.Set("fillStyle", "#152040")
	ctx.Call("fillRect", headCx-hatW*0.44, bandY, hatW*0.88, 10*s)
	ctx.Set("strokeStyle", "rgba(235,240,248,0.6)")
	ctx.Set("lineWidth", 0.8)
	ctx.Call("strokeRect", headCx-hatW*0.44, bandY, hatW*0.88, 10*s)

	// Black visor — wide brim
	ctx.Set("fillStyle", "#10161f")
	ctx.Call("beginPath")
	ctx.Call("moveTo", headCx-hatW*0.52, bandY+6)
	ctx.Call("quadraticCurveTo", headCx, bandY+16, headCx+hatW*0.52, bandY+6)
	ctx.Call("lineTo", headCx+hatW*0.38, bandY+10)
	ctx.Call("quadraticCurveTo", headCx, bandY+18, headCx-hatW*0.38, bandY+10)
	ctx.Call("closePath")
	ctx.Call("fill")
	// Visor shine
	ctx.Set("fillStyle", "rgba(255,255,255,0.15)")
	ctx.Call("beginPath")
	ctx.Call("moveTo", headCx-hatW*0.15, bandY+8)
	ctx.Call("quadraticCurveTo", headCx, bandY+11, headCx+hatW*0.18, bandY+8)
	ctx.Call("fill")

	// Badge
	ctx.Set("fillStyle", "#c8961a")
	ctx.Call("beginPath")
	ctx.Call("arc", headCx, bandY+3.5, 5.0*s, 0, math.Pi*2)
	ctx.Call("fill")
	ctx.Set("fillStyle", "#0a1830")
	ctx.Set("font", fmt.Sprintf("bold 3px %s", uiFontBody))
	ctx.Set("textAlign", "center")
	ctx.Set("textBaseline", "middle")
	ctx.Call("fillText", "AOP", headCx, bandY+3.5)

	ctx.Call("restore") // hat tilt

	ctx.Call("restore") // captainScale

	// ── speech bubble ─────────────────────────────────────────────────────
	if c.BubbleTime > 0 && c.BubbleText != "" {
		alpha := 1.0
		if c.BubbleTime < 0.5 {
			alpha = c.BubbleTime / 0.5
		}
		if c.BubbleDur-c.BubbleTime < 0.3 {
			alpha = (c.BubbleDur - c.BubbleTime) / 0.3
		}
		scaledHatTop := captainMidY - 55.0*captainScale + captainScale*bob
		e.drawSpeechBubble(cx, scaledHatTop-50, c.BubbleText, alpha)
	}
}
