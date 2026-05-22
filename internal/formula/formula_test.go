package formula

import "testing"

func TestParseAndEval(t *testing.T) {
	f, err := Parse(`1 + 3 * x2 - 4*x5 + "б"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if _, ok := f.Eval([]int{1, 2, 3, 4, 5, 6}); ok {
		t.Fatalf("eval must reject non-positive house numbers")
	}

	if _, ok := f.Eval([]int{1, 2, 3, 4}); ok {
		t.Fatalf("eval must fail when formula references missing x index")
	}
}

func TestParseAndEvalPositive(t *testing.T) {
	f, err := Parse(`10 + 2*x1 + "а"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	got, ok := f.Eval([]int{3})
	if !ok {
		t.Fatalf("eval must succeed")
	}
	if got != "16а" {
		t.Fatalf("unexpected result: %s", got)
	}
}
