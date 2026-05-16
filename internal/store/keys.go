package store

// Pebble schema keys are versioned singleton records. See README.md in this
// package for payload ownership, read paths, and durability expectations.

func statusCurrentKey() []byte {
	return []byte("v1:status:current")
}

func statusRawKey() []byte {
	return []byte("v1:status:raw")
}

func availabilityRawKey() []byte {
	return []byte("v1:availability:raw")
}

func availabilityCurrentKey() []byte {
	return []byte("v1:availability:current")
}

func availabilityHolidaysEnglandKey() []byte {
	return []byte("v1:availability:holidays:england")
}

func availabilityDeployDirtyKey() []byte {
	return []byte("v1:availability:deploy:dirty")
}

func availabilityDeployLastKey() []byte {
	return []byte("v1:availability:deploy:last")
}
