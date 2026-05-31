package config

import (
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/walkline/ToCloud9/shared/config"
)

// Config is config of application
type Config struct {
	config.Logging `yaml:"logging"`

	// GRPCPort is port that would be used to listen for GRPC requests
	GRPCPort string `yaml:"grpcPort" env:"GRPC_PORT" env-default:"9501"`

	// HealthCheckPort is port that would be used to listen for health checks
	HealthCheckPort string `yaml:"healthCheckPort" env:"HEALTH_CHECK_PORT" env-default:"8901"`

	// PreferredHostname is referred host name that will be used to connect from gateway and for health checks
	PreferredHostname string `yaml:"preferredHostname" env:"PREFERRED_HOSTNAME"`

	// LoopbackHostOverride rewrites loopback service targets for split WSL/Windows local deployments.
	LoopbackHostOverride string `yaml:"loopbackHostOverride" env:"LOOPBACK_HOST_OVERRIDE" env-default:"auto"`

	// ServersRegistryServiceAddress is address of servers registry service
	ServersRegistryServiceAddress string `yaml:"serversRegistryServiceAddress" env:"SERVERS_REGISTRY_SERVICE_ADDRESS" env-default:"localhost:8999"`

	// MatchmakingServiceAddress is address of matchmaking service
	MatchmakingServiceAddress string `yaml:"matchmakingServiceAddress" env:"MATCHMAKING_SERVICE_ADDRESS" env-default:"localhost:8994"`

	// GuidProviderServiceAddress is address of service that provides guids to use
	GuidProviderServiceAddress string `yaml:"guidProviderServiceAddress" env:"GUID_PROVIDER_SERVICE_ADDRESS" env-default:"localhost:8996"`

	GroupServiceAddress string `yaml:"groupServiceAddress" env:"GROUP_SERVICE_ADDRESS" env-default:"localhost:8998"`

	// CharacterServiceAddress is address of characters service.
	CharacterServiceAddress string `yaml:"charactersServiceAddress" env:"CHAR_SERVICE_ADDRESS" env-default:"localhost:8991"`

	// CharacterGuidsBufferSize is the size of the buffer for characters guids
	CharacterGuidsBufferSize int `yaml:"characterGuidsBufferSize" env:"CHARACTER_GUIDS_BUFFER_SIZE" env-default:"50"`

	// CharacterGuidsBufferSize is the size of the buffer for items guids
	ItemGuidsBufferSize int `yaml:"itemGuidsBufferSize" env:"ITEM_GUIDS_BUFFER_SIZE" env-default:"200"`

	// InstanceGuidsBufferSize is the size of the buffer for dungeon/raid instances guids
	InstanceGuidsBufferSize int `yaml:"instanceGuidsBufferSize" env:"INSTANCE_GUIDS_BUFFER_SIZE" env-default:"10"`

	// NatsURL is nats connection url
	NatsURL string `yaml:"natsUrl" env:"NATS_URL" env-default:"nats://localhost:4222"`
}

// LoadConfig loads config from env variables
func LoadConfig() (*Config, error) {
	var c struct {
		Root Config `yaml:"gameserver"`
	}

	config.EnvVarConfigFilePath = "TC9_CONFIG_FILE"
	config.ConfigPathFlagName = "tc9config"

	err := config.LoadConfig(&c)
	if err != nil {
		return nil, err
	}

	c.Root.applyRuntimeOverrides()

	return &c.Root, nil
}

func (c *Config) applyRuntimeOverrides() {
	loopbackOverride := resolveLoopbackOverride(c.LoopbackHostOverride)
	if loopbackOverride == "" {
		return
	}

	c.ServersRegistryServiceAddress = rewriteLoopbackHostPort(c.ServersRegistryServiceAddress, loopbackOverride)
	c.MatchmakingServiceAddress = rewriteLoopbackHostPort(c.MatchmakingServiceAddress, loopbackOverride)
	c.GuidProviderServiceAddress = rewriteLoopbackHostPort(c.GuidProviderServiceAddress, loopbackOverride)
	c.GroupServiceAddress = rewriteLoopbackHostPort(c.GroupServiceAddress, loopbackOverride)
	c.CharacterServiceAddress = rewriteLoopbackHostPort(c.CharacterServiceAddress, loopbackOverride)
	c.NatsURL = rewriteLoopbackURL(c.NatsURL, loopbackOverride)
}

func resolveLoopbackOverride(value string) string {
	normalized := strings.TrimSpace(strings.ToLower(value))
	switch normalized {
	case "", "off", "false", "disabled":
		return ""
	case "auto":
		return detectWSLHostIP()
	default:
		return strings.TrimSpace(value)
	}
}

func rewriteLoopbackHostPort(address string, override string) string {
	host, port, err := net.SplitHostPort(address)
	if err != nil || !isLoopbackHost(host) {
		return address
	}

	return net.JoinHostPort(override, port)
}

func rewriteLoopbackURL(rawURL string, override string) string {
	parsedURL, err := url.Parse(rawURL)
	if err != nil || parsedURL.Host == "" {
		return rawURL
	}

	host, port, err := net.SplitHostPort(parsedURL.Host)
	if err != nil {
		host = parsedURL.Host
	}

	if !isLoopbackHost(host) {
		return rawURL
	}

	if port == "" {
		parsedURL.Host = override
	} else {
		parsedURL.Host = net.JoinHostPort(override, port)
	}

	return parsedURL.String()
}

func isLoopbackHost(host string) bool {
	normalized := strings.Trim(strings.ToLower(host), "[]")
	return normalized == "localhost" || normalized == "::1" || normalized == "0:0:0:0:0:0:0:1" || strings.HasPrefix(normalized, "127.")
}

func detectWSLHostIP() string {
	osRelease, err := os.ReadFile("/proc/sys/kernel/osrelease")
	if err != nil {
		return ""
	}

	normalizedRelease := strings.ToLower(string(osRelease))
	if !strings.Contains(normalizedRelease, "microsoft") && !strings.Contains(normalizedRelease, "wsl") {
		return ""
	}

	if ip := detectWSLDefaultGatewayIP(); ip != "" {
		return ip
	}

	return detectWSLNameserverIP()
}

func detectWSLDefaultGatewayIP() string {
	route, err := os.ReadFile("/proc/net/route")
	if err != nil {
		return ""
	}

	return parseWSLDefaultGatewayIP(string(route))
}

func parseWSLDefaultGatewayIP(route string) string {
	for _, line := range strings.Split(route, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[0] == "lo" || fields[1] != "00000000" {
			continue
		}

		gateway, err := strconv.ParseUint(fields[2], 16, 32)
		if err != nil || gateway == 0 {
			continue
		}

		ip := net.IPv4(byte(gateway), byte(gateway>>8), byte(gateway>>16), byte(gateway>>24))
		if ip == nil || ip.IsLoopback() {
			continue
		}

		return ip.String()
	}

	return ""
}

func detectWSLNameserverIP() string {
	resolvConf, err := os.ReadFile("/etc/resolv.conf")
	if err != nil {
		return ""
	}

	for _, line := range strings.Split(string(resolvConf), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != "nameserver" {
			continue
		}

		ip := net.ParseIP(fields[1])
		if ip == nil || ip.IsLoopback() {
			continue
		}

		return ip.String()
	}

	return ""
}
