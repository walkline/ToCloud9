-- grind.lua - Basic grinding behavior for a warrior bot.
-- This script is called every tick via on_tick(). It implements a simple
-- combat loop: find nearby enemies, engage, use abilities, loot, repeat.

-- Warrior spell IDs
local HEROIC_STRIKE = 78
local REND = 772
local THUNDER_CLAP = 6343
local CHARGE = 100
local EXECUTE = 5308
local BATTLE_SHOUT = 2457
local VICTORY_RUSH = 34428
local HAMSTRING = 1715
local SUNDER_ARMOR = 7386

-- State
local current_target = 0
local last_loot_time = 0

-- Find the closest attackable unit within range
local function find_target(max_dist)
    local units = bot.get_nearby_units(max_dist or 30)
    local best = nil
    local best_dist = max_dist or 30

    for _, unit in ipairs(units) do
        if unit.is_alive and unit.distance < best_dist and unit.level <= bot.get_level() + 3 then
            best = unit
            best_dist = unit.distance
        end
    end

    return best
end

-- Use combat abilities based on situation
local function use_abilities(target_guid)
    local hp, max_hp = bot.get_health()
    local hp_pct = hp / math.max(max_hp, 1) * 100

    local target = bot.get_unit(target_guid)
    if not target then return end

    local target_hp_pct = target.health / math.max(target.max_health, 1) * 100

    -- Execute at low health
    if target_hp_pct < 20 and bot.is_spell_ready(EXECUTE) then
        bot.cast_spell(EXECUTE, target_guid)
        return
    end

    -- Victory Rush if available (free, after a kill)
    if bot.is_spell_ready(VICTORY_RUSH) then
        bot.cast_spell(VICTORY_RUSH, target_guid)
        return
    end

    -- Rend if not already applied
    if bot.is_spell_ready(REND) then
        bot.cast_spell(REND, target_guid)
        return
    end

    -- Sunder Armor
    if bot.is_spell_ready(SUNDER_ARMOR) then
        bot.cast_spell(SUNDER_ARMOR, target_guid)
        return
    end

    -- Heroic Strike as filler
    if bot.is_spell_ready(HEROIC_STRIKE) then
        bot.cast_spell(HEROIC_STRIKE, target_guid)
        return
    end
end

-- Main tick function called every 200ms
function on_tick()
    -- Check if alive
    if not bot.is_alive() then
        bot.log("Dead! Attempting to revive...")
        bot.send_command(".revive")
        return
    end

    -- If in combat, fight
    if bot.in_combat() then
        local target_guid = bot.get_target()
        if target_guid ~= 0 then
            local target = bot.get_unit(target_guid)
            if target and target.is_alive then
                -- Move to target if too far
                if target.distance > 5 then
                    bot.move_to(target.x, target.y, target.z)
                else
                    bot.stop_moving()
                    use_abilities(target_guid)
                end
            else
                -- Target is dead, try to loot
                if target_guid ~= 0 then
                    bot.loot_all(target_guid)
                    current_target = 0
                end
            end
        end
        return
    end

    -- Not in combat - find a target
    local target = find_target(30)
    if target then
        current_target = target.guid
        bot.set_target(target.guid)

        -- Charge if far enough
        if target.distance > 8 and target.distance < 25 and bot.is_spell_ready(CHARGE) then
            bot.cast_spell(CHARGE, target.guid)
        elseif target.distance > 5 then
            bot.move_to(target.x, target.y, target.z)
        else
            bot.stop_moving()
            bot.attack(target.guid)

            -- Apply Battle Shout
            if bot.is_spell_ready(BATTLE_SHOUT) then
                bot.cast_spell(BATTLE_SHOUT, 0)
            end
        end
    end
end