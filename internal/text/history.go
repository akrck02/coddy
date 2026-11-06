package text

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// editKind is what an edit was, so the edits of one word are undone together.
type editKind int

const (
	otherEdit    editKind = iota // pasting, cutting, replacing a selection: each undone on its own
	typingEdit                   // one character typed
	deletingEdit                 // one character deleted
)

// mostUndoSteps bounds the edits a buffer keeps to undo.
const mostUndoSteps = 1000

// edit is one change of the text: what was taken out and put in at a position, and where the
// caret was before and after, to undo it and do it again.
type edit struct {
	start                     Position
	removed, inserted         string
	caretBefore, anchorBefore Position
	caretAfter                Position
	removedBreaks             []string // the line endings removed, as they were, to undo exactly
	kind                      editKind
	version                   int
}

// history is what can be undone and redone, and which version of the text was saved last.
type history struct {
	undo, redo   []edit
	version      int // grows with every change, undoing included
	savedVersion int
	nextVersion  int
	// baseVersion is the version of the text under the oldest edit kept: 0, the text as read,
	// until the oldest edits are dropped
	baseVersion int
}

// push keeps an edit, joined to the one before when both are the same word typed or deleted.
func (history *history) push(record edit) {
	history.redo = history.redo[:0]
	history.nextVersion++
	record.version = history.nextVersion
	history.version = record.version
	// an edit made after saving is never joined to one before, so undoing comes back to the save
	count := len(history.undo)
	if count > 0 && history.undo[count-1].version != history.savedVersion && joins(history.undo[count-1], record) {
		history.undo[count-1] = joined(history.undo[count-1], record)
		return
	}
	if len(history.undo) == mostUndoSteps {
		history.baseVersion = history.undo[0].version
		history.undo = append(history.undo[:0], history.undo[1:]...)
	}
	history.undo = append(history.undo, record)
}

// joins tells whether an edit continues the one before: a character typed right after the last,
// within one word, or a character deleted right beside the last, on one line.
func joins(last, next edit) bool {
	if last.kind != next.kind || next.kind == otherEdit {
		return false
	}
	if next.kind == typingEdit {
		typed, _ := utf8.DecodeRuneInString(next.inserted)
		before, _ := utf8.DecodeLastRuneInString(last.inserted)
		startsWord := unicode.IsSpace(typed) && !unicode.IsSpace(before)
		return next.start == endOf(last.start, last.inserted) && !startsWord
	}
	if strings.Contains(next.removed, "\n") {
		return false
	}
	backwards := endOf(next.start, next.removed) == last.start
	forwards := next.start == last.start
	return backwards || forwards
}

// joined is two edits as one.
func joined(last, next edit) edit {
	if next.kind == typingEdit {
		last.inserted += next.inserted
	} else if next.start == last.start {
		last.removed += next.removed
	} else {
		last.start = next.start
		last.removed = next.removed + last.removed
	}
	last.caretAfter = next.caretAfter
	last.version = next.version
	return last
}

// Undo takes back the last edit and tells whether there was one.
func (buffer *Buffer) Undo() bool {
	history := &buffer.history
	count := len(history.undo)
	if count == 0 {
		return false
	}
	record := history.undo[count-1]
	history.undo = history.undo[:count-1]
	buffer.replace(record.start, endOf(record.start, record.inserted), record.removed, record.removedBreaks)
	buffer.caret, buffer.anchor = record.caretBefore, record.anchorBefore
	buffer.goalColumn = buffer.caret.Column
	history.redo = append(history.redo, record)
	history.version = history.topVersion()
	return true
}

// Redo makes the last edit undone again and tells whether there was one.
func (buffer *Buffer) Redo() bool {
	history := &buffer.history
	count := len(history.redo)
	if count == 0 {
		return false
	}
	record := history.redo[count-1]
	history.redo = history.redo[:count-1]
	after, _ := buffer.replace(record.start, endOf(record.start, record.removed), record.inserted, nil)
	buffer.Place(after, false)
	history.undo = append(history.undo, record)
	history.version = record.version
	return true
}

// topVersion is the version of the text the edits still kept make: the base when none are.
func (history *history) topVersion() int {
	if len(history.undo) == 0 {
		return history.baseVersion
	}
	return history.undo[len(history.undo)-1].version
}

// Version changes whenever the text does, undoing and redoing included: whoever draws or colours
// the text compares it to know when to do it again.
func (buffer *Buffer) Version() int { return buffer.history.version }

// Modified tells whether the text differs from the one last saved (or read).
func (buffer *Buffer) Modified() bool {
	return buffer.history.version != buffer.history.savedVersion
}

// MarkSaved records that the text as it is now was saved.
func (buffer *Buffer) MarkSaved() { buffer.history.savedVersion = buffer.history.version }
