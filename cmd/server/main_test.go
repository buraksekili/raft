package main

import (
	"testing"
)

func Test_bootstrap(t *testing.T) {
	type args struct {
		n int
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "testing with 1 replica",
			args: args{n: 1},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bootstrap(tt.args.n)
		})
	}
}
