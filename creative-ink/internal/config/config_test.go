package config

import (
	"encoding/json"
	"errors"
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
