package window

// DocumentMegabytes is how much memory, in megabytes, the files read may keep, to start with it
// the next time.
func (application *Window) DocumentMegabytes() int { return application.session.DocumentMegabytes() }

// ChooseDocumentMegabytes changes how much memory the files read may keep; a smaller budget
// forgets the files opened longest ago at once.
func (application *Window) ChooseDocumentMegabytes(megabytes int) {
	application.session.SetDocumentMegabytes(megabytes)
}

// DocumentUse is how many files read are kept, and about how many bytes they take.
func (application *Window) DocumentUse() (documents, bytes int) {
	return application.session.DocumentUse()
}
