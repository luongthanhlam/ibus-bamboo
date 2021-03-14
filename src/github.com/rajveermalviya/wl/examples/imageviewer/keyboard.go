package main

import "github.com/rajveermalviya/wl"

func (app *appState) HandleKeyboardKey(ev wl.KeyboardKeyEvent) {
	// close on "q"
	if ev.Key == 16 {
		close(app.exitChan)
	}
}
