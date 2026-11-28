package pane

// Words are words made from a value, kept while the value stays the same: a bar drawn every
// frame makes them again only when what they say changed, so drawing it allocates nothing.
type Words[Value comparable] struct {
	value Value
	made  bool
	text  string
}

// Of is the words for a value, written by write only when the value is not the one of last time.
// write should not hold anything but the value, so it costs nothing to pass.
func (words *Words[Value]) Of(value Value, write func(Value) string) string {
	if !words.made || value != words.value {
		words.value, words.made, words.text = value, true, write(value)
	}
	return words.text
}
