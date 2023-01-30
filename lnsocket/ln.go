package lnsocket

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/golang/glog"

	"github.com/lightningnetwork/lnd/keychain"
	"github.com/lightningnetwork/lnd/lnwire"

	"github.com/lightningnetwork/lnd/brontide"
	"github.com/lightningnetwork/lnd/tor"
)

// LN struct - heavily borrowed from https://github.com/jb55/lnsocket/blob/master/go/lnsocket.go
type LN struct {
	Conn        net.Conn
	PrivKeyECDH *keychain.PrivKeyECDH
	Proxy       *tor.ProxyNet
	Timeout     time.Duration
}

// MakeLN creates a new LN instance
func MakeLN(timeout time.Duration) *LN {
	server := os.Getenv("SOCKS_PROXY")
	if server == "" {
		server = "127.0.0.1:9050"
	}

	proxy := &tor.ProxyNet{SOCKS: server, SkipProxyForClearNetTargets: true}
	_, err := net.DialTimeout("tcp", server, timeout)
	if err != nil {
		glog.Warning("Ensure SOCKS_PROXY points to valid TOR SOCKS proxy\n")
		proxy = nil
	}

	return &LN{Proxy: proxy, Timeout: timeout}
}

// Clone copies the given LN instance to new onne
func (ln *LN) Clone() *LN {
	return &LN{
		Conn:        nil,
		PrivKeyECDH: ln.PrivKeyECDH,
		Proxy:       ln.Proxy,
	}
}

// Disconnect disconnect
func (ln *LN) Disconnect() {
	ln.Conn.Close()
}

// GenKey generates a keypair
func (ln *LN) GenKey() {
	remotePriv, _ := btcec.NewPrivateKey()
	ln.PrivKeyECDH = &keychain.PrivKeyECDH{PrivKey: remotePriv}
}

// SerializeKey serializes the private key to base64
func (ln *LN) SerializeKey() string {
	if ln.PrivKeyECDH == nil {
		return ""
	}

	return base64.StdEncoding.EncodeToString(ln.PrivKeyECDH.PrivKey.Serialize())
}

// DeserializeKey deserializes the private key from base64
func (ln *LN) DeserializeKey(key string) error {
	arr, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return err
	}

	priv, _ := btcec.PrivKeyFromBytes(arr)
	ln.PrivKeyECDH = &keychain.PrivKeyECDH{PrivKey: priv}

	return nil
}

// Read reads from connection
func (ln *LN) Read() (uint16, []byte, error) {
	res := make([]byte, 65535)
	ln.Conn.SetDeadline(time.Now().Add(ln.Timeout))
	n, err := ln.Conn.Read(res)
	if err != nil {
		return 0, nil, err
	}
	if n < 2 {
		return 0, nil, fmt.Errorf("read too small")
	}
	res = res[:n]
	msgtype := ParseMsgType(res)
	return msgtype, res[2:], nil
}

// Write writes to connection
func (ln *LN) Write(b []byte) (int, error) {
	ln.Conn.SetDeadline(time.Now().Add(ln.Timeout))
	return ln.Conn.Write(b)
}

// Handshake performs a handshake
func (ln *LN) Handshake() error {
	t, data, err := ln.Read()
	typ := lnwire.MessageType(t)
	if err != nil {
		return err
	}

	if typ != lnwire.MsgInit {
		return fmt.Errorf("unexpected message type: %v", typ)
	}

	init := lnwire.Init{}
	init.Decode(bytes.NewReader(data), 0)

	// Set some conservative options
	result := GetMandatoryFeatures(&init)
	features := ToRawFeatureVector(result)
	if _, ok := result[0]; ok {
		features = lnwire.NewRawFeatureVector(lnwire.DataLossProtectRequired)
	} else {
		if init.GlobalFeatures.IsEmpty() {
			// Features should be ok for Eclair
		} else {
			// Old corelightning
			features.Unset(lnwire.PaymentAddrRequired)
			features.Unset(lnwire.PaymentAddrOptional)
		}
	}

	// Override
	/*
		features = lnwire.NewRawFeatureVector()
		features.Set(lnwire.WumboChannelsOptional)
		features.Set(lnwire.DataLossProtectRequired)
		features.Set(lnwire.DataLossProtectOptional)
	*/

	initReplyMsg := lnwire.NewInitMessage(features, features)

	var b bytes.Buffer
	_, err = lnwire.WriteMessage(&b, initReplyMsg, 0)
	if err != nil {
		return err

	}

	_, err = ln.Write(b.Bytes())
	if err != nil {
		return err
	}

	return nil
}

// Ping sends a ping message
func (ln *LN) Ping() error {
	ping := lnwire.NewPing(16)
	var b bytes.Buffer

	_, err := lnwire.WriteMessage(&b, ping, 0)
	if err != nil {
		return err
	}
	_, err = ln.Write(b.Bytes())
	if err != nil {
		return err
	}

	for {
		t, _, err := ln.Read()
		typ := lnwire.MessageType(t)
		if typ == lnwire.MsgPong {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

func (ln *LN) connectWith(netAddr *lnwire.NetAddress) error {
	var (
		err  error
		conn *brontide.Conn
	)

	if ln.Proxy == nil {
		conn, err = brontide.Dial(ln.PrivKeyECDH, netAddr, ln.Timeout, net.DialTimeout)
	} else {
		conn, err = brontide.Dial(ln.PrivKeyECDH, netAddr, ln.Timeout, ln.Proxy.Dial)
	}

	ln.Conn = conn

	return err
}

// Connect connects to pubkey@host
func (ln *LN) Connect(s string) error {
	if ln.PrivKeyECDH == nil {
		ln.GenKey()
	}

	split := strings.Split(s, "@")
	if len(split) != 2 {
		return fmt.Errorf("wrong url")
	}

	bytes, err := hex.DecodeString(split[0])
	if err != nil {
		return err
	}

	key, err := btcec.ParsePubKey(bytes)
	if err != nil {
		return err
	}

	endpoint := split[1]

	var addr net.Addr

	if strings.Contains(endpoint, ".onion") {
		if ln.Proxy == nil {
			return fmt.Errorf("tor is not available")
		}
		s := strings.Split(endpoint, ":")
		if len(s) != 2 {
			return fmt.Errorf("wrong url")
		}

		port, err := strconv.Atoi(s[1])
		if err != nil {
			return err
		}
		addr = &tor.OnionAddr{OnionService: s[0], Port: port}
	} else {
		if ln.Proxy == nil {
			addr, err = net.ResolveTCPAddr("tcp", endpoint)
		} else {
			addr, err = ln.Proxy.ResolveTCPAddr("tcp", endpoint)
		}

		if err != nil {
			return err
		}
	}

	netAddr := &lnwire.NetAddress{
		IdentityKey: key,
		Address:     addr,
	}

	return ln.connectWith(netAddr)
}
