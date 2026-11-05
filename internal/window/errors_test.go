package window

import (
	"errors"
	"testing"
)

func TestAReaderWithoutAModeSaysWhy(test *testing.T) {
	reader, err := New(Settings{Synchronous: true})
	if reader != nil || !errors.Is(err, NoMode) {
		test.Errorf("a reader without a mode gave %v, %v; want no reader and the code %v", reader, err, NoMode)
	}
}
