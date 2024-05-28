package util

import (
	"fmt"
	"sort"
	"strings"

	"golang.org/x/exp/constraints"
)

// ThousandSep format int with thousands divided by sep
func ThousandSep(n int64, sep string) string {
	s := fmt.Sprintf("%v", n)

	// divide by 3 symbols from end
	size := len(s)
	res := make([]string, 0, 1+size/3)
	for i := 0; i < size; i += 3 {
		from := size - i - 3
		if from < 0 {
			from = 0
		}
		res = append(res, s[from:size-i])
	}

	ReverseInPlace(res)

	return strings.Join(res, sep)
}

// ReverseInPlace reverses any slice in place
func ReverseInPlace[T any](arr []T) {
	if len(arr) < 2 {
		return
	}
	size := len(arr)
	for i := 0; i < size/2; i++ {
		arr[i], arr[size-i-1] = arr[size-i-1], arr[i]
	}
}

func FilterSliceInPlace[T comparable](arr []T, check func(int) bool) []T {
	if len(arr) == 0 {
		return arr
	}

	size := 0
	for i := range arr {
		if check(i) {
			arr[i], arr[size] = arr[size], arr[i]
			size += 1
		}
	}

	return arr[:size]
}

func FilterUnique[T, K comparable](arr []T, key func(int) K) []T {
	if len(arr) == 0 {
		return arr
	}

	keys := make(map[K]struct{})

	size := 0
	for i := range arr {
		k := key(i)
		if _, ok := keys[k]; !ok {
			arr[i], arr[size] = arr[size], arr[i]
			size += 1
			keys[k] = struct{}{}
		}
	}

	return arr[:size]
}

func RemoveSliceElement[T any](s []T, i int) []T {
	s[i] = s[len(s)-1]
	return s[:len(s)-1]
}

func Max[T constraints.Ordered](a, b T) T {
	if a > b {
		return a
	}
	return b
}

func SortedKeys[K constraints.Ordered, V any](m map[K]V) []K {
	keys := make([]K, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i] < keys[j]
	})
	return keys
}
