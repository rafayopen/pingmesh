package client

import (
	"github.com/getsentry/sentry-go"

	"errors"
	"fmt"
	"log"
	"sync"
)

var smu sync.Mutex // protect sentry calls (does not look to be reentrant)

////
//  LogSentry writes a message to the log (stderr) and also sends to sentry.io
func LogSentry(l sentry.Level, format string, args ...interface{}) {
	e := errors.New(fmt.Sprintf(format, args...))
	smu.Lock()
	defer smu.Unlock()
	log.Println(e)
	sentry.ConfigureScope(func(scope *sentry.Scope) {
		scope.SetLevel(l)
	})
	sentry.CaptureException(e)
	//	sentry.Flush(time.Second * 5)
}
