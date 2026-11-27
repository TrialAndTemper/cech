module github.com/TrialAndTemper/cech/cli

go 1.27.0

require (
	github.com/BurntSushi/toml v1.6.0 // TOML data files; the standard library reads no TOML
	github.com/spf13/cobra v1.10.2 // the command tree, help and completion (ADR 16 item 1)
	go.opentelemetry.io/proto/otlp v1.11.1 // the OTLP messages the harnesses export to cech eval (eval-runs criterion 24)
	go.yaml.in/yaml/v3 v3.0.5 // SKILL.md and agent frontmatter, read as the harnesses read it
	google.golang.org/protobuf v1.36.12 // decodes those OTLP messages from protobuf; the standard library has no protobuf
)

require (
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.31.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	golang.org/x/net v0.59.0 // indirect
	golang.org/x/sys v0.48.0 // indirect
	golang.org/x/text v0.42.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260928230214-8a89bd6388cc // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260928230214-8a89bd6388cc // indirect
	google.golang.org/grpc v1.84.0 // indirect
)
