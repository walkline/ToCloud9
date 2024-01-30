package appctrl

import (
	"reflect"
	"testing"
)

func TestEnvMapToEnvSliceString(t *testing.T) {
	tests := []struct {
		name string
		envs map[string]string
		want []string
	}{
		{
			name: "Empty map",
			envs: map[string]string{},
			want: []string{},
		},
		{
			name: "Non-empty map",
			envs: map[string]string{"KEY1": "VALUE1", "KEY2": "VALUE2"},
			want: []string{"KEY1=VALUE1", "KEY2=VALUE2"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := envMapToEnvSliceString(tt.envs); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("envMapToEnvSliceString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEnvSliceStringToEnvMap(t *testing.T) {
	tests := []struct {
		name string
		envs []string
		want map[string]string
	}{
		{
			name: "Empty slice",
			envs: []string{},
			want: map[string]string{},
		},
		{
			name: "Non-empty slice",
			envs: []string{"KEY1=VALUE1", "KEY2=VALUE2"},
			want: map[string]string{"KEY1": "VALUE1", "KEY2": "VALUE2"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := envSliceStringToEnvMap(tt.envs); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("envSliceStringToEnvMap() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMergeToEnvs(t *testing.T) {
	tests := []struct {
		name           string
		firstPriority  map[string]string
		secondPriority map[string]string
		want           map[string]string
	}{
		{
			name:           "Both maps empty",
			firstPriority:  map[string]string{},
			secondPriority: map[string]string{},
			want:           map[string]string{},
		},
		{
			name:           "First map empty, second map non-empty",
			firstPriority:  map[string]string{},
			secondPriority: map[string]string{"KEY1": "VALUE1", "KEY2": "VALUE2"},
			want:           map[string]string{"KEY1": "VALUE1", "KEY2": "VALUE2"},
		},
		{
			name:           "First map non-empty, second map empty",
			firstPriority:  map[string]string{"KEY1": "VALUE1", "KEY2": "VALUE2"},
			secondPriority: map[string]string{},
			want:           map[string]string{"KEY1": "VALUE1", "KEY2": "VALUE2"},
		},
		{
			name:           "Both maps non-empty",
			firstPriority:  map[string]string{"KEY1": "VALUE1"},
			secondPriority: map[string]string{"KEY2": "VALUE2"},
			want:           map[string]string{"KEY1": "VALUE1", "KEY2": "VALUE2"},
		},
		{
			name:           "Overlapping keys, firstPriority should overwrite secondPriority",
			firstPriority:  map[string]string{"KEY1": "VALUE1"},
			secondPriority: map[string]string{"KEY1": "NEW_VALUE"},
			want:           map[string]string{"KEY1": "VALUE1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mergeTwoEnvs(tt.firstPriority, tt.secondPriority); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("mergeToEnvs() = %v, want %v", got, tt.want)
			}
		})
	}
}
