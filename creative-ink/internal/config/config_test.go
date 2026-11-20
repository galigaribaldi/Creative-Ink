package config

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestCompileAutoIsOffByDefault(t *testing.T) {
	if (Layers{}).Bool(CompileAuto) {
		t.Fatal("compile.auto must default to off")
	}
}

func TestLayersPrecedence(t *testing.T) {
	k, _ := Lookup(CompileAuto)
	l := Layers{User: map[string]json.RawMessage{CompileAuto: Encode(true)}}
	if e := l.Resolve(k); e.Value != true || e.Scope != ScopeUser {
		t.Fatalf("user = %+v", e)
	}
	l.Project = map[string]json.RawMessage{CompileAuto: Encode(false)}
	if e := l.Resolve(k); e.Value != false || e.Scope != ScopeProject {
		t.Fatalf("project must win: %+v", e)
	}
	// A corrupt value is ignored: the next layer down applies.
	l.Project[CompileAuto] = json.RawMessage(`"quizá"`)
	if e := l.Resolve(k); e.Value != true || e.Scope != ScopeUser {
		t.Fatalf("corrupt project value: %+v", e)
	}
}

func TestLookupAndParse(t *testing.T) {
	k, err := Lookup("AutoCompile")
	if err != nil || k.ID != CompileAuto {
		t.Fatalf("alias: %+v, %v", k, err)
	}
	if _, err := Lookup("compile.nada"); !errors.Is(err, ErrUnknownKey) {
		t.Fatalf("unknown: %v", err)
	}
	for text, want := range map[string]bool{"on": true, "Sí": true, "off": false, "0": false} {
		if v, err := k.Parse(text); err != nil || v != want {
			t.Errorf("Parse(%q) = %v, %v", text, v, err)
		}
	}
	if _, err := k.Parse("quizá"); !errors.Is(err, ErrValue) {
		t.Errorf("bad value: %v", err)
	}
	if !k.Allows(ScopeProject) || k.Allows(ScopeDefault) {
		t.Error("scopes")
	}
}

// Every key's default is a valid value of its own type, and every key is
// writable somewhere (guards future keys such as compile.engine).
func TestRegistryDefaultsAreValid(t *testing.T) {
	for _, k := range Keys() {
		if _, ok := k.decode(Encode(k.Default)); !ok {
			t.Errorf("%s: default %v is not a valid %s", k.ID, k.Default, k.Type)
		}
		if len(k.Scopes) == 0 {
			t.Errorf("%s: no writable scope", k.ID)
		}
		if k.Type == TypeEnum && len(k.Values) == 0 {
			t.Errorf("%s: enum without values", k.ID)
		}
	}
}

func TestEnumKey(t *testing.T) {
	k := Key{ID: "test.engine", Type: TypeEnum, Default: "xelatex",
		Values: []string{"pdflatex", "xelatex", "lualatex"}, Scopes: []Scope{ScopeUser, ScopeProject}}
	if v, err := k.Parse(" XeLaTeX "); err != nil || v != "xelatex" {
		t.Fatalf("Parse = %v, %v (case-insensitive, canonical value)", v, err)
	}
	if _, err := k.Parse("luatex"); !errors.Is(err, ErrValue) {
		t.Fatalf("value outside the list: %v", err)
	}
	l := Layers{Project: map[string]json.RawMessage{k.ID: Encode("lualatex")}}
	if e := l.Resolve(k); e.Value != "lualatex" || e.Scope != ScopeProject {
		t.Fatalf("stored enum: %+v", e)
	}
	// A stored value outside the list, or of another type, is ignored.
	for _, bad := range []string{`"luatex"`, `true`, `["xelatex"]`} {
		l.Project[k.ID] = json.RawMessage(bad)
		if e := l.Resolve(k); e.Value != "xelatex" || e.Scope != ScopeDefault {
			t.Errorf("corrupt %s: %+v", bad, e)
		}
	}
	data, _ := json.Marshal(Entry{Key: k, Value: "xelatex"})
	if !strings.Contains(string(data), `"values":["pdflatex","xelatex","lualatex"]`) {
		t.Errorf("the UI needs the allowed values: %s", data)
	}
}

func TestTemplatesSourcesIsUserOnlyList(t *testing.T) {
	k, err := Lookup("sources")
	if err != nil || k.ID != TemplatesSources || k.Type != TypeList {
		t.Fatalf("alias: %+v, %v", k, err)
	}
	var l Layers
	if items := l.Strings(TemplatesSources); items == nil || len(items) != 0 {
		t.Fatalf("default = %#v, want an empty list", items)
	}
	data, _ := json.Marshal(l.Resolve(k))
	if !strings.Contains(string(data), `"value":[]`) {
		t.Errorf("an empty list must reach the UI as [] (not null): %s", data)
	}

	// A project (e.g. a cloned repository) can never add a catalog.
	if _, _, err := Prepare(TemplatesSources, "https://evil.example/index.json", ScopeProject); !errors.Is(err, ErrScope) {
		t.Fatalf("project scope: %v", err)
	}
	l.Project = map[string]json.RawMessage{TemplatesSources: Encode([]string{"https://evil.example/index.json"})}
	if items := l.Strings(TemplatesSources); len(items) != 0 {
		t.Fatalf("a project value must be ignored: %v", items)
	}

	// Typed in the terminal: comma separated, trimmed, without duplicates.
	_, raw, err := Prepare("templates.sources", " https://a.example/index.json , https://b.example/v1/index.json,https://a.example/index.json", ScopeUser)
	if err != nil {
		t.Fatal(err)
	}
	l.User = map[string]json.RawMessage{TemplatesSources: raw}
	want := []string{"https://a.example/index.json", "https://b.example/v1/index.json"}
	if got := l.Strings(TemplatesSources); !slices.Equal(got, want) {
		t.Fatalf("user value = %v, want %v", got, want)
	}
	if e := l.Resolve(k); e.Scope != ScopeUser || Format(e.Value) != "https://a.example/index.json, https://b.example/v1/index.json" {
		t.Errorf("entry: %+v", e)
	}

	for _, bad := range []string{
		"http://a.example/index.json",      // not https
		"https://user:pw@a.example/i.json", // credentials
		"https://a.example/i.json#x",       // fragment
		"a.example/index.json",             // not absolute
		"https:///index.json",              // no host
		" , ",                              // empty list
		"https://a.example/a b",            // space
	} {
		if _, _, err := Prepare(TemplatesSources, bad, ScopeUser); !errors.Is(err, ErrValue) {
			t.Errorf("Prepare(%q) = %v, want ErrValue", bad, err)
		}
	}
	// One invalid item in the stored list discards the layer's value.
	l.User[TemplatesSources] = json.RawMessage(`["https://a.example/i.json","http://b.example/i.json"]`)
	if e := l.Resolve(k); e.Scope != ScopeDefault {
		t.Errorf("corrupt stored list: %+v", e)
	}
}

func TestWithItem(t *testing.T) {
	k, _ := Lookup(TemplatesSources)
	a, b := "https://a.example/index.json", "https://b.example/index.json"
	list, err := k.WithItem(nil, a, true)
	if err != nil || !slices.Equal(list, []string{a}) {
		t.Fatalf("add: %v, %v", list, err)
	}
	list, _ = k.WithItem(list, b, true)
	if _, err := k.WithItem(list, a, true); !errors.Is(err, ErrValue) {
		t.Errorf("duplicate add: %v", err)
	}
	if _, err := k.WithItem(list, "http://c.example", true); !errors.Is(err, ErrValue) {
		t.Errorf("invalid item: %v", err)
	}
	next, err := k.WithItem(list, a, false)
	if err != nil || !slices.Equal(next, []string{b}) || !slices.Equal(list, []string{a, b}) {
		t.Fatalf("remove: %v, %v (the input must not change: %v)", next, err, list)
	}
	if _, err := k.WithItem(next, a, false); !errors.Is(err, ErrValue) {
		t.Errorf("remove a missing item: %v", err)
	}
	auto, _ := Lookup(CompileAuto)
	if _, err := auto.WithItem(nil, "on", true); !errors.Is(err, ErrValue) {
		t.Errorf("not a list: %v", err)
	}
}
