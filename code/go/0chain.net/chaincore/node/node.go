package node

import (
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rcrowley/go-metrics"

	"0chain.net/chaincore/client"
	"0chain.net/chaincore/config"
	"0chain.net/core/common"
	"0chain.net/core/datastore"
	"0chain.net/core/encryption"
	"0chain.net/core/viper"
)

var nodes = make(map[string]*Node)
var nodesMutex = &sync.RWMutex{}

/*RegisterNode - register a node to a global registry
* We need to keep track of a global register of nodes. This is required to ensure we can verify a signed request
* coming from a node
 */
func RegisterNode(node *Node) {
	nodesMutex.Lock()
	defer nodesMutex.Unlock()
	nodes[node.GetKey()] = node
}

/*DeregisterNode - deregister a node */
func DeregisterNode(nodeID string) {

	return // TODO (sfxdx): temporary disable nodes deregistering

	nodesMutex.Lock()
	defer nodesMutex.Unlock()
	delete(nodes, nodeID)
}

// DeregisterNodes unregisters all nodes not from given list.
func DeregisterNodes(keep map[string]struct{}) {
	return // never deregister nodes for now

	nodesMutex.Lock()
	defer nodesMutex.Unlock()

	var newNodes = make(map[string]*Node)
	for k := range keep {
		if n, ok := nodes[k]; ok {
			newNodes[k] = n
		}
	}

	nodes = newNodes // replace with new list
}

// CopyNodes returns copy of all registered nodes.
func CopyNodes() (cp map[string]*Node) {
	nodesMutex.RLock()
	defer nodesMutex.RUnlock()

	cp = make(map[string]*Node, len(nodes))
	for k, v := range nodes {
		cp[k] = v
	}

	return
}

func GetMinerNodesKeys() []string {
	nodesMutex.RLock()
	defer nodesMutex.RUnlock()
	var keys []string
	for k, n := range nodes {
		if n.Type == NodeTypeMiner {
			keys = append(keys, k)
		}
	}
	return keys
}

/*GetNode - get the node from the registery */
func GetNode(nodeID string) *Node {
	nodesMutex.RLock()
	defer nodesMutex.RUnlock()
	return nodes[nodeID]
}

var (
	NodeStatusActive   = 0
	NodeStatusInactive = 1
)

var (
	NodeTypeMiner   int8 = 0
	NodeTypeSharder int8 = 1
	NodeTypeBlobber int8 = 2
)

var NodeTypeNames = common.CreateLookups("m", "Miner", "s", "Sharder", "b", "Blobber")

/*Node - a struct holding the node information */
type Node struct {
	client.Client  `yaml:",inline"`
	N2NHost        string        `json:"n2n_host" yaml:"n2n_ip"`
	Host           string        `json:"host" yaml:"public_ip"`
	Port           int           `json:"port" yaml:"port"`
	Path           string        `json:"path" yaml:"path"`
	Type           int8          `json:"type"`
	Description    string        `json:"description" yaml:"description"`
	SetIndex       int           `json:"set_index" yaml:"set_index"`
	Status         int           `json:"status"`
	InPrevMB       bool          `json:"in_prev_mb"`
	LastActiveTime time.Time     `json:"-" msgpack:"-"`
	ErrorCount     int64         `json:"-" msgpack:"-"`
	CommChannel    chan struct{} `json:"-" msgpack:"-"`
	//These are approximiate as we are not going to lock to update
	sent       int64 `json:"-" msgpack:"-"` // messages sent to this node
	sendErrors int64 `json:"-" msgpack:"-"` // failed message sent to this node
	received   int64 `json:"-" msgpack:"-"` // messages received from this node

	TimersByURI map[string]metrics.Timer     `json:"-" msgpack:"-"`
	SizeByURI   map[string]metrics.Histogram `json:"-" msgpack:"-"`

	largeMessageSendTime uint64
	smallMessageSendTime uint64

	LargeMessagePullServeTime float64 `json:"-" msgpack:"-"`
	SmallMessagePullServeTime float64 `json:"-" msgpack:"-"`

	mutex sync.RWMutex `json:"-" msgpack:"-"`

	ProtocolStats interface{} `json:"-" msgpack:"-"`

	idBytes []byte

	Info Info `json:"info"`
}

/*Provider - create a node object */
func Provider() *Node {
	node := &Node{}
	node.TimersByURI = make(map[string]metrics.Timer, 10)
	node.SizeByURI = make(map[string]metrics.Histogram, 10)
	node.setupCommChannel()
	return node
}

func Setup(node *Node) {
	// queue up at most these many messages to a node
	// because of this, we don't want the status monitoring to use this communication layer
	node.mutex.Lock()
	node.setupCommChannel()
	node.TimersByURI = make(map[string]metrics.Timer, 10)
	node.SizeByURI = make(map[string]metrics.Histogram, 10)
	node.mutex.Unlock()
	node.ComputeProperties()
	Self.SetNodeIfPublicKeyIsEqual(node)
}

func (n *Node) setupCommChannel() {
	// queue up at most these many messages to a node
	// because of this, we don't want the status monitoring to use this
	// communication layer
	if n.CommChannel == nil {
		n.CommChannel = make(chan struct{}, 15)
	}
}

// GetErrorCount asynchronously.
func (n *Node) GetErrorCount() int64 {
	n.mutex.RLock()
	defer n.mutex.RUnlock()
	return n.ErrorCount
}

// AddSendErrors add sent errors
func (n *Node) AddSendErrors(num int64) {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	n.sendErrors += num
	//n.recordChangesFunc(func(tn *Node) {
	//	tn.sendErrors += num
	//	fmt.Println("add send errors")
	//})
}

// GetSendErrors returns the send errors num
func (n *Node) GetSendErrors() int64 {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	return n.sendErrors
}

// SetErrorCount asynchronously.
func (n *Node) SetErrorCount(ec int64) {
	n.mutex.Lock()
	defer n.mutex.Unlock()

	n.ErrorCount = ec
}

// AddErrorCount add given value to errors count asynchronously.
func (n *Node) AddErrorCount(ecd int64) {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	n.ErrorCount += ecd
}

// GetNodeInfo returns the node info
func (n *Node) GetNodeInfo() Info {
	n.mutex.RLock()
	defer n.mutex.RUnlock()
	return n.Info
}

// SetNodeInfo updates the node info
func (n *Node) SetNodeInfo(info *Info) {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	n.Info = *info
}

// GetStatus asynchronously.
func (n *Node) GetStatus() int {
	n.mutex.RLock()
	defer n.mutex.RUnlock()

	return n.Status
}

// SetStatus asynchronously.
func (n *Node) SetStatus(st int) {
	n.mutex.Lock()
	defer n.mutex.Unlock()

	n.Status = st
}

// GetLastActiveTime asynchronously.
func (n *Node) GetLastActiveTime() time.Time {
	n.mutex.RLock()
	defer n.mutex.RUnlock()

	return n.LastActiveTime
}

// SetLastActiveTime asynchronously.
func (n *Node) SetLastActiveTime(lat time.Time) {
	n.mutex.Lock()
	defer n.mutex.Unlock()

	n.LastActiveTime = lat
}

/*Equals - if two nodes are equal. Only check by id, we don't accept configuration from anyone */
func (n *Node) Equals(n2 *Node) bool {
	if datastore.IsEqual(n.GetKey(), n2.GetKey()) {
		return true
	}
	if n.Port == n2.Port && n.Host == n2.Host {
		return true
	}
	return false
}

/*Print - print node's info that is consumable by Read */
func (n *Node) Print(w io.Writer) {
	fmt.Fprintf(w, "%v,%v,%v,%v,%v\n", n.GetNodeType(), n.Host, n.Port, n.GetKey(), n.PublicKey)
}

/*Read - read a node config line and create the node */
func Read(line string) (*Node, error) {
	node := Provider()
	fields := strings.Split(line, ",")
	if len(fields) != 5 {
		return nil, common.NewError("invalid_num_fields", fmt.Sprintf("invalid number of fields [%v]", line))
	}
	switch fields[0] {
	case "m":
		node.Type = NodeTypeMiner
	case "s":
		node.Type = NodeTypeSharder
	case "b":
		node.Type = NodeTypeBlobber
	default:
		return nil, common.NewError("unknown_node_type", fmt.Sprintf("Unkown node type %v", fields[0]))
	}
	node.Host = fields[1]
	if node.Host == "" {
		if node.Port != config.Configuration.Port {
			node.Host = config.Configuration.Host
		} else {
			panic(fmt.Sprintf("invalid node setup for %v\n", node.GetKey()))
		}
	}

	port, err := strconv.ParseInt(fields[2], 10, 32)
	if err != nil {
		return nil, err
	}
	node.Port = int(port)
	node.SetID(fields[3])
	node.Client.SetPublicKey(fields[4])
	hash := encryption.Hash(node.PublicKeyBytes)
	if node.ID != hash {
		return nil, common.NewError("invalid_client_id", fmt.Sprintf("public key: %v, client_id: %v, hash: %v\n", node.PublicKey, node.ID, hash))
	}
	node.ComputeProperties()
	Self.SetNodeIfPublicKeyIsEqual(node)
	return node, nil
}

/*NewNode - read a node config line and create the node */
func NewNode(nc map[interface{}]interface{}) (*Node, error) {
	node := Provider()
	node.Type = nc["type"].(int8)
	node.Host = nc["public_ip"].(string)
	node.N2NHost = nc["n2n_ip"].(string)
	node.Port = nc["port"].(int)
	node.SetID(nc["id"].(string))
	if description, ok := nc["description"]; ok {
		node.Description = description.(string)
	} else {
		node.Description = node.GetNodeType() + node.GetKey()[:6]
	}

	node.Client.SetPublicKey(nc["public_key"].(string))
	hash := encryption.Hash(node.PublicKeyBytes)
	if node.ID != hash {
		return nil, common.NewErrorf("invalid_client_id",
			"public key: %v, client_id: %v, hash: %v\n", node.PublicKey,
			node.ID, hash)
	}
	node.ComputeProperties()
	Self.SetNodeIfPublicKeyIsEqual(node)
	return node, nil
}

/*ComputeProperties - implement entity interface */
func (n *Node) ComputeProperties() {
	n.Client.ComputeProperties()
	if n.Host == "" {
		n.Host = "localhost"
	}
	if n.N2NHost == "" {
		n.N2NHost = n.Host
	}
}

/*GetURLBase - get the end point base */
func (n *Node) GetURLBase() string {
	return fmt.Sprintf("http://%v:%v", n.Host, n.Port)
}

/*GetN2NURLBase - get the end point base for n2n communication */
func (n *Node) GetN2NURLBase() string {
	return fmt.Sprintf("http://%v:%v", n.N2NHost, n.Port)
}

/*GetStatusURL - get the end point where to ping for the status */
func (n *Node) GetStatusURL() string {
	return fmt.Sprintf("%v/_nh/status", n.GetN2NURLBase())
}

/*GetNodeType - as a string */
func (n *Node) GetNodeType() string {
	return NodeTypeNames[n.Type].Code
}

/*GetNodeTypeName - get the name of this node type */
func (n *Node) GetNodeTypeName() string {
	return NodeTypeNames[n.Type].Value
}

//Grab - grab a slot to send message
func (n *Node) Grab() {
	n.CommChannel <- struct{}{}

	n.mutex.Lock()
	defer n.mutex.Unlock()

	n.sent++
}

//Release - release a slot after sending the message
func (n *Node) Release() {
	<-n.CommChannel
}

// GetSent returns the sent num
func (n *Node) GetSent() int64 {
	n.mutex.RLock()
	defer n.mutex.RUnlock()
	return n.sent
}

// GetReceived returns the received num
func (n *Node) GetReceived() int64 {
	n.mutex.RLock()
	defer n.mutex.RUnlock()
	return n.received
}

// AddReceived increases received num
func (n *Node) AddReceived(num int64) {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	n.received += num
}

//GetTimer - get the timer
func (n *Node) GetTimer(uri string) metrics.Timer {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	return n.getTimer(uri)
}

func (n *Node) getTimer(uri string) metrics.Timer {
	timer, ok := n.TimersByURI[uri]
	if !ok {
		timerID := fmt.Sprintf("%v.%v.time", n.ID, uri)
		timer = metrics.GetOrRegisterTimer(timerID, nil)
		n.TimersByURI[uri] = timer
	}
	return timer
}

//GetSizeMetric - get the size metric
func (n *Node) GetSizeMetric(uri string) metrics.Histogram {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	return n.getSizeMetric(uri)
}

//getSizeMetric - get the size metric
func (n *Node) getSizeMetric(uri string) metrics.Histogram {
	metric, ok := n.SizeByURI[uri]
	if !ok {
		metricID := fmt.Sprintf("%v.%v.size", n.ID, uri)
		metric = metrics.NewHistogram(metrics.NewUniformSample(256))
		n.SizeByURI[uri] = metric
		metrics.Register(metricID, metric)
	}
	return metric
}

//GetLargeMessageSendTime - get the time it takes to send a large message to this node
func (n *Node) GetLargeMessageSendTime() float64 {
	return math.Float64frombits(atomic.LoadUint64(&n.largeMessageSendTime))
}

func (n *Node) GetLargeMessageSendTimeSec() float64 {
	return math.Float64frombits(atomic.LoadUint64(&n.largeMessageSendTime)) / 1000000
}

func (n *Node) setLargeMessageSendTime(value float64) {
	atomic.StoreUint64(&n.largeMessageSendTime, math.Float64bits(value))
}

// GetSmallMessageSendTimeSec gets the time it takes to send a small message to this node
func (n *Node) GetSmallMessageSendTimeSec() float64 {
	return math.Float64frombits(atomic.LoadUint64(&n.smallMessageSendTime)) / 1000000
}

func (n *Node) GetSmallMessageSendTime() float64 {
	return math.Float64frombits(atomic.LoadUint64(&n.smallMessageSendTime))
}

func (n *Node) setSmallMessageSendTime(value float64) {
	atomic.StoreUint64(&n.smallMessageSendTime, math.Float64bits(value))
}

func (n *Node) updateMessageTimings() {
	n.updateSendMessageTimings()
	n.updateRequestMessageTimings()
}

func (n *Node) updateSendMessageTimings() {
	n.mutex.Lock()
	defer n.mutex.Unlock()

	var minval = math.MaxFloat64
	var maxval float64
	var maxCount int64
	for uri, timer := range n.TimersByURI {
		if timer.Count() == 0 {
			continue
		}
		if isGetRequest(uri) {
			continue
		}
		if sizer, ok := n.SizeByURI[uri]; ok {
			tv := timer.Mean()
			sv := sizer.Mean()
			sc := sizer.Count()
			if int(sv) < LargeMessageThreshold {
				if tv < minval {
					minval = tv
				}
			} else {
				if sc > maxCount {
					maxval = tv
					maxCount = sc
				}
			}
		}
	}
	if minval > maxval {
		if minval != math.MaxFloat64 {
			maxval = minval
		} else {
			minval = maxval
		}
	}
	n.setLargeMessageSendTime(maxval)
	n.setSmallMessageSendTime(minval)
}

func (n *Node) updateRequestMessageTimings() {
	n.mutex.Lock()
	defer n.mutex.Unlock()

	var minval = math.MaxFloat64
	var maxval float64
	var minSize = math.MaxFloat64
	var maxSize float64
	for uri, timer := range n.TimersByURI {
		if timer.Count() == 0 {
			continue
		}
		if !isGetRequest(uri) {
			continue
		}
		v := timer.Mean()
		if sizer, ok := n.SizeByURI[uri]; ok {
			if sizer.Mean() == 0 {
				continue
			}
			if sizer.Mean() > maxSize {
				maxSize = sizer.Mean()
				if v > maxval {
					maxval = v
				}
			}
			if sizer.Mean() < minSize {
				minSize = sizer.Mean()
				if v < minval {
					minval = v
				}
			}
		}
	}
	if minval > maxval {
		if minval != math.MaxFloat64 {
			maxval = minval
		} else {
			minval = maxval
		}
	}
	n.LargeMessagePullServeTime = maxval
	n.SmallMessagePullServeTime = minval
}

//ReadConfig - read configuration from the default config
func ReadConfig() {
	SetTimeoutSmallMessage(viper.GetDuration("network.timeout.small_message") * time.Millisecond)
	SetTimeoutLargeMessage(viper.GetDuration("network.timeout.large_message") * time.Millisecond)
	SetMaxConcurrentRequests(viper.GetInt("network.max_concurrent_requests"))
	SetLargeMessageThresholdSize(viper.GetInt("network.large_message_th_size"))
}

//SetID - set the id of the node
func (n *Node) SetID(id string) error {
	n.ID = id
	bytes, err := hex.DecodeString(id)
	if err != nil {
		return err
	}
	n.idBytes = bytes
	return nil
}

//IsActive - returns if this node is active or not
func (n *Node) IsActive() bool {
	n.mutex.RLock()
	defer n.mutex.RUnlock()

	return n.Status == NodeStatusActive
}

func serveMetricKey(uri string) string {
	return "p?" + uri
}

func isPullRequestURI(uri string) bool {
	return strings.HasPrefix(uri, "p?")
}

func isGetRequest(uri string) bool {
	if strings.HasPrefix(uri, "p?") {
		return true
	}
	return strings.HasSuffix(uri, "/get")
}

//GetPseudoName - create a pseudo name that is unique in the current active set
func (n *Node) GetPseudoName() string {
	return fmt.Sprintf("%v%.3d", n.GetNodeTypeName(), n.SetIndex)
}

//GetOptimalLargeMessageSendTime - get the push or pull based optimal large message send time
func (n *Node) GetOptimalLargeMessageSendTime() float64 {
	return n.getOptimalLargeMessageSendTime() / 1000000
}

func (n *Node) getOptimalLargeMessageSendTime() float64 {
	p2ptime := getPushToPullTime(n)
	sendTime := n.GetLargeMessageSendTime()
	if p2ptime < sendTime {
		return p2ptime
	}
	if sendTime == 0 {
		return p2ptime
	}
	return sendTime
}

func (n *Node) getTime(uri string) float64 {
	pullTimer := n.GetTimer(uri)
	return pullTimer.Mean()
}

func (n *Node) SetNode(old *Node) {
	// Copy timers and size to new map from clone
	if n == old {
		return
	}

	clone := old.Clone()
	n.mutex.Lock()
	defer n.mutex.Unlock()

	// NOTE:
	// We can avoid copying and simply assign the new maps if
	// n.TimersByURI and n.SizeByURI are expected to be empty while
	// calling this method
	n.TimersByURI = make(map[string]metrics.Timer, len(clone.TimersByURI))
	n.SizeByURI = make(map[string]metrics.Histogram, len(clone.SizeByURI))
	for k, v := range clone.TimersByURI {
		n.TimersByURI[k] = v
	}
	for k, v := range clone.SizeByURI {
		n.SizeByURI[k] = v
	}

	n.sent = clone.sent
	n.sendErrors = clone.sendErrors
	n.received = clone.received
	n.largeMessageSendTime = clone.largeMessageSendTime
	n.setLargeMessageSendTime(clone.GetLargeMessageSendTime())
	n.setSmallMessageSendTime(clone.GetSmallMessageSendTime())
	n.LargeMessagePullServeTime = clone.LargeMessagePullServeTime
	n.SmallMessagePullServeTime = clone.SmallMessagePullServeTime
	if clone.ProtocolStats != nil {
		n.ProtocolStats = clone.ProtocolStats.(interface{ Clone() interface{} }).Clone()
	}
	n.Info = clone.Info
	n.Status = clone.Status
}

func (n *Node) SetInfo(info Info) {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	n.Info = info
}

// GetInfo returns copy Info.
func (n *Node) GetInfo() Info {
	n.mutex.RLock()
	defer n.mutex.RUnlock()
	return n.Info
}

// Clone returns a clone of Node instance.
func (n *Node) Clone() *Node {
	n.mutex.RLock()
	defer n.mutex.RUnlock()

	clone := &Node{
		N2NHost:                   n.N2NHost,
		Host:                      n.Host,
		Port:                      n.Port,
		Path:                      n.Path,
		Type:                      n.Type,
		Description:               n.Description,
		SetIndex:                  n.SetIndex,
		Status:                    n.Status,
		InPrevMB:                  n.InPrevMB,
		LastActiveTime:            n.LastActiveTime,
		ErrorCount:                n.ErrorCount,
		sent:                      n.sent,
		sendErrors:                n.sendErrors,
		received:                  n.received,
		largeMessageSendTime:      n.largeMessageSendTime,
		smallMessageSendTime:      n.smallMessageSendTime,
		LargeMessagePullServeTime: n.LargeMessagePullServeTime,
		SmallMessagePullServeTime: n.SmallMessagePullServeTime,
		CommChannel:               make(chan struct{}, 15),
	}

	cc := n.Client.Clone()
	if cc != nil {
		clone.Client = *cc
	}

	clone.TimersByURI = make(map[string]metrics.Timer, len(n.TimersByURI))
	for k, v := range n.TimersByURI {
		clone.TimersByURI[k] = v
	}

	clone.SizeByURI = make(map[string]metrics.Histogram, len(n.SizeByURI))
	for k, v := range n.SizeByURI {
		clone.SizeByURI[k] = v
	}

	clone.idBytes = make([]byte, len(n.idBytes))
	copy(clone.idBytes, n.idBytes)

	ps, ok := n.ProtocolStats.(interface{ Clone() interface{} })
	if ok {
		clone.ProtocolStats = ps.Clone()
	}

	return clone
}
