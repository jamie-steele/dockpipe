module dorkpipe.orchestrator

go 1.25

require dockpipe v0.0.0

require (
	github.com/lib/pq v1.10.9
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/santhosh-tekuri/jsonschema/v5 v5.3.1 // indirect
	golang.org/x/sys v0.28.0 // indirect
	golang.org/x/term v0.27.0 // indirect
)

replace dockpipe => ../../..
