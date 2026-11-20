package render

import (
	"reflect"
	"testing"

	"bb-dpi/client/bb-vpn/pkg/bundle"
)

// setRouteExcludeAddress must keep the skeleton's entries (the LAN bypass)
// and append the server /32s, treating missing/null/false as empty and
// failing on any other non-array — the same outcomes as jq's
// `(.route_exclude_address // []) + [...]` in scripts/render-config.
func TestSetRouteExcludeAddress(t *testing.T) {
	servers := []bundle.Server{{Host: "192.0.2.10"}, {Host: "192.0.2.11"}}
	hosts := []any{"192.0.2.10/32", "192.0.2.11/32"}
	missing := struct{}{}

	cases := []struct {
		name    string
		value   any // missing = key absent
		want    []any
		wantErr bool
	}{
		{name: "missing", value: missing, want: hosts},
		{name: "null", value: nil, want: hosts},
		{name: "false", value: false, want: hosts},
		{name: "empty", value: []any{}, want: hosts},
		{name: "array", value: []any{"192.168.0.0/16"},
			want: []any{"192.168.0.0/16", "192.0.2.10/32", "192.0.2.11/32"}},
		{name: "string", value: "192.168.0.0/16", wantErr: true},
		{name: "object", value: map[string]any{"a": "b"}, wantErr: true},
		{name: "true", value: true, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			inb0 := map[string]any{"type": "tun"}
			if tc.value != missing {
				inb0["route_exclude_address"] = tc.value
			}
			tree := map[string]any{"inbounds": []any{inb0}}

			err := setRouteExcludeAddress(tree, servers)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("got nil error, result %v", inb0["route_exclude_address"])
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := inb0["route_exclude_address"]; !reflect.DeepEqual(got, tc.want) {
				t.Errorf("route_exclude_address = %v, want %v", got, tc.want)
			}
		})
	}
}
