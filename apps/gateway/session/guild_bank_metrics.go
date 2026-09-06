package session

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Request-path two-phase bank metrics (world + guildserver). Not a durable saga:
// compensate runs only in the same request; process death mid-hop is not recovered.
var (
	guildBankTwoPhaseSuccess = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "gateway_guild_bank_twophase_success_total",
		Help: "Completed guild bank two-phase transfers (world + guildserver) by operation",
	}, []string{"op"})

	guildBankTwoPhaseFail = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "gateway_guild_bank_twophase_fail_total",
		Help: "Guild bank two-phase transfers that failed after the first hop (business or transport)",
	}, []string{"op", "stage"})

	guildBankTwoPhaseRestoreOK = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "gateway_guild_bank_twophase_restore_ok_total",
		Help: "Successful best-effort compensate/restore after a mid-transfer failure",
	}, []string{"op"})

	guildBankTwoPhaseRestoreFail = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "gateway_guild_bank_twophase_restore_fail_total",
		Help: "Failed best-effort compensate/restore — may leave money/items inconsistent; requires ops attention",
	}, []string{"op"})
)

const (
	bankXferOpDepositMoney  = "deposit_money"
	bankXferOpWithdrawMoney = "withdraw_money"
	bankXferOpDepositItem   = "deposit_item"
	bankXferOpWithdrawItem  = "withdraw_item"
	bankXferOpBuyTab        = "buy_tab"

	bankXferStageGuild = "guildserver"
	bankXferStageWorld = "worldserver"
)
