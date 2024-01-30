package media

import (
	"github.com/retrozoid/control/protocol"
)

/*
Enables the Media domain
*/
func Enable(c protocol.Caller) error {
	return c.Call("Media.enable", nil, nil)
}

/*
Disables the Media domain.
*/
func Disable(c protocol.Caller) error {
	return c.Call("Media.disable", nil, nil)
}
