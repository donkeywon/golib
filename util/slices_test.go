package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnique(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  any
	}{
		{
			name:  "empty int slice",
			input: []int{},
			want:  []int{},
		},
		{
			name:  "single element int",
			input: []int{42},
			want:  []int{42},
		},
		{
			name:  "no duplicates int",
			input: []int{1, 2, 3, 4},
			want:  []int{1, 2, 3, 4},
		},
		{
			name:  "duplicates int",
			input: []int{1, 2, 2, 3, 3, 3, 4},
			want:  []int{1, 2, 3, 4},
		},
		{
			name:  "empty string slice",
			input: []string{},
			want:  []string{},
		},
		{
			name:  "single element string",
			input: []string{"hello"},
			want:  []string{"hello"},
		},
		{
			name:  "no duplicates string",
			input: []string{"a", "b", "c"},
			want:  []string{"a", "b", "c"},
		},
		{
			name:  "duplicates string",
			input: []string{"a", "b", "b", "c", "c", "c"},
			want:  []string{"a", "b", "c"},
		},
		{
			name:  "all same int",
			input: []int{7, 7, 7, 7, 7},
			want:  []int{7},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			switch in := tt.input.(type) {
			case []int:
				got := Unique(in)
				assert.Equal(t, tt.want, got)
			case []string:
				got := Unique(in)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestUniqueBelowMinLen(t *testing.T) {
	// Slice length 0 is below minLen (2), should return as-is
	s0 := []int{}
	got0 := Unique(s0)
	assert.Equal(t, []int{}, got0)

	// Slice length 1 is below minLen (2), should return as-is
	s1 := []int{99}
	got1 := Unique(s1)
	assert.Equal(t, []int{99}, got1)

	// Verify the returned slice is the same underlying slice (no allocation)
	s1dup := []int{99}
	got1dup := Unique(s1dup)
	assert.True(t, &s1dup[0] == &got1dup[0], "below minLen, should return same slice")
}
