package repo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"strconv"
	"strings"

	redis "github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

var aliasModifiers = [...]string{"red", "blue", "gold", "dark", "snow", "lime", "ice", "fire", "cold", "holy", "void", "ash"}
var aliasRaidBosses = [...]string{
	// World of Warcraft
	"high-priestess-jeklik", "high-priest-venoxis", "high-priest-thekal",
	"high-priestess-arlokk", "bloodlord-mandokir", "renataki", "wushoolay",
	"hakkar-the-soulflayer", "kurinnaxx", "general-rajaxx", "moam",
	"buru-the-gorger", "ayamiss-the-hunter", "ossirian-the-unscarred",
	"lucifron", "magmadar", "gehennas", "garr", "baron-geddon", "shazzrah",
	"sulfuron-harbinger", "golemagg-the-incinerator", "majordomo-executus",
	"ragnaros", "onyxia",
	"razorgore-the-untamed", "vaelastrasz-the-corrupt", "broodlord-lashlayer",
	"firemaw", "ebonroc", "flamegor", "chromaggus", "nefarian",
	"the-prophet-skeram", "lord-kri", "princess-yauj", "vem",
	"battleguard-sartura", "fankriss-the-unyielding", "viscidus",
	"princess-huhuran", "ouro", "grand-widow-faerlina", "maexxna",
	"noth-the-plaguebringer", "heigan-the-unclean", "loatheb",
	"instructor-razuvious", "gothik-the-harvester", "lady-blaumeux",
	"baron-rivendare", "sir-zeliek", "patchwerk", "grobbulus", "gluth",
	"thaddius", "sapphiron", "azuregos", "lord-kazzak", "emeriss", "lethon",
	"taerar", "ysondre",

	// The Burning Crusade
	"attumen-the-huntsman", "moroes", "maiden-of-virtue", "big-bad-wolf",
	"julianne", "romulo", "the-crone", "the-curator", "terestian-illhoof",
	"shade-of-aran", "netherspite", "prince-malchezaar", "nightbane",
	"high-king-maulgar", "gruul-the-dragonkiller", "magtheridon",
	"hydross-the-unstable", "the-lurker-below", "leotheras-the-blind",
	"fathom-lord-karathress", "morogrim-tidewalker", "lady-vashj",
	"void-reaver", "high-astromancer-solarian", "rage-winterchill",
	"anetheron", "kazrogal", "azgalor", "archimonde", "supremus",
	"shade-of-akama", "teron-gorefiend", "gurtogg-bloodboil",
	"reliquary-of-souls", "mother-shahraz", "illidari-council",
	"illidan-stormrage", "nalorakk", "halazzi", "hex-lord-malacrass",
	"kalecgos", "sathrovarr-the-corruptor", "brutallus", "felmyst",
	"grand-warlock-alythess", "lady-sacrolash", "entropius",
	"doom-lord-kazzak", "doomwalker",

	// Wrath of the Lich King
	"tenebron", "shadron", "vesperon", "sartharion", "malygos",
	"archavon-the-stone-watcher", "emalon-the-storm-watcher",
	"koralon-the-flame-watcher", "toravon-the-ice-watcher",
	"flame-leviathan", "ignis-the-furnace-master", "razorscale",
	"steelbreaker", "runemaster-molgeim", "stormcaller-brundir", "kologarn",
	"auriaya", "hodir", "thorim", "freya", "mimiron", "general-vezax",
	"algalon-the-observer", "gormok-the-impaler", "acidmaw", "dreadscale",
	"icehowl", "lord-jaraxxus", "fjola-lightbane", "eydis-darkbane",
	"lord-marrowgar", "lady-deathwhisper", "deathbringer-saurfang",
	"festergut", "rotface", "professor-putricide", "prince-keleseth",
	"prince-taldaram", "prince-valanar", "valithria-dreamwalker",
	"sindragosa", "the-lich-king", "saviana-ragefire",
	"baltharus-the-warborn", "general-zarithrian", "halion",
}

type gameServerRedisRepo struct {
	rdb *redis.Client
}

func NewGameServerRedisRepo(rdb *redis.Client) GameServerRepo {
	return &gameServerRedisRepo{rdb: rdb}
}

func (g *gameServerRedisRepo) Upsert(ctx context.Context, server *GameServer) error {
	server.Address = strings.ToLower(server.Address)
	if server.ID == "" {
		server.ID = g.generateID(server.Address)
	}
	if server.Alias == "" {
		server.Alias = g.generateAlias(server.Address)
	}

	d, err := json.Marshal(server)
	if err != nil {
		return err
	}

	key := g.key(server.ID)
	status := g.rdb.Set(ctx, key, d, 0)
	if status.Err() != nil {
		return status.Err()
	}

	res := g.rdb.SAdd(ctx, g.realmIndexKey(server.RealmID, server.IsCrossRealm), key)
	if res.Err() != nil {
		g.rdb.Del(ctx, key)
		return res.Err()
	}

	return nil
}

func (g *gameServerRedisRepo) Update(ctx context.Context, id string, f func(*GameServer) *GameServer) error {
	key := g.key(id)
	for attempt := 0; attempt < 5; attempt++ {
		err := g.rdb.Watch(ctx, func(tx *redis.Tx) error {
			value, err := tx.Get(ctx, key).Bytes()
			if err != nil {
				return err
			}
			server := &GameServer{}
			if err := json.Unmarshal(value, server); err != nil {
				return err
			}
			updated := f(server)
			data, err := json.Marshal(updated)
			if err != nil {
				return err
			}
			_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
				pipe.Set(ctx, g.key(updated.ID), data, 0)
				return nil
			})
			return err
		}, key)
		if !errors.Is(err, redis.TxFailedErr) {
			return err
		}
	}
	return redis.TxFailedErr
}

func (g *gameServerRedisRepo) Remove(ctx context.Context, id string) error {
	key := g.key(id)
	res := g.rdb.Get(ctx, key)
	if res.Err() != nil {
		if errors.Is(res.Err(), redis.Nil) {
			return nil
		}

		return res.Err()
	}

	v := &GameServer{}
	err := json.Unmarshal([]byte(res.Val()), v)
	if err != nil {
		return err
	}

	delRes := g.rdb.SRem(ctx, g.realmIndexKey(v.RealmID, v.IsCrossRealm), key)
	if delRes.Err() != nil {
		return delRes.Err()
	}

	delRes = g.rdb.Del(ctx, key)
	return delRes.Err()
}

func (g *gameServerRedisRepo) ListByRealm(ctx context.Context, realmID uint32) ([]GameServer, error) {
	return g.listForRealmOrCrossRealm(ctx, realmID, false)
}

func (g *gameServerRedisRepo) ListOfCrossRealms(ctx context.Context) ([]GameServer, error) {
	return g.listForRealmOrCrossRealm(ctx, 0, true)
}

func (g *gameServerRedisRepo) ListAll(ctx context.Context) ([]GameServer, error) {
	pattern := "ws:*"

	var cursor uint64
	var keys []string

	// Use SCAN to find all keys matching the pattern
	for {
		// Scan with the current cursor value
		var newKeys []string
		var err error
		newKeys, cursor, err = g.rdb.Scan(ctx, cursor, pattern, 10).Result()
		if err != nil {
			return nil, err
		}

		keys = append(keys, newKeys...)

		if cursor == 0 {
			break
		}
	}

	// Retrieve values for all matching keys
	result := make([]GameServer, 0, len(keys))
	for _, key := range keys {
		value, err := g.rdb.Get(ctx, key).Result()
		if err != nil {
			continue
		}

		obj := &GameServer{}
		if err := json.Unmarshal([]byte(value), obj); err != nil {
			return nil, err
		}
		result = append(result, *obj)
	}

	return result, nil
}

func (g *gameServerRedisRepo) listForRealmOrCrossRealm(ctx context.Context, realmID uint32, isCrossRealm bool) ([]GameServer, error) {
	res := g.rdb.SMembers(ctx, g.realmIndexKey(realmID, isCrossRealm))
	if res.Err() != nil {
		return nil, res.Err()
	}

	if len(res.Val()) == 0 {
		return []GameServer{}, nil
	}

	mGetRes := g.rdb.MGet(ctx, res.Val()...)
	if mGetRes.Err() != nil {
		return nil, mGetRes.Err()
	}

	resInterface := mGetRes.Val()
	result := make([]GameServer, 0, len(resInterface))
	for i := range resInterface {
		if resInterface[i] == nil {
			log.Warn().Str("key", res.Val()[i]).Msg("fetched nil game server from set")
			continue
		}
		obj := &GameServer{}
		if err := json.Unmarshal([]byte(resInterface[i].(string)), obj); err != nil {
			return nil, err
		}
		result = append(result, *obj)
	}

	return result, nil
}

func (g *gameServerRedisRepo) One(ctx context.Context, id string) (*GameServer, error) {
	getRes := g.rdb.Get(ctx, g.key(id))
	if getRes.Err() != nil {
		if errors.Is(getRes.Err(), redis.Nil) {
			return nil, nil
		}
		return nil, getRes.Err()
	}

	resBytes, err := getRes.Bytes()
	if err != nil {
		return nil, err
	}

	obj := &GameServer{}
	if err = json.Unmarshal(resBytes, obj); err != nil {
		return nil, err
	}

	return obj, nil
}

func (g *gameServerRedisRepo) realmIndexKey(realmID uint32, isCrossRealm bool) string {
	if isCrossRealm {
		return "crossrealm:wss"
	}
	return fmt.Sprintf("realm:%d:wss", realmID)
}

func (g *gameServerRedisRepo) key(id string) string {
	return fmt.Sprintf("ws:%s", id)
}

func (g *gameServerRedisRepo) generateID(address string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(address))
	return strconv.FormatUint(uint64(h.Sum32()), 10)
}

func (g *gameServerRedisRepo) generateAlias(address string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(strings.ToLower(address)))
	sum := h.Sum32()
	modifier := aliasModifiers[sum%uint32(len(aliasModifiers))]
	boss := aliasRaidBosses[(sum/uint32(len(aliasModifiers)))%uint32(len(aliasRaidBosses))]
	code := sum / uint32(len(aliasModifiers)*len(aliasRaidBosses)) % (36 * 36)
	return fmt.Sprintf("%s-%s-%02s", modifier, boss, strconv.FormatUint(uint64(code), 36))
}
