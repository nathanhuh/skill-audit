package audit

// Windows cannot fsync the read-only directory handle used on Unix. The state
// file is still flushed before replacement. Directory crash durability and
// atomic replacement on native Windows are not guaranteed by this MVP.
func syncStateDirectory(string) error { return nil }
