package main

import (
	goruntime "runtime"

	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
)

// EventMenuCommand is emitted when the user clicks an item of the native
// menu. The UI runs the command with the same dispatcher as its keyboard
// shortcuts (actions.js runCommand), so both paths behave identically.
const EventMenuCommand = "menu:command"

// MenuCommandEvent is the payload of EventMenuCommand.
type MenuCommandEvent struct {
	ID string `json:"id"` // e.g. "new-file", "find", "split-editor"
}

// menuEntry is one item of a submenu; an empty id is a separator.
type menuEntry struct {
	id, label   string
	accelerator *keys.Accelerator
}

// applicationMenu is the native menu of the app, only on macOS: there the
// menu bar lives outside the window, and WKWebView needs the Edit menu for
// ⌘C/⌘X/⌘V (without it the clipboard shortcuts do nothing). On Linux and
// Windows a menu bar would be drawn inside the window, and the webviews
// already handle the clipboard, so they get none (nil).
//
// Accelerators are labels and a fallback: WKWebView gives the page the
// first chance at every key equivalent, so the keyboard shortcuts are run
// by the UI (App.svelte, CodeMirror) and the menu only fires on a click or
// when no page handler took the key.
func (a *App) applicationMenu() *menu.Menu {
	return a.buildMenu(goruntime.GOOS)
}

func (a *App) buildMenu(goos string) *menu.Menu {
	if goos != "darwin" {
		return nil
	}
	cmd := keys.CmdOrCtrl
	combo := func(key string, mods ...keys.Modifier) *keys.Accelerator {
		return keys.Combo(key, keys.CmdOrCtrlKey, mods[0], mods[1:]...)
	}

	root := menu.NewMenu()
	root.Append(menu.AppMenu())

	a.addSubmenu(root, "Archivo", []menuEntry{
		{"new-file", "Nuevo archivo", cmd("n")},
		{"new-window", "Nueva ventana", combo("n", keys.ShiftKey)},
		{"new-quick-doc", "Nuevo documento rápido", combo("n", keys.OptionOrAltKey)},
		{},
		{"open-folder", "Abrir carpeta…", cmd("o")},
		{"start", "Pantalla de inicio", nil},
		{},
		{"save", "Guardar", cmd("s")},
		{"save-as", "Guardar como…", combo("s", keys.ShiftKey)},
	})
	// Native Undo/Redo/Cut/Copy/Paste/Select All (labels in the system's
	// default language: Wails roles cannot be relabelled).
	root.Append(menu.EditMenu())
	a.addSubmenu(root, "Buscar", []menuEntry{
		{"find", "Buscar y reemplazar…", cmd("f")},
		{"search-project", "Buscar en el proyecto…", combo("f", keys.ShiftKey)},
		{"find-next", "Buscar siguiente", cmd("g")},
		{"find-previous", "Buscar anterior", combo("g", keys.ShiftKey)},
		{"select-next", "Seleccionar la siguiente coincidencia", cmd("d")},
		{},
		{"goto-line", "Ir a la línea…", combo("g", keys.OptionOrAltKey)},
		{"toggle-comment", "Comentar o descomentar líneas", cmd("/")},
	})
	a.addSubmenu(root, "Ver", []menuEntry{
		{"toggle-sidebar", "Explorador", cmd("b")},
		{"toggle-terminal", "Terminal CI", cmd("j")},
		{"mode-pdf", "Modo: Solo PDF", nil},
		{"mode-minimal", "Modo: Minimalista", nil},
		{"mode-multi", "Modo: Multiventana (pestañas)", nil},
		{"split-editor", "Dividir el editor", cmd("\\")},
		{"toggle-preview", "Vista previa", combo("p", keys.OptionOrAltKey)},
		{"preview-side", "Cambiar de lado la vista previa", nil},
		{},
		{"zoom-in", "Acercar", cmd("+")},
		{"zoom-out", "Alejar", cmd("-")},
		{"zoom-reset", "Tamaño real", cmd("0")},
	})
	root.Append(menu.WindowMenu())
	return root
}

func (a *App) addSubmenu(root *menu.Menu, label string, entries []menuEntry) {
	sub := root.AddSubmenu(label)
	for _, e := range entries {
		if e.id == "" {
			sub.AddSeparator()
			continue
		}
		id := e.id
		sub.AddText(e.label, e.accelerator, func(*menu.CallbackData) {
			a.emit(EventMenuCommand, MenuCommandEvent{ID: id})
		})
	}
}
