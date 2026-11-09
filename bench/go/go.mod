module github.com/sematext/drain3-go/bench

go 1.25.0

require (
	github.com/kloudmate/drain3 v0.0.0-20260304082619-bc42b50af108
	github.com/sematext/drain3-go v0.0.0
)

require (
	github.com/dlclark/regexp2 v1.11.5 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/sematext/drain3-go => ../..
