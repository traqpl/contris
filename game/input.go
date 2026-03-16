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
				if e.state == StateLevelEnd {
					e.nextLevel()
				}
			case "KeyP", "Escape":
				switch e.state {
				case StatePlaying:
					e.state = StatePaused
				case StatePaused:
					e.state = StatePlaying
				case StateLevelEnd:
					e.state = StateGameOver
				}
			}
			// Prevent page scroll
			switch code {
			case "ArrowLeft", "ArrowRight", "ArrowUp", "ArrowDown", "Space", "KeyP", "KeyM", "KeyT", "Escape":
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
