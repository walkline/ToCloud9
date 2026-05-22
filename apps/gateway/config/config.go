package config

import (
	"strconv"

	"github.com/walkline/ToCloud9/apps/gateway"
	"github.com/walkline/ToCloud9/shared/config"
)

// Config is config of application
type Config struct {
	config.Logging `yaml:"logging"`

	// Port is port that would be used to listen the game client
	Port string `yaml:"port" env:"PORT" env-default:"8085"`

	// RealmID is id of realm that gateway works with
	RealmID int `yaml:"realmId" env:"REALM_ID" env-default:"1"`

	// AuthDBConnection is connection string to the auth database
	AuthDBConnection string `yaml:"authDB" env:"AUTH_DB_CONNECTION" env-default:"trinity:trinity@tcp(127.0.0.1:3306)/auth"`

	// DBSchemaType is the schema type of database. Supported values: "tc", "ac".
	DBSchemaType string `yaml:"dbSchemaType" env:"DB_SCHEMA_TYPE" env-default:"tc"`

	// HealthCheckPort is port that would be used to listen for health checks
	HealthCheckPort string `yaml:"healthCheckPort" env:"HEALTH_CHECK_PORT" env-default:"8900"`

	// PreferredHostname is referred host name that will be used to connect from game client
	PreferredHostname string `yaml:"preferredHostname" env:"PREFERRED_HOSTNAME" env-default:"localhost"`

	// CharServiceAddress is address of characters service
	CharServiceAddress string `yaml:"charactersServiceAddress" env:"CHAR_SERVICE_ADDRESS" env-default:"localhost:8991"`

	// ServersRegistryServiceAddress is address of servers registry service
	ServersRegistryServiceAddress string `yaml:"serversRegistryServiceAddress" env:"SERVERS_REGISTRY_SERVICE_ADDRESS" env-default:"localhost:8999"`

	// ChatServiceAddress is address of chat service
	ChatServiceAddress string `yaml:"chatServiceAddress" env:"CHAT_SERVICE_ADDRESS" env-default:"localhost:8992"`

	// MatchmakingServiceAddress is address of matchmaking service.
	MatchmakingServiceAddress string `yaml:"matchmakingServiceAddress" env:"MATCHMAKING_SERVICE_ADDRESS" env-default:"localhost:8994"`

	// GuildsServiceAddress is address of guilds service
	GuildsServiceAddress string `yaml:"guildsServiceAddress" env:"GUILDS_SERVICE_ADDRESS" env-default:"localhost:8995"`

	// MailServiceAddress is address of mail service
	MailServiceAddress string `yaml:"mailServiceAddress" env:"MAIL_SERVICE_ADDRESS" env-default:"localhost:8997"`

	// GroupServiceAddress is address of group service
	GroupServiceAddress string `yaml:"groupServiceAddress" env:"GROUP_SERVICE_ADDRESS" env-default:"localhost:8998"`

	// NatsURL is nats connection url
	NatsURL string `yaml:"natsUrl" env:"NATS_URL" env-default:"nats://nats:4222"`

	// PacketProcessTimeoutSecs is the time given to process single opcode (if it's not forwarded to game server).
	PacketProcessTimeoutSecs uint32 `yaml:"packetProcessTimeoutSecs" env:"PACKET_PROCESS_TIMEOUT_SECS" env-default:"20"`

	// WorldAuthAttemptTimeoutMs is the per-attempt guard while waiting for worldserver SMSG_AUTH_RESPONSE.
	WorldAuthAttemptTimeoutMs uint32 `yaml:"worldAuthAttemptTimeoutMs" env:"WORLD_AUTH_ATTEMPT_TIMEOUT_MS" env-default:"5000"`

	// WorldAuthSessionReadyDelayMs is the post-auth delay before CMSG_PLAYER_LOGIN is sent to worldserver.
	WorldAuthSessionReadyDelayMs uint32 `yaml:"worldAuthSessionReadyDelayMs" env:"WORLD_AUTH_SESSION_READY_DELAY_MS" env-default:"300"`

	// WorldserverConnectRetryWaitMs is the initial wait between retryable worldserver login attempts.
	WorldserverConnectRetryWaitMs uint32 `yaml:"worldserverConnectRetryWaitMs" env:"WORLDSERVER_CONNECT_RETRY_WAIT_MS" env-default:"200"`

	// WorldserverConnectRetryMaxWaitMs caps exponential backoff between retryable worldserver login attempts.
	WorldserverConnectRetryMaxWaitMs uint32 `yaml:"worldserverConnectRetryMaxWaitMs" env:"WORLDSERVER_CONNECT_RETRY_MAX_WAIT_MS" env-default:"2000"`

	// AuthSessionKeyRefreshDelayMs is the delay between auth DB session-key refresh attempts after a digest mismatch.
	AuthSessionKeyRefreshDelayMs uint32 `yaml:"authSessionKeyRefreshDelayMs" env:"AUTH_SESSION_KEY_REFRESH_DELAY_MS" env-default:"250"`

	// AuthSessionKeyRefreshAttempts is the number of auth DB session-key refresh attempts after a digest mismatch.
	AuthSessionKeyRefreshAttempts uint32 `yaml:"authSessionKeyRefreshAttempts" env:"AUTH_SESSION_KEY_REFRESH_ATTEMPTS" env-default:"120"`

	// ShowGameserverConnChangeToClient when enabled sends chat system message to the player with information about connection change.
	ShowGameserverConnChangeToClient bool `yaml:"showGameserverConnChangeToClient" env:"SHOW_GAMESERVER_CONN_CHANGE_TO_CLIENT" env-default:"true"`

	// AllowTwoSideInteractionGuild mirrors AzerothCore's AllowTwoSide.Interaction.Guild for gateway-owned guild operations.
	AllowTwoSideInteractionGuild bool `yaml:"allowTwoSideInteractionGuild" env:"ALLOW_TWO_SIDE_INTERACTION_GUILD" env-default:"false"`

	// AllowTwoSideInteractionChannel mirrors AzerothCore's AllowTwoSide.Interaction.Channel for gateway-owned channel operations.
	AllowTwoSideInteractionChannel bool `yaml:"allowTwoSideInteractionChannel" env:"ALLOW_TWO_SIDE_INTERACTION_CHANNEL" env-default:"false"`

	// AllowTwoSideInteractionArena mirrors AzerothCore's AllowTwoSide.Interaction.Arena for gateway-owned arena-team operations.
	AllowTwoSideInteractionArena bool `yaml:"allowTwoSideInteractionArena" env:"ALLOW_TWO_SIDE_INTERACTION_ARENA" env-default:"false"`

	// MaxPlayerLevel mirrors AzerothCore's MaxPlayerLevel for gateway-owned arena-team invitation checks.
	MaxPlayerLevel uint32 `yaml:"maxPlayerLevel" env:"MAX_PLAYER_LEVEL" env-default:"80"`

	// ArenaCurrentSeason mirrors AzerothCore's ArenaSeasonMgr for gateway-owned arena charter creation.
	ArenaCurrentSeason uint32 `yaml:"arenaCurrentSeason" env:"ARENA_CURRENT_SEASON" env-default:"8"`

	// LegacyArenaStartRating mirrors AzerothCore's Arena.LegacyStartRating.
	LegacyArenaStartRating uint32 `yaml:"legacyArenaStartRating" env:"LEGACY_ARENA_START_RATING" env-default:"1500"`

	// ArenaStartRating mirrors AzerothCore's Arena.StartRating.
	ArenaStartRating uint32 `yaml:"arenaStartRating" env:"ARENA_START_RATING" env-default:"0"`

	// ArenaStartPersonalRating mirrors AzerothCore's Arena.StartPersonalRating.
	ArenaStartPersonalRating uint32 `yaml:"arenaStartPersonalRating" env:"ARENA_START_PERSONAL_RATING" env-default:"0"`

	// ArenaStartMatchmakerRating mirrors AzerothCore's Arena.StartMatchmakerRating.
	ArenaStartMatchmakerRating uint32 `yaml:"arenaStartMatchmakerRating" env:"ARENA_START_MATCHMAKER_RATING" env-default:"1500"`
}

func (c Config) PortInt() (p int) {
	p, _ = strconv.Atoi(c.Port)
	return
}

func (c Config) HealthCheckPortInt() (p int) {
	p, _ = strconv.Atoi(c.HealthCheckPort)
	return
}

// LoadConfig loads config from env variables
func LoadConfig() (*Config, error) {
	var c struct {
		Root Config `yaml:"gateway"`
	}

	err := config.LoadConfig(&c)
	if err != nil {
		return nil, err
	}

	gateway.RealmID = uint32(c.Root.RealmID)
	gateway.AllowTwoSideInteractionGuild = c.Root.AllowTwoSideInteractionGuild
	gateway.AllowTwoSideInteractionChannel = c.Root.AllowTwoSideInteractionChannel
	gateway.AllowTwoSideInteractionArena = c.Root.AllowTwoSideInteractionArena
	gateway.MaxPlayerLevel = c.Root.MaxPlayerLevel
	gateway.ArenaCurrentSeason = c.Root.ArenaCurrentSeason
	gateway.LegacyArenaStartRating = c.Root.LegacyArenaStartRating
	gateway.ArenaStartRating = c.Root.ArenaStartRating
	gateway.ArenaStartPersonalRating = c.Root.ArenaStartPersonalRating
	gateway.ArenaStartMatchmakerRating = c.Root.ArenaStartMatchmakerRating

	return &c.Root, nil
}
