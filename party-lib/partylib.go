package partylib

import (
	"time"
)

type Chat struct {
	Time    time.Time
	Id      string
	Channel string
	Message string
}