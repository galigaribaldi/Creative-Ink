// Package config is the layered configuration service of Creative-ink
// (docs/MAP.md §4.6, step 2). Every setting is a typed key declared here;
// its value is resolved from layers with this precedence:
//
//	default (code) < user (settings.json) < project (.creative-ink-meta)
//
// The terminal (`ci config …`), the UI (GetConfig/SetConfig and the
// config:changed event) and future plugins all go through this package:
// nobody reads the raw files on their own.
//
// It has no Wails or file-system dependency: callers load the layers.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
)

// Scope is the layer a value comes from or is written to.
type Scope string

const (
	ScopeDefault Scope = "default" // built into the code
	ScopeUser    Scope = "user"    // <UserConfigDir>/creative-ink/settings.json
	ScopeProject Scope = "project" // <project>/.creative-ink-meta
)

// Type of a key's value.
type Type string

const TypeBool Type = "bool"

// Key ids.
const (
	// CompileAuto: compile automatically after a pause in typing. Off by
	// default (decision of 2026-11-08): heavy theses only compile with ⌘S,
	// the Compilar button or `ci compile`.
	CompileAuto = "compile.auto"
)

// Key declares one setting.
type Key struct {
	ID          string  `json:"id"`
	Type        Type    `json:"type"`
	Default     any     `json:"default"`
	Scopes      []Scope `json:"scopes"` // writable layers
	Description string  `json:"description"`
	// Aliases are short names accepted by `ci config` (e.g. "autocompile").
	Aliases []string `json:"-"`
}

var registry = []Key{
	{
		ID:          CompileAuto,
		Type:        TypeBool,
		Default:     false,
		Scopes:      []Scope{ScopeUser, ScopeProject},
		Description: "Compilar automáticamente al dejar de escribir",
		Aliases:     []string{"autocompile"},
	},
}

var (
	// ErrUnknownKey is returned for ids that are not in the registry.
	ErrUnknownKey = errors.New("clave de configuración desconocida")
	// ErrScope is returned when a key cannot be written to a layer.
	ErrScope = errors.New("ámbito no permitido para esta clave")
	// ErrValue is returned for a value of the wrong type.
	ErrValue = errors.New("valor inválido")
)

// Keys returns the registry, in declaration order.
func Keys() []Key { return slices.Clone(registry) }

// Lookup finds a key by id or alias (case-insensitive).
func Lookup(name string) (Key, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	for _, k := range registry {
		if k.ID == name || slices.Contains(k.Aliases, name) {
			return k, nil
		}
	}
	return Key{}, fmt.Errorf("%w: %q (usa `ci config list`)", ErrUnknownKey, name)
}

// Allows reports whether the key can be written to scope.
func (k Key) Allows(scope Scope) bool { return slices.Contains(k.Scopes, scope) }

// Parse converts text typed in the terminal to the key's type.
func (k Key) Parse(text string) (any, error) {
	switch k.Type {
	case TypeBool:
		switch strings.ToLower(strings.TrimSpace(text)) {
		case "on", "true", "1", "yes", "si", "sí":
			return true, nil
		case "off", "false", "0", "no":
			return false, nil
		}
		return nil, fmt.Errorf("%w para %s: %q (usa on u off)", ErrValue, k.ID, text)
	}
	return nil, fmt.Errorf("%w: tipo %q sin soporte", ErrValue, k.Type)
}

// decode validates a stored value; ok is false for a corrupt one (ignored).
func (k Key) decode(raw json.RawMessage) (any, bool) {
	switch k.Type {
	case TypeBool:
		var b bool
		if json.Unmarshal(raw, &b) == nil {
			return b, true
		}
	}
	return nil, false
}

// Encode turns a parsed value into what the layer files store.
func Encode(v any) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}

// Format shows a value in the terminal ("on"/"off" for booleans).
func Format(v any) string {
	if b, ok := v.(bool); ok {
		if b {
			return "on"
		}
		return "off"
	}
	return fmt.Sprint(v)
}

// Layers are the stored values of each layer, as raw JSON by key id.
type Layers struct {
	User    map[string]json.RawMessage
	Project map[string]json.RawMessage
}

// Entry is a key with its effective value and where it came from.
type Entry struct {
	Key
	Value any   `json:"value"`
	Scope Scope `json:"scope"` // layer that set Value
}

// Resolve returns the effective value of one key.
func (l Layers) Resolve(k Key) Entry {
	e := Entry{Key: k, Value: k.Default, Scope: ScopeDefault}
	if raw, ok := l.User[k.ID]; ok && k.Allows(ScopeUser) {
		if v, ok := k.decode(raw); ok {
			e.Value, e.Scope = v, ScopeUser
		}
	}
	if raw, ok := l.Project[k.ID]; ok && k.Allows(ScopeProject) {
		if v, ok := k.decode(raw); ok {
			e.Value, e.Scope = v, ScopeProject
		}
	}
	return e
}

// All resolves every key of the registry.
func (l Layers) All() []Entry {
	entries := make([]Entry, 0, len(registry))
	for _, k := range registry {
		entries = append(entries, l.Resolve(k))
	}
	return entries
}

// Bool is the effective boolean value of id (its default if id is unknown
// or not a boolean, which is a programming error caught by the tests).
func (l Layers) Bool(id string) bool {
	k, err := Lookup(id)
	if err != nil {
		return false
	}
	b, _ := l.Resolve(k).Value.(bool)
	return b
}

// Prepare validates a write before it is stored: the key (id or alias), the
// scope and the value (text as typed; "" removes the scope's value, raw is
// then nil). Every writer (App.SetConfig, tests) goes through it.
func Prepare(name, value string, scope Scope) (Key, json.RawMessage, error) {
	k, err := Lookup(name)
	if err != nil {
		return Key{}, nil, err
	}
	if !k.Allows(scope) {
		return Key{}, nil, fmt.Errorf("%w: %s no se puede guardar en %q", ErrScope, k.ID, scope)
	}
	if strings.TrimSpace(value) == "" {
		return k, nil, nil
	}
	v, err := k.Parse(value)
	if err != nil {
		return Key{}, nil, err
	}
	return k, Encode(v), nil
}
