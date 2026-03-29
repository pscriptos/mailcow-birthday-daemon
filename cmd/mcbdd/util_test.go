package main

import "testing"

func Test_sanitizeDate(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		input   string
		want    uint16
		want2   uint16
		want3   uint16
		wantErr bool
	}{
		{
			name:    "test normal date w/o dashes",
			input:   "20001203",
			want:    2000,
			want2:   12,
			want3:   0o3,
			wantErr: false,
		},
		{
			name:    "test normal date w/ dashes",
			input:   "2005-05-16",
			want:    2005,
			want2:   5,
			want3:   16,
			wantErr: false,
		},
		{
			name:    "test normal date w/o year w/o dashes",
			input:   "1203",
			want:    0,
			want2:   12,
			want3:   0o3,
			wantErr: false,
		},
		{
			name:    "test normal date w/o year w/ dashes",
			input:   "-05-16",
			want:    0,
			want2:   5,
			want3:   16,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got2, got3, gotErr := sanitizeDate(tt.input)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("sanitizeDate() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("sanitizeDate() succeeded unexpectedly")
			}
			if got != tt.want {
				t.Errorf("sanitizeDate() = %v, want %v", got, tt.want)
			}
			if got2 != tt.want2 {
				t.Errorf("sanitizeDate() = %v, want %v", got2, tt.want2)
			}
			if got3 != tt.want3 {
				t.Errorf("sanitizeDate() = %v, want %v", got3, tt.want3)
			}
		})
	}
}
