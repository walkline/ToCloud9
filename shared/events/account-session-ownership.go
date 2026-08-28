package events

const (
	AccountSessionClaimSubject   = "tc9.servers-registry.account-session.claim"
	AccountSessionReleaseSubject = "tc9.servers-registry.account-session.release"
	accountSessionEvictPrefix    = "tc9.gateway.account-session.evict."
)

func AccountSessionEvictSubject(gatewayInstanceID string) string {
	return accountSessionEvictPrefix + gatewayInstanceID
}

type AccountSessionRequest struct {
	AccountID         uint32 `json:"account_id"`
	RealmID           uint32 `json:"realm_id"`
	GatewayID         string `json:"gateway_id"`
	GatewayInstanceID string `json:"gateway_instance_id"`
	Token             string `json:"token"`
}

type AccountSessionResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

type AccountSessionEvictRequest struct {
	Token string `json:"token"`
}
