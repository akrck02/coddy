package main

import (
	"os"

	"github.com/akrck02/coddy/sdk"
)

// fakeReviews is a review source with one pull request, #7, from feature/brighter into main, and
// one comment on the fourth line of src/lamp.go. Asked the comments of #8, it ends.
type fakeReviews struct{}

func (fakeReviews) List(string) ([]sdk.Review, error) {
	return []sdk.Review{{ID: "7", Title: "Brighter lamp", Author: "keeper", BaseRef: "main", HeadRef: "feature/brighter"}}, nil
}

func (fakeReviews) Comments(asked sdk.ReviewParams) ([]sdk.ReviewComment, error) {
	if asked.ID == "8" {
		os.Exit(3)
	}
	return []sdk.ReviewComment{{File: "src/lamp.go", Line: 4, Author: "keeper", Text: "always on?"}}, nil
}
