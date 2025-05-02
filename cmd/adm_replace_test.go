package cmd

import (
	"github.com/jfardello/tdns/config"
	"testing"
)

func tConf() {
	c := &config.Config{
		Server: config.Server{
			APIAddr: "127.0.0.1:90909",
		},
	}
	config.SetRunningConfig(c)
}

func Test_handleStubs(t *testing.T) {
	tConf()
	type args struct {
		stubs []string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name:    "test",
			args:    args{stubs: []string{"google.es,udp://8.8.8.8", "google.com,udp://8.8.8.8"}},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := handleStubs(tt.args.stubs); (err != nil) != tt.wantErr {
				t.Errorf("handleStubs() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
