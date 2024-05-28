package util

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestThousandSep(t *testing.T) {
	tests := []struct {
		n        int64
		sep      string
		expected string
	}{
		{
			0, " ", "0",
		},
		{
			10, " ", "10",
		},
		{
			999, "_", "999",
		},
		{
			1000, "_", "1_000",
		},
		{
			100000, "_", "100_000",
		},
		{
			1234567, "_", "1_234_567",
		},
	}

	for i, test := range tests {
		res := ThousandSep(test.n, test.sep)
		require.Equal(t, test.expected, res, fmt.Sprintf("failed case %v", i))
	}
}

func TestReverseInPlace(t *testing.T) {
	tests := []struct {
		arr      []int
		expected []int
	}{
		{
			nil, nil,
		},
		{
			[]int{}, []int{},
		},
		{
			[]int{10}, []int{10},
		},
		{
			[]int{1, 2}, []int{2, 1},
		},
		{
			[]int{1, 2, 3, 4, 5}, []int{5, 4, 3, 2, 1},
		},
	}

	for i, test := range tests {
		ReverseInPlace(test.arr)
		require.Equal(t, test.expected, test.arr, fmt.Sprintf("failed case %v", i))
	}
}
