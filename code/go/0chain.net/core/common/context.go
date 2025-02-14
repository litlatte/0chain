package common

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	. "0chain.net/core/logging"
	"go.uber.org/zap"
)

var ErrStop = NewError("stop_error", "Stop signal error")

var rootContext context.Context
var rootCancel context.CancelFunc

/*SetupRootContext - sets up the root context that can be used to shutdown the node */
func SetupRootContext(nodectx context.Context) {
	rootContext, rootCancel = context.WithCancel(nodectx)
	// TODO: This go routine is not needed. Workaround for the "vet" error
	done := make(chan bool)
	go func() {
		select {
		case <-done:
			Logger.Info("Shutting down all workers...")
			rootCancel()
		}
	}()
}

/*GetRootContext - get the root context for the server
* This will be used to control shutting down the server but cleanup all the workers
 */
func GetRootContext() context.Context {
	return rootContext
}

/*Done - call this when the program needs to stop and notify all workers */
func Done() {
	Logger.Info("Initiating shutdown...")
	rootCancel()
	//TODO: How do we ensure every worker is completed any shutdown sequence before we finally shut down
	//the server using server.Shutdown(ctx)
}

/*HandleShutdown - handles various shutdown signals */
func HandleShutdown(server *http.Server) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGQUIT)
	go func() {
		for sig := range c {
			switch sig {
			case syscall.SIGINT:
				Done()
				ctx, cancelf := context.WithTimeout(context.Background(), 5*time.Second)
				server.Shutdown(ctx)
				cancelf()
			case syscall.SIGQUIT:
				Done()
				ctx, cancelf := context.WithTimeout(context.Background(), 5*time.Second)
				server.Shutdown(ctx)
				cancelf()
			default:
				Logger.Info("unhandled signal", zap.Any("signal", sig))
			}
		}
	}()
}

// WithContextFunc provides the capacity for canceling a function by context
type WithContextFunc struct {
	c chan struct{}
}

// NewWithContextFunc returns a WithContextFunc instance
//
// params:
// - concurrent: represents the max concurrent processing number
func NewWithContextFunc(concurrent int) *WithContextFunc {
	return &WithContextFunc{
		c: make(chan struct{}, concurrent),
	}
}

// Run tries to acquire a slot from a buffered channel and runs the function
func (wcf *WithContextFunc) Run(ctx context.Context, f func() error) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case wcf.c <- struct{}{}:
		defer func() {
			<-wcf.c
		}()
		return f()
	}
}
