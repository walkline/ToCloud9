-- hogger.lua - Solo Hogger kill behavior for a warrior bot.
-- The bot teleports near Hogger, sets an appropriate level, finds Hogger,
-- and engages in melee combat using warrior abilities.

local HOGGER_ENTRY = 448
local HEROIC_STRIKE = 78
local REND = 772
local EXECUTE = 5308
local BATTLE_SHOUT = 2457
local CHARGE = 100
local SUNDER_ARMOR = 7386
local VICTORY_RUSH = 34428

local state = "prepare"
local prepare_step = 0
local hogger_guid = 0
local fight_start_time = 0

function on_tick()
    if not bot.is_alive() then
        bot.log("Dead! Reviving to try again...")
        bot.send_command(".revive")
        state = "prepare"
        prepare_step = 0
        return
    end

    if state == "prepare" then
        do_prepare()
    elseif state == "find_hogger" then
        do_find_hogger()
    elseif state == "engage" then
        do_engage()
    elseif state == "fight" then
        do_fight()
    elseif state == "victory" then
        do_victory()
    end
end

function do_prepare()
    if prepare_step == 0 then
        bot.log("Preparing for Hogger fight...")
        -- Teleport to Hogger's location in Elwynn Forest
        bot.send_command(".go creature 448")
        prepare_step = 1
        return
    end

    if prepare_step == 1 then
        -- Set level to 10 (similar to Hogger's level 11)
        bot.send_command(".character level 10")
        prepare_step = 2
        return
    end

    if prepare_step == 2 then
        -- Full heal
        bot.send_command(".die")
        bot.send_command(".revive")
        prepare_step = 3
        return
    end

    if prepare_step >= 3 then
        bot.log("Preparation complete, searching for Hogger...")
        state = "find_hogger"
    end
end

function do_find_hogger()
    local units = bot.get_nearby_units(50)
    for _, unit in ipairs(units) do
        if unit.entry == HOGGER_ENTRY and unit.is_alive then
            hogger_guid = unit.guid
            bot.log(string.format("Found Hogger! GUID=%d Level=%d HP=%d/%d Distance=%.1f",
                unit.guid, unit.level, unit.health, unit.max_health, unit.distance))
            state = "engage"
            return
        end
    end

    -- Hogger not in range yet, wander a bit
    bot.log("Hogger not found nearby, waiting...")
end

function do_engage()
    local hogger = bot.get_unit(hogger_guid)
    if not hogger or not hogger.is_alive then
        bot.log("Hogger disappeared, searching again...")
        state = "find_hogger"
        return
    end

    bot.set_target(hogger_guid)

    -- Use Charge if in range
    if hogger.distance > 8 and hogger.distance < 25 and bot.is_spell_ready(CHARGE) then
        bot.log("Charging Hogger!")
        bot.cast_spell(CHARGE, hogger_guid)
        state = "fight"
        fight_start_time = os.clock()
        return
    end

    -- Move closer
    if hogger.distance > 5 then
        bot.move_to(hogger.x, hogger.y, hogger.z)
        return
    end

    -- In melee range, start fighting
    bot.stop_moving()
    bot.attack(hogger_guid)

    -- Apply Battle Shout
    if bot.is_spell_ready(BATTLE_SHOUT) then
        bot.cast_spell(BATTLE_SHOUT, 0)
    end

    state = "fight"
    fight_start_time = os.clock()
    bot.log("Engaging Hogger in melee combat!")
end

function do_fight()
    local hogger = bot.get_unit(hogger_guid)
    if not hogger then
        bot.log("Lost track of Hogger during fight!")
        state = "find_hogger"
        return
    end

    if not hogger.is_alive then
        bot.log(string.format("VICTORY! Hogger has been slain! Fight duration: %.1fs",
            os.clock() - fight_start_time))
        -- Loot Hogger
        bot.loot_all(hogger_guid)
        state = "victory"
        return
    end

    -- Stay in melee range
    if hogger.distance > 5 then
        bot.move_to(hogger.x, hogger.y, hogger.z)
        return
    end

    bot.stop_moving()
    bot.attack(hogger_guid)

    -- Use abilities
    local hp, max_hp = bot.get_health()
    local hp_pct = hp / math.max(max_hp, 1) * 100
    local hogger_hp_pct = hogger.health / math.max(hogger.max_health, 1) * 100

    -- Execute at low health
    if hogger_hp_pct < 20 and bot.is_spell_ready(EXECUTE) then
        bot.cast_spell(EXECUTE, hogger_guid)
        return
    end

    -- Victory Rush if available
    if bot.is_spell_ready(VICTORY_RUSH) then
        bot.cast_spell(VICTORY_RUSH, hogger_guid)
        return
    end

    -- Rend
    if bot.is_spell_ready(REND) then
        bot.cast_spell(REND, hogger_guid)
        return
    end

    -- Sunder Armor
    if bot.is_spell_ready(SUNDER_ARMOR) then
        bot.cast_spell(SUNDER_ARMOR, hogger_guid)
        return
    end

    -- Heroic Strike
    if bot.is_spell_ready(HEROIC_STRIKE) then
        bot.cast_spell(HEROIC_STRIKE, hogger_guid)
        return
    end
end

function do_victory()
    bot.log("Hogger defeated! Test complete.")
    bot.send_chat("I have defeated Hogger!")
end