package node

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptrace"
	"time"

	"0chain.net/chaincore/config"
	"0chain.net/core/common"
	"0chain.net/core/datastore"
	"0chain.net/core/logging"
	"go.uber.org/zap"
)

var (
	TimeoutSmallMessage   = 1000 * time.Millisecond
	TimeoutLargeMessage   = 3000 * time.Millisecond
	LargeMessageThreshold = 10 * 1024
)

var compDecomp common.CompDe

func init() {
	//compDecomp = common.NewSnappyCompDe()
	compDecomp = common.NewZStdCompDe()
}

//SetTimeoutSmallMessage - set the timeout for small message
func SetTimeoutSmallMessage(ts time.Duration) {
	TimeoutSmallMessage = ts
}

//SetTimeoutLargeMessage - set the timeout for large message
func SetTimeoutLargeMessage(ts time.Duration) {
	TimeoutLargeMessage = ts
}

//SetLargeMessageThresholdSize - set the size above which it is considered as a large message
func SetLargeMessageThresholdSize(size int) {
	LargeMessageThreshold = size
}

var pullDataRequestor EntityRequestor

/*SetupN2NHandlers - Setup all the node 2 node communiations*/
func SetupN2NHandlers() {
	http.HandleFunc("/v1/_n2n/entity/post", common.N2NRateLimit(ToN2NReceiveEntityHandler(datastore.PrintEntityHandler, nil)))
	http.HandleFunc(pullURL, common.N2NRateLimit(ToN2NSendEntityHandler(PushToPullHandler)))
	options := &SendOptions{Timeout: TimeoutLargeMessage, CODEC: CODEC_MSGPACK, Compress: true}
	pullDataRequestor = RequestEntityHandler(pullURL, options, nil)
}

var (
	HeaderRequestTimeStamp      = "X-Request-Timestamp"
	HeaderRequestHash           = "X-Request-Hash"
	HeaderRequestRelayLength    = "X-Request-Relay-Length"
	HeaderRequestMaxRelayLength = "X-Request-Max-Relay-Length"
	HeaderRequestEntityName     = "X-Request-Entity-Name"
	HeaderRequestEntityID       = "X-Request-Entity-ID"
	HeaderRequestChainID        = "X-Chain-Id"
	HeaderRequestCODEC          = "X-Chain-CODEC"
	HeaderRequestToPull         = "X-Request-To-Pull"

	HeaderInitialNodeID        = "X-Initial-Node-Id"
	HeaderNodeID               = "X-Node-Id"
	HeaderNodeRequestSignature = "X-Node-Request-Signature"
)

//N2NTimeTolerance - only a message signed within this time is considered valid
const N2NTimeTolerance = 4 // in seconds

const (
	CODEC_JSON    = 0
	CODEC_MSGPACK = 1
)

const (
	CodecJSON    = "JSON"
	CodecMsgpack = "Msgpack"
)

/*SendOptions - options to tune how the messages are sent within the network */
type SendOptions struct {
	Timeout            time.Duration
	MaxRelayLength     int64
	CurrentRelayLength int64
	Compress           bool
	InitialNodeID      string
	CODEC              int
	Pull               bool
}

/*MessageFilterI - tells wether the given message should be processed or not
* This will be useful since if for example a notarized block is received multiple times
* the cost of decoding and decompressing can be avoided */
type MessageFilterI interface {
	AcceptMessage(entityName string, entityID string) bool
}

/*ReceiveOptions - options to tune how the messages are received within the network */
type ReceiveOptions struct {
	MessageFilter MessageFilterI
}

var httpClient *http.Client

var n2nTrace = &httptrace.ClientTrace{}

func init() {
	var transport *http.Transport
	transport = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).DialContext,
		MaxIdleConns:          1000,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		MaxIdleConnsPerHost:   5,
	}
	httpClient = &http.Client{Transport: transport}

	n2nTrace.GotConn = func(connInfo httptrace.GotConnInfo) {
		fmt.Printf("GOT conn: %+v\n", connInfo)
	}
}

/*SENDER - key used to get the connection object from the context */
const SENDER common.ContextKey = "node.sender"

/*WithNode takes a context and adds a connection value to it */
func WithNode(ctx context.Context, node *Node) context.Context {
	return context.WithValue(ctx, SENDER, node)
}

/*GetSender returns a connection stored in the context which got created via WithConnection */
func GetSender(ctx context.Context) *Node {
	return ctx.Value(SENDER).(*Node)
}

/*SetHeaders - set common request headers */
func SetHeaders(req *http.Request) {
	req.Header.Set(HeaderRequestChainID, config.GetServerChainID())
	req.Header.Set(HeaderNodeID, Self.Underlying().GetKey())
}

func getHashData(clientID datastore.Key, ts common.Timestamp, key datastore.Key) string {
	return clientID + ":" + common.TimeToString(ts) + ":" + key
}

var NoDataErr = common.NewError("no_data", "No data")

func readAndClose(reader io.ReadCloser) {
	io.Copy(ioutil.Discard, reader)
	reader.Close()
}

func getDataAndClose(reader io.ReadCloser) []byte {
	buf := &bytes.Buffer{}
	io.Copy(buf, reader)
	reader.Close()
	return buf.Bytes()
}

func getRequestEntity(r *http.Request, reader io.Reader, entityMetadata datastore.EntityMetadata) (datastore.Entity, error) {
	buffer := reader
	if r.Header.Get("Content-Encoding") == compDecomp.Encoding() {
		cbuffer := new(bytes.Buffer)
		cbuffer.ReadFrom(buffer)
		cbytes := cbuffer.Bytes()
		if len(cbytes) == 0 {
			return nil, NoDataErr
		}
		cbytes, err := compDecomp.Decompress(cbytes)
		if err != nil {
			logging.N2n.Error("decoding", zap.String("encoding", compDecomp.Encoding()), zap.Error(err))
			return nil, err
		}
		buffer = bytes.NewReader(cbytes)
	}
	return getEntity(r.Header.Get(HeaderRequestCODEC), buffer, entityMetadata)
}

func getResponseEntity(resp *http.Response, reader io.Reader, entityMetadata datastore.EntityMetadata) (int, datastore.Entity, error) {
	buffer := reader
	var size int
	if resp.Header.Get("Content-Encoding") == compDecomp.Encoding() {
		cbuffer := new(bytes.Buffer)
		cbuffer.ReadFrom(reader)
		size = cbuffer.Len()
		cbytes, err := compDecomp.Decompress(cbuffer.Bytes())
		if err != nil {
			logging.N2n.Error("decoding", zap.String("encoding", compDecomp.Encoding()), zap.Error(err))
			return size, nil, err
		}
		buffer = bytes.NewReader(cbytes)
	}
	entity, err := getEntity(resp.Header.Get(HeaderRequestCODEC), buffer, entityMetadata)
	return size, entity, err
}

func getEntity(codec string, reader io.Reader, entityMetadata datastore.EntityMetadata) (datastore.Entity, error) {
	entity := entityMetadata.Instance()
	switch codec {
	case CodecMsgpack:
		if err := datastore.FromMsgpack(reader, entity.(datastore.Entity)); err != nil {
			logging.N2n.Error("msgpack decoding", zap.Error(err))
			return nil, err
		}
		return entity, nil
	case CodecJSON:
		if err := datastore.FromJSON(reader, entity.(datastore.Entity)); err != nil {
			logging.N2n.Error("json decoding", zap.Error(err))
			return nil, err
		}
		return entity, nil
	default:
		if err := datastore.FromJSON(reader, entity.(datastore.Entity)); err != nil {
			logging.N2n.Error("json decoding", zap.Error(err))
			return nil, err
		}
		return entity, nil
	}
	logging.N2n.Error("unknown_encoding", zap.String("encoding", codec))
	return nil, common.NewError("unkown_encoding", "unknown encoding")
}

func getResponseData(options *SendOptions, entity datastore.Entity) *bytes.Buffer {
	var buffer *bytes.Buffer
	if options.CODEC == datastore.CodecJSON {
		buffer = datastore.ToJSON(entity)
	} else {
		buffer = datastore.ToMsgpack(entity)
	}
	if options.Compress {
		cbytes := compDecomp.Compress(buffer.Bytes())
		buffer = bytes.NewBuffer(cbytes)
	}
	return buffer
}

func validateChain(sender *Node, r *http.Request) bool {
	chainID := r.Header.Get(HeaderRequestChainID)
	return config.GetServerChainID() == chainID
}

func validateEntityMetadata(sender *Node, r *http.Request) bool {
	if r.URL.Path == pullURL {
		return true
	}
	entityName := r.Header.Get(HeaderRequestEntityName)
	if entityName == "" {
		logging.N2n.Error("message received - entity name blank", zap.Int("from", sender.SetIndex),
			zap.Int("to", Self.Underlying().SetIndex), zap.String("handler", r.RequestURI))
		return false
	}
	entityMetadata := datastore.GetEntityMetadata(entityName)
	if entityMetadata == nil {
		logging.N2n.Error("message received - unknown entity", zap.Int("from", sender.SetIndex),
			zap.Int("to", Self.Underlying().SetIndex), zap.String("handler", r.RequestURI), zap.String("entity", entityName))
		return false
	}
	return true
}
