//go:build js && wasm

package main

import "syscall/js"

func (e *Engine) touchCancelLongPress() {
	js.Global().Call("clearTimeout", e.touchLongPressTimer)
	e.touchLongPressTimer = js.Undefined()
}

func (e *Engine) touchHandleTap() {
	switch e.state {
	case StatePlaying:
		e.rotPiece()
	case StateMainMenu:
		js.Global().Call("playMenuKnockSound")
		e.newGame()
	case StateGameOver, StateVictory:
		e.enterMainMenu()
	case StateLevelEnd:
		e.nextLevel()
	case StatePaused:
		e.state = StatePlaying
	}
}

func (e *Engine) touchHandleLongPress() {
	switch e.state {
	case StatePlaying:
		e.hardDrop()
	case StateMainMenu:
		js.Global().Call("playMenuKnockSound")
		e.newGame()
	case StateGameOver, StateVictory:
		e.enterMainMenu()
	case StateLevelEnd:
		e.nextLevel()
	case StatePaused:
		e.state = StatePlaying
	}
}

func (e *Engine) registerTouchInput() {
	e.touchLongPressTimer = js.Undefined()
	opts := js.Global().Get("Object").New()
	opts.Set("passive", false)

	onTouchStart := js.FuncOf(func(_ js.Value, args []js.Value) any {
		ev := args[0]
		ev.Call("preventDefault")
		touches := ev.Get("changedTouches")
		if touches.Length() == 0 {
			return nil
		}
		t := touches.Index(0)
		e.touchStartX = t.Get("clientX").Float()
		e.touchStartY = t.Get("clientY").Float()
		e.touchStartMs = js.Global().Get("performance").Call("now").Float()
		e.touchCancelLongPress()
		e.touchSwipeCols = 0
		e.touchSwiped = false
		e.touchLongPressTriggered = false

		e.touchLongPressTimer = js.Global().Call("setTimeout", js.FuncOf(func(_ js.Value, _ []js.Value) any {
			if e.touchLongPressTriggered {
				return nil
			}
			e.touchLongPressTriggered = true
			e.touchLongPressTimer = js.Undefined()
			e.touchHandleLongPress()
			return nil
		}), 400)
		return nil
	})

	onTouchMove := js.FuncOf(func(_ js.Value, args []js.Value) any {
		ev := args[0]
		ev.Call("preventDefault")
		touches := ev.Get("changedTouches")
		if touches.Length() == 0 {
			return nil
		}
		t := touches.Index(0)
		x := t.Get("clientX").Float()
		y := t.Get("clientY").Float()

		adx := x - e.touchStartX
		if adx < 0 {
			adx = -adx
		}
		ady := y - e.touchStartY
		if ady < 0 {
			ady = -ady
		}
		// Cancel long press on any significant movement
		if adx > 10 || ady > 10 {
			e.touchCancelLongPress()
		}

		if e.state != StatePlaying || e.touchLongPressTriggered {
			return nil
		}

		rect := e.canvas.Call("getBoundingClientRect")
		renderedW := rect.Get("width").Float()
		if renderedW <= 0 {
			return nil
		}
		scaleX := canvasW / renderedW
		dxLogical := (x - e.touchStartX) * scaleX
		targetCols := int(dxLogical / CELL)
		diff := targetCols - e.touchSwipeCols
		if diff > 0 {
			e.touchSwiped = true
			for i := 0; i < diff; i++ {
				e.moveH(1)
			}
		} else if diff < 0 {
			e.touchSwiped = true
			for i := 0; i < -diff; i++ {
				e.moveH(-1)
			}
		}
		e.touchSwipeCols = targetCols
		return nil
	})

	onTouchEnd := js.FuncOf(func(_ js.Value, args []js.Value) any {
		ev := args[0]
		ev.Call("preventDefault")
		e.touchCancelLongPress()

		if e.touchLongPressTriggered || e.touchSwiped {
			return nil
		}

		touches := ev.Get("changedTouches")
		if touches.Length() == 0 {
			return nil
		}
		t := touches.Index(0)
		x := t.Get("clientX").Float()
		y := t.Get("clientY").Float()
		now := js.Global().Get("performance").Call("now").Float()

		dx := x - e.touchStartX
		if dx < 0 {
			dx = -dx
		}
		dy := y - e.touchStartY
		if dy < 0 {
			dy = -dy
		}
		if dx < 20 && dy < 20 && now-e.touchStartMs < 300 {
			e.touchHandleTap()
		}
		return nil
	})

	e.canvas.Call("addEventListener", "touchstart", onTouchStart, opts)
	e.canvas.Call("addEventListener", "touchmove", onTouchMove, opts)
	e.canvas.Call("addEventListener", "touchend", onTouchEnd, opts)
}

func (e *Engine) registerInput() {
	js.Global().Get("document").Call("addEventListener", "keydown",
		js.FuncOf(func(_ js.Value, args []js.Value) any {
			code := args[0].Get("code").String()
			e.keys[code] = true
			switch code {
			case "ArrowLeft":
				if e.state == StatePlaying {
					e.moveH(-1)
				}
			case "ArrowRight":
				if e.state == StatePlaying {
					e.moveH(1)
				}
			case "ArrowUp":
				if e.state == StatePlaying {
					e.rotPiece()
				}
			case "ArrowDown":
				if e.state == StatePlaying {
					e.drop()
					e.dropTimer = 0
				}
			case "Space":
				switch e.state {
				case StateMainMenu:
					js.Global().Call("playMenuKnockSound")
					e.newGame()
				case StatePlaying:
					e.hardDrop()
				case StateGameOver:
					e.enterMainMenu()
				case StateLevelEnd:
					e.nextLevel()
				case StateVictory:
					e.enterMainMenu()
				}
			case "Enter":
				switch e.state {
				case StateMainMenu:
					js.Global().Call("playMenuKnockSound")
					e.newGame()
				case StateLevelEnd:
					e.nextLevel()
				case StateGameOver, StateVictory:
					e.enterMainMenu()
				}
			case "KeyQ":
				if e.state == StatePaused {
					e.enterMainMenu()
				}
			case "KeyP", "Escape":
				switch e.state {
				case StatePlaying:
					e.state = StatePaused
				case StatePaused:
					e.state = StatePlaying
				case StateLevelEnd:
					e.enterMainMenu()
				case StateGameOver, StateVictory:
					e.enterMainMenu()
				}
			}
			// Prevent page scroll
			switch code {
			case "ArrowLeft", "ArrowRight", "ArrowUp", "ArrowDown", "Space", "Enter", "KeyP", "KeyQ", "KeyM", "KeyT", "Escape":
				args[0].Call("preventDefault")
			}
			return nil
		}),
	)

	js.Global().Get("document").Call("addEventListener", "keyup",
		js.FuncOf(func(_ js.Value, args []js.Value) any {
			e.keys[args[0].Get("code").String()] = false
			return nil
		}),
	)
}
