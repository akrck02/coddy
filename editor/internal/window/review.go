package window

import (
	"context"

	"github.com/akrck02/coddy/editor/internal/git"
	"github.com/akrck02/coddy/editor/internal/plugins"
	"github.com/akrck02/coddy/editor/internal/workspace"
	"github.com/akrck02/coddy/sdk"
	ui "github.com/akrck02/go-ui"
)

// reviewWork is the key of the pull requests' jobs in work: a new one replaces the one running.
const reviewWork = "pull request"

// reviewTitle is the menu's item that lists the pull requests of a review source.
const reviewTitle = "Review a pull request…"

// TODO(akrck02): with two review sources on, the menu shows two items of the same words; they could
// name their source, once someone has two.

// reviewDialog lists the open pull requests of a project, to choose one to review.
type reviewDialog struct {
	open    bool
	plugin  *plugins.Plugin
	project string // the project's path
	reviews []sdk.Review
	labels  []string // each pull request in words, made when they are listed
}

// pullRequest is the pull request being reviewed: its project, and its comments as the viewer shows
// them, by file and by line.
type pullRequest struct {
	project  string
	comments map[string]map[int]string
}

// buildReviewItems are the menu's items of the review sources on.
func (application *Window) buildReviewItems(context *ui.Context) {
	for _, plugin := range application.settings.Plugins {
		if plugin.Ready() && plugin.Contributes.ReviewSource != nil && context.MenuItem("git-pull-request", reviewTitle, "") {
			application.listReviews(plugin)
		}
	}
}

// reviewedProject is the project a pull request is chosen for: the one of the file open, or of the
// row selected, or the first; nil with none open.
func (application *Window) reviewedProject() *workspace.Project {
	projects := application.session.Projects
	switch row, selected := application.sidebar.SelectedRow(); {
	case application.viewer.IsOpen() && application.viewer.Project != nil:
		return application.viewer.Project
	case selected && row.Project >= 0 && row.Project < len(projects):
		return projects[row.Project]
	case len(projects) > 0:
		return projects[0]
	}
	return nil
}

// listReviews asks a review source for the open pull requests of the project, off the frame, and
// lists them in the dialog.
func (application *Window) listReviews(plugin *plugins.Plugin) {
	project := application.reviewedProject()
	if project == nil {
		application.toast = "Open a project to review its pull requests"
		return
	}
	path := project.Path
	application.work.Run(reviewWork, func(stop context.Context) func() {
		reviews, err := plugin.Reviews(stop, path)
		return func() { application.showReviews(plugin, path, reviews, err) }
	})
}

// showReviews opens the dialog with the pull requests listed, or tells why there are none.
func (application *Window) showReviews(plugin *plugins.Plugin, path string, reviews []sdk.Review, err error) {
	plugin.Tell(err)
	if err != nil {
		application.toast = err.Error()
		return
	}
	if len(reviews) == 0 {
		application.toast = "No pull request is open"
		return
	}
	labels := make([]string, len(reviews))
	for index, each := range reviews {
		labels[index] = "#" + each.ID + "  " + each.Title + " · " + each.Author + " (" + each.BaseRef + ")"
	}
	application.reviewing = reviewDialog{open: true, plugin: plugin, project: path, reviews: reviews, labels: labels}
}

// buildReviewDialog lists the pull requests; choosing one reviews it.
func (application *Window) buildReviewDialog(context *ui.Context) {
	dialog := &application.reviewing
	chosen := -1
	dismissed := context.Sheet("Review a pull request", ui.SheetOptions{}, func() {
		for index, label := range dialog.labels {
			if context.Selectable("git-pull-request", label, false) {
				chosen = index
			}
		}
	})
	if dismissed {
		dialog.open = false
	}
	if chosen >= 0 {
		dialog.open = false
		application.chooseReview(dialog.plugin, dialog.project, dialog.reviews[chosen])
	}
}

// chooseReview fetches a pull request's commits and its comments, off the frame, then compares its
// head against its base in its project and shows it in a mode that shows the changes.
// TODO(akrck02): pull requests of projects read through a version control plugin.
func (application *Window) chooseReview(plugin *plugins.Plugin, path string, chosen sdk.Review) {
	index := application.projectIndex(path)
	if index < 0 || application.session.Projects[index].Control != nil {
		application.toast = "Only git projects can review a pull request yet"
		return
	}
	application.work.Run(reviewWork, func(stop context.Context) func() {
		head, base, err := git.Fetch(stop, path, chosen.HeadRef, chosen.BaseRef)
		if err != nil {
			return func() { application.toast = err.Error() }
		}
		comments, err := plugin.ReviewComments(stop, path, chosen.ID)
		return func() {
			plugin.Tell(err)
			if err != nil {
				application.toast = err.Error() // the changes still show, without their comments
			}
			application.startReview(path, head, base, comments)
		}
	})
}

// startReview compares a project's head against base, keeps the comments for the viewer, and
// turns on the first mode that shows the changes.
func (application *Window) startReview(path, head, base string, comments []sdk.ReviewComment) {
	application.review = pullRequest{project: path, comments: commentWords(comments)}
	application.session.SetReview(path, head, base)
	if index := application.projectIndex(path); index >= 0 {
		application.reload(index)
	}
	application.projectsChanged()
	for _, each := range application.modes {
		if each.ShowsChanges() {
			application.setMode(each)
			break
		}
	}
	application.viewer.Comments = application.review.commentsOf(application.viewer.Project, application.viewer.Reference.Path)
}

// commentWords are the comments as the viewer shows them: by file, by line, each "author: text",
// those on one line joined.
func commentWords(comments []sdk.ReviewComment) map[string]map[int]string {
	words := map[string]map[int]string{}
	for _, comment := range comments {
		if comment.Line <= 0 {
			continue // on the file, or on a line the pull request no longer has: not shown yet
		}
		lines := words[comment.File]
		if lines == nil {
			lines = map[int]string{}
			words[comment.File] = lines
		}
		said := comment.Author + ": " + comment.Text
		if earlier, found := lines[comment.Line]; found {
			said = earlier + "  ·  " + said
		}
		lines[comment.Line] = said
	}
	return words
}

// commentsOf are the comments on a file of a project, when it is the project under review.
func (reviewed pullRequest) commentsOf(project *workspace.Project, file string) map[int]string {
	if project == nil || project.Path != reviewed.project {
		return nil
	}
	return reviewed.comments[file]
}
