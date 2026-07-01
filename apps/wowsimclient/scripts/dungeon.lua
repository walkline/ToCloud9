-- dungeon.lua - Group dungeon behavior for warrior bots.
-- This script handles dungeon combat in a group setting, suitable for
-- Ragefire Chasm and The Deadmines. Each bot follows the group leader,
-- fights mobs, and progresses through the dungeon.

-- Warrior spell IDs
local HEROIC_STRIKE = 78
local REND = 772
local THUNDER_CLAP = 6343
local CHARGE = 100
local EXECUTE = 5308
local BATTLE_SHOUT = 2457
local SUNDER_ARMOR = 7386
local CLEAVE = 845
local DEMORALIZING_SHOUT = 1160
local TAUNT = 355
local VICTORY_RUSH = 34428

-- Configuration
local FOLLOW_DISTANCE = 5
local ENGAGE_DISTANCE = 30
local MELEE_RANGE = 5
local PULL_RANGE = 25

-- State
local role = "dps"     -- "tank", "dps" - determined by group position
local leader_guid = 0
local party_members = {}
local bosses_killed = 0

-- Set the bot's role based on its position (first bot is tank)
local function determine_role()
    local players = bot.get_nearby_players(100)
    if #players == 0 then
        role = "tank" -- Solo, be tank
    else
        -- Simple heuristic: lowest-numbered character is tank
        role = "dps"
    end
    bot.log("Role: " .. role)
end

-- Find the nearest enemy
local function find_nearest_enemy(max_dist)
    local units = bot.get_nearby_units(max_dist or ENGAGE_DISTANCE)
    local best = nil
    local best_dist = max_dist or ENGAGE_DISTANCE

    for _, unit in ipairs(units) do
        if unit.is_alive and unit.distance < best_dist then
            best = unit
            best_dist = unit.distance
        end
    end

    return best
end

-- Tank combat rotation
local function tank_rotation(target_guid)
    local target = bot.get_unit(target_guid)
    if not target then return end

    local target_hp_pct = target.health / math.max(target.max_health, 1) * 100

    -- Execute at low health
    if target_hp_pct < 20 and bot.is_spell_ready(EXECUTE) then
        bot.cast_spell(EXECUTE, target_guid)
        return
    end

    -- Sunder Armor for threat
    if bot.is_spell_ready(SUNDER_ARMOR) then
        bot.cast_spell(SUNDER_ARMOR, target_guid)
        return
    end

    -- Thunder Clap for AoE threat
    if bot.is_spell_ready(THUNDER_CLAP) then
        bot.cast_spell(THUNDER_CLAP, target_guid)
        return
    end

    -- Demoralizing Shout
    if bot.is_spell_ready(DEMORALIZING_SHOUT) then
        bot.cast_spell(DEMORALIZING_SHOUT, 0)
        return
    end

    -- Cleave for multiple targets
    if bot.is_spell_ready(CLEAVE) then
        bot.cast_spell(CLEAVE, target_guid)
        return
    end

    -- Heroic Strike filler
    if bot.is_spell_ready(HEROIC_STRIKE) then
        bot.cast_spell(HEROIC_STRIKE, target_guid)
        return
    end
end

-- DPS combat rotation
local function dps_rotation(target_guid)
    local target = bot.get_unit(target_guid)
    if not target then return end

    local target_hp_pct = target.health / math.max(target.max_health, 1) * 100

    -- Execute at low health
    if target_hp_pct < 20 and bot.is_spell_ready(EXECUTE) then
        bot.cast_spell(EXECUTE, target_guid)
        return
    end

    -- Victory Rush
    if bot.is_spell_ready(VICTORY_RUSH) then
        bot.cast_spell(VICTORY_RUSH, target_guid)
        return
    end

    -- Rend
    if bot.is_spell_ready(REND) then
        bot.cast_spell(REND, target_guid)
        return
    end

    -- Heroic Strike
    if bot.is_spell_ready(HEROIC_STRIKE) then
        bot.cast_spell(HEROIC_STRIKE, target_guid)
        return
    end
end

-- Main tick function
function on_tick()
    -- Check if alive
    if not bot.is_alive() then
        bot.log("Dead in dungeon! Waiting for revive...")
        bot.send_command(".revive")
        return
    end

    -- If in combat, fight
    if bot.in_combat() then
        local target_guid = bot.get_target()
        if target_guid == 0 then
            -- Find a target
            local enemy = find_nearest_enemy(ENGAGE_DISTANCE)
            if enemy then
                target_guid = enemy.guid
                bot.set_target(target_guid)
            else
                return
            end
        end

        local target = bot.get_unit(target_guid)
        if not target or not target.is_alive then
            -- Target dead, loot and find new target
            if target_guid ~= 0 then
                bot.loot_all(target_guid)
                bosses_killed = bosses_killed + 1
                bot.log(string.format("Enemy down! Total kills in dungeon: %d", bosses_killed))
            end
            -- Find new target
            local enemy = find_nearest_enemy(ENGAGE_DISTANCE)
            if enemy then
                bot.set_target(enemy.guid)
                bot.attack(enemy.guid)
            end
            return
        end

        -- Stay in melee range
        if target.distance > MELEE_RANGE then
            bot.move_to(target.x, target.y, target.z)
            return
        end

        bot.stop_moving()
        bot.attack(target_guid)

        if role == "tank" then
            tank_rotation(target_guid)
        else
            dps_rotation(target_guid)
        end
        return
    end

    -- Not in combat
    -- Apply Battle Shout if not active
    if bot.is_spell_ready(BATTLE_SHOUT) then
        bot.cast_spell(BATTLE_SHOUT, 0)
    end

    -- Look for enemies to pull
    local enemy = find_nearest_enemy(PULL_RANGE)
    if enemy then
        bot.set_target(enemy.guid)

        if role == "tank" then
            -- Tank initiates
            if enemy.distance > 8 and enemy.distance < PULL_RANGE and bot.is_spell_ready(CHARGE) then
                bot.cast_spell(CHARGE, enemy.guid)
            elseif enemy.distance > MELEE_RANGE then
                bot.move_to(enemy.x, enemy.y, enemy.z)
            else
                bot.stop_moving()
                bot.attack(enemy.guid)
            end
        else
            -- DPS waits for tank to pull, or attacks if close
            if enemy.distance <= MELEE_RANGE + 2 then
                bot.attack(enemy.guid)
            elseif enemy.distance > MELEE_RANGE then
                bot.move_to(enemy.x, enemy.y, enemy.z)
            end
        end
    end
end

-- Initialize
determine_role()
bot.log(string.format("Dungeon bot initialized (role: %s)", role))