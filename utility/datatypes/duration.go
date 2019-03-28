package datatypes

import (
	"time"
)

type Duration struct {
	time.Duration
}

func NewDuration(duration time.Duration) Duration {
	return Duration{Duration: duration}
}

func (d Duration) MarshalJSON() (data []byte, err error) {
	data = []byte(d.String())
	return
}

func (d *Duration) UnmarshalJSON(data []byte) (err error) {
	if d.Duration, err = time.ParseDuration(string(data)); err != nil {
		return
	}
	return
}

func (d *Duration) UnmarshalText(data []byte) (err error) {
	if d.Duration, err = time.ParseDuration(string(data)); err != nil {
		return
	}
	return
}
