package main

import "time"

type jtime time.Time

func (t jtime) MarshalJSON() ([]byte, error) {
	return []byte(time.Time(t).UTC().Format(`"2006-01-02T15:04:05Z"`)), nil
}
