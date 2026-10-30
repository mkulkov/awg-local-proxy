//go:build !awgtrace

// Package diagtrace provides fixed startup markers in diagnostic builds.
package diagtrace

func Step(string) {}
