//go:build plugins
// +build plugins

package boltz

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/BoltzExchange/boltz-lnd/boltz"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/lightningnetwork/lnd/input"
	"github.com/stretchr/testify/assert"
	bip39 "github.com/tyler-smith/go-bip39"
)

const DataDir = "/tmp/lnregtest-data/dev_network/bitcoin"

type DummyStruct struct {
	Data *SwapData
}

func (d DummyStruct) GetSwapData() *SwapData {
	return d.Data
}

func TestScripting(t *testing.T) {
	preimageHash, err := hex.DecodeString("abab")
	assert.NoError(t, err)
	timeoutBlockHeight := int64(1337)

	privKey, err := secp256k1.GeneratePrivateKey()
	assert.NoError(t, err)

	for _, b := range []bool{false, true} {
		script := GetScript(t, b, preimageHash, privKey, privKey, timeoutBlockHeight)
		addr := GetAddress(t, b, script)
		t.Logf("%v\n", addr)
	}

	return
}

func TestRedeemLockedFunds(t *testing.T) {
	// Reverse swap finished, prepare redeemer to claim

	const (
		Node    = "F"
		Id      = 1
		Attempt = 1
	)

	ctx := context.Background()

	ln := getLocalLndByName(t, Node)
	if ln == nil {
		t.Logf("Ignoring redeemer test since regtest network is not available\n")
		return
	}

	lnAPI, err := ln()
	assert.NoError(t, err)
	defer lnAPI.Cleanup()

	f, err := lnAPI.GetOnChainFunds(ctx)
	assert.NoError(t, err)
	balanceBefore := f.TotalBalance

	// ARRANGE
	entropy, err := bip39.NewEntropy(256)
	c := NewCryptoAPI(entropy)
	k, err := c.GetKeys(fmt.Sprintf("%d-%d", Id, Attempt))
	assert.NoError(t, err)
	dummyKey, err := secp256k1.GeneratePrivateKey()
	assert.NoError(t, err)

	blockNum := GetCurrentBlockNum(t)

	script := GetScript(t, true, k.Preimage.Hash, k.Keys.PrivateKey, dummyKey, int64(blockNum+10))
	addr := GetAddress(t, true, script)

	t.Logf("Address: %s %d\n", addr, blockNum)

	txid := FundAddresss(t, addr, "0.1")
	tx := GetRawTx(t, txid)

	t.Logf("Transaction was %v\n", tx)

	sd := &SwapData{BoltzID: "dummy", JobID: Id, Attempt: Attempt, State: ClaimReverseFunds, TransactionHex: tx, Address: addr, TimoutBlockHeight: uint32(blockNum + 10), Script: hex.EncodeToString(script)}

	finished := false
	cb := func(data DummyStruct, success bool) {
		assert.Equal(t, true, success)

		lnAPI, err := ln()
		assert.NoError(t, err)
		defer lnAPI.Cleanup()

		f, err := lnAPI.GetOnChainFunds(ctx)
		assert.NoError(t, err)
		balanceAfter := f.TotalBalance

		assert.Greater(t, balanceAfter, balanceBefore)
		t.Logf("BalanceBefore %v BalanceAfter: %v\n", balanceBefore, balanceAfter)

		finished = true
	}

	redeemer := NewRedeemer(ctx, (RedeemForward | RedeemReverse), &chaincfg.RegressionNetParams, NewTestBitcoinOnChainCommunicator(t), ln, 100*time.Millisecond, c, cb)

	// ACT
	redeemer.AddEntry(DummyStruct{Data: sd})

	// ASSERT
	for i := 0; i < 100; i++ {
		if finished {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}

	t.Fatalf("Did not succeed")
}

func GetAddress(t *testing.T, reverse bool, script []byte) string {
	if reverse {
		lockupAddress, err := boltz.WitnessScriptHashAddress(&chaincfg.RegressionNetParams, script)
		assert.NoError(t, err)
		return lockupAddress
	} else {
		encodedAddress, err := boltz.NestedScriptHashAddress(&chaincfg.RegressionNetParams, script)
		assert.NoError(t, err)
		err = boltz.CheckSwapAddress(&chaincfg.RegressionNetParams, encodedAddress, script, true)
		assert.NoError(t, err)
		return encodedAddress
	}
}

func GetScript(t *testing.T, reverse bool, preimageHash []byte, myKey, theirKey *secp256k1.PrivateKey, timeoutBlockHeight int64) []byte {
	builder := txscript.NewScriptBuilder()
	var (
		script []byte
		err    error
	)

	if reverse {
		builder.AddOp(txscript.OP_SIZE).AddInt64(32)
		builder.AddOp(txscript.OP_EQUAL)
		builder.AddOp(txscript.OP_IF)
		builder.AddOp(txscript.OP_HASH160)
		builder.AddData(input.Ripemd160H(preimageHash))
		builder.AddOp(txscript.OP_EQUALVERIFY)
		builder.AddData(myKey.PubKey().SerializeCompressed())
		builder.AddOp(txscript.OP_ELSE)
		builder.AddOp(txscript.OP_DROP)
		builder.AddInt64(int64(timeoutBlockHeight))
		builder.AddOp(txscript.OP_CHECKLOCKTIMEVERIFY)
		builder.AddOp(txscript.OP_DROP)
		builder.AddData(theirKey.PubKey().SerializeCompressed())
		builder.AddOp(txscript.OP_ENDIF)
		builder.AddOp(txscript.OP_CHECKSIG)

		script, err = builder.Script()
		assert.NoError(t, err)

		err = boltz.CheckReverseSwapScript(script, preimageHash, myKey, uint32(timeoutBlockHeight))
		assert.NoError(t, err)
	} else {
		builder.AddOp(txscript.OP_HASH160)
		builder.AddData(input.Ripemd160H(preimageHash))
		builder.AddOp(txscript.OP_EQUAL)
		builder.AddOp(txscript.OP_IF)
		builder.AddData(theirKey.PubKey().SerializeCompressed())
		builder.AddOp(txscript.OP_ELSE)
		builder.AddInt64(int64(timeoutBlockHeight))
		builder.AddOp(txscript.OP_CHECKLOCKTIMEVERIFY)
		builder.AddOp(txscript.OP_DROP)
		builder.AddData(myKey.PubKey().SerializeCompressed())
		builder.AddOp(txscript.OP_ENDIF)
		builder.AddOp(txscript.OP_CHECKSIG)

		script, err = builder.Script()
		assert.NoError(t, err)

		err = boltz.CheckSwapScript(script, preimageHash, myKey, uint32(timeoutBlockHeight))
		assert.NoError(t, err)
	}

	return script
}

func GetRawTx(t *testing.T, hash string) string {
	out, err := exec.Command("bitcoin-cli", fmt.Sprintf("-datadir=%s", DataDir), "getrawtransaction", hash).Output()
	assert.NoError(t, err)
	tx := strings.Trim(string(out), " \r\n")

	raw, err := hex.DecodeString(tx)
	assert.NoError(t, err)

	actual, err := btcutil.NewTxFromBytes(raw)
	assert.NoError(t, err)

	assert.Equal(t, hash, actual.Hash().String())

	return tx
}

func FundAddresss(t *testing.T, addr string, amount string) string {
	out, err := exec.Command("bitcoin-cli", fmt.Sprintf("-datadir=%s", DataDir), "sendtoaddress", addr, amount).Output()
	assert.NoError(t, err)

	tx := strings.Trim(string(out), " \r\n")

	return tx
}

func BroadcastTransaction(t *testing.T, transaction string) string {
	out, err := exec.Command("bitcoin-cli", fmt.Sprintf("-datadir=%s", DataDir), "sendrawtransaction", transaction).Output()
	fmt.Printf("OUTPUT |%s| %s %v\n", transaction, string(out), err)
	assert.NoError(t, err)

	tx := strings.Trim(string(out), " \r\n")

	fmt.Printf("Txid %v\n", tx)
	return tx
}

type BlockChainInfo struct {
	Blocks uint64 `json:"blocks"`
}

func GetCurrentBlockNum(t *testing.T) uint64 {
	out, err := exec.Command("bitcoin-cli", fmt.Sprintf("-datadir=%s", DataDir), "getblockchaininfo").Output()
	assert.NoError(t, err)

	var b BlockChainInfo
	data := []byte(strings.Trim(string(out), " \r\n"))
	err = json.Unmarshal(data, &b)
	assert.NoError(t, err)

	return b.Blocks
}

type TestBitcoinOnChainCommunicator struct {
	t *testing.T
}

func NewTestBitcoinOnChainCommunicator(t *testing.T) *TestBitcoinOnChainCommunicator {
	return &TestBitcoinOnChainCommunicator{
		t: t,
	}
}

func (c *TestBitcoinOnChainCommunicator) BroadcastTransaction(transaction *wire.MsgTx) error {
	transactionHex, err := boltz.SerializeTransaction(transaction)

	if err != nil {
		return fmt.Errorf("could not serialize transaction: %v", err)
	}

	BroadcastTransaction(c.t, transactionHex)

	return nil
}

func (c *TestBitcoinOnChainCommunicator) GetFeeEstimation() (uint64, error) {
	return 10, nil
}
