package client

import "time"

type PsycheClient struct {
	Url                string
	AutoUpdateDuration time.Duration
}
