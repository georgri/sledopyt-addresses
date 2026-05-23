package kladr

import (
	"reflect"
	"testing"
)

func TestStreetLetterValues_IncludeDigitsInSequence(t *testing.T) {
	t.Parallel()

	got := streetLetterValues("1-й Нагатинский")
	want := []int{1, 11, 15, 1, 4, 1, 20, 10, 15, 19, 12, 10, 11}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected letter values: got=%v want=%v", got, want)
	}
}

func TestStreetLetterValues_StandaloneDigits(t *testing.T) {
	t.Parallel()

	got := streetLetterValues("50 лет Октября")
	want := []int{5, 0, 13, 6, 20, 16, 12, 20, 33, 2, 18, 33}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected letter values: got=%v want=%v", got, want)
	}
}
