package proxy

import (
	"fmt"
	"sync/atomic"
)

var (
	proxyList = []string{
		"http://user-kdb2012_EZH07:tWJjzARtM%2Bk8xr%2B@dc.oxylabs.io:8001",
		"",
	}
	rrIndex uint64
)

func init() {
	for port := 8002; port <= 8050; port++ {
		proxy := "http://user-kdb2012_EZH07:tWJjzARtM%2Bk8xr%2B@dc.oxylabs.io:" +
			fmt.Sprintf("%d", port)
		proxyList = append(proxyList, proxy)
	}
}

func NextProxy() string {
	// simple round-robin
	n := atomic.AddUint64(&rrIndex, 1)
	return proxyList[int(n)%len(proxyList)]
}
