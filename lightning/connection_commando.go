package lightning

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/bolt-observer/agent/lnsocket"
)

// ClnCommandoConnection struct
type ClnCommandoConnection struct {
	ln   *lnsocket.LN
	addr string
	rune string

	ClnConnection
}

// Compile time check for the interface
var _ ClnConnectionAPI = &ClnCommandoConnection{}

// NewCommandoConnection creates a new CLN connection
func NewCommandoConnection(addr string, rune string, timeout time.Duration) *ClnCommandoConnection {
	ret := &ClnCommandoConnection{}

	ret.addr = addr
	ret.rune = rune
	ret.ln = lnsocket.MakeLN(timeout)

	return ret
}

// Call calls serviceMethod with args and fills reply with response
func (l *ClnCommandoConnection) Call(ctx context.Context, serviceMethod string, args any, reply any) error {
	err := l.initConnection()
	if err != nil {
		return err
	}

	params := convertArgs(args)

	reader, err := l.ln.NewCommandoReader(ctx, l.rune, serviceMethod, params)
	if err != nil {
		return err
	}

	// To get meaningful error messages
	data, err := parseResp(reader)
	if err != nil {
		return err
	}

	err = json.Unmarshal(data.Result, &reply)
	if err != nil {
		return err
	}

	return nil
}

// StreamResponse is meant for streaming responses it calls serviceMethod with args and returns an io.Reader
func (l *ClnCommandoConnection) StreamResponse(ctx context.Context, serviceMethod string, args any) (io.Reader, error) {
	err := l.initConnection()
	if err != nil {
		return nil, err
	}

	params := convertArgs(args)

	reader, err := l.ln.NewCommandoReader(ctx, l.rune, serviceMethod, params)
	if err != nil {
		return nil, err
	}

	return reader, nil
}

func (l *ClnCommandoConnection) initConnection() error {
	// Check if connection is still usable
	err := l.ln.Ping()
	if err == nil {
		return nil
	}

	l.ln.Disconnect()

	err = l.ln.Connect(l.addr)
	if err != nil {
		return err
	}

	err = l.ln.Handshake()
	if err != nil {
		return err
	}

	return nil
}

func parseResp(reader io.Reader) (ClnSuccessResp, error) {
	// Need to buffer the response (to parse it twice)
	data, err := io.ReadAll(reader)
	if err != nil {
		return ClnSuccessResp{}, err
	}

	r := bytes.NewReader(data)

	var errResp ClnErrorResp
	err = json.NewDecoder(r).Decode(&errResp)
	if err != nil {
		return ClnSuccessResp{}, err
	}
	if errResp.Error.Code != 0 {
		return ClnSuccessResp{}, fmt.Errorf("invalid response: %s", errResp.Error.Message)
	}

	var successResp ClnSuccessResp
	r = bytes.NewReader(data)

	err = json.NewDecoder(r).Decode(&successResp)
	if err != nil {
		return ClnSuccessResp{}, err
	}
	return successResp, nil
}

func convertArgs(param any) string {
	const Empty = "[]"
	if param == nil {
		return Empty
	}

	sb := new(strings.Builder)
	enc := json.NewEncoder(sb)
	err := enc.Encode(param)
	if err != nil {
		return Empty
	}

	return strings.TrimRight(sb.String(), "\r\n\t")
}
