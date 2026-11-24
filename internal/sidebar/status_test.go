package sidebar

import (
	"testing"

	"github.com/akrck02/code-review/internal/workspace"
)

// statusListing decorates as a mode of every file does: tinted names, dotted folders and a count
// of changed files, beside a word that is not status.
type statusListing struct{ changesOnly bool }

func (statusListing) Name() string                        { return "Files" }
func (listing statusListing) ChangesOnly() bool           { return listing.changesOnly }
func (statusListing) Header() string                      { return "" }
func (statusListing) Press(workspace.FileReference, bool) {}
func (statusListing) Decorate(row Row) Decoration {
	return Decoration{Tinted: true, Dot: true, Words: []Word{{Text: "3", Status: true}, {Text: "bin"}}}
}

func TestHidingTheStatusLeavesOutHowFilesChangedOutsideTheReview(test *testing.T) {
	sidebar := &Sidebar{Listing: statusListing{}, HidesStatus: true}
	decoration := sidebar.decorate(Row{Kind: FileRow})
	if decoration.Tinted || decoration.Dot || len(decoration.Words) != 1 || decoration.Words[0].Text != "bin" {
		test.Errorf("with the status hidden, a row is decorated %+v", decoration)
	}
	sidebar.HidesStatus = false
	if decoration := sidebar.decorate(Row{Kind: FileRow}); !decoration.Tinted || !decoration.Dot || len(decoration.Words) != 2 {
		test.Errorf("with the status shown, a row is decorated %+v", decoration)
	}
}

func TestTheReviewShowsTheStatusEvenWhenItIsHidden(test *testing.T) {
	sidebar := &Sidebar{Listing: statusListing{changesOnly: true}, HidesStatus: true}
	if decoration := sidebar.decorate(Row{Kind: FileRow}); !decoration.Tinted || len(decoration.Words) != 2 {
		test.Errorf("in a mode of only the changes, the status is hidden: %+v", decoration)
	}
}
