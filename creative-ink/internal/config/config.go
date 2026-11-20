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
	"net/url"
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

const (
	TypeBool Type = "bool"
	// TypeEnum is one of Key.Values (stored as a JSON string).
	TypeEnum Type = "enum"
	// TypeList is a list of strings (stored as a JSON array), each one
	// checked by the key's item validator. Typed as "a,b" in the terminal,
	// or one by one with `ci config add|remove`.
	TypeList Type = "list"
)

// Key ids.
const (
	// CompileAuto: compile automatically after a pause in typing. Off by
	// default (decision of 2026-11-08): heavy theses only compile with ⌘S,
	// the Compilar button or `ci compile`.
	CompileAuto = "compile.auto"
	// TemplatesSources: third-party template catalogs (docs/MAP.md §4.8),
	// https URLs of their index. Only in the user layer: a cloned project
	// must never add a catalog. The official catalog is fixed in the code
	// and is not part of this list.
	TemplatesSources = "templates.sources"
)

// Key declares one setting.
type Key struct {
	ID          string  `json:"id"`
	Type        Type    `json:"type"`
	Default     any     `json:"default"`
	Scopes      []Scope `json:"scopes"` // writable layers
	Description string  `json:"description"`
	// Values are the allowed values of a TypeEnum key, in display order.
	Values []string `json:"values,omitempty"`
	// Aliases are short names accepted by `ci config` (e.g. "autocompile").
	Aliases []string `json:"-"`
	// validItem checks one item of a TypeList key (nil: any non-empty text).
	validItem func(string) error
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
	{
		ID:          TemplatesSources,
		Type:        TypeList,
		Default:     []string{},
		Scopes:      []Scope{ScopeUser},
		Description: "Catálogos de plantillas de terceros (URL https de su índice)",
		Aliases:     []string{"sources"},
		validItem:   validSourceURL,
	},
}

// validSourceURL accepts an absolute https URL without credentials or
// fragment: it is fetched by Go, and shown to the user as the source.
func validSourceURL(text string) error {
	u, err := url.Parse(text)
	switch {
	case err != nil:
		return fmt.Errorf("%q no es una URL", text)
	case u.Scheme != "https" || u.Host == "":
		return fmt.Errorf("%q: el catálogo debe ser una URL https://", text)
	case u.User != nil:
		return fmt.Errorf("%q: la URL no puede llevar usuario ni contraseña", text)
	case u.Fragment != "" || strings.Contains(text, "#"):
		return fmt.Errorf("%q: la URL no puede llevar #", text)
	case strings.ContainsAny(text, " \t\n\r"):
		return fmt.Errorf("%q: la URL no puede llevar espacios", text)
	}
	return nil
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
	case TypeEnum:
		if v, ok := k.enumValue(text); ok {
			return v, nil
		}
		return nil, fmt.Errorf("%w para %s: %q (usa %s)", ErrValue, k.ID, text, strings.Join(k.Values, ", "))
	case TypeList:
		var items []string
		for _, part := range strings.Split(text, ",") {
			if part = strings.TrimSpace(part); part != "" && !slices.Contains(items, part) {
				items = append(items, part)
			}
		}
		if len(items) == 0 {
			return nil, fmt.Errorf("%w para %s: lista vacía (para quitarla: ci config unset %s)", ErrValue, k.ID, k.ID)
		}
		for _, item := range items {
			if err := k.checkItem(item); err != nil {
				return nil, err
			}
		}
		return items, nil
	}
	return nil, fmt.Errorf("%w: tipo %q sin soporte", ErrValue, k.Type)
}

// enumValue matches text to one of the allowed values (case-insensitive).
func (k Key) enumValue(text string) (string, bool) {
	text = strings.TrimSpace(text)
	for _, v := range k.Values {
		if strings.EqualFold(v, text) {
			return v, true
		}
	}
	return "", false
}

// checkItem validates one item of a list key.
func (k Key) checkItem(item string) error {
	switch {
	case strings.TrimSpace(item) == "":
		return fmt.Errorf("%w para %s: elemento vacío", ErrValue, k.ID)
	case strings.Contains(item, ","): // the separator typed in the terminal
		return fmt.Errorf("%w para %s: %q no puede llevar comas", ErrValue, k.ID, item)
	}
	if k.validItem != nil {
		if err := k.validItem(item); err != nil {
			return fmt.Errorf("%w para %s: %v", ErrValue, k.ID, err)
		}
	}
	return nil
}

// WithItem returns the list with item added (at the end, once) or removed:
// the value `ci config add|remove` writes. current is the list the layer
// has now.
func (k Key) WithItem(current []string, item string, add bool) ([]string, error) {
	if k.Type != TypeList {
		return nil, fmt.Errorf("%w: %s no es una lista", ErrValue, k.ID)
	}
	item = strings.TrimSpace(item)
	if err := k.checkItem(item); err != nil {
		return nil, err
	}
	next := slices.Clone(current)
	switch i := slices.Index(next, item); {
	case add && i < 0:
		next = append(next, item)
	case add:
		return nil, fmt.Errorf("%w: %s ya contiene %q", ErrValue, k.ID, item)
	case i < 0:
		return nil, fmt.Errorf("%w: %s no contiene %q", ErrValue, k.ID, item)
	default:
		next = slices.Delete(next, i, i+1)
	}
	return next, nil
}

// decode validates a stored value; ok is false for a corrupt one (ignored).
func (k Key) decode(raw json.RawMessage) (any, bool) {
	switch k.Type {
	case TypeBool:
		var b bool
		if json.Unmarshal(raw, &b) == nil {
			return b, true
		}
	case TypeEnum:
		var text string
		if json.Unmarshal(raw, &text) == nil {
			if v, ok := k.enumValue(text); ok {
				return v, true
			}
		}
	case TypeList:
		var items []string
		if json.Unmarshal(raw, &items) != nil || items == nil {
			return nil, false
		}
		for _, item := range items {
			if k.checkItem(item) != nil {
				return nil, false // one bad item discards the whole layer value
			}
		}
		return items, true
	}
	return nil, false
}

// Encode turns a parsed value into what the layer files store.
func Encode(v any) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}

// Format shows a value in the terminal ("on"/"off" for booleans, lists
// separated by commas, "(vacía)" for an empty list).
func Format(v any) string {
	switch v := v.(type) {
	case bool:
		if v {
			return "on"
		}
		return "off"
	case []string:
		if len(v) == 0 {
			return "(vacía)"
		}
		return strings.Join(v, ", ")
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

// String is the effective value of an enum key ("" if id is unknown or not
// an enum, a programming error caught by the tests).
func (l Layers) String(id string) string {
	k, err := Lookup(id)
	if err != nil {
		return ""
	}
	text, _ := l.Resolve(k).Value.(string)
	return text
}

// Strings is the effective value of a list key (a copy; nil if id is
// unknown or not a list).
func (l Layers) Strings(id string) []string {
	k, err := Lookup(id)
	if err != nil {
		return nil
	}
	items, _ := l.Resolve(k).Value.([]string)
	return slices.Clone(items)
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
		if slices.Equal(k.Scopes, []Scope{ScopeUser}) {
			return Key{}, nil, fmt.Errorf("%w: %s solo se guarda en tu configuración de usuario (un proyecto no puede cambiarla)", ErrScope, k.ID)
		}
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
