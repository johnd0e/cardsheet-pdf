package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestExpandWildcardsSortsMatches(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"b.png", "a.png"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	got, err := expandWildcards([]string{filepath.Join(dir, "*.png")})
	if err != nil {
		t.Fatal(err)
	}

	want := []string{filepath.Join(dir, "a.png"), filepath.Join(dir, "b.png")}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expandWildcards() = %v, want %v", got, want)
	}
}

func TestExpandWildcardsErrorsOnNoMatch(t *testing.T) {
	_, err := expandWildcards([]string{filepath.Join(t.TempDir(), "*.jpg")})
	if err == nil {
		t.Fatal("expected no-match error")
	}
}

func TestRepeatedFloatFlagCollectsValues(t *testing.T) {
	var values repeatedFloatFlag
	if err := values.Set("5"); err != nil {
		t.Fatal(err)
	}
	if err := values.Set("10"); err != nil {
		t.Fatal(err)
	}

	want := repeatedFloatFlag{5, 10}
	if !reflect.DeepEqual(values, want) {
		t.Fatalf("values = %v, want %v", values, want)
	}
}
