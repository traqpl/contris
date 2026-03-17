//go:build js && wasm

package main

import "syscall/js"

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
					e.newGame()
				case StatePlaying:
					e.hardDrop()
				case StateGameOver:
					e.newGame()
				case StateLevelEnd:
					e.nextLevel()
				case StateVictory:
					e.newGame()
				}
			case "Enter":
				switch e.state {
				case StateMainMenu:
					e.newGame()
				case StateLevelEnd:
					e.nextLevel()
				case StateGameOver, StateVictory:
					e.newGame()
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
