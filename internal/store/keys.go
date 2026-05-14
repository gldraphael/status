package store

// Key schema:
//   status             → current status (single-tenant)
//   availability       → cached availability calendar snapshot
//   availability_dirty → flag indicating availability changed since last deploy
//   availability_last_deployed → availability entries JSON from the last successful deploy
//   availability_holidays → cached England bank holiday snapshot
//   event:{eventID}    → Google Calendar event state

func statusKey() []byte {
	return []byte("status")
}

func availabilityKey() []byte {
	return []byte("availability")
}

func availabilityDirtyKey() []byte {
	return []byte("availability_dirty")
}

func availabilityLastDeployedKey() []byte {
	return []byte("availability_last_deployed")
}

func availabilityHolidaysKey() []byte {
	return []byte("availability_holidays")
}

func eventKey(eventID string) []byte {
	return []byte("event:" + eventID)
}

func eventKeyPrefix() []byte {
	return []byte("event:")
}

// prefixUpperBound returns the smallest key that is lexicographically greater
// than all keys with the given prefix, for use as an iterator upper bound.
func prefixUpperBound(prefix []byte) []byte {
	upper := make([]byte, len(prefix))
	copy(upper, prefix)
	for i := len(upper) - 1; i >= 0; i-- {
		upper[i]++
		if upper[i] != 0 {
			return upper[:i+1]
		}
	}
	return nil
}
