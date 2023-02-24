package lightning

import (
	"encoding/json"
	"errors"
	"math"

	entities "github.com/bolt-observer/go_common/entities"
)

var (
	// ErrNoNode means node was not found.
	ErrNoNode = errors.New("node not found")
	// ErrNoChan means channel was not found.
	ErrNoChan = errors.New("channel not found")
	// ErrInvalidResponse indicates invaid response.
	ErrInvalidResponse = errors.New("invalid response")
)

// ClnResp struct is the base type for all responses.
type ClnResp struct{}

// ClnInfo struct.
type ClnInfo struct {
	PubKey                string            `json:"id"`
	Alias                 string            `json:"alias"`
	Color                 string            `json:"color"`
	Network               string            `json:"network"`
	Addresses             []ClnListNodeAddr `json:"address,omitempty"`
	Features              ClnFeatures       `json:"our_features"`
	Version               string            `json:"version"`
	Blockheight           uint32            `json:"blockheight"`
	WarningBitcoindSync   string            `json:"warning_bitcoind_sync,omitempty"`
	WarningLightningdSync string            `json:"warning_lightningd_sync,omitempty"`
}

// ClnFeatures struct.
type ClnFeatures struct {
	Init    string `json:"init"`
	Node    string `json:"node"`
	Channel string `json:"channe"`
	Invoice string `json:"invoice"`
}

// ClnSetChan struct.
type ClnSetChan struct {
	PeerID         string `json:"peer_id"`
	LongChanID     string `json:"channel_id"`
	FeeBase        string `json:"fee_base_msat"`
	FeePpm         string `json:"fee_proportional_milli"`
	MiHtlc         string `json:"minimum_htlc_out_msat"`
	MaxHtlc        string `json:"maximum_htlc_out_msat"`
	ShortChannelID string `json:"short_channel_id,omitempty"`
}

// ClnSetChanResp struct.
type ClnSetChanResp struct {
	Settings []ClnSetChan `json:"channels,omitempty"`
	ClnResp
}

// ClnListChan struct.
type ClnListChan struct {
	Source         string            `json:"source"`
	Destination    string            `json:"destination"`
	Public         bool              `json:"public"`
	Capacity       uint64            `json:"satoshis"`
	Active         bool              `json:"active"`
	LastUpdate     entities.JsonTime `json:"last_update"`
	FeeBase        uint64            `json:"base_fee_millisatoshi"`
	FeePpm         uint64            `json:"fee_per_millionth"`
	MinHtlc        string            `json:"htlc_minimum_msat"`
	MaxHtlc        string            `json:"htlc_maximum_msat"`
	ShortChannelID string            `json:"short_channel_id,omitempty"`
	Delay          uint64            `json:"delay"`
}

// ClnListChanResp struct.
type ClnListChanResp struct {
	Channels []ClnListChan `json:"channels,omitempty"`
	ClnResp
}

// ClnFundsChan struct.
type ClnFundsChan struct {
	PeerID         string `json:"peer_id"`
	Connected      bool   `json:"connected,omitempty"`
	ShortChannelID string `json:"short_channel_id"`
	State          string `json:"state"`
	Capacity       uint64 `json:"channel_total_sat"`
	OurAmount      uint64 `json:"channel_sat"`
	FundingTxID    string `json:"funding_txid"`
	FundingOutput  int    `json:"funding_output"`
}

// ClnFundsChanResp struct.
type ClnFundsChanResp struct {
	Channels []ClnFundsChan `json:"channels,omitempty"`
	ClnResp
}

// ClnListNodeAddr struct.
type ClnListNodeAddr struct {
	Type    string `json:"type"`
	Address string `json:"address"`
	Port    int    `json:"port"`
}

// ClnListNode struct.
type ClnListNode struct {
	PubKey     string             `json:"nodeid"`
	Alias      string             `json:"alias,omitempty"`
	Color      string             `json:"color,omitempty"`
	Features   string             `json:"features,omitempty"`
	Addresses  []ClnListNodeAddr  `json:"addresses,omitempty"`
	LastUpdate *entities.JsonTime `json:"last_update,omitempty"`
}

// ClnListNodeResp struct.
type ClnListNodeResp struct {
	Nodes []ClnListNode `json:"nodes,omitempty"`
	ClnResp
}

// ClnForwardEntry struct.
type ClnForwardEntry struct {
	InChannel    string            `json:"in_channel"`
	InMsat       string            `json:"in_msat"`
	Status       string            `json:"status"` // one of "offered", "settled", "local_failed", "failed"
	ReceivedTime entities.JsonTime `json:"received_time"`

	OutChannel string `json:"out_channel,omitempty"`
	OutMsat    string `json:"out_msat,omitempty"`
	FeeMsat    string `json:"fee_msat,omitempty"`
	FailCode   uint32 `json:"fail_code,omitempty"`
	FailReason string `json:"fail_reason,omitempty"`
}

// ClnForwardEntries struct.
type ClnForwardEntries struct {
	Entries []ClnForwardEntry `json:"forwards,omitempty"`
}

// ClnPaymentEntries struct.
type ClnPaymentEntries struct {
	Entries []ClnPaymentEntry `json:"payments,omitempty"`
}

// ClnInvoiceEntries struct.
type ClnInvoiceEntries struct {
	Entries []ClnInvoiceEntry `json:"invoices,omitempty"`
}

// ClnPaymentEntry struct.
type ClnPaymentEntry struct {
	PaymentHash     string `json:"payment_hash,omitempty"`
	Status          string `json:"status"` // (one of "pending", "failed", "complete")
	PaymentPreimage string `json:"payment_preimage,omitempty"`
}

// ClnInvoiceEntry struct.
type ClnInvoiceEntry struct {
	Status      string `json:"status"` //  (one of "unpaid", "paid", "expired")
	PaymentHash string `json:"payment_hash,omitempty"`
}

// ClnRawMessageItf interface.
type ClnRawMessageItf interface {
	GetEntries() []json.RawMessage
}

// ClnRawTimeItf interface.
type ClnRawTimeItf interface {
	GetUnixTimeMs() uint64
}

// ClnRawForwardEntries struct.
type ClnRawForwardEntries struct {
	Entries []json.RawMessage `json:"forwards,omitempty"`
}

// GetEntries to comply with ClnRawMessageItf.
func (r ClnRawForwardEntries) GetEntries() []json.RawMessage {
	return r.Entries
}

// ClnRawInvoices struct.
type ClnRawInvoices struct {
	Entries []json.RawMessage `json:"invoices,omitempty"`
}

// GetEntries to comply with ClnRawMessageItf.
func (r ClnRawInvoices) GetEntries() []json.RawMessage {
	return r.Entries
}

// ClnRawPayments struct.
type ClnRawPayments struct {
	Entries []json.RawMessage `json:"payments,omitempty"`
}

// GetEntries to comply with ClnRawMessageItf.
func (r ClnRawPayments) GetEntries() []json.RawMessage {
	return r.Entries
}

// ClnRawPayTime struct.
type ClnRawPayTime struct {
	Time uint64 `json:"created_at,omitempty"`
}

// GetUnixTimeMs to comply with ClnRawTimeItf.
func (r ClnRawPayTime) GetUnixTimeMs() uint64 {
	return r.Time * 1000
}

// ClnRawInvoiceTime struct.
type ClnRawInvoiceTime struct {
	Time uint64 `json:"expires_at,omitempty"`
}

// GetUnixTimeMs to comply with ClnRawTimeItf.
func (r ClnRawInvoiceTime) GetUnixTimeMs() uint64 {
	return r.Time * 1000
}

// ClnRawForwardsTime struct.
type ClnRawForwardsTime struct {
	Time float64 `json:"received_time,omitempty"`
}

// GetUnixTimeMs to comply with ClnRawTimeItf.
func (r ClnRawForwardsTime) GetUnixTimeMs() uint64 {
	return uint64(math.Round(r.Time * 1000))
}

// ClnErrorResp struct.
type ClnErrorResp struct {
	ClnResp

	Error struct {
		Code    int    `json:"code,omitempty"`
		Message string `json:"message,omitempty"`
	} `json:"error"`
}

// ClnSuccessResp struct.
type ClnSuccessResp struct {
	ClnResp

	Result json.RawMessage `json:"result,omitempty"`
}

// ClnConnectResp struct.
type ClnConnectResp struct {
	ID string `json:"id"`
	ClnResp
}

// ClnNewAddrResp struct.
type ClnNewAddrResp struct {
	Bech32 string `json:"bech32,omitempty"`
	ClnResp
}

// ClnFundsChainResp struct.
type ClnFundsChainResp struct {
	Outputs []ClnFundsOutput `json:"outputs,omitempty"`
	ClnResp
}

// ClnFundsOutput struct.
type ClnFundsOutput struct {
	AmountMsat string `json:"amount_msat,omitempty"`
	Status     string `json:"status,omitempty"`
	Reserved   bool   `json:"reserved,omitempty"`
}

// ClnWithdrawResp struct.
type ClnWithdrawResp struct {
	TxID string `json:"txid,omitempty"`
	ClnResp
}

// ClnPayResp struct.
type ClnPayResp struct {
	PaymentPreimage string `json:"payment_preimage,omitempty"`
	PaymentHash     string `json:"payment_hash,omitempty"`
	AmountMsat      string `json:"amount_msat,omitempty"`
	AmountSentMsat  string `json:"amount_sent_msat,omitempty"`
	Status          string `json:"status,omitempty"`
	Destination     string `json:"destination,omitempty"`

	ClnResp
}

// ClnInvoiceResp struct.
type ClnInvoiceResp struct {
	Bolt11      string `json:"bolt11,omitempty"`
	PaymentHash string `json:"payment_hash,omitempty"`
	// omitted stuff
	ClnResp
}
