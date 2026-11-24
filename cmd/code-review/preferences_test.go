package main

import "testing"

func TestEveryProjectIsShownWhenNothingWasChosen(test *testing.T) {
	if rememberedOnlyChanged("") {
		test.Error("a first start shows only the changes; it should show every project")
	}
}

func TestOnlyTheChangesAreShownWhenThatWasChosen(test *testing.T) {
	if !rememberedOnlyChanged("true") {
		test.Error("only the changes were chosen last time, but every project is shown")
	}
	if rememberedOnlyChanged("false") {
		test.Error("every project was chosen last time, but only the changes are shown")
	}
}
