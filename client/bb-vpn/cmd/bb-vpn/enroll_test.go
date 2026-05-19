package main

import "testing"

func TestParseEnrollArg(t *testing.T) {
	const validUUID = "11111111-2222-3333-4444-555555555555"
	cases := []struct {
		name    string
		arg     string
		want    string
		wantErr bool
	}{
		{"full URI", "bb-vpn://enroll?uuid=" + validUUID, validUUID, false},
		{"bare UUID", validUUID, validUUID, false},
		{"wrong scheme", "https://enroll?uuid=" + validUUID, "", true},
		{"wrong host", "bb-vpn://other?uuid=" + validUUID, "", true},
		{"missing uuid in URI", "bb-vpn://enroll", "", true},
		{"malformed UUID bare", "not-a-uuid", "", true},
		{"empty arg", "", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseEnrollArg(tc.arg)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("want error, got nil (got=%q)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("want %q, got %q", tc.want, got)
			}
		})
	}
}
