package config

import (
	"fmt"
	"net"

	"github.com/rs/zerolog/log"
)

var trustedProxies []*net.IPNet = []*net.IPNet{}

// Get the configured trusted proxy ips.
func TrustedProxies() []*net.IPNet {
	return trustedProxies
}

func ParseTrustedProxies(strs []string) error {

	trustedProxies = []*net.IPNet{}

	for _, str := range strs {

		_, net, _ := net.ParseCIDR(str)

		if net == nil {
			return fmt.Errorf("invalid inet: %s", str)
		}

		log.Info().Str("subnet", str).Msg("Adding trusted proxy")

		trustedProxies = append(trustedProxies, net)
	}

	return nil
}
