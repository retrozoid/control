package profiler

import (
	"github.com/retrozoid/control/protocol/debugger"
)

/*
 */
type ConsoleProfileFinished struct {
	Id       string             `json:"id"`
	Location *debugger.Location `json:"location"`
	Profile  *Profile           `json:"profile"`
	Title    string             `json:"title,omitempty"`
}

/*
Sent when new profile recording is started using console.profile() call.
*/
type ConsoleProfileStarted struct {
	Id       string             `json:"id"`
	Location *debugger.Location `json:"location"`
	Title    string             `json:"title,omitempty"`
}

/*
	Reports coverage delta since the last poll (either from an event like this, or from

`takePreciseCoverage` for the current isolate. May only be sent if precise code
coverage has been started. This event can be trigged by the embedder to, for example,
trigger collection of coverage data immediately at a certain point in time.
*/
type PreciseCoverageDeltaUpdate struct {
	Timestamp float64           `json:"timestamp"`
	Occasion  string            `json:"occasion"`
	Result    []*ScriptCoverage `json:"result"`
}
