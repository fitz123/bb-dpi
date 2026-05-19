package render

import (
	"encoding/json"
	"fmt"

	"bb-dpi/client/bb-vpn/pkg/bundle"
)

func renderXray(b *bundle.Bundle, env Env) ([]byte, error) {
	// xray skeleton does NOT go through envsubst — bash flow at
	// scripts/render-config:298 reads the file with `cat`, not
	// envsubst. The skeleton has no $VAR references either.
	var tree map[string]any
	if err := json.Unmarshal(b.Skeletons.XrayXHTTP, &tree); err != nil {
		return nil, fmt.Errorf("parse xray skeleton: %w", err)
	}

	// Build inbounds: one SOCKS listener per server, port 1080+i.
	inbounds := make([]any, 0, len(b.Servers))
	for i, s := range b.Servers {
		inbounds = append(inbounds, map[string]any{
			"tag":      "socks-in-" + s.Name,
			"protocol": "socks",
			"listen":   "127.0.0.1",
			"port":     float64(1080 + i),
			"settings": map[string]any{
				"udp": true,
			},
		})
	}
	tree["inbounds"] = inbounds

	// Build outbounds: VLESS+XHTTP+REALITY per server, then a direct freedom.
	outbounds := make([]any, 0, len(b.Servers)+1)
	for _, s := range b.Servers {
		outbounds = append(outbounds, map[string]any{
			"tag":      "xhttp-" + s.Name,
			"protocol": "vless",
			"settings": map[string]any{
				"vnext": []any{
					map[string]any{
						"address": s.Host,
						"port":    float64(443),
						"users": []any{
							map[string]any{
								"id":         env.UUID,
								"encryption": "none",
							},
						},
					},
				},
			},
			"streamSettings": map[string]any{
				"network": "xhttp",
				"xhttpSettings": map[string]any{
					"path": "/" + s.XHTTPPath,
				},
				"security": "reality",
				"realitySettings": map[string]any{
					"serverName":  s.XHTTPSNI,
					"publicKey":   s.PublicKey,
					"shortId":     s.ShortID,
					"fingerprint": "chrome",
				},
			},
		})
	}
	outbounds = append(outbounds, map[string]any{
		"tag":      "direct",
		"protocol": "freedom",
	})
	tree["outbounds"] = outbounds

	// Build routing.rules: one inbound→outbound per server, plus geoip:private→direct.
	rules := make([]any, 0, len(b.Servers)+1)
	for _, s := range b.Servers {
		rules = append(rules, map[string]any{
			"type":        "field",
			"inboundTag":  []any{"socks-in-" + s.Name},
			"outboundTag": "xhttp-" + s.Name,
		})
	}
	rules = append(rules, map[string]any{
		"type":        "field",
		"ip":          []any{"geoip:private"},
		"outboundTag": "direct",
	})

	routing, ok := tree["routing"].(map[string]any)
	if !ok {
		// Skeleton missing routing — initialize.
		routing = map[string]any{}
		tree["routing"] = routing
	}
	routing["rules"] = rules

	return marshalCanonical(tree)
}
