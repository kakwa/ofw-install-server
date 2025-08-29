package main

import (
	"reflect"
	"testing"
)

func TestParseMapping(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    map[[6]byte][4]byte
		wantErr bool
	}{
		{
			name:  "empty",
			input: "",
			want:  map[[6]byte][4]byte{},
		},
		{
			name:  "single mapping",
			input: "52:54:00:12:34:56=192.168.1.10",
			want: func() map[[6]byte][4]byte {
				m := make(map[[6]byte][4]byte)
				m[[6]byte{0x52, 0x54, 0x00, 0x12, 0x34, 0x56}] = [4]byte{192, 168, 1, 10}
				return m
			}(),
		},
		{
			name:    "invalid kv",
			input:   "52:54:00:12:34:56",
			wantErr: true,
		},
		{
			name:    "invalid mac",
			input:   "gg:gg:gg:gg:gg:gg=192.168.1.10",
			wantErr: true,
		},
		{
			name:    "invalid ip",
			input:   "52:54:00:12:34:56=999.168.1.10",
			wantErr: true,
		},
		{
			name:  "multiple mappings with spaces",
			input: "52:54:00:12:34:56=192.168.1.10, aa:bb:cc:dd:ee:ff=10.0.0.2",
			want: func() map[[6]byte][4]byte {
				m := make(map[[6]byte][4]byte)
				m[[6]byte{0x52, 0x54, 0x00, 0x12, 0x34, 0x56}] = [4]byte{192, 168, 1, 10}
				m[[6]byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}] = [4]byte{10, 0, 0, 2}
				return m
			}(),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseMapping(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("mismatch:\n got: %#v\nwant: %#v", got, tc.want)
			}
		})
	}
}
