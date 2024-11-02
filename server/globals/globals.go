package globals

import (
	"time"
	"github.com/tinode/chat/server/auth"
	"github.com/tinode/chat/server/datamodel"
	"google.golang.org/grpc"
	"github.com/tinode/chat/server/store/types"
)

type SessionStore interface {
	NewSession(conn any, sid string) (*Session, int)
	Get(sid string) *Session
	Delete(s *Session)
	Range(f func(sid string, s *Session) bool)
	Shutdown()
	EvictUser(uid types.Uid, skipSid string)
	NodeRestarted(nodeName string, fingerprint int64)
}

type Subscription interface {}

type Session interface {
	AddSub(topic string, sub *Subscription)
	GetSub(topic string) *Subscription
	DelSub(topic string)
	CountSub() int
	UnsubAll()
	IsMultiplex() bool
	IsProxy() bool
	IsCluster() bool
	ScheduleClusterWriteLoop()
	SupportsMessageBatching()
	QueueOutBatch(msgs []*datamodel.ServerComMessage) bool
	QueueOut(msg *datamodel.ServerComMessage) bool
	QueueOutBytes(data []byte) bool
	MaybeScheduleClusterWriteLoop()
	DetachSession(fromTopic string)
	StopSession(data any)
	CleanUp(expired bool)
	DispatchRaw(raw []byte)
	Dispatch(msg *datamodel.ClientComMessage)
	Subscribe(msg *datamodel.ClientComMessage)
	Leave(msg *datamodel.ClientComMessage)
	Publish(msg *datamodel.ClientComMessage)
	Hello(msg *datamodel.ClientComMessage)
	Acc(msg *datamodel.ClientComMessage)
	Login(msg *datamodel.ClientComMessage)
	AuthSecretReset(params []byte) error
	OnLogin(msgID string, timestamp time.Time, rec *auth.Rec, missing []string) *datamodel.ServerComMessage
	Get(msg *datamodel.ClientComMessage)
	Set(msg *datamodel.ClientComMessage)
	Del(msg *datamodel.ClientComMessage)
	Note(msg *datamodel.ClientComMessage)
	ExpandTopicName(msg *datamodel.ClientComMessage)
	SerializeAndUpdateStats(msg *datamodel.ServerComMessage) any
	OnBackgroundTimer()
}

type Hub interface {
	TopicGet(name string) *Topic
	TopicPut(name string, t *Topic)
	TopicDel(name string)
}

type Topic interface {}

type UserCacheReq interface {}

type Cluster interface {
	TopicMaster(msg *ClusterReq, rejected *bool) error
	TopicProxy(msg *ClusterResp, unused *bool) error
	Route(msg *ClusterRoute, rejected *bool) error
	UserCacheUpdate(msg *UserCacheReq, rejected *bool) error
	Ping(ping *ClusterPing, unused *bool) error
	RouteUserReq(req *UserCacheReq) error
	NodeForTopic(topic string) *ClusterNode
	IsRemoteTopic(topic string) bool
	GenLocalTopicName() string
	IsPartitioned() bool
	MakeClusterReq(reqType ProxyReqType, msg *datamodel.ClientComMessage, topic string, sess *Session) *ClusterReq
	RouteToTopicMaster(reqType ProxyReqType, msg *datamodel.ClientComMessage, topic string, sess *Session) error
	RouteToTopicIntraCluster(topic string, msg *datamodel.ServerComMessage, sess *Session) error
	TopicProxyGone(topicName string) error

}

type ProxyReqType int

type ClusterReq interface {}
type ClusterRoute interface {}
type ClusterResp interface {}
type ClusterPing interface {}

type ClusterNode interface {
	AsyncRpcLoop()
P2mSenderLoop()

}

type Plugin interface {}

var Globals struct {
	// Topics cache and processing.
	hub *Hub
	// Indicator that shutdown is in progress
	shuttingDown bool
	// Sessions cache.
	sessionStore *SessionStore
	// Cluster data.
	cluster *Cluster
	// gRPC server.
	grpcServer *grpc.Server
	// Plugins.
	plugins []Plugin
	// Runtime statistics communication channel.
	statsUpdate chan *varUpdate
	// Users cache communication channel.
	usersUpdate chan *UserCacheReq

	// Credential validators.
	validators map[string]CredValidator
	// Credential validator config to pass to clients.
	validatorClientConfig map[string][]string
	// Validators required for each auth level.
	authValidators map[auth.Level][]string

	// Salt used for signing API key.
	apiKeySalt []byte
	// Tag namespaces (prefixes) which are immutable to the client.
	immutableTagNS map[string]bool
	// Tag namespaces which are immutable on User and partially mutable on Topic:
	// user can only mutate tags he owns.
	maskedTagNS map[string]bool

	// Add Strict-Transport-Security to headers, the value signifies age.
	// Empty string "" turns it off
	tlsStrictMaxAge string
	// Listen for connections on this address:port and redirect them to HTTPS port.
	tlsRedirectHTTP string
	// Maximum message size allowed from peer.
	maxMessageSize int64
	// Maximum number of group topic subscribers.
	maxSubscriberCount int
	// Maximum number of indexable tags.
	maxTagCount int
	// If true, ordinary users cannot delete their accounts.
	permanentAccounts bool

	// Maximum allowed upload size.
	maxFileUploadSize int64
	// Periodicity of a garbage collector for abandoned media uploads.
	mediaGcPeriod time.Duration

	// Prioritize X-Forwarded-For header as the source of IP address of the client.
	useXForwardedFor bool

	// Country code to assign to sessions by default.
	defaultCountryCode string

	// Time before the call is dropped if not answered.
	callEstablishmentTimeout int

	// ICE servers config (video calling)
	iceServers []IceServer

	// Websocket per-message compression negotiation is enabled.
	wsCompression bool

	// URL of the main endpoint.
	// TODO: implement file-serving API for gRPC and remove this feature.
	servingAt string
}

type varUpdate struct {
	// Name of the variable to update
	varname string
	// Value to publish (int, float, etc.)
	value any
	// Treat the count as an increment as opposite to the final value.
	inc bool
}

// Async publish int variable.
func StatsSet(name string, val int64) {
	if Globals.statsUpdate != nil {
		select {
		case Globals.statsUpdate <- &varUpdate{name, val, false}:
		default:
		}
	}
}

// Async publish a value (add a sample) to a histogram variable.
func StatsAddHistSample(name string, val float64) {
	if Globals.statsUpdate != nil {
		select {
		case Globals.statsUpdate <- &varUpdate{varname: name, value: val}:
		default:
		}
	}
}

// Initialize stats reporting through expvar.
func StatsInit(mux *http.ServeMux, path string) {
	if path == "" || path == "-" {
		return
	}

	mux.Handle(path, expvar.Handler())
	Globals.statsUpdate = make(chan *varUpdate, 1024)

	start := time.Now()
	expvar.Publish("Uptime", expvar.Func(func() any {
		return time.Since(start).Seconds()
	}))
	expvar.Publish("NumGoroutines", expvar.Func(func() any {
		return runtime.NumGoroutine()
	}))

	go statsUpdater()

	logs.Info.Printf("stats: variables exposed at '%s'", path)
}


// Stop publishing stats.
func statsShutdown() {
	if Globals.statsUpdate != nil {
		Globals.statsUpdate <- nil
	}
}

// The go routine which actually publishes stats updates.
func statsUpdater() {
	for upd := range Globals.statsUpdate {
		if upd == nil {
			Globals.statsUpdate = nil
			// Dont' care to close the channel.
			break
		}

		// Handle var update
		if ev := expvar.Get(upd.varname); ev != nil {
			switch v := ev.(type) {
			case *expvar.Int:
				count := upd.value.(int64)
				if upd.inc {
					v.Add(count)
				} else {
					v.Set(count)
				}
			case *histogram:
				val := upd.value.(float64)
				v.addSample(val)
			default:
				logs.Err.Panicf("stats: unsupported expvar type %T", ev)
			}
		} else {
			panic("stats: update to unknown variable " + upd.varname)
		}
	}

	logs.Info.Println("stats: shutdown")
}
type IceServer interface {}

// CredValidator holds additional config params for a credential validator.
type CredValidator struct {
	// AuthLevel(s) which require this validator.
	requiredAuthLvl []auth.Level
	addToTags       bool
}

// Async publish an increment (decrement) to int variable.
func StatsInc(name string, val int) {
	if Globals.statsUpdate != nil {
		select {
		case Globals.statsUpdate <- &varUpdate{name, int64(val), true}:
		default:
		}
	}
}