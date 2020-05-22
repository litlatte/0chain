package conductrpc

import (
	"strings"
	"sync"

	"github.com/valyala/gorpc"

	"0chain.net/conductor/config"
)

func init() {
	gorpc.RegisterType(MinerID(""))
	gorpc.RegisterType(SharderID(""))
	gorpc.RegisterType(ViewChange{})
	gorpc.RegisterType(config.Phase(0))
}

// common types
type (
	NodeID string
	Phase  = config.Phase // type alias
)

// ViewChange represents view change information.
type ViewChange struct {
	Node     NodeID   // node that sends the VC
	Round    int64    // view change round
	Miners   []NodeID // magic block miners
	Sharders []NodeID // magic block sharders
}

// known locks
const (
	Locked   = false // should wait
	Unlocked = true  // can join
)

type nodeLock struct {
	lock    bool //
	counter int  //
}

type Server struct {
	server *gorpc.Server
	disp   *gorpc.Dispatcher

	// server events

	// onViewChange occurs where BC made VC (round == view change round)
	onViewChange chan ViewChange
	// onPhase occurs for every phase change
	onPhase chan Phase
	// onAddMiner occurs where miner SC proceed add_miner function
	onAddMiner chan MinerID
	// onAddSharder occurs where miner SC proceed add_sharder function
	onAddSharder chan SharderID

	// onMinerReady used by miners to notify the server that miner has started
	// and ready to register (if needed) in miner SC and start it work. E.g.
	// the miner has started and waits the conductor to enter BC.
	onMinerReady chan MinerID
	// onSharderReady used by sharders to notify the server that sharder has
	// started and ready to register (if needed) in miner SC and start it work.
	//  E.g. the sharder has started and waits the conductor to enter BC.
	onSharderReady chan SharderID

	// add / lock  miner / sharder
	mutex sync.Mutex
	locks map[string]*nodeLock // expected miner/sharder -> locked/unlocked

	quitOnce sync.Once
	quit     chan struct{}
}

// NewServer Conductor RPC server.
func NewServer(address string) (s *Server) {
	s = new(Server)
	s.quit = make(chan struct{})

	// without a buffer
	s.onViewChange = make(chan ViewChange)
	s.onAddMiner = make(chan MinerID)
	s.onAddSharder = make(chan SharderID)
	s.onMinerReady = make(chan MinerID)
	s.onSharderReady = make(chan SharderID)

	s.disp = gorpc.NewDispatcher()
	s.disp.AddFunc("onViewChange", s.onViewChangeHandler)
	s.disp.AddFunc("onPhase", s.onPhaseHandler)
	s.disp.AddFunc("onAddMiner", s.onAddMinerHandler)
	s.disp.AddFunc("onAddSharder", s.onAddSharderHandler)
	s.disp.AddFunc("onMinerReady", s.onMinerReadyHandler)
	s.disp.AddFunc("onSharderReady", s.onSharderReadyHandler)

	s.server = gorpc.NewTCPServer(address, s.disp.NewHandlerFunc())
	return
}

//
// add/lock miner/sharder
//

// AddNode adds miner of sharder and, optionally, locks it.
func (s *Server) AddNode(nodeID string, lock bool) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.locks[nodeID] = &nodeLock{counter: 0, lock: lock}
}

// UnlockNode unlocks a miner.
func (s *Server) UnlockNode(nodeID string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.locks[nodeID] = &nodeLock{counter: 0, lock: Unlocked}
}

func (s *Server) nodeLock(nodeID string) (lock, ok bool) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	var nl *nodeLock
	nl, ok = s.locks[nodeID]
	if !ok {
		return // false, false
	}
	return nl.lock, ok // lock, true
}

// events handling

// OnViewChange events channel. The event occurs where
// BC made VC (round == view change round).
func (s *Server) OnViewChange() chan ViewChange {
	return s.onViewChange
}

// OnPhase events channel. The event occurs where miner SC changes its phase.
func (s *Server) OnPhase() chan Phase {
	return s.onPhase
}

// OnAddMiner events channel. The event occurs
// where miner SC proceed add_miner function.
func (s *Server) OnAddMiner() chan MinerID {
	return s.onAddMiner
}

// OnAddSharder events channel. The event occurs
// where miner SC proceed add_sharder function.
func (s *Server) OnAddSharder() chan SharderID {
	return s.onAddSharder
}

// OnMinerReady used by miners to notify the server that miner has started
// and ready to register (if needed) in miner SC and start it work. E.g.
// the miner has started and waits the conductor to enter BC.
func (s *Server) OnMinerReady() chan MinerID {
	return s.onMinerReady
}

// OnSharderReady used by sharders to notify the server that sharder has
// started and ready to register (if needed) in miner SC and start it work.
//  E.g. the sharder has started and waits the conductor to enter BC.
func (s *Server) OnSharderReady() chan SharderID {
	return s.onSharderReady
}

//
// handlers
//

func (s *Server) onViewChangeHandler(viewChange ViewChange) {
	select {
	case s.onViewChange <- viewChange:
	case <-s.quit:
	}
}

func (s *Server) onPhaseHandler(phase Phase) {
	select {
	case s.onPhase <- phase:
	case <-s.quit:
	}
}

func (s *Server) onAddMinerHandler(minerID MinerID) {
	select {
	case s.onAddMiner <- minerID:
	case <-s.quit:
	}
}

func (s *Server) onAddSharderHandler(sharderID SharderID) {
	select {
	case s.onAddSharder <- sharderID:
	case <-s.quit:
	}
}

func (s *Server) onMinerReadyHandler(minerID MinerID) (join bool) {

	var ok bool
	if join, ok = s.nodeLock(string(minerID)); ok {
		return // don't trigger onMinerReady twice or more times
	}

	select {
	case s.onMinerReady <- minerID:
	case <-s.quit:
	}
	return
}

func (s *Server) onSharderReadyHandler(sharderID SharderID) (join bool) {

	var ok bool
	if join, ok = s.nodeLock(string(sharderID)); ok {
		return // don't trigger onSharderReady twice or more times
	}

	select {
	case s.onSharderReady <- sharderID:
	case <-s.quit:
	}
	return
}

//
// flow
//

// Serve starts the server blocking.
func (s *Server) Serve() (err error) {
	return s.server.Serve()
}

// Close the server waiting.
func (s *Server) Close() {
	s.quitOnce.Do(func() { close(s.quit) })
	s.server.Stop()
}
