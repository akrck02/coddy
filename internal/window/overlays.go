package window

import ui "github.com/akrck02/go-ui"

// overlay is a dialog or panel drawn over the window, which takes the input while it is open.
// Adding one means adding it to overlaysOf: the frame draws it and ModalOpen knows of it.
type overlay struct {
	open  func() bool
	build func(context *ui.Context)
}

// overlaysOf lists the window's overlays in the order they are drawn, the last one on top.
func overlaysOf(application *Window) []overlay {
	return []overlay{
		{func() bool { return application.finder.Open }, func(context *ui.Context) { application.finder.Build(context, application) }},
		{func() bool { return application.base.open }, application.buildBaseDialog},
		{func() bool { return application.serversOpen }, application.buildServers},
		{func() bool { return application.pickerOpen }, application.buildPicker},
		{func() bool { return application.questions.unsavedOpen || application.questions.conflictOpen }, application.buildQuestions},
	}
}

// buildOverlays draws the overlays that are open.
func (application *Window) buildOverlays(context *ui.Context) {
	for _, each := range application.overlays {
		if each.open() {
			each.build(context)
		}
	}
}

// ModalOpen tells whether the settings or an overlay cover the window, taking the input.
func (application *Window) ModalOpen() bool {
	if application.settingsScreen.Open {
		return true
	}
	for _, each := range application.overlays {
		if each.open() {
			return true
		}
	}
	return false
}
