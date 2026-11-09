package filters_test

import (
	"fmt"

	"github.com/aerth/mostly/filters"
)

func ExampleFilterCopy() {
	nums := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	evenCopy := filters.Copy(nums, func(n int) bool {
		return n%2 == 0
	})
	fmt.Println("Original:", nums)
	fmt.Println("Copy:", evenCopy)
	oddInPlace := filters.InPlace(nums, func(n int) bool {
		return n%2 != 0
	})
	fmt.Println("InPlace:", oddInPlace)
	fmt.Println("Backing:", nums)
	// Output:
	// Original: [1 2 3 4 5 6 7 8 9 10]
	// Copy: [2 4 6 8 10]
	// InPlace: [1 3 5 7 9]
	// Backing: [1 3 5 7 9 6 7 8 9 10]
}
