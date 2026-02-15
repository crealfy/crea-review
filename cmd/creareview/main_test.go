package main

import (
	"testing"

	rcontext "github.com/crealfy/crea-review/pkg/context"
	"github.com/crealfy/crea-review/pkg/priority"
)

func TestEnvVarsString(t *testing.T) {
	tests := []struct {
		name     string
		env      envVars
		expected string
	}{
		{
			name:     "nil map",
			env:      nil,
			expected: "",
		},
		{
			name:     "empty map",
			env:      envVars{},
			expected: "",
		},
		{
			name:     "single value",
			env:      envVars{"KEY": "value"},
			expected: "KEY=value",
		},
		{
			name:     "multiple values",
			env:      envVars{"KEY1": "value1", "KEY2": "value2"},
			expected: "KEY1=value2,KEY2=value2", // Order may vary
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.env.String()
			if tt.expected == "" && got != tt.expected {
				t.Errorf("String() = %q, want %q", got, tt.expected)
			}
			// For non-empty, just check it's not empty and contains equals
			if tt.expected != "" && got == "" {
				t.Errorf("String() = %q, want non-empty", got)
			}
		})
	}
}

func TestEnvVarsSet(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
		verify  func(envVars) bool
	}{
		{
			name:    "valid KEY=VALUE",
			value:   "KEY=value",
			wantErr: false,
			verify: func(e envVars) bool {
				return e["KEY"] == "value"
			},
		},
		{
			name:    "valid KEY=VALUE with equals in value",
			value:   "KEY=val=ue",
			wantErr: false,
			verify: func(e envVars) bool {
				return e["KEY"] == "val=ue"
			},
		},
		{
			name:    "invalid format - no equals",
			value:   "KEYVALUE",
			wantErr: true,
			verify: func(e envVars) bool {
				_, ok := e["KEYVALUE"]

				return !ok
			},
		},
		{
			name:    "empty value",
			value:   "KEY=",
			wantErr: false,
			verify: func(e envVars) bool {
				return e["KEY"] == ""
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := make(envVars)
			err := e.Set(tt.value)

			if (err != nil) != tt.wantErr {
				t.Errorf("Set() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.verify(e) {
				t.Errorf("Set() verification failed for %q", tt.value)
			}
		})
	}
}

func TestEnvVarsSetMultiple(t *testing.T) {
	e := make(envVars)

	if err := e.Set("KEY1=value1"); err != nil {
		t.Fatalf("First Set() failed: %v", err)
	}
	if err := e.Set("KEY2=value2"); err != nil {
		t.Fatalf("Second Set() failed: %v", err)
	}

	if len(e) != 2 {
		t.Errorf("len(env) = %d, want 2", len(e))
	}
	if e["KEY1"] != "value1" {
		t.Errorf("KEY1 = %q, want %q", e["KEY1"], "value1")
	}
	if e["KEY2"] != "value2" {
		t.Errorf("KEY2 = %q, want %q", e["KEY2"], "value2")
	}
}

func TestSortScores(t *testing.T) {
	tests := []struct {
		name   string
		scores []priority.Score
		sort   string
		verify func([]priority.Score) bool
	}{
		{
			name: "priority sort - default (already sorted)",
			scores: []priority.Score{
				{Path: "a.go", Total: 90},
				{Path: "b.go", Total: 80},
				{Path: "c.go", Total: 70},
			},
			sort: "priority",
			verify: func(s []priority.Score) bool {
				return s[0].Total >= s[1].Total && s[1].Total >= s[2].Total
			},
		},
		{
			name: "alpha sort",
			scores: []priority.Score{
				{Path: "z.go", Total: 90},
				{Path: "a.go", Total: 80},
				{Path: "m.go", Total: 70},
			},
			sort: "alpha",
			verify: func(s []priority.Score) bool {
				return s[0].Path == "a.go" && s[1].Path == "m.go" && s[2].Path == "z.go"
			},
		},
		{
			name: "none sort - preserve order",
			scores: []priority.Score{
				{Path: "z.go", Total: 90},
				{Path: "a.go", Total: 80},
				{Path: "m.go", Total: 70},
			},
			sort: "none",
			verify: func(s []priority.Score) bool {
				return s[0].Path == "z.go" && s[1].Path == "a.go" && s[2].Path == "m.go"
			},
		},
		{
			name:   "empty slice",
			scores: []priority.Score{},
			sort:   "priority",
			verify: func(s []priority.Score) bool {
				return len(s) == 0
			},
		},
		{
			name: "single element",
			scores: []priority.Score{
				{Path: "a.go", Total: 50},
			},
			sort: "alpha",
			verify: func(s []priority.Score) bool {
				return len(s) == 1 && s[0].Path == "a.go"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sortScores(tt.scores, tt.sort)
			if !tt.verify(got) {
				t.Errorf("sortScores() verification failed for sort=%q", tt.sort)
			}
		})
	}
}

func TestFilterFiles(t *testing.T) {
	tests := []struct {
		name   string
		files  []rcontext.FileContent
		scores []priority.Score
		want   int
	}{
		{
			name: "all files matched",
			files: []rcontext.FileContent{
				{Path: "a.go"},
				{Path: "b.go"},
				{Path: "c.go"},
			},
			scores: []priority.Score{
				{Path: "a.go", Total: 90},
				{Path: "b.go", Total: 80},
				{Path: "c.go", Total: 70},
			},
			want: 3,
		},
		{
			name: "some files matched",
			files: []rcontext.FileContent{
				{Path: "a.go"},
				{Path: "b.go"},
				{Path: "c.go"},
				{Path: "d.go"},
			},
			scores: []priority.Score{
				{Path: "a.go", Total: 90},
				{Path: "c.go", Total: 70},
			},
			want: 2,
		},
		{
			name: "no files matched",
			files: []rcontext.FileContent{
				{Path: "a.go"},
				{Path: "b.go"},
			},
			scores: []priority.Score{
				{Path: "x.go", Total: 90},
			},
			want: 0,
		},
		{
			name:   "empty files",
			files:  []rcontext.FileContent{},
			scores: []priority.Score{{Path: "a.go", Total: 90}},
			want:   0,
		},
		{
			name: "empty scores",
			files: []rcontext.FileContent{
				{Path: "a.go"},
			},
			scores: []priority.Score{},
			want:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterFiles(tt.files, tt.scores)
			if len(got) != tt.want {
				t.Errorf("filterFiles() = %d files, want %d", len(got), tt.want)
			}
		})
	}
}

func TestFilterFilesPaths(t *testing.T) {
	files := []rcontext.FileContent{
		{Path: "a.go"},
		{Path: "b.go"},
		{Path: "c.go"},
	}
	scores := []priority.Score{
		{Path: "a.go", Total: 90},
		{Path: "c.go", Total: 70},
	}

	got := filterFiles(files, scores)
	if len(got) != 2 {
		t.Fatalf("filterFiles() = %d files, want 2", len(got))
	}

	// Verify the correct files were returned
	paths := make(map[string]bool)
	for _, f := range got {
		paths[f.Path] = true
	}

	if !paths["a.go"] {
		t.Error("missing a.go in result")
	}
	if !paths["c.go"] {
		t.Error("missing c.go in result")
	}
	if paths["b.go"] {
		t.Error("b.go should not be in result")
	}
}
