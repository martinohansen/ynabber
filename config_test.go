package ynabber

import (
	"testing"
	"time"
)

func TestDateDecode(t *testing.T) {
	type args struct {
		value string
	}
	tests := []struct {
		name    string
		date    *Date
		args    args
		want    time.Time
		wantErr bool
	}{
		{
			date:    &Date{},
			args:    args{value: "2000-12-24"},
			want:    time.Date(2000, 12, 24, 0, 0, 0, 0, time.UTC),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := &Date{}
			if err := got.Decode(tt.args.value); (err != nil) != tt.wantErr {
				t.Errorf("Date.Decode() error = %v, wantErr %v", err, tt.wantErr)
			}
			if time.Time(*got) != tt.want {
				t.Errorf("Date.Decode() got = %v, want %v", time.Time(*got), tt.want)
			}
		})
	}
}
