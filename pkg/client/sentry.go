package client

import (
	"github.com/getsentry/sentry-go"

	"errors"
	"fmt"
	"log"
	"os"
	"sync"
)

var (
	sOn bool
	smu sync.Mutex // protect sentry calls (does not look to be reentrant)
)

func SentryInit() {
	dsn := os.Getenv("SENTRY_DSN")
	if len(dsn) > 0 {
		err := sentry.Init(sentry.ClientOptions{
			Dsn: dsn,
		})
		if err == nil {
			sOn = true
		} else {
			log.Println("Error initializing sentry:", err)
		}
	}
}

////
//  LogSentry writes a message to the log (stderr) and also sends to sentry.io
func LogSentry(l sentry.Level, format string, args ...interface{}) {
	e := errors.New(fmt.Sprintf(format, args...))
	log.Println(e)
	if sOn {
		smu.Lock()
		defer smu.Unlock()

		sentry.ConfigureScope(func(scope *sentry.Scope) {
			scope.SetLevel(l)
		})
		sentry.CaptureException(e)
	}
}
