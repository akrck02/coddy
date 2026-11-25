package plugins

import (
	"context"

	"github.com/akrck02/coddy/sdk"
)

// ReviewSourceContribution is a review source a plugin gives: where pull requests come from
// ("GitHub").
type ReviewSourceContribution struct {
	Name string `json:"name"`
}

// Reviews are the open pull requests of the project in folder, as the plugin, which must
// contribute a review source, lists them. It waits for the plugin, so it runs off the frame.
func (plugin *Plugin) Reviews(stop context.Context, folder string) ([]sdk.Review, error) {
	var reviews []sdk.Review
	err := plugin.ask(stop, sdk.ReviewListMethod, sdk.FolderParams{Folder: folder}, &reviews)
	return reviews, err
}

// ReviewComments are the comments on the lines of a pull request of the project in folder.
func (plugin *Plugin) ReviewComments(stop context.Context, folder, id string) ([]sdk.ReviewComment, error) {
	var comments []sdk.ReviewComment
	asked := sdk.ReviewParams{Folder: folder, ID: id}
	err := plugin.ask(stop, sdk.ReviewCommentsMethod, asked, &comments)
	return comments, err
}
