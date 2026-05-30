package main

import "testing"

func TestLooksLikeJWT(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{"lan control token uuid", "3d7a44a1-8f6f-4e2e-92d7-c9baf1d1df30", false},
		{"jwt shaped token", "eyJhbGciOiJIUzI1NiJ9.eyJpZCI6IjEyMyJ9.signature", true},
		{"empty middle segment", "header..signature", false},
		{"plain text", "not-a-jwt", false},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if got := looksLikeJWT(testCase.value); got != testCase.want {
				t.Fatalf("looksLikeJWT(%q) = %v, want %v", testCase.value, got, testCase.want)
			}
		})
	}
}
