package logging

import (
	"log"
	"os"
)

func New() *log.Logger {
	return log.New(os.Stderr, "hark: ", log.LstdFlags|log.Lmsgprefix)
}
