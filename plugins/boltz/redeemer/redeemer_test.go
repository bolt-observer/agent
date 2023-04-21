//go:build plugins
// +build plugins

package redeemer

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
	api "github.com/bolt-observer/agent/lightning"
	common "github.com/bolt-observer/agent/plugins/boltz/common"
	crypto "github.com/bolt-observer/agent/plugins/boltz/crypto"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/lightningnetwork/lnd/input"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	bip39 "github.com/tyler-smith/go-bip39"
)

const (
	DataDir = "/tmp/lnregtest-data/dev_network/bitcoin"
	Node    = "F"
)

type DummyStruct struct {
	Data *common.SwapData
}

func (d DummyStruct) GetSwapData() *common.SwapData {
	return d.Data
}

func TestScripting(t *testing.T) {
	preimageHash, err := hex.DecodeString("abab")
	assert.NoError(t, err)
	timeoutBlockHeight := int64(1337)

	privKey, err := secp256k1.GeneratePrivateKey()
	require.NoError(t, err)

	for _, b := range []bool{false, true} {
		script := GetScript(t, b, preimageHash, privKey, privKey, timeoutBlockHeight)
		addr := GetAddress(t, b, script)
		t.Logf("%v\n", addr)
	}
}

func TestRedeemerTypes(t *testing.T) {
	// Test all possibe redeemer types
	ctx := context.Background()

	ln := api.GetLocalLndByName(t, Node)
	if ln == nil {
		t.Logf("Ignoring redeemer test since regtest network is not available\n")
		return
	}

	lnAPI, err := ln()
	require.NoError(t, err)
	defer lnAPI.Cleanup()

	redeemerFail := NewRedeemer[DummyStruct](ctx, 0, &chaincfg.RegressionNetParams, NewTestBitcoinOnChainCommunicator(t), ln, 100*time.Millisecond, nil)
	assert.Nil(t, redeemerFail)

	forwardSd := &common.SwapData{State: common.RedeemingLockedFunds, TransactionHex: "dummy"}
	reverseSd := &common.SwapData{State: common.ClaimReverseFunds, TransactionHex: "dummy"}

	redeemerForward := NewRedeemer[DummyStruct](ctx, RedeemForward, &chaincfg.RegressionNetParams, NewTestBitcoinOnChainCommunicator(t), ln, 100*time.Millisecond, nil)
	require.NotNil(t, redeemerForward)

	err = redeemerForward.AddEntry(DummyStruct{Data: forwardSd})
	assert.NoError(t, err)
	err = redeemerForward.AddEntry(DummyStruct{Data: reverseSd})
	assert.Error(t, err)

	redeemerReverse := NewRedeemer[DummyStruct](ctx, RedeemReverse, &chaincfg.RegressionNetParams, NewTestBitcoinOnChainCommunicator(t), ln, 100*time.Millisecond, nil)
	require.NotNil(t, redeemerReverse)

	err = redeemerReverse.AddEntry(DummyStruct{Data: forwardSd})
	assert.Error(t, err)
	err = redeemerReverse.AddEntry(DummyStruct{Data: reverseSd})
	assert.NoError(t, err)

	redeemerBoth := NewRedeemer[DummyStruct](ctx, (RedeemReverse | RedeemForward), &chaincfg.RegressionNetParams, NewTestBitcoinOnChainCommunicator(t), ln, 100*time.Millisecond, nil)
	require.NotNil(t, redeemerBoth)

	err = redeemerBoth.AddEntry(DummyStruct{Data: forwardSd})
	assert.NoError(t, err)
	err = redeemerBoth.AddEntry(DummyStruct{Data: reverseSd})
	assert.NoError(t, err)
}

func TestRedeemLockedFunds(t *testing.T) {
	// Reverse swap finished, prepare redeemer to claim
	ctx := context.Background()

	ln := api.GetLocalLndByName(t, Node)
	if ln == nil {
		t.Logf("Ignoring redeemer test since regtest network is not available\n")
		return
	}

	lnAPI, err := ln()
	require.NoError(t, err)
	defer lnAPI.Cleanup()

	f, err := lnAPI.GetOnChainFunds(ctx)
	require.NoError(t, err)
	balanceBefore := f.TotalBalance

	// ARRANGE
	entropy, err := bip39.NewEntropy(256)
	c := crypto.NewCryptoAPI(entropy)
	blockNum := GetCurrentBlockNum(t)

	sd := GenerateClaimSd(t, c, "dummy1", 1, int(blockNum+10), "0.1")

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

	redeemer := NewRedeemer[DummyStruct](ctx, (RedeemForward | RedeemReverse), &chaincfg.RegressionNetParams, NewTestBitcoinOnChainCommunicator(t), ln, 100*time.Millisecond, c)
	redeemer.SetCallback(cb)

	// ACT
	err = redeemer.AddEntry(DummyStruct{Data: sd})
	assert.NoError(t, err)

	// ASSERT
	for i := 0; i < 100; i++ {
		if finished {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}

	t.Fatalf("Did not succeed")
}

func TestRedeemLockedFundsTwoSources(t *testing.T) {
	// Reverse swap finished and forward swap failed, prepare redeemer to claim both
	ctx := context.Background()

	ln := api.GetLocalLndByName(t, Node)
	if ln == nil {
		t.Logf("Ignoring redeemer test since regtest network is not available\n")
		return
	}

	lnAPI, err := ln()
	require.NoError(t, err)
	defer lnAPI.Cleanup()

	f, err := lnAPI.GetOnChainFunds(ctx)
	require.NoError(t, err)
	balanceBefore := f.TotalBalance

	// ARRANGE
	entropy, err := bip39.NewEntropy(256)
	c := crypto.NewCryptoAPI(entropy)
	blockNum := GetCurrentBlockNum(t)

	claimSd := GenerateClaimSd(t, c, "dummy2", 2, int(blockNum+10), "0.1")
	failSd := GenerateFailSd(t, c, "dummy1", 1, int(blockNum-2), "0.2")
	// Dummy cannot be claimed
	dummySd := GenerateFailSd(t, c, "dummy3", 3, int(blockNum+5), "0.1")

	cnt := 0
	cb := func(data DummyStruct, success bool) {
		t.Logf("Dummy %+v success %v\n", data, success)
		cnt++
	}

	redeemer := NewRedeemer[DummyStruct](ctx, (RedeemForward | RedeemReverse), &chaincfg.RegressionNetParams, NewTestBitcoinOnChainCommunicator(t), ln, 100*time.Millisecond, c)
	redeemer.SetCallback(cb)
	// ACT
	err = redeemer.AddEntry(DummyStruct{Data: dummySd})
	assert.NoError(t, err)
	err = redeemer.AddEntry(DummyStruct{Data: claimSd})
	assert.NoError(t, err)
	err = redeemer.AddEntry(DummyStruct{Data: failSd})
	assert.NoError(t, err)

	// ASSERT
	for i := 0; i < 100; i++ {
		if cnt == 2 {
			time.Sleep(100 * time.Millisecond)

			lnAPI, err := ln()
			assert.NoError(t, err)
			defer lnAPI.Cleanup()

			f, err := lnAPI.GetOnChainFunds(ctx)
			assert.NoError(t, err)
			balanceAfter := f.TotalBalance

			assert.Greater(t, balanceAfter, balanceBefore)
			t.Logf("BalanceBefore %v BalanceAfter: %v\n", balanceBefore, balanceAfter)
			// Not sure about fees, but this is a good approximation
			assert.Less(t, int64(29_000_000), balanceAfter-balanceBefore)
			assert.Greater(t, int64(31_000_000), balanceAfter-balanceBefore)

			return
		}
		time.Sleep(100 * time.Millisecond)
	}

	t.Fatalf("Did not succeed")
}

func TestRedeemFailThatIsNotYetMature(t *testing.T) {
	// Swap failed, redeem locked funds that are still locked for one block
	ctx := context.Background()

	ln := api.GetLocalLndByName(t, Node)
	if ln == nil {
		t.Logf("Ignoring redeemer test since regtest network is not available\n")
		return
	}

	lnAPI, err := ln()
	require.NoError(t, err)
	defer lnAPI.Cleanup()

	f, err := lnAPI.GetOnChainFunds(ctx)
	require.NoError(t, err)
	balanceBefore := f.TotalBalance

	// ARRANGE
	entropy, err := bip39.NewEntropy(256)
	c := crypto.NewCryptoAPI(entropy)
	blockNum := GetCurrentBlockNum(t)

	failSd := GenerateFailSd(t, c, "dummy1", 1, int(blockNum+1), "0.1")
	notRedeemableSd := GenerateFailSd(t, c, "notredeemable", 2, int(blockNum+10), "0.1")
	dummySd := GenerateFailSd(t, c, "notused", 3, int(blockNum+1), "0.1")
	dummySd.State = common.RedeemLockedFunds

	cnt := 0
	cb := func(data DummyStruct, success bool) {
		cnt++
	}

	redeemer := NewRedeemer[DummyStruct](ctx, (RedeemForward | RedeemReverse), &chaincfg.RegressionNetParams, NewTestBitcoinOnChainCommunicator(t), ln, 100*time.Millisecond, c)
	redeemer.SetCallback(cb)

	// ACT
	err = redeemer.AddEntry(DummyStruct{Data: failSd})
	assert.NoError(t, err)
	err = redeemer.AddEntry(DummyStruct{Data: notRedeemableSd})
	assert.NoError(t, err)
	err = redeemer.AddEntry(DummyStruct{Data: dummySd})
	assert.Error(t, err)

	err = api.RegtestMine(2)
	assert.NoError(t, err)

	// ASSERT
	for i := 0; i < 100; i++ {
		if cnt == 1 {
			time.Sleep(100 * time.Millisecond)

			lnAPI, err := ln()
			assert.NoError(t, err)
			defer lnAPI.Cleanup()

			f, err := lnAPI.GetOnChainFunds(ctx)
			assert.NoError(t, err)
			balanceAfter := f.TotalBalance

			assert.Greater(t, balanceAfter, balanceBefore)
			t.Logf("BalanceBefore %v BalanceAfter: %v\n", balanceBefore, balanceAfter)
			// Not sure about fees, but this is a good approximation
			assert.Less(t, int64(9_000_000), balanceAfter-balanceBefore)
			assert.Greater(t, int64(11_000_000), balanceAfter-balanceBefore)

			return
		} else if cnt > 1 {
			t.Fatalf("Too many callbacks %v\n", cnt)
		}
		time.Sleep(100 * time.Millisecond)
	}

	t.Fatalf("Did not succeed")
}

func GenerateClaimSd(t *testing.T, c *crypto.CryptoAPI, name string, id int, blockNum int, funds string) *common.SwapData {
	k, err := c.GetKeys(fmt.Sprintf("%d-%d", id, 1))
	require.NoError(t, err)
	dummyKey, err := secp256k1.GeneratePrivateKey()
	require.NoError(t, err)

	script := GetScript(t, true, k.Preimage.Hash, k.Keys.PrivateKey, dummyKey, int64(blockNum))
	addr := GetAddress(t, true, script)

	txid := FundAddresss(t, addr, funds)
	tx := GetRawTx(t, txid)
	t.Logf("Transaction was %v\n", tx)

	return &common.SwapData{BoltzID: name, JobID: common.JobID(id), Attempt: 1, State: common.ClaimReverseFunds, TransactionHex: tx, Address: addr, TimoutBlockHeight: uint32(blockNum), Script: hex.EncodeToString(script)}
}

func GenerateFailSd(t *testing.T, c *crypto.CryptoAPI, name string, id int, blockNum int, funds string) *common.SwapData {
	k, err := c.GetKeys(fmt.Sprintf("%d-%d", id, 1))
	require.NoError(t, err)
	dummyKey, err := secp256k1.GeneratePrivateKey()
	require.NoError(t, err)

	script := GetScript(t, false, k.Preimage.Hash, k.Keys.PrivateKey, dummyKey, int64(blockNum))
	addr := GetAddress(t, false, script)

	txid := FundAddresss(t, addr, funds)
	tx := GetRawTx(t, txid)
	t.Logf("Transaction was %v\n", tx)

	return &common.SwapData{BoltzID: name, JobID: common.JobID(id), Attempt: 1, State: common.RedeemingLockedFunds, TransactionHex: tx, Address: addr, TimoutBlockHeight: uint32(blockNum), Script: hex.EncodeToString(script)}
}

func GetAddress(t *testing.T, reverse bool, script []byte) string {
	if reverse {
		lockupAddress, err := boltz.WitnessScriptHashAddress(&chaincfg.RegressionNetParams, script)
		require.NoError(t, err)
		return lockupAddress
	} else {
		encodedAddress, err := boltz.NestedScriptHashAddress(&chaincfg.RegressionNetParams, script)
		require.NoError(t, err)
		err = boltz.CheckSwapAddress(&chaincfg.RegressionNetParams, encodedAddress, script, true)
		require.NoError(t, err)
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
		require.NoError(t, err)

		err = boltz.CheckReverseSwapScript(script, preimageHash, myKey, uint32(timeoutBlockHeight))
		require.NoError(t, err)
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
		require.NoError(t, err)

		err = boltz.CheckSwapScript(script, preimageHash, myKey, uint32(timeoutBlockHeight))
		require.NoError(t, err)
	}

	return script
}

func GetRawTx(t *testing.T, hash string) string {
	out, err := exec.Command("bitcoin-cli", fmt.Sprintf("-datadir=%s", DataDir), "getrawtransaction", hash).Output()
	require.NoError(t, err)
	tx := strings.Trim(string(out), " \r\n")

	raw, err := hex.DecodeString(tx)
	require.NoError(t, err)

	actual, err := btcutil.NewTxFromBytes(raw)
	require.NoError(t, err)

	require.Equal(t, hash, actual.Hash().String())

	return tx
}

func FundAddresss(t *testing.T, addr string, amount string) string {
	out, err := exec.Command("bitcoin-cli", fmt.Sprintf("-datadir=%s", DataDir), "sendtoaddress", addr, amount).Output()
	require.NoError(t, err)

	tx := strings.Trim(string(out), " \r\n")

	return tx
}

func BroadcastTransaction(t *testing.T, transaction string) string {
	out, err := exec.Command("bitcoin-cli", fmt.Sprintf("-datadir=%s", DataDir), "sendrawtransaction", transaction).Output()
	require.NoError(t, err)

	tx := strings.Trim(string(out), " \r\n")

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
	require.NoError(t, err)

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

func TestSwapRedeemScriptToAddr(t *testing.T) {
	script := "a91404ea436acef6809e51881b00c96542cec83b3a1f876321031eaf7e597ba5ff427350dcf6cc5f9c2925bf2d89c02befcc92ecf2700a0513246703511325b1752103659a587a801dc66b115d0f62e096909e099cceda53f753ca08c232b5066279f368ac"

	bytes, err := hex.DecodeString(script)
	require.NoError(t, err)

	addr, err := boltz.NestedScriptHashAddress(&chaincfg.TestNet3Params, bytes)
	require.NoError(t, err)

	t.Logf("Address: %v", addr)
	//t.Fail()
}
