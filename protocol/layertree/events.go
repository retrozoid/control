package layertree

import (
	"github.com/retrozoid/control/protocol/common"
)

/*
 */
type LayerPainted struct {
	LayerId LayerId      `json:"layerId"`
	Clip    *common.Rect `json:"clip"`
}

/*
 */
type LayerTreeDidChange struct {
	Layers []*Layer `json:"layers,omitempty"`
}
