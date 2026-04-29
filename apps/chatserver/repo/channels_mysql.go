package repo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	shrepo "github.com/walkline/ToCloud9/shared/repo"
)

type ChannelsPreparedStatements uint32

func (s ChannelsPreparedStatements) Stmt() string {
	switch s {
	case StmtCreateChannel:
		return "INSERT INTO channels (name, team, announce, ownership, password, lastUsed) VALUES (?, ?, ?, ?, ?, ?)"
	case StmtGetChannelByID:
		return "SELECT channelId, name, team, announce, ownership, password, lastUsed FROM channels WHERE channelId = ?"
	case StmtGetChannelByName:
		return "SELECT channelId, name, team, announce, ownership, password, lastUsed FROM channels WHERE name = ? AND team = ?"
	case StmtGetAllChannels:
		return "SELECT channelId, name, team, announce, ownership, password, lastUsed FROM channels"
	case StmtUpdateChannel:
		return "UPDATE channels SET announce = ?, ownership = ?, password = ?, lastUsed = ? WHERE channelId = ?"
	case StmtDeleteChannel:
		return "DELETE FROM channels WHERE channelId = ?"
	case StmtUpdateChannelLastUsed:
		return "UPDATE channels SET lastUsed = ? WHERE channelId = ?"
	case StmtCleanOldChannels:
		return "DELETE FROM channels WHERE lastUsed < ?"
	case StmtGetChannelRights:
		return "SELECT name, flags, speakdelay, joinmessage, delaymessage, moderators FROM channels_rights WHERE name = ?"
	case StmtSetChannelRights:
		return "INSERT INTO channels_rights (name, flags, speakdelay, joinmessage, delaymessage, moderators) VALUES (?, ?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE flags = VALUES(flags), speakdelay = VALUES(speakdelay), joinmessage = VALUES(joinmessage), delaymessage = VALUES(delaymessage), moderators = VALUES(moderators)"
	case StmtAddChannelBan:
		return "INSERT INTO channels_bans (channelId, playerGUID, banTime) VALUES (?, ?, ?) ON DUPLICATE KEY UPDATE banTime = VALUES(banTime)"
	case StmtRemoveChannelBan:
		return "DELETE FROM channels_bans WHERE channelId = ? AND playerGUID = ?"
	case StmtGetChannelBans:
		return "SELECT channelId, playerGUID, banTime FROM channels_bans WHERE channelId = ?"
	case StmtIsPlayerBanned:
		return "SELECT COUNT(*) FROM channels_bans WHERE channelId = ? AND playerGUID = ? AND banTime > ?"
	case StmtCleanExpiredBans:
		return "DELETE FROM channels_bans WHERE banTime <= ?"
	case StmtSaveChannelMember:
		return "INSERT INTO channels_members (channelId, playerGUID, playerName, flags, joinedAt) VALUES (?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE playerName = VALUES(playerName), flags = VALUES(flags)"
	case StmtRemoveChannelMember:
		return "DELETE FROM channels_members WHERE channelId = ? AND playerGUID = ?"
	case StmtLoadChannelMembers:
		return "SELECT playerGUID, playerName, flags, joinedAt FROM channels_members WHERE channelId = ?"
	case StmtUpdateMemberFlags:
		return "UPDATE channels_members SET flags = ? WHERE channelId = ? AND playerGUID = ?"
	}

	panic(fmt.Errorf("unk stmt %d", s))
}

func (s ChannelsPreparedStatements) ID() uint32 {
	return uint32(s)
}

const (
	StmtCreateChannel ChannelsPreparedStatements = iota
	StmtGetChannelByID
	StmtGetChannelByName
	StmtGetAllChannels
	StmtUpdateChannel
	StmtDeleteChannel
	StmtUpdateChannelLastUsed
	StmtCleanOldChannels
	StmtGetChannelRights
	StmtSetChannelRights
	StmtAddChannelBan
	StmtRemoveChannelBan
	StmtGetChannelBans
	StmtIsPlayerBanned
	StmtCleanExpiredBans
	StmtSaveChannelMember
	StmtRemoveChannelMember
	StmtLoadChannelMembers
	StmtUpdateMemberFlags
)

type ChannelsMYSQL struct {
	db shrepo.CharactersDB
}

func NewChannelsMYSQL(db shrepo.CharactersDB) ChannelsRepo {
	// Register prepared statements
	db.SetPreparedStatement(StmtCreateChannel)
	db.SetPreparedStatement(StmtGetChannelByID)
	db.SetPreparedStatement(StmtGetChannelByName)
	db.SetPreparedStatement(StmtGetAllChannels)
	db.SetPreparedStatement(StmtUpdateChannel)
	db.SetPreparedStatement(StmtDeleteChannel)
	db.SetPreparedStatement(StmtUpdateChannelLastUsed)
	db.SetPreparedStatement(StmtCleanOldChannels)
	db.SetPreparedStatement(StmtGetChannelRights)
	db.SetPreparedStatement(StmtSetChannelRights)
	db.SetPreparedStatement(StmtAddChannelBan)
	db.SetPreparedStatement(StmtRemoveChannelBan)
	db.SetPreparedStatement(StmtGetChannelBans)
	db.SetPreparedStatement(StmtIsPlayerBanned)
	db.SetPreparedStatement(StmtCleanExpiredBans)
	db.SetPreparedStatement(StmtSaveChannelMember)
	db.SetPreparedStatement(StmtRemoveChannelMember)
	db.SetPreparedStatement(StmtLoadChannelMembers)
	db.SetPreparedStatement(StmtUpdateMemberFlags)

	return &ChannelsMYSQL{db: db}
}

func (r *ChannelsMYSQL) CreateChannel(ctx context.Context, realmID uint32, channel *Channel) error {
	result, err := r.db.PreparedStatement(realmID, StmtCreateChannel).ExecContext(ctx,
		channel.Name,
		channel.Team,
		channel.Announce,
		channel.Ownership,
		channel.Password,
		channel.LastUsed.Unix(),
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	channel.ChannelID = uint32(id)
	return nil
}

func (r *ChannelsMYSQL) GetChannelByID(ctx context.Context, realmID uint32, channelID uint32) (*Channel, error) {
	row := r.db.PreparedStatement(realmID, StmtGetChannelByID).QueryRowContext(ctx, channelID)

	channel := &Channel{}
	var lastUsed int64
	var password sql.NullString
	err := row.Scan(
		&channel.ChannelID,
		&channel.Name,
		&channel.Team,
		&channel.Announce,
		&channel.Ownership,
		&password,
		&lastUsed,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	channel.Password = password.String
	channel.LastUsed = time.Unix(lastUsed, 0)
	return channel, nil
}

func (r *ChannelsMYSQL) GetChannelByName(ctx context.Context, realmID uint32, name string, team uint32) (*Channel, error) {
	row := r.db.PreparedStatement(realmID, StmtGetChannelByName).QueryRowContext(ctx, name, team)

	channel := &Channel{}
	var lastUsed int64
	var password sql.NullString
	err := row.Scan(
		&channel.ChannelID,
		&channel.Name,
		&channel.Team,
		&channel.Announce,
		&channel.Ownership,
		&password,
		&lastUsed,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	channel.Password = password.String
	channel.LastUsed = time.Unix(lastUsed, 0)
	return channel, nil
}

func (r *ChannelsMYSQL) GetAllChannels(ctx context.Context, realmID uint32) ([]Channel, error) {
	rows, err := r.db.PreparedStatement(realmID, StmtGetAllChannels).QueryContext(ctx)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var channels []Channel
	for rows.Next() {
		var channel Channel
		var lastUsed int64
		var password sql.NullString
		if err := rows.Scan(
			&channel.ChannelID,
			&channel.Name,
			&channel.Team,
			&channel.Announce,
			&channel.Ownership,
			&password,
			&lastUsed,
		); err != nil {
			return nil, err
		}
		channel.Password = password.String
		channel.LastUsed = time.Unix(lastUsed, 0)
		channels = append(channels, channel)
	}

	return channels, rows.Err()
}

func (r *ChannelsMYSQL) UpdateChannel(ctx context.Context, realmID uint32, channel *Channel) error {
	_, err := r.db.PreparedStatement(realmID, StmtUpdateChannel).ExecContext(ctx,
		channel.Announce,
		channel.Ownership,
		channel.Password,
		channel.LastUsed.Unix(),
		channel.ChannelID,
	)
	return err
}

func (r *ChannelsMYSQL) DeleteChannel(ctx context.Context, realmID uint32, channelID uint32) error {
	_, err := r.db.PreparedStatement(realmID, StmtDeleteChannel).ExecContext(ctx, channelID)
	return err
}

func (r *ChannelsMYSQL) UpdateChannelLastUsed(ctx context.Context, realmID uint32, channelID uint32) error {
	_, err := r.db.PreparedStatement(realmID, StmtUpdateChannelLastUsed).ExecContext(ctx, time.Now().Unix(), channelID)
	return err
}

func (r *ChannelsMYSQL) CleanOldChannels(ctx context.Context, realmID uint32, olderThan time.Time) error {
	_, err := r.db.PreparedStatement(realmID, StmtCleanOldChannels).ExecContext(ctx, olderThan.Unix())
	return err
}

func (r *ChannelsMYSQL) GetChannelRights(ctx context.Context, realmID uint32, name string) (*ChannelRights, error) {
	row := r.db.PreparedStatement(realmID, StmtGetChannelRights).QueryRowContext(ctx, name)

	rights := &ChannelRights{}
	var moderatorsStr sql.NullString
	err := row.Scan(
		&rights.Name,
		&rights.Flags,
		&rights.SpeakDelay,
		&rights.JoinMessage,
		&rights.DelayMessage,
		&moderatorsStr,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// Parse moderators string (comma-separated GUIDs)
	if moderatorsStr.Valid && moderatorsStr.String != "" {
		parts := strings.Split(moderatorsStr.String, ",")
		rights.Moderators = make([]uint32, 0, len(parts))
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}

			var guid uint32
			// Sscanf is safe from injection, but validate the result
			n, err := fmt.Sscanf(part, "%d", &guid)
			if err != nil || n != 1 || guid == 0 {
				// Skip malformed or zero GUIDs
				continue
			}
			rights.Moderators = append(rights.Moderators, guid)
		}
	}

	return rights, nil
}

func (r *ChannelsMYSQL) SetChannelRights(ctx context.Context, realmID uint32, rights *ChannelRights) error {
	// Validate and convert moderators to comma-separated string
	moderatorsStr := ""
	if len(rights.Moderators) > 0 {
		// Validate that all moderator GUIDs are valid (non-zero)
		parts := make([]string, 0, len(rights.Moderators))
		for _, guid := range rights.Moderators {
			if guid == 0 {
				// Skip invalid GUIDs - don't store them
				continue
			}
			parts = append(parts, fmt.Sprintf("%d", guid))
		}
		moderatorsStr = strings.Join(parts, ",")
	}

	_, err := r.db.PreparedStatement(realmID, StmtSetChannelRights).ExecContext(ctx,
		rights.Name,
		rights.Flags,
		rights.SpeakDelay,
		rights.JoinMessage,
		rights.DelayMessage,
		moderatorsStr,
	)
	return err
}

func (r *ChannelsMYSQL) AddChannelBan(ctx context.Context, realmID uint32, ban *ChannelBan) error {
	_, err := r.db.PreparedStatement(realmID, StmtAddChannelBan).ExecContext(ctx,
		ban.ChannelID,
		ban.PlayerGUID,
		ban.BanTime.Unix(),
	)
	return err
}

func (r *ChannelsMYSQL) RemoveChannelBan(ctx context.Context, realmID uint32, channelID uint32, playerGUID uint64) error {
	_, err := r.db.PreparedStatement(realmID, StmtRemoveChannelBan).ExecContext(ctx, channelID, playerGUID)
	return err
}

func (r *ChannelsMYSQL) GetChannelBans(ctx context.Context, realmID uint32, channelID uint32) ([]ChannelBan, error) {
	rows, err := r.db.PreparedStatement(realmID, StmtGetChannelBans).QueryContext(ctx, channelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bans []ChannelBan
	for rows.Next() {
		var ban ChannelBan
		var banTime int64
		if err := rows.Scan(&ban.ChannelID, &ban.PlayerGUID, &banTime); err != nil {
			return nil, err
		}
		ban.BanTime = time.Unix(banTime, 0)
		bans = append(bans, ban)
	}

	return bans, rows.Err()
}

func (r *ChannelsMYSQL) IsPlayerBanned(ctx context.Context, realmID uint32, channelID uint32, playerGUID uint64) (bool, error) {
	row := r.db.PreparedStatement(realmID, StmtIsPlayerBanned).QueryRowContext(ctx, channelID, playerGUID, time.Now().Unix())

	var count int
	if err := row.Scan(&count); err != nil {
		return false, err
	}

	return count > 0, nil
}

func (r *ChannelsMYSQL) CleanExpiredBans(ctx context.Context, realmID uint32) error {
	_, err := r.db.PreparedStatement(realmID, StmtCleanExpiredBans).ExecContext(ctx, time.Now().Unix())
	return err
}

func (r *ChannelsMYSQL) SaveChannelMember(ctx context.Context, realmID uint32, channelID uint32, member *ChannelMember) error {
	_, err := r.db.PreparedStatement(realmID, StmtSaveChannelMember).ExecContext(ctx,
		channelID,
		member.PlayerGUID,
		member.PlayerName,
		member.Flags,
		time.Now().Unix(),
	)
	return err
}

func (r *ChannelsMYSQL) RemoveChannelMember(ctx context.Context, realmID uint32, channelID uint32, playerGUID uint64) error {
	_, err := r.db.PreparedStatement(realmID, StmtRemoveChannelMember).ExecContext(ctx, channelID, playerGUID)
	return err
}

func (r *ChannelsMYSQL) LoadChannelMembers(ctx context.Context, realmID uint32, channelID uint32) ([]ChannelMember, error) {
	rows, err := r.db.PreparedStatement(realmID, StmtLoadChannelMembers).QueryContext(ctx, channelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []ChannelMember
	for rows.Next() {
		var member ChannelMember
		var joinedAt int64
		if err := rows.Scan(&member.PlayerGUID, &member.PlayerName, &member.Flags, &joinedAt); err != nil {
			return nil, err
		}
		members = append(members, member)
	}

	return members, rows.Err()
}

func (r *ChannelsMYSQL) UpdateMemberFlags(ctx context.Context, realmID uint32, channelID uint32, playerGUID uint64, flags uint8) error {
	_, err := r.db.PreparedStatement(realmID, StmtUpdateMemberFlags).ExecContext(ctx, flags, channelID, playerGUID)
	return err
}
