package lightning

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/bolt-observer/agent/lnsocket"
	"github.com/lightningnetwork/lnd/lnwire"
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

	if param != nil {
		switch k := reflect.TypeOf(param).Kind(); k {
		case reflect.Map:
			if reflect.TypeOf(param).Key().Kind() == reflect.String {
				if reflect.ValueOf(param).IsNil() {
					param = nil
				}
			}
		case reflect.Slice:
			if reflect.ValueOf(param).IsNil() {
				param = nil
			}
		case reflect.Array, reflect.Struct:
		case reflect.Ptr:
			switch kk := reflect.TypeOf(param).Elem().Kind(); kk {
			case reflect.Map:
				if reflect.TypeOf(param).Elem().Key().Kind() == reflect.String {
					if reflect.ValueOf(param).Elem().IsNil() {
						param = nil
					}
				}
			case reflect.Slice:
				if reflect.ValueOf(param).Elem().IsNil() {
					param = nil
				}
			case reflect.Array, reflect.Struct:
			default:
				return Empty
			}
		default:
			return Empty
		}
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

	err := l.ln.Connect(l.addr)
	if err != nil {
		return err
	}
	defer l.ln.Disconnect()

	err = l.ln.Handshake()
	if err != nil {
		return err
	}

	params := convertArgs(args)

	commando := lnsocket.NewCommandoMsg(l.rune, serviceMethod, params)
	var b bytes.Buffer
	_, err = lnwire.WriteMessage(&b, &commando, 0)
	if err != nil {
		return err
	}

	_, err = l.ln.Write(b.Bytes())
	if err != nil {
		return err
	}

	resp, err := l.ln.CommandoReadAll()
	if err != nil {
		return err
	}

	var errResp ClnErrorResp
	err = json.Unmarshal([]byte(resp), &errResp)
	if err != nil {
		return err
	}
	if errResp.Error.Code != 0 {
		return fmt.Errorf("invalid response %s", errResp.Error.Message)
	}

	var successResp ClnSuccessResp
	err = json.Unmarshal([]byte(resp), &successResp)
	if err != nil {
		return err
	}

	err = json.Unmarshal(successResp.Result, &reply)
	if err != nil {
		return err
	}

	return nil
}
