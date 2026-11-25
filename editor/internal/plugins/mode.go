package plugins

import (
	"context"

	"github.com/akrck02/coddy/sdk"
)

// ModeContribution is a mode a plugin adds: its name and its icon on the rail, one of Lucide's
// names ("list-todo").
type ModeContribution struct {
	Name string `json:"name"`
	Icon string `json:"icon"`
}

// DrawMode asks the plugin, which must contribute a mode, for a frame of its screen: params and
// result are those of the pluginsdk/screen package, which this package leaves to the window, as
// they are go-ui's. It waits for the plugin, so it runs off the frame.
func (plugin *Plugin) DrawMode(stop context.Context, params, result any) error {
	return plugin.ask(stop, sdk.ModeFrameMethod, params, result)
}
