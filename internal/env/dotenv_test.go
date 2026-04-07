package env

import (
	"slices"
	"testing"
)

func mapKeys[K comparable, V any](m map[K]V) []K {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func TestEnvSliceToMap(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		wantKeys []string
		wantVals []string
	}{
		{
			name:     "empty slice returns empty map",
			input:    nil,
			wantKeys: nil,
			wantVals: nil,
		},
		{
			name:     "single key value",
			input:    []string{"FOO=bar"},
			wantKeys: []string{"FOO"},
			wantVals: []string{"bar"},
		},
		{
			name:     "value containing equals sign",
			input:    []string{"DB=host=localhost"},
			wantKeys: []string{"DB"},
			wantVals: []string{"host=localhost"},
		},
		{
			name:     "empty value preserved",
			input:    []string{"EMPTY="},
			wantKeys: []string{"EMPTY"},
			wantVals: []string{""},
		},
		{
			name:     "no separator skipped",
			input:    []string{"NOSEPS"},
			wantKeys: []string{},
			wantVals: []string{},
		},
		{
			name:     "mixed valid and invalid",
			input:    []string{"A=b", "NOCOLS", "C=d"},
			wantKeys: []string{"A", "C"},
			wantVals: []string{"b", "d"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EnvSliceToMap(tt.input)
			keys := mapKeys(got)
			slices.Sort(keys)
			vals := make([]string, len(keys))
			for i, k := range keys {
				vals[i] = got[k]
			}
			if !slices.Equal(keys, tt.wantKeys) {
				t.Errorf("keys = %v, want %v", keys, tt.wantKeys)
			}
			if !slices.Equal(vals, tt.wantVals) {
				t.Errorf("vals = %v, want %v", vals, tt.wantVals)
			}
		})
	}
}
