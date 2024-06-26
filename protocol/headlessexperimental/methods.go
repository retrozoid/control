package headlessexperimental

import (
	"github.com/retrozoid/control/protocol"
)

/*
	Sends a BeginFrame to the target and returns when the frame was completed. Optionally captures a

screenshot from the resulting frame. Requires that the target was created with enabled
BeginFrameControl. Designed for use with --run-all-compositor-stages-before-draw, see also
https://goo.gle/chrome-headless-rendering for more background.
*/
func BeginFrame(c protocol.Caller, args BeginFrameArgs) (*BeginFrameVal, error) {
	var val = &BeginFrameVal{}
	return val, c.Call("HeadlessExperimental.beginFrame", args, val)
}
