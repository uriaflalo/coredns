package whitelist

import (
	"errors"
	"fmt"
	"os"
	"time"
)

func RetryWithTimeout(timeout time.Duration, sleep time.Duration, f func() bool) error {

	deadline := time.After(timeout)
	for {
		select {
		case <-deadline:
			return errors.New("timeout")
		default:
			if ok := f(); ok {
				return nil
			}
			time.Sleep(sleep)
		}
	}
}

func GetEnv(key string) string {

	ret := os.Getenv(key)
	log.Infof("'%s': '%s'", key, ret)

	return ret
}

func ServiceFormat(name string, ns string) string {

	return fmt.Sprintf("%s.%s", name, ns)
}
