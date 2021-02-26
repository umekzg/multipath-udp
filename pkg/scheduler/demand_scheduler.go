package scheduler

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"time"

	metrics "github.com/rcrowley/go-metrics"
)

type DemandScheduler struct {
	sync.RWMutex

	sendPackets       map[string]metrics.Counter
	sendPacketsOffset map[string]int64

	recvPackets map[string]metrics.Counter
}

func NewDemandScheduler() *DemandScheduler {
	return &DemandScheduler{
		sendPackets:       make(map[string]metrics.Counter),
		sendPacketsOffset: make(map[string]int64),
		recvPackets:       make(map[string]metrics.Counter),
	}
}

func (s *DemandScheduler) SenderInit(receiverAddr *net.UDPAddr) {
	go func() {
		client := &http.Client{Timeout: 3 * time.Second}
		for {
			r, err := client.Get(fmt.Sprintf("http://%s:%d/", receiverAddr.IP.String(), receiverAddr.Port))
			if err != nil {
				fmt.Printf("error getting: %v\n", err)
				time.Sleep(3 * time.Second)
				continue
			}

			s.Lock()
			if err = json.NewDecoder(r.Body).Decode(&s.sendPacketsOffset); err != nil {
				fmt.Printf("error decoding: %v\n", err)
			}
			fmt.Printf("stats ----\n")
			for sender, count := range s.sendPacketsOffset {
				if counter, ok := s.sendPackets[sender]; ok {
					fmt.Printf("%s\tsent: %d\trecv: %d\n", sender, counter.Count(), count)
				}
			}

			s.Unlock()

			time.Sleep(3 * time.Second)
		}
	}()
}

func (s *DemandScheduler) ReceiverInit(port int) {
	go func() {
		mux := http.NewServeMux()
		mux.Handle("/", s)
		fmt.Printf("listening on port %d\n", port)
		if err := http.ListenAndServe("0.0.0.0:"+strconv.Itoa(port), mux); err != nil {
			panic(err)
		}
	}()
}

func (s *DemandScheduler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.RLock()
	defer s.RUnlock()

	data := make(map[string]int64)

	for sender, meter := range s.recvPackets {
		data[sender] = meter.Count()
	}

	out, err := json.Marshal(data)
	if err != nil {
		w.WriteHeader(500)
		return
	}

	w.WriteHeader(200)
	w.Write(out)
}

func (s *DemandScheduler) getInFlight(sender string) int64 {
	inflight := int64(0)
	if counter, ok := s.sendPackets[sender]; ok {
		inflight += counter.Count()
	}
	if offset, ok := s.sendPacketsOffset[sender]; ok {
		inflight -= offset
	}
	return inflight
}

func (s *DemandScheduler) Schedule(senders []string, msg []byte) []string {
	s.Lock()
	defer s.Unlock()

	chosen := senders
	if len(senders) >= 2 {
		// choose the two links with the lowest in-flight packet count.
		sort.Slice(senders, func(i, j int) bool {
			return s.getInFlight(senders[i]) < s.getInFlight(senders[j])
		})
	}

	// update the packet counts
	for _, sender := range chosen {
		meter, ok := s.sendPackets[sender]
		if !ok {
			meter = metrics.NewCounter()
			s.sendPackets[sender] = meter
		}
		meter.Inc(1)
	}

	return chosen
}

func (s *DemandScheduler) OnReceive(sender string, msg []byte) {
	s.Lock()
	defer s.Unlock()

	meter, ok := s.recvPackets[sender]
	if !ok {
		meter = metrics.NewCounter()
		s.recvPackets[sender] = meter
	}
	meter.Inc(1)
}

func (s *DemandScheduler) Close() {

}
