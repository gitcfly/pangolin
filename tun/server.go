package tun

import (
	"time"

	"github.com/gitcfly/tunnet/cache"
	"github.com/gitcfly/tunnet/logging"
	"github.com/xitongsys/ethernet-go/header"
)

var TUNCHANBUFFSIZE = 1024

type TunServer struct {
	TunConn *Tun
	//Key: clientProtocol:clientIP:clientPort  Value: chan string
	RouteMap *cache.Cache
	//write to tun
	InputChan chan string
}

func NewTunServer(tname string, ipNet string, mtu int) (*TunServer, error) {
	ts := &TunServer{
		RouteMap:  cache.NewCache(time.Minute * 10),
		InputChan: make(chan string, TUNCHANBUFFSIZE),
	}
	if tun, err := NewLinuxTun(tname, ipNet, mtu); err != nil {
		return nil, err
	} else {
		ts.TunConn = tun
	}
	return ts, nil
}

func (ts *TunServer) Start() {
	ts.TunConn.Start()
	logging.Log.Info("TunServer started")
	//tun to client
	go func() {
		defer func() {
			recover()
		}()

		for {
			data := make([]byte, ts.TunConn.GetMtu()*2)
			if n, err := ts.TunConn.Read(data); err == nil && n > 0 {
				logging.Log.Debug("Received Data from Tun...")
				if proto, src, dst, err := header.GetBase(data); err == nil {
					key := proto + ":" + dst + ":" + src
					if outputChan := ts.RouteMap.Get(key); outputChan != nil {
						go func() {
							defer func() {
								recover()
							}()
							outputChan.(chan string) <- string(data[:n])
						}()

						logging.Log.Debugf("FromTun: src:%v dst:%v proto:%v", src, dst, proto)
					}
				}
			}
		}
	}()

	//chan to tun
	go func() {
		defer func() {
			recover()
		}()
		for {
			if data, ok := <-ts.InputChan; ok && len(data) > 0 {
				logging.Log.Debug("Write Data to Tun...")
				ts.TunConn.Write([]byte(data))
			}
		}
	}()
}

func (ts *TunServer) StartClient(client string, inputChan chan string, outputChan chan string) {
	go func() {
		defer func() {
			recover()
		}()

		for {
			data, ok := <-inputChan
			if !ok {
				return
			}
			if proto, src, dst, err := header.GetBase([]byte(data)); err == nil {
				key := proto + ":" + src + ":" + dst
				ts.RouteMap.Put(key, outputChan)
				ts.InputChan <- data
				logging.Log.Debugf("ToTun: protocol:%v, src:%v, dst:%v", proto, src, dst)
			}
		}
	}()
}

func (ts *TunServer) Stop() {
	logging.Log.Info("TunServer stopped")
	defer func() {
		recover()
	}()

	close(ts.InputChan)
	ts.RouteMap.Clear()
}
