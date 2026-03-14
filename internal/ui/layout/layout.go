// Package layout provides layout utilities for the UI.
package layout

// SplitVertical splits a rectangle vertically.
func SplitVertical(main, height interface{}) (interface{}, interface{}) {
	return main, main
}

// Fixed returns a fixed size place item.
func Fixed(height int) int {
	return height
}
