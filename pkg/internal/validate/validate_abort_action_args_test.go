package validate_test

import (
	"testing"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/validate"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func TestValidAbortActionArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    *wdk.AbortActionArgs
		wantErr bool
	}{
		{
			name:    "NilArgs",
			args:    nil,
			wantErr: true,
		},
		{
			name: "BlankReference",
			args: &wdk.AbortActionArgs{
				Reference: "",
			},
			wantErr: true,
		},
		{
			name: "Base64StringNotDivisibleBy4",
			args: &wdk.AbortActionArgs{
				Reference: "ybQus1rq4M4gi/7LT",
			},
			wantErr: true,
		},
		{
			name: "ValidArgs",
			args: &wdk.AbortActionArgs{
				Reference: "ybQus1rq4M4gi/7L",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate.ValidAbortActionArgs(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidAbortActionArgs() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
