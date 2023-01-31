package lightning

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
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

// MakeCommandoConnection create a new CLN connection
func MakeCommandoConnection(addr string, rune string, timeout time.Duration) *ClnCommandoConnection {
	ret := &ClnCommandoConnection{}

	ret.addr = addr
	ret.rune = rune
	ret.ln = lnsocket.MakeLN(timeout)
	ret.Timeout = timeout

	return ret
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

// CallWithTimeout - helper to call rpc method with a timeout
func (l *ClnCommandoConnection) CallWithTimeout(serviceMethod string, args any, reply any) error {

	disconnect, err := l.initConnection()
	if disconnect != nil {
		defer disconnect()
	}
	if err != nil {
		return err
	}

	params := convertArgs(args)

	reader, err := l.ln.NewCommandoReader(l.rune, serviceMethod, params, l.ln.Timeout)
	if err != nil {
		return err
	}

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

func (l *ClnCommandoConnection) initConnection() (func(), error) {
	err := l.ln.Connect(l.addr)
	if err != nil {
		return nil, err
	}
	disconnect := func() {
		l.ln.Disconnect()
	}

	err = l.ln.Handshake()
	if err != nil {
		return disconnect, err
	}

	return disconnect, nil
}

func parseResp(reader io.Reader) (ClnSuccessResp, error) {
	// I have to buffer the response (to parse it twice)
	data, err := ioutil.ReadAll(reader)
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
		return ClnSuccessResp{}, fmt.Errorf("invalid response %s", errResp.Error.Message)
	}

	var successResp ClnSuccessResp
	r = bytes.NewReader(data)

	err = json.NewDecoder(r).Decode(&successResp)
	if err != nil {
		return ClnSuccessResp{}, err
	}
	return successResp, nil
}
