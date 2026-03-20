//go:build js && wasm

package main

import (
	"fmt"
	"math"
	"math/rand"
)

// ── mood types ────────────────────────────────────────────────────────────────

type CharacterMood int

const (
	MoodIdle     CharacterMood = iota
	MoodHappy                  // cleared rows
	MoodExcited                // level complete
	MoodWorried                // yellow zone
	MoodScared                 // red zone
	MoodSad                    // game over initial shock
	MoodComfort                // game over follow-up consoling
	MoodCheering               // victory
)

// ── character state ───────────────────────────────────────────────────────────

type Character struct {
	Mood          CharacterMood
	MoodTimer     float64
	BubbleText    string
	BubbleDur     float64 // total duration for fade calc
	BubbleTime    float64 // remaining
	AnimTime      float64 // master clock for animations
	BlinkCD       float64 // countdown to next blink toggle
	Blinking      bool
	PrevScore     int
	PrevLines     int
	PrevLevel     int
	PrevState     GameState
	LastBubble    float64 // cooldown to avoid bubble spam
	LastMilestone int     // last score milestone reached (multiples of 500)
}

func newCharacter() Character {
	return Character{
		BlinkCD: 2.0 + rand.Float64()*2.0,
	}
}

// ── message pools ─────────────────────────────────────────────────────────────

var (
	msgHappy = []string{
		"Sugoi~! ☆",
		"Nice clear!",
		"Good job~!",
		"Keep going!",
		"Great work!",
		"Yatta~! ♪",
	}
	msgCombo = []string{
		"COMBO!! ★",
		"Amazing~!",
		"So good!!",
		"Incredible!",
		"Multi-clear!",
	}
	msgExcited = []string{
		"Level clear! ☆",
		"Deck sealed~!",
		"Fantastic!!",
		"Omedetou~! ♪",
		"New level! ★",
	}
	msgWorried = []string{
		"Careful~!",
		"Watch trim!",
		"Balance it!",
		"Hmm... ⚠",
		"Be careful!",
	}
	msgScared = []string{
		"Danger!! ☠",
		"We'll sink!",
		"Oh no~!",
		"Fix trim!!",
		"Kowai~!",
	}
	msgSad = []string{
		"Oh no~!",
		"Aah...!",
		"Uwaah~!",
	}
	msgComfort = []string{
		"Don't give up! ♡",
		"Ganbatte ne~!",
		"Try again ♡",
		"Next time for sure~!",
		"You can do it!",
		"Daijoubu~! ♡",
		"I believe in you!",
		"One more try~!",
		"You were so close!",
		"Keep going ♡",
		"I'm cheering for you!",
		"Almost had it~!",
		"You'll get it! ☆",
		"Fighto~! ♡",
	}
	msgCheer = []string{
		"KANPEKI!! ★",
		"Perfect run!",
		"Mission done!",
		"You're the best!",
		"Sugoi sugoi~!",
	}
	msgIdle = []string{
		"Ganbatte~! ♪",
		"You got this!",
		"Hmm hmm~ ♪",
		"Let's go~!",
		"I'm watching~ ♡",
		"Nice focus~!",
		"You're so cool!",
		"Kakkoi~! ☆",
		"Do your best ♡",
		"I like your style~",
		"La la la~ ♪",
		"Show me more~!",
		"So reliable ♡",
		"Keep it up~!",
		"Nee nee~ ♪",
	}
	msgMilestone = []string{
		"Wow, nice score! ☆",
		"You're on fire!",
		"Sugoi ne~! ♡",
		"So impressive!",
		"Captain material~ ☆",
		"Pro stacker~! ★",
	}
)

func pickMsg(pool []string) string {
	return pool[rand.Intn(len(pool))]
}

// ── update ────────────────────────────────────────────────────────────────────

func (c *Character) update(dt float64, e *Engine) {
	c.AnimTime += dt

	// blink cycle
	c.BlinkCD -= dt
	if c.BlinkCD <= 0 {
		if c.Blinking {
			c.Blinking = false
			c.BlinkCD = 2.5 + rand.Float64()*3.0
		} else {
			c.Blinking = true
			c.BlinkCD = 0.1
		}
	}

	// decay bubble
	if c.BubbleTime > 0 {
		c.BubbleTime -= dt
	}
	// cooldown
	if c.LastBubble > 0 {
		c.LastBubble -= dt
	}
	// mood decay
	if c.MoodTimer > 0 {
		c.MoodTimer -= dt
		if c.MoodTimer <= 0 && c.Mood != MoodSad && c.Mood != MoodComfort && c.Mood != MoodCheering {
			c.Mood = MoodIdle
		}
	}

	// detect game events
	switch e.state {
	case StatePlaying:
		// reset mood when entering gameplay from a non-playing screen
		if c.PrevState == StateLevelEnd || c.PrevState == StateVictory || c.PrevState == StateGameOver {
			c.Mood = MoodIdle
			c.MoodTimer = 0
			c.BubbleTime = 0
			c.LastBubble = 0
		}
		c.detectPlayEvents(e)
	case StateGameOver:
		if c.PrevState != StateGameOver {
			// initial shock
			c.setMood(MoodSad, 2.5)
			c.showBubble(pickMsg(msgSad), 2.5)
		} else if c.Mood == MoodSad && c.MoodTimer <= 0 {
			// transition to warm consoling after initial shock
			c.setMood(MoodComfort, 99)
			c.showBubble(pickMsg(msgComfort), 4.0)
		} else if c.Mood == MoodComfort && c.BubbleTime <= 0 && c.LastBubble <= 0 {
			// keep cycling comforting messages
			c.showBubble(pickMsg(msgComfort), 4.0)
		}
	case StateLevelEnd:
		if c.PrevState != StateLevelEnd {
			c.setMood(MoodExcited, 99)
			c.showBubble(pickMsg(msgExcited), 4.0)
		}
	case StateVictory:
		if c.PrevState != StateVictory {
			c.setMood(MoodCheering, 99)
			c.showBubble(pickMsg(msgCheer), 5.0)
		}
	}

	c.PrevScore = e.score
	c.PrevLines = e.lines
	c.PrevLevel = e.level
	c.PrevState = e.state
}

func (c *Character) detectPlayEvents(e *Engine) {
	// row clears
	if e.lines > c.PrevLines {
		cleared := e.lines - c.PrevLines
		if cleared >= 2 {
			c.setMood(MoodHappy, 2.5)
			c.showBubble(pickMsg(msgCombo), 2.5)
		} else {
			c.setMood(MoodHappy, 2.0)
			c.showBubble(pickMsg(msgHappy), 2.0)
		}
		return
	}

	// score milestones — react every 500 points
	milestone := e.score / 500
	if milestone > c.LastMilestone && c.LastMilestone >= 0 {
		c.LastMilestone = milestone
		c.setMood(MoodHappy, 2.5)
		c.showBubble(pickMsg(msgMilestone), 3.0)
		return
	}
	c.LastMilestone = milestone

	// trim zone warnings
	zone := heelZone(e.curHeel, e.level)
	if zone == "red" {
		if c.Mood != MoodScared {
			c.setMood(MoodScared, 1.0)
			c.showBubble(pickMsg(msgScared), 2.0)
		}
		return
	}
	if zone == "yellow" && c.Mood != MoodHappy && c.Mood != MoodScared {
		if c.Mood != MoodWorried {
			c.setMood(MoodWorried, 1.5)
			c.showBubble(pickMsg(msgWorried), 2.0)
		}
		return
	}

	// charming idle chatter — every ~6-8s she says something cute
	if c.Mood == MoodIdle && c.BubbleTime <= 0 && c.LastBubble <= 0 {
		if rand.Float64() < 0.012 {
			c.showBubble(pickMsg(msgIdle), 3.5)
		}
	}
}

func (c *Character) setMood(mood CharacterMood, dur float64) {
	c.Mood = mood
	c.MoodTimer = dur
}

func (c *Character) showBubble(text string, dur float64) {
	if c.LastBubble > 0 && c.BubbleTime > 0.5 {
		return // don't spam
	}
	c.BubbleText = text
	c.BubbleDur = dur
	c.BubbleTime = dur
	c.LastBubble = dur * 0.7
}

// ── render ────────────────────────────────────────────────────────────────────

func (e *Engine) renderCharacter() {
	c := &e.char
	ctx := e.ctx

	const charScale = 2.0

	cx := charPanelW / 2.0
	// Position: moved higher so she's centered in the board area
	drawBaseY := boardY + float64(ROWS)*CELL*0.58
	charMidY := drawBaseY - 65.0

	// ── anime-style motion layers ─────────────────────────────────────────
	t := c.AnimTime

	// idle bobbing — gentle sine breathing
	bob := math.Sin(t*1.6) * 2.0
	// body sway — subtle side-to-side like anime idle
	sway := math.Sin(t*1.1) * 1.5
	// head tilt — slight rotation following sway
	headTilt := math.Sin(t*1.1+0.3) * 0.04
	// hair flow — delayed from head, gentle wave
	hairFlow := math.Sin(t*1.4+0.6) * 2.5
	// breathing — chest rise
	breathe := math.Sin(t*2.0) * 1.0

	if c.Mood == MoodExcited || c.Mood == MoodCheering {
		bob = math.Abs(math.Sin(t*4.5)) * 5.0
		sway = math.Sin(t*3.0) * 3.0
		headTilt = math.Sin(t*3.0+0.3) * 0.08
		hairFlow = math.Sin(t*3.5+0.5) * 5.0
	}
	if c.Mood == MoodScared {
		bob = math.Sin(t*12.0) * 1.5
		sway = math.Sin(t*10.0) * 2.0
		headTilt = math.Sin(t*10.0) * 0.06
		hairFlow = math.Sin(t*8.0) * 3.0
	}
	if c.Mood == MoodHappy {
		bob = math.Sin(t*2.8) * 3.0
		sway = math.Sin(t*1.8) * 2.0
		headTilt = math.Sin(t*1.8+0.4) * 0.06
	}
	if c.Mood == MoodComfort {
		bob = math.Sin(t*1.2) * 1.5
		sway = math.Sin(t*0.9) * 2.0
		headTilt = math.Sin(t*0.9+0.3) * 0.05
	}

	y := drawBaseY + bob

	// scale 2x around the character center
	ctx.Call("save")
	ctx.Call("translate", cx, charMidY)
	ctx.Call("scale", charScale, charScale)
	ctx.Call("translate", -cx, -charMidY)

	s := 1.0

	// ── legs (with subtle weight shift) ───────────────────────────────────
	legY := y
	legW := 8.0 * s
	legH := 22.0 * s
	bootH := 7.0 * s
	// weight shift — one leg slightly shorter
	leftLegShift := math.Sin(t*1.1) * 1.5
	rightLegShift := -leftLegShift

	// left leg
	ctx.Set("fillStyle", "#1a3a5a")
	ctx.Call("fillRect", cx-14*s+sway*0.3, legY-legH+leftLegShift, legW, legH-leftLegShift)
	ctx.Set("fillStyle", "#3a2a1a")
	ctx.Call("fillRect", cx-15*s+sway*0.3, legY-bootH, legW+2, bootH)

	// right leg
	ctx.Set("fillStyle", "#1a3a5a")
	ctx.Call("fillRect", cx+6*s+sway*0.3, legY-legH+rightLegShift, legW, legH-rightLegShift)
	ctx.Set("fillStyle", "#3a2a1a")
	ctx.Call("fillRect", cx+5*s+sway*0.3, legY-bootH, legW+2, bootH)

	// ── body (with sway + breathing) ─────────────────────────────────────
	bodyTop := legY - legH - 32*s
	bodyH := 34.0 * s
	bodyW := 36.0 * s
	bodyCx := cx + sway*0.5

	// coverall
	ctx.Set("fillStyle", "#1a3a6a")
	ctx.Call("beginPath")
	ctx.Call("moveTo", bodyCx-bodyW/2, bodyTop+bodyH)
	ctx.Call("lineTo", bodyCx-bodyW/2-2, bodyTop+4-breathe)
	ctx.Call("lineTo", bodyCx+bodyW/2+2, bodyTop+4-breathe)
	ctx.Call("lineTo", bodyCx+bodyW/2, bodyTop+bodyH)
	ctx.Call("closePath")
	ctx.Call("fill")

	// orange safety vest
	vestW := bodyW * 0.8
	ctx.Set("fillStyle", "#d96a10")
	ctx.Call("fillRect", bodyCx-vestW/2, bodyTop+4-breathe, vestW, bodyH*0.7+breathe)
	// vest stripe
	ctx.Set("fillStyle", "#e8e8e8")
	ctx.Call("fillRect", bodyCx-vestW/2, bodyTop+bodyH*0.45, vestW, 3*s)

	// ── arms (hang naturally at sides, gentle sway) ─────────────────────
	armW := 8.0 * s
	armLen := 26.0 * s
	armY := bodyTop + 6*s

	// arm offsets: tiny sway that follows body, like cloth
	armSway := sway * 0.3
	armBob := math.Sin(t*1.6+0.5) * 0.8

	if c.Mood == MoodCheering || c.Mood == MoodExcited {
		// arms slightly raised outward, gentle bounce (NOT rotating)
		liftL := math.Sin(t*3.5) * 3.0
		liftR := math.Sin(t*3.5+0.8) * 3.0
		// left arm — slightly away from body, bouncing
		ctx.Set("fillStyle", "#d96a10")
		ctx.Call("fillRect", bodyCx-bodyW/2-armW-1+armSway, armY-4-liftL, armW, armLen*0.75)
		ctx.Set("fillStyle", "#f5d0b0")
		ctx.Call("beginPath")
		ctx.Call("arc", bodyCx-bodyW/2-armW/2-1+armSway, armY-4-liftL+armLen*0.75, 4*s, 0, math.Pi*2)
		ctx.Call("fill")
		// right arm
		ctx.Set("fillStyle", "#d96a10")
		ctx.Call("fillRect", bodyCx+bodyW/2+1+armSway, armY-4-liftR, armW, armLen*0.75)
		ctx.Set("fillStyle", "#f5d0b0")
		ctx.Call("beginPath")
		ctx.Call("arc", bodyCx+bodyW/2+armW/2+1+armSway, armY-4-liftR+armLen*0.75, 4*s, 0, math.Pi*2)
		ctx.Call("fill")
	} else if c.Mood == MoodWorried || c.Mood == MoodScared {
		// arms held close, slight tremble
		tremble := math.Sin(t*8.0) * 0.8
		ctx.Set("fillStyle", "#d96a10")
		ctx.Call("fillRect", bodyCx-bodyW/2-armW/2+tremble, armY+2, armW, armLen*0.65)
		ctx.Call("fillRect", bodyCx+bodyW/2-armW/2-tremble, armY+2, armW, armLen*0.65)
		ctx.Set("fillStyle", "#f5d0b0")
		ctx.Call("beginPath")
		ctx.Call("arc", bodyCx-bodyW/2+tremble, armY+2+armLen*0.65, 4*s, 0, math.Pi*2)
		ctx.Call("fill")
		ctx.Call("beginPath")
		ctx.Call("arc", bodyCx+bodyW/2-tremble, armY+2+armLen*0.65, 4*s, 0, math.Pi*2)
		ctx.Call("fill")
	} else {
		// natural idle — arms hanging at sides, tiny sway like breathing
		// left arm
		ctx.Set("fillStyle", "#d96a10")
		ctx.Call("fillRect", bodyCx-bodyW/2-armW+1+armSway, armY+2+armBob, armW, armLen*0.8)
		ctx.Set("fillStyle", "#f5d0b0")
		ctx.Call("beginPath")
		ctx.Call("arc", bodyCx-bodyW/2-armW/2+1+armSway, armY+2+armBob+armLen*0.8, 4*s, 0, math.Pi*2)
		ctx.Call("fill")
		// right arm — same natural hang
		ctx.Set("fillStyle", "#d96a10")
		ctx.Call("fillRect", bodyCx+bodyW/2-1+armSway, armY+2-armBob, armW, armLen*0.8)
		ctx.Set("fillStyle", "#f5d0b0")
		ctx.Call("beginPath")
		ctx.Call("arc", bodyCx+bodyW/2+armW/2-1+armSway, armY+2-armBob+armLen*0.8, 4*s, 0, math.Pi*2)
		ctx.Call("fill")
	}

	// ── head (tilts gently with sway) ────────────────────────────────────
	headR := 24.0 * s
	headCx := cx + sway*0.7
	headY := bodyTop - headR + 6*s - breathe*0.5

	ctx.Call("save")
	ctx.Call("translate", headCx, headY)
	ctx.Call("rotate", headTilt)
	ctx.Call("translate", -headCx, -headY)

	// hair back
	ctx.Set("fillStyle", "#2a1830")
	ctx.Call("beginPath")
	ctx.Call("arc", headCx, headY, headR+4, 0, math.Pi*2)
	ctx.Call("fill")

	// face
	ctx.Set("fillStyle", "#fce4c8")
	ctx.Call("beginPath")
	ctx.Call("arc", headCx, headY, headR, 0, math.Pi*2)
	ctx.Call("fill")

	// side hair (flowing with hairFlow)
	ctx.Set("fillStyle", "#2a1830")
	sideHairW := 10.0 * s
	sideHairH := 38.0 * s
	// left side hair
	ctx.Call("beginPath")
	ctx.Call("moveTo", headCx-headR-2, headY-8)
	ctx.Call("quadraticCurveTo", headCx-headR-sideHairW-hairFlow, headY+sideHairH*0.4, headCx-headR+2-hairFlow*0.5, headY+sideHairH)
	ctx.Call("lineTo", headCx-headR+6, headY-4)
	ctx.Call("closePath")
	ctx.Call("fill")
	// right side hair
	ctx.Call("beginPath")
	ctx.Call("moveTo", headCx+headR+2, headY-8)
	ctx.Call("quadraticCurveTo", headCx+headR+sideHairW+hairFlow, headY+sideHairH*0.4, headCx+headR-2+hairFlow*0.5, headY+sideHairH)
	ctx.Call("lineTo", headCx+headR-6, headY-4)
	ctx.Call("closePath")
	ctx.Call("fill")

	// bangs
	ctx.Set("fillStyle", "#2a1830")
	ctx.Call("beginPath")
	ctx.Call("arc", headCx, headY-headR*0.3, headR+3, math.Pi*1.05, math.Pi*1.95)
	ctx.Call("lineTo", headCx+headR-2, headY-2)
	ctx.Call("quadraticCurveTo", headCx+headR*0.3, headY+4, headCx, headY-2)
	ctx.Call("quadraticCurveTo", headCx-headR*0.3, headY+4, headCx-headR+2, headY-2)
	ctx.Call("closePath")
	ctx.Call("fill")

	// ── eyes (mood-dependent) ─────────────────────────────────────────────
	eyeY := headY + 2*s
	eyeSpread := 10.0 * s
	eyeCx := headCx

	if c.Blinking && c.Mood != MoodHappy && c.Mood != MoodSad && c.Mood != MoodComfort {
		// blinking - horizontal lines
		ctx.Set("strokeStyle", "#2a1830")
		ctx.Set("lineWidth", 2)
		ctx.Call("beginPath")
		ctx.Call("moveTo", eyeCx-eyeSpread-5, eyeY)
		ctx.Call("lineTo", eyeCx-eyeSpread+5, eyeY)
		ctx.Call("moveTo", eyeCx+eyeSpread-5, eyeY)
		ctx.Call("lineTo", eyeCx+eyeSpread+5, eyeY)
		ctx.Call("stroke")
	} else {
		switch c.Mood {
		case MoodHappy, MoodExcited, MoodCheering:
			ctx.Set("strokeStyle", "#2a1830")
			ctx.Set("lineWidth", 2.5)
			ctx.Call("beginPath")
			ctx.Call("arc", eyeCx-eyeSpread, eyeY+2, 5*s, math.Pi*1.1, math.Pi*1.9)
			ctx.Call("stroke")
			ctx.Call("beginPath")
			ctx.Call("arc", eyeCx+eyeSpread, eyeY+2, 5*s, math.Pi*1.1, math.Pi*1.9)
			ctx.Call("stroke")
			if c.Mood == MoodCheering || c.Mood == MoodExcited {
				sparkle := math.Abs(math.Sin(c.AnimTime * 4.0))
				ctx.Set("fillStyle", fmt.Sprintf("rgba(255,220,80,%.2f)", sparkle))
				e.drawStar(eyeCx-eyeSpread-8, eyeY-6, 3, 4)
				e.drawStar(eyeCx+eyeSpread+8, eyeY-6, 3, 4)
			}
		case MoodSad:
			ctx.Set("strokeStyle", "#2a1830")
			ctx.Set("lineWidth", 2.5)
			ctx.Call("beginPath")
			ctx.Call("arc", eyeCx-eyeSpread, eyeY-1, 5*s, math.Pi*0.1, math.Pi*0.9)
			ctx.Call("stroke")
			ctx.Call("beginPath")
			ctx.Call("arc", eyeCx+eyeSpread, eyeY-1, 5*s, math.Pi*0.1, math.Pi*0.9)
			ctx.Call("stroke")
			tearPhase := math.Mod(c.AnimTime*1.5, 1.0)
			tearY := eyeY + 6 + tearPhase*12
			tearAlpha := 1.0 - tearPhase
			ctx.Set("fillStyle", fmt.Sprintf("rgba(100,180,255,%.2f)", tearAlpha))
			ctx.Call("beginPath")
			ctx.Call("arc", eyeCx-eyeSpread-3, tearY, 2, 0, math.Pi*2)
			ctx.Call("fill")
			ctx.Call("beginPath")
			ctx.Call("arc", eyeCx+eyeSpread+3, tearY, 2, 0, math.Pi*2)
			ctx.Call("fill")
		case MoodComfort:
			e.drawNormalEyes(eyeCx, eyeY, eyeSpread, s)
			ctx.Set("strokeStyle", "#2a1830")
			ctx.Set("lineWidth", 1.8)
			ctx.Call("beginPath")
			ctx.Call("moveTo", eyeCx-eyeSpread-5, eyeY-9)
			ctx.Call("lineTo", eyeCx-eyeSpread+5, eyeY-10)
			ctx.Call("moveTo", eyeCx+eyeSpread+5, eyeY-9)
			ctx.Call("lineTo", eyeCx+eyeSpread-5, eyeY-10)
			ctx.Call("stroke")
			heartBob := math.Sin(c.AnimTime*2.0) * 3
			heartAlpha := 0.5 + 0.3*math.Sin(c.AnimTime*1.5)
			e.drawHeart(eyeCx+headR+4, eyeY-10+heartBob, 5*s, fmt.Sprintf("rgba(255,120,140,%.2f)", heartAlpha))
		case MoodScared:
			ctx.Set("fillStyle", "#ffffff")
			ctx.Call("beginPath")
			ctx.Call("arc", eyeCx-eyeSpread, eyeY, 7*s, 0, math.Pi*2)
			ctx.Call("fill")
			ctx.Call("beginPath")
			ctx.Call("arc", eyeCx+eyeSpread, eyeY, 7*s, 0, math.Pi*2)
			ctx.Call("fill")
			ctx.Set("fillStyle", "#1a1020")
			ctx.Call("beginPath")
			ctx.Call("arc", eyeCx-eyeSpread, eyeY, 2.5*s, 0, math.Pi*2)
			ctx.Call("fill")
			ctx.Call("beginPath")
			ctx.Call("arc", eyeCx+eyeSpread, eyeY, 2.5*s, 0, math.Pi*2)
			ctx.Call("fill")
			ctx.Set("strokeStyle", "#2a1830")
			ctx.Set("lineWidth", 1.5)
			ctx.Call("beginPath")
			ctx.Call("arc", eyeCx-eyeSpread, eyeY, 7*s, 0, math.Pi*2)
			ctx.Call("stroke")
			ctx.Call("beginPath")
			ctx.Call("arc", eyeCx+eyeSpread, eyeY, 7*s, 0, math.Pi*2)
			ctx.Call("stroke")
			sweatY := headY - headR*0.3 + math.Sin(c.AnimTime*3)*3
			ctx.Set("fillStyle", "rgba(140,200,255,0.7)")
			ctx.Call("beginPath")
			ctx.Call("moveTo", eyeCx+headR-2, sweatY)
			ctx.Call("quadraticCurveTo", eyeCx+headR+6, sweatY+8, eyeCx+headR, sweatY+12)
			ctx.Call("quadraticCurveTo", eyeCx+headR-4, sweatY+8, eyeCx+headR-2, sweatY)
			ctx.Call("fill")
		case MoodWorried:
			e.drawNormalEyes(eyeCx, eyeY, eyeSpread, s)
			ctx.Set("strokeStyle", "#2a1830")
			ctx.Set("lineWidth", 2)
			ctx.Call("beginPath")
			ctx.Call("moveTo", eyeCx-eyeSpread-6, eyeY-9)
			ctx.Call("lineTo", eyeCx-eyeSpread+4, eyeY-7)
			ctx.Call("moveTo", eyeCx+eyeSpread+6, eyeY-9)
			ctx.Call("lineTo", eyeCx+eyeSpread-4, eyeY-7)
			ctx.Call("stroke")
		default:
			e.drawNormalEyes(eyeCx, eyeY, eyeSpread, s)
		}
	}

	// ── blush ─────────────────────────────────────────────────────────────
	blushAlpha := 0.25
	if c.Mood == MoodHappy || c.Mood == MoodExcited || c.Mood == MoodCheering || c.Mood == MoodComfort {
		blushAlpha = 0.45
	}
	ctx.Set("fillStyle", fmt.Sprintf("rgba(255,130,130,%.2f)", blushAlpha))
	ctx.Call("beginPath")
	ctx.Call("arc", eyeCx-eyeSpread-3, eyeY+9, 5*s, 0, math.Pi*2)
	ctx.Call("fill")
	ctx.Call("beginPath")
	ctx.Call("arc", eyeCx+eyeSpread+3, eyeY+9, 5*s, 0, math.Pi*2)
	ctx.Call("fill")

	// ── mouth (mood-dependent) ────────────────────────────────────────────
	mouthY := headY + 12*s
	mouthCx := headCx

	switch c.Mood {
	case MoodHappy, MoodExcited, MoodCheering:
		ctx.Set("strokeStyle", "#8a3a2a")
		ctx.Set("lineWidth", 2)
		ctx.Call("beginPath")
		ctx.Call("arc", mouthCx, mouthY-2, 7*s, math.Pi*0.15, math.Pi*0.85)
		ctx.Call("stroke")
		if c.Mood == MoodExcited || c.Mood == MoodCheering {
			ctx.Set("fillStyle", "#8a3a2a")
			ctx.Call("beginPath")
			ctx.Call("arc", mouthCx, mouthY-1, 5*s, math.Pi*0.1, math.Pi*0.9)
			ctx.Call("fill")
		}
	case MoodSad:
		ctx.Set("strokeStyle", "#8a3a2a")
		ctx.Set("lineWidth", 2)
		ctx.Call("beginPath")
		ctx.Call("arc", mouthCx, mouthY+6, 6*s, math.Pi*1.2, math.Pi*1.8)
		ctx.Call("stroke")
	case MoodComfort:
		ctx.Set("strokeStyle", "#8a3a2a")
		ctx.Set("lineWidth", 2)
		ctx.Call("beginPath")
		ctx.Call("arc", mouthCx, mouthY, 5*s, math.Pi*0.2, math.Pi*0.8)
		ctx.Call("stroke")
	case MoodScared:
		ctx.Set("fillStyle", "#8a3a2a")
		ctx.Call("beginPath")
		ctx.Call("arc", mouthCx, mouthY+1, 4*s, 0, math.Pi*2)
		ctx.Call("fill")
	case MoodWorried:
		ctx.Set("strokeStyle", "#8a3a2a")
		ctx.Set("lineWidth", 1.8)
		ctx.Call("beginPath")
		ctx.Call("moveTo", mouthCx-6, mouthY)
		ctx.Call("quadraticCurveTo", mouthCx-3, mouthY+3, mouthCx, mouthY)
		ctx.Call("quadraticCurveTo", mouthCx+3, mouthY-3, mouthCx+6, mouthY)
		ctx.Call("stroke")
	default:
		ctx.Set("strokeStyle", "#8a3a2a")
		ctx.Set("lineWidth", 1.8)
		ctx.Call("beginPath")
		ctx.Call("arc", mouthCx, mouthY, 4*s, math.Pi*0.2, math.Pi*0.8)
		ctx.Call("stroke")
	}

	// restore head tilt transform
	ctx.Call("restore")

	// ── hard hat (follows head position) ──────────────────────────────────
	ctx.Call("save")
	ctx.Call("translate", headCx, headY)
	ctx.Call("rotate", headTilt)
	ctx.Call("translate", -headCx, -headY)

	// ── hard hat ──────────────────────────────────────────────────────────
	hatY := headY - headR*0.55
	hatR := headR * 1.15

	ctx.Set("fillStyle", "#f0c020")
	ctx.Call("beginPath")
	ctx.Call("arc", headCx, hatY+4, hatR, math.Pi, 0)
	ctx.Call("closePath")
	ctx.Call("fill")

	ctx.Set("fillStyle", "rgba(255,240,180,0.4)")
	ctx.Call("beginPath")
	ctx.Call("arc", headCx-4, hatY, hatR*0.7, math.Pi*1.1, math.Pi*1.8)
	ctx.Call("closePath")
	ctx.Call("fill")

	ctx.Set("fillStyle", "#d8a810")
	ctx.Call("fillRect", headCx-hatR-4, hatY+2, hatR*2+8, 5*s)

	ctx.Set("fillStyle", "#ffffff")
	ctx.Call("fillRect", headCx-hatR*0.6, hatY-4, hatR*1.2, 3*s)

	ctx.Call("restore") // restore hat/head tilt

	// ── name badge ────────────────────────────────────────────────────────
	ctx.Set("fillStyle", "rgba(255,255,255,0.75)")
	ctx.Call("fillRect", bodyCx-12, bodyTop+bodyH*0.15, 24, 10)
	ctx.Set("fillStyle", "#1a3a5a")
	ctx.Set("font", fmt.Sprintf("600 7px %s", uiFontBody))
	ctx.Set("textAlign", "center")
	ctx.Set("textBaseline", "middle")
	ctx.Call("fillText", "MIKU", bodyCx, bodyTop+bodyH*0.15+5.5)

	ctx.Call("restore")

	// ── speech bubble (drawn at screen coordinates, outside 2x transform) ──
	if c.BubbleTime > 0 && c.BubbleText != "" {
		alpha := 1.0
		if c.BubbleTime < 0.5 {
			alpha = c.BubbleTime / 0.5
		}
		if c.BubbleDur-c.BubbleTime < 0.3 {
			alpha = (c.BubbleDur - c.BubbleTime) / 0.3
		}
		// hat top in screen space: charMidY - 130 + 2*bob
		scaledHatTop := charMidY - 65*charScale + charScale*bob
		e.drawSpeechBubble(cx, scaledHatTop-64, c.BubbleText, alpha)
	}
}

// drawNormalEyes draws standard anime eyes with iris and highlight.
func (e *Engine) drawNormalEyes(cx, eyeY, spread, s float64) {
	ctx := e.ctx
	eyeR := 6.0 * s

	for _, side := range []float64{-1, 1} {
		ex := cx + side*spread
		// white
		ctx.Set("fillStyle", "#ffffff")
		ctx.Call("beginPath")
		ctx.Call("arc", ex, eyeY, eyeR, 0, math.Pi*2)
		ctx.Call("fill")
		// iris
		ctx.Set("fillStyle", "#4a2a8a")
		ctx.Call("beginPath")
		ctx.Call("arc", ex, eyeY+1, eyeR*0.65, 0, math.Pi*2)
		ctx.Call("fill")
		// pupil
		ctx.Set("fillStyle", "#1a1020")
		ctx.Call("beginPath")
		ctx.Call("arc", ex, eyeY+1.5, eyeR*0.35, 0, math.Pi*2)
		ctx.Call("fill")
		// highlight
		ctx.Set("fillStyle", "#ffffff")
		ctx.Call("beginPath")
		ctx.Call("arc", ex+2, eyeY-1.5, 2.5*s, 0, math.Pi*2)
		ctx.Call("fill")
		// outline
		ctx.Set("strokeStyle", "#2a1830")
		ctx.Set("lineWidth", 1.5)
		ctx.Call("beginPath")
		ctx.Call("arc", ex, eyeY, eyeR, 0, math.Pi*2)
		ctx.Call("stroke")
	}
}

// drawStar draws a small 4-pointed star.
func (e *Engine) drawStar(x, y, inner, outer float64) {
	ctx := e.ctx
	ctx.Call("beginPath")
	for i := 0; i < 8; i++ {
		r := outer
		if i%2 != 0 {
			r = inner
		}
		angle := float64(i)*math.Pi/4 - math.Pi/2
		px := x + math.Cos(angle)*r
		py := y + math.Sin(angle)*r
		if i == 0 {
			ctx.Call("moveTo", px, py)
		} else {
			ctx.Call("lineTo", px, py)
		}
	}
	ctx.Call("closePath")
	ctx.Call("fill")
}

// drawHeart draws a small heart shape.
func (e *Engine) drawHeart(x, y, size float64, color string) {
	ctx := e.ctx
	ctx.Call("save")
	ctx.Set("fillStyle", color)
	ctx.Call("beginPath")
	ctx.Call("moveTo", x, y+size*0.3)
	ctx.Call("bezierCurveTo", x, y, x-size, y, x-size, y+size*0.3)
	ctx.Call("bezierCurveTo", x-size, y+size*0.7, x, y+size, x, y+size*1.2)
	ctx.Call("bezierCurveTo", x, y+size, x+size, y+size*0.7, x+size, y+size*0.3)
	ctx.Call("bezierCurveTo", x+size, y, x, y, x, y+size*0.3)
	ctx.Call("fill")
	ctx.Call("restore")
}

// drawSpeechBubble draws a comic-style speech bubble above the character.
func (e *Engine) drawSpeechBubble(cx, topY float64, text string, alpha float64) {
	ctx := e.ctx
	ctx.Call("save")
	ctx.Set("globalAlpha", alpha)

	bubW := math.Min(charPanelW-20.0, 220.0)
	bubH := 50.0
	bx := cx - bubW/2
	by := topY
	radius := 10.0

	// bubble body - rounded rect
	ctx.Set("fillStyle", "rgba(255,255,255,0.92)")
	ctx.Call("beginPath")
	ctx.Call("moveTo", bx+radius, by)
	ctx.Call("lineTo", bx+bubW-radius, by)
	ctx.Call("quadraticCurveTo", bx+bubW, by, bx+bubW, by+radius)
	ctx.Call("lineTo", bx+bubW, by+bubH-radius)
	ctx.Call("quadraticCurveTo", bx+bubW, by+bubH, bx+bubW-radius, by+bubH)
	ctx.Call("lineTo", bx+radius, by+bubH)
	ctx.Call("quadraticCurveTo", bx, by+bubH, bx, by+bubH-radius)
	ctx.Call("lineTo", bx, by+radius)
	ctx.Call("quadraticCurveTo", bx, by, bx+radius, by)
	ctx.Call("closePath")
	ctx.Call("fill")

	// tail triangle pointing down
	ctx.Set("fillStyle", "rgba(255,255,255,0.92)")
	ctx.Call("beginPath")
	ctx.Call("moveTo", cx-8, by+bubH)
	ctx.Call("lineTo", cx, by+bubH+14)
	ctx.Call("lineTo", cx+8, by+bubH)
	ctx.Call("closePath")
	ctx.Call("fill")

	// border
	ctx.Set("strokeStyle", "rgba(60,40,80,0.4)")
	ctx.Set("lineWidth", 1.5)
	ctx.Call("beginPath")
	ctx.Call("moveTo", bx+radius, by)
	ctx.Call("lineTo", bx+bubW-radius, by)
	ctx.Call("quadraticCurveTo", bx+bubW, by, bx+bubW, by+radius)
	ctx.Call("lineTo", bx+bubW, by+bubH-radius)
	ctx.Call("quadraticCurveTo", bx+bubW, by+bubH, bx+bubW-radius, by+bubH)
	ctx.Call("lineTo", cx+8, by+bubH)
	ctx.Call("lineTo", cx, by+bubH+14)
	ctx.Call("lineTo", cx-8, by+bubH)
	ctx.Call("lineTo", bx+radius, by+bubH)
	ctx.Call("quadraticCurveTo", bx, by+bubH, bx, by+bubH-radius)
	ctx.Call("lineTo", bx, by+radius)
	ctx.Call("quadraticCurveTo", bx, by, bx+radius, by)
	ctx.Call("closePath")
	ctx.Call("stroke")

	// text
	ctx.Set("fillStyle", "#2a1830")
	ctx.Set("font", fmt.Sprintf("600 14px %s", uiFontBody))
	ctx.Set("textAlign", "center")
	ctx.Set("textBaseline", "middle")
	ctx.Call("fillText", text, cx, by+bubH/2)

	ctx.Call("restore")
}
