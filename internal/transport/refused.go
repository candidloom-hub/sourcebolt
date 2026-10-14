package transport

import "errors"

// ConnRefused reports whether err comes from a TCP connection the target
// actively refused (errno ECONNREFUSED, or WSAECONNREFUSED on Windows).
// It matches the errno, never the message text.
func ConnRefused(err error) bool { return errors.Is(err, errConnRefused) }
