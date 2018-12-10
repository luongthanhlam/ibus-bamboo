/*
 * Bamboo - A Vietnamese Input method editor
 * Copyright (C) 2018 Luong Thanh Lam <ltlam93@gmail.com>
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package main

import (
	"github.com/BambooEngine/bamboo-core"
	"github.com/godbus/dbus"
	"log"
	"time"
)

var backspaceUpdateChan = make(chan []rune)

func (e *IBusBambooEngine) startBackspaceAutoCommit() {
	for {
		select {
		case <-backspaceUpdateChan:
			time.Sleep(3 * time.Millisecond)
			x11SendText("`")
			break
		}
	}
}

func (e *IBusBambooEngine) backspaceProcessKeyEvent(keyVal uint32, keyCode uint32, state uint32) (bool, *dbus.Error) {
	var rawKeyLen = e.getRawKeyLen()
	if keyVal == IBUS_BackSpace {
		if e.nBackSpace == 0 && rawKeyLen > 0 {
			oldRunes := []rune(e.getPreeditString())
			e.preediter.RemoveLastChar()
			newRunes := []rune(e.getPreeditString())
			e.updatePreviousText(newRunes, oldRunes, state)
			return true, nil
		}
		if e.nBackSpace > 0 {
			e.nBackSpace--
			if e.nBackSpace == 0 {
				backspaceUpdateChan <- e.newChars
			}
		}

		//No thing left, just ignore
		return false, nil
	}

	if keyVal == IBUS_Return || keyVal == IBUS_KP_Enter {
		if rawKeyLen > 0 {
			e.preediter.Reset()
		}
		return false, nil
	}

	if keyVal == IBUS_Escape {
		if rawKeyLen > 0 {
			e.preediter.Reset()
			return true, nil
		}
		return false, nil
	}
	var keyRune = rune(keyVal)

	if e.inX11BackspaceList() && keyCode == 0x0058 {
		e.SendText(e.newChars)
		e.newChars = nil
		return true, nil
	}

	if keyVal == IBUS_space || keyVal == IBUS_KP_Space {
		if rawKeyLen > 0 {
			if e.mustFallbackToEnglish() {
				oldRunes := []rune(e.preediter.GetProcessedString(bamboo.VietnameseMode))
				newRunes := []rune(e.preediter.GetProcessedString(bamboo.EnglishMode))
				e.updatePreviousText(newRunes, oldRunes, state)
			}
			e.preediter.Reset()
			return false, nil
		}
	}

	if (keyVal >= 'a' && keyVal <= 'z') ||
		(keyVal >= 'A' && keyVal <= 'Z') ||
		(keyVal >= '0' && keyVal <= '9') ||
		(inKeyMap(e.preediter.GetInputMethod().Keys, rune(keyVal))) {
		if state&IBUS_LOCK_MASK != 0 {
			keyRune = toUpper(keyRune)
		}
		if e.config.IBflags&IBautoNonVnRestore == 0 {
			oldRunes := []rune(e.preediter.GetProcessedString(bamboo.VietnameseMode))
			e.preediter.ProcessChar(keyRune, bamboo.VietnameseMode)
			newRunes := []rune(e.preediter.GetProcessedString(bamboo.VietnameseMode))
			e.updatePreviousText(newRunes, oldRunes, state)
			return true, nil
		}
		oldRunes := []rune(e.getPreeditString())
		e.preediter.ProcessChar(keyRune, e.getMode())
		newRunes := []rune(e.getPreeditString())
		e.updatePreviousText(newRunes, oldRunes, state)
		return true, nil
	} else {
		if rawKeyLen > 0 {
			e.preediter.Reset()
			return false, nil
		}
		//pre-edit empty, just forward key
		return false, nil
	}
	return false, nil
}

func (e *IBusBambooEngine) inSurroundingList() bool {
	return inWhiteList(e.config.SurroundingWhiteList, e.wmClasses)
}

func (e *IBusBambooEngine) inIBusForwardList() bool {
	return inWhiteList(e.config.IBusBackspaceWhiteList, e.wmClasses)
}

func (e *IBusBambooEngine) inX11BackspaceList() bool {
	return inWhiteList(e.config.X11BackspaceWhiteList, e.wmClasses)
}

func (e *IBusBambooEngine) updatePreviousText(newRunes, oldRunes []rune, state uint32) {
	mouseCaptureUnlock()
	oldLen := len(oldRunes)
	newLen := len(newRunes)
	minLen := oldLen
	if newLen < minLen {
		minLen = newLen
	}

	sameTo := -1
	for i := 0; i < minLen; i++ {
		if oldRunes[i] == newRunes[i] {
			sameTo = i
		} else {
			break
		}
	}
	diffFrom := sameTo + 1

	nBackSpace := 0
	if diffFrom < newLen && diffFrom < oldLen {
		e.SendText([]rune{0x200A}) // https://en.wikipedia.org/wiki/Whitespace_character
		nBackSpace += 1
	}

	if diffFrom < oldLen {
		nBackSpace += oldLen - diffFrom
	}

	if nBackSpace > 0 {
		e.SendBackSpace(state, nBackSpace)
	}
	if e.inX11BackspaceList() {
		e.nBackSpace = nBackSpace
		e.newChars = newRunes[diffFrom:]
		if nBackSpace == 0 {
			e.SendText(newRunes[diffFrom:])
		}
	} else {
		e.SendText(newRunes[diffFrom:])
	}
}

func (e *IBusBambooEngine) SendBackSpace(state uint32, n int) {
	log.Printf("Sendding %d backSpace\n", n)
	if n == 0 {
		return
	}

	if inWhiteList(e.config.SurroundingWhiteList, e.wmClasses) {
		log.Println("Send backspace via SurroundingText")
		e.DeleteSurroundingText(-int32(n), uint32(n))
	} else if inWhiteList(e.config.X11BackspaceWhiteList, e.wmClasses) {
		log.Println("Send backspace via X11 KeyEvent")
		//x11Sync(e.display)
		for i := 0; i < n; i++ {
			//x11Sync(e.display)
			x11Backspace()
		}
	} else if inWhiteList(e.config.IBusBackspaceWhiteList, e.wmClasses) {
		log.Println("Send backspace via IBus ForwardKeyEvent")
		x11Flush(e.display)
		x11Sync(e.display)
		for i := 0; i < n; i++ {
			e.ForwardKeyEvent(IBUS_BackSpace, 14, state)
			e.ForwardKeyEvent(IBUS_BackSpace, 14, state|IBUS_RELEASE_MASK)
		}
	} else {
		log.Println("There's something wrong with wmClasses")
	}
}

func (e *IBusBambooEngine) SendText(rs []rune) {
	log.Println("Send text", string(rs))
	e.HidePreeditText()

	//e.CommitText(ibus.NewText(string(rs)))
	e.commitText(string(rs))
}

func (e *IBusBambooEngine) inBackspaceWhiteList(wmClasses []string) bool {
	if inWhiteList(e.config.IBusBackspaceWhiteList, wmClasses) {
		return true
	}
	if inWhiteList(e.config.X11BackspaceWhiteList, wmClasses) {
		return true
	}
	if inWhiteList(e.config.SurroundingWhiteList, wmClasses) {
		return true
	}
	return false
}
