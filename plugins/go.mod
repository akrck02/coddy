module github.com/akrck02/coddy/plugins

go 1.25.6

require (
	github.com/akrck02/coddy/sdk v0.0.0
	github.com/akrck02/go-renderer v0.4.1-0.20261124160721-64873f987a26
)

// the plugins are built with the sdk beside them, in the same repository
replace github.com/akrck02/coddy/sdk => ../sdk
