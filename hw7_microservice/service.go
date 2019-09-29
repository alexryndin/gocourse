package main

import (
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"encoding/json"
	"fmt"
	"golang.org/x/net/context"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

// тут вы пишете код
// обращаю ваше внимание - в этом задании запрещены глобальные переменные
func ChainUnaryServerInterceptors(interceptors ...grpc.UnaryServerInterceptor) grpc.UnaryServerInterceptor {
	n := len(interceptors)

	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		chainer := func(currentInter grpc.UnaryServerInterceptor, currentHandler grpc.UnaryHandler) grpc.UnaryHandler {
			return func(currentCtx context.Context, currentReq interface{}) (interface{}, error) {
				return currentInter(currentCtx, currentReq, info, currentHandler)
			}
		}

		chainedHandler := handler
		for i := n - 1; i >= 0; i-- {
			chainedHandler = chainer(interceptors[i], chainedHandler)
		}

		return chainedHandler(ctx, req)
	}
}
func ChainStreamServer(interceptors ...grpc.StreamServerInterceptor) grpc.StreamServerInterceptor {
	n := len(interceptors)

	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		chainer := func(currentInter grpc.StreamServerInterceptor, currentHandler grpc.StreamHandler) grpc.StreamHandler {
			return func(currentSrv interface{}, currentStream grpc.ServerStream) error {
				return currentInter(currentSrv, currentStream, info, currentHandler)
			}
		}

		chainedHandler := handler
		for i := n - 1; i >= 0; i-- {
			chainedHandler = chainer(interceptors[i], chainedHandler)
		}

		return chainedHandler(srv, ss)
	}
}

type ACLs map[string][]string

type AllServer struct {
	*BizServerImpl
	*AdminServerImpl
	ACLs
}

func checkACL(path string, acpath string) bool {
	path_s := strings.Split(path, "/")
	acpath_s := strings.Split(acpath, "/")
	fmt.Println("scpath_s = ", acpath_s[1:])
	fmt.Println("path_s = ", path_s[1:])
	for i, v := range acpath_s[1:] {
		if v == "*" || v == path_s[i+1] {
			continue
		} else {
			return false
		}
	}
	return true

}

func getMDString(ctx context.Context, key string) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Errorf(codes.DataLoss, "AuthInterceptor: failed to get metadata")
	}
	acls, ok := md[key]
	if !ok {
		return "", status.Errorf(codes.NotFound, "AuthInterceptor: Requested key not found")
	}
	acl := acls[0]
	return acl, nil
}

func logEvent(srv interface{}, ctx context.Context, method string) error {
	switch srv.(type) {
	case AllServer:
		return srv.(AllServer).logEvent(ctx, method)
	case *AdminServerImpl:
		return srv.(*AdminServerImpl).logEvent(ctx, method)
	default:
		fmt.Println("No log defined")
		return nil
	}
}

func (srv *AdminServerImpl) logEvent(ctx context.Context, method string) error {
	consumer, err := getMDString(ctx, "consumer")
	if err != nil {
		if v := status.Code(err); v != codes.Unknown {
			if v == codes.NotFound {
				consumer = "err: Unknown"
			} else {
				consumer = "err: Couldn't get consumer, Unknown"
			}
		} else {
			return err

		}
	}
	var event Event
	event.Consumer = consumer
	event.Timestamp = 0
	event.Method = method
	event.Host = "127.0.0.1:"
	srv.Logs <- &event
	fmt.Println("I'm here2, consumer = ", consumer)
	return nil
}

func authInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	if err := checkAccess(info.Server, ctx, info.FullMethod); err == nil {
		return handler(ctx, req)
	} else {
		return nil, err
	}
}

func streamAuthInterceptor(
	srv interface{},
	ss grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) error {
	if err := checkAccess(srv, ss.Context(), info.FullMethod); err == nil {
		return handler(srv, ss)
	} else {
		return err
	}
}

func logInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	err := logEvent(info.Server, ctx, info.FullMethod)

	if err != nil {
		return nil, err
	}

	return handler(ctx, req)

}

func streamLogInterceptor(
	srv interface{},
	ss grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler) error {

	err := logEvent(srv, ss.Context(), info.FullMethod)

	if err != nil {
		return err
	}

	return handler(srv, ss)
}

func checkAccess(srv interface{}, ctx context.Context, method string) error {
	var aclm map[string][]string
	switch srv.(type) {
	case AllServer:
		aclm = srv.(AllServer).ACLs
	case *BizServerImpl:
		aclm = srv.(*BizServerImpl).ACLs
	case *AdminServerImpl:
		aclm = srv.(*AdminServerImpl).ACLs
	}
	consumer, err := getMDString(ctx, "consumer")
	if err != nil {
		if v := status.Code(err); v != codes.Unknown {
			if v == codes.NotFound {
				return status.Errorf(codes.Unauthenticated, "Unknown user or method")
			} else {
				return err
			}
		}
		return err
	}
	acls_s, ok := (aclm)[consumer]
	fmt.Println("consumer = ", consumer)
	fmt.Println("consumer acls_s = ", acls_s)
	if !ok {
		return status.Errorf(codes.Unauthenticated, "Unknown user or method")
	}
	allow := false
	for _, v := range acls_s {
		if checkACL(method, v) {
			allow = true
			break
		}
	}
	if !allow {
		return status.Errorf(codes.Unauthenticated, "Unauthenticated: disallowed method for %s\n", consumer)
	} else {
		return nil
	}

}

type BizServerImpl struct{ ACLs ACLs }

func NewBizServerImpl() *BizServerImpl { return &BizServerImpl{} }

func (srv *BizServerImpl) Check(ctx context.Context, in *Nothing) (*Nothing, error) {
	return &Nothing{}, nil
}
func (srv *BizServerImpl) Add(ctx context.Context, in *Nothing) (*Nothing, error) {
	return &Nothing{}, nil
}
func (srv *BizServerImpl) Test(ctx context.Context, in *Nothing) (*Nothing, error) {
	return &Nothing{}, nil
}

type AdminServerImpl struct {
	ACLs               ACLs
	LogSubscribers     map[chan *Event]struct{}
	NewLogSubscribers  chan chan *Event
	LSLock             sync.RWMutex
	Logs               chan *Event
	StatSubscribers    map[chan *Event]struct{}
	NewStatSubscribers chan chan *Event
	SSLock             sync.RWMutex
	Stats              chan *Event
}

func NewAdminServerImpl() *AdminServerImpl {
	return &AdminServerImpl{
		Logs:               make(chan *Event),
		Stats:              make(chan *Event),
		NewLogSubscribers:  make(chan chan *Event),
		NewStatSubscribers: make(chan chan *Event),
		LogSubscribers:     make(map[chan *Event]struct{}),
		StatSubscribers:    make(map[chan *Event]struct{}),
	}
}

func (srv *AdminServerImpl) Logging(_ *Nothing, out Admin_LoggingServer) error {
	lch := make(chan *Event)

	srv.NewLogSubscribers <- lch

	for {
		select {
		case log := <-lch:
			fmt.Println("logging log = ", log)
			if err := out.Send(log); err != nil {
				// return status.Errorf(codes.Internal, "Couldn't send log to client")
				srv.LSLock.Lock()
				delete(srv.LogSubscribers, lch)
				srv.LSLock.Unlock()
				return err
			}
		case <-out.Context().Done():
			srv.LSLock.Lock()
			delete(srv.LogSubscribers, lch)
			srv.LSLock.Unlock()
			return out.Context().Err()
		}
	}
}

func (srv *AdminServerImpl) Statistics(t *StatInterval, out Admin_StatisticsServer) error {
	lch := make(chan *Event)
	srv.NewStatSubscribers <- lch
	ticker := time.NewTicker(time.Duration(t.GetIntervalSeconds()) * time.Second)

	stat := &Stat{ByMethod: make(map[string]uint64), ByConsumer: make(map[string]uint64), Timestamp: time.Now().Unix()}

	for {
		select {
		case <-ticker.C:
			if err := out.Send(stat); err != nil {
				srv.SSLock.Lock()
				delete(srv.StatSubscribers, lch)
				srv.SSLock.Unlock()
				stat = &Stat{ByMethod: make(map[string]uint64), ByConsumer: make(map[string]uint64), Timestamp: time.Now().Unix()}
				return err
			} else {
				stat = &Stat{ByMethod: make(map[string]uint64), ByConsumer: make(map[string]uint64), Timestamp: time.Now().Unix()}
			}
		case s := <-lch:
			stat.ByConsumer[s.Consumer]++
			stat.ByMethod[s.Method]++

		case <-out.Context().Done():
			srv.SSLock.Lock()
			delete(srv.StatSubscribers, lch)
			srv.SSLock.Unlock()
			return out.Context().Err()
		}
	}
}

func StartAndWatchServcice(ctx context.Context, server *grpc.Server, lis net.Listener) {

	go server.Serve(lis)
	fmt.Println("Started")
	select {
	case <-ctx.Done():
		fmt.Println("Stopped")
		server.Stop()
	}
	fmt.Println("Started")
}

func (srv *AdminServerImpl) broadcastLogs(ctx context.Context) {
	for {
		select {
		case logm := <-srv.Logs:
			fmt.Println("I'm here4")
			for k, _ := range srv.LogSubscribers {
				k <- logm
			}
			for k, _ := range srv.StatSubscribers {
				k <- logm
			}
		case newLogSubCh := <-srv.NewLogSubscribers:
			fmt.Println("I'm here5")
			srv.LSLock.Lock()
			srv.LogSubscribers[newLogSubCh] = struct{}{}
			srv.LSLock.Unlock()
		case newStatSubCh := <-srv.NewStatSubscribers:
			srv.LSLock.Lock()
			srv.StatSubscribers[newStatSubCh] = struct{}{}
			srv.LSLock.Unlock()
		case <-ctx.Done():
			return
		}
	}
}

func StartMyMicroservice(ctx context.Context, listenAddr string, ACLData string) error {

	var aclm map[string][]string
	if err := json.Unmarshal([]byte(ACLData), &aclm); err != nil {
		return err
	}
	srv := AllServer{
		&BizServerImpl{},
		NewAdminServerImpl(),
		aclm,
	}

	lis, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatalln("cant listet port", err)
	}

	server := grpc.NewServer(
		grpc.UnaryInterceptor(
			ChainUnaryServerInterceptors(
				authInterceptor,
				logInterceptor,
			),
		),
		grpc.StreamInterceptor(
			ChainStreamServer(
				streamAuthInterceptor,
				streamLogInterceptor,
			),
		),
	)

	RegisterBizServer(server, srv)
	RegisterAdminServer(server, srv)

	fmt.Printf("starting server at %s\n", listenAddr)
	go srv.broadcastLogs(ctx)
	go StartAndWatchServcice(ctx, server, lis)

	return nil

}
