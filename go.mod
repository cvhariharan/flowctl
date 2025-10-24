module github.com/cvhariharan/flowctl

go 1.24.5

require (
	github.com/casbin/casbin/v2 v2.110.0
	github.com/coreos/go-oidc/v3 v3.14.1
	github.com/cvhariharan/flowctl/executors/docker v0.0.0-20250728145512-aa56676fc961
	github.com/cvhariharan/flowctl/executors/script v0.0.0-20250728145512-aa56676fc961
	github.com/cvhariharan/flowctl/remote/qssh v0.0.0-20250728145512-aa56676fc961
	github.com/cvhariharan/flowctl/remote/ssh v0.0.0-20250728145512-aa56676fc961
	github.com/cvhariharan/flowctl/sdk/executor v0.0.0-20250728145512-aa56676fc961
	github.com/expr-lang/expr v1.17.5
	github.com/go-playground/validator/v10 v10.27.0
	github.com/golang-migrate/migrate/v4 v4.18.3
	github.com/google/uuid v1.6.0
	github.com/gosimple/slug v1.15.0
	github.com/huml-lang/go-huml v0.1.0
	github.com/jmoiron/sqlx v1.4.0
	github.com/knadh/koanf/parsers/toml v0.1.0
	github.com/knadh/koanf/providers/env v1.1.0
	github.com/knadh/koanf/providers/file v1.2.0
	github.com/knadh/koanf/providers/structs v1.0.0
	github.com/knadh/koanf/v2 v2.2.2
	github.com/labstack/echo/v4 v4.13.4
	github.com/lib/pq v1.10.9
	github.com/memwey/casbin-sqlx-adapter v0.3.0
	github.com/nxadm/tail v1.4.11
	github.com/robfig/cron/v3 v3.0.1
	github.com/spf13/cobra v1.9.1
	github.com/zerodha/simplesessions/stores/postgres/v3 v3.0.0
	github.com/zerodha/simplesessions/v3 v3.0.0
	gocloud.dev v0.43.0
	golang.org/x/crypto v0.40.0
	golang.org/x/oauth2 v0.30.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/bmatcuk/doublestar/v4 v4.6.1 // indirect
	github.com/casbin/govaluate v1.3.0 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/containerd/errdefs v1.0.0 // indirect
	github.com/containerd/errdefs/pkg v0.3.0 // indirect
	github.com/containerd/log v0.1.0 // indirect
	github.com/cvhariharan/flowctl/sdk/remoteclient v0.0.0-20250728145512-aa56676fc961 // indirect
	github.com/cvhariharan/qssh v0.1.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/distribution/reference v0.6.0 // indirect
	github.com/docker/docker v28.2.2+incompatible // indirect
	github.com/docker/go-connections v0.5.0 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/fatih/structs v1.1.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/gabriel-vasile/mimetype v1.4.8 // indirect
	github.com/go-jose/go-jose/v4 v4.1.1 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-viper/mapstructure/v2 v2.4.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/mock v1.7.0-rc.1 // indirect
	github.com/googleapis/gax-go/v2 v2.15.0 // indirect
	github.com/gosimple/unidecode v1.0.1 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-envparse v0.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/kr/fs v0.1.0 // indirect
	github.com/labstack/gommon v0.4.2 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/melbahja/goph v1.4.0 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/moby/docker-image-spec v1.3.1 // indirect
	github.com/moby/go-archive v0.1.0 // indirect
	github.com/moby/patternmatcher v0.6.0 // indirect
	github.com/moby/sys/atomicwriter v0.1.0 // indirect
	github.com/moby/sys/sequential v0.6.0 // indirect
	github.com/moby/sys/user v0.4.0 // indirect
	github.com/moby/sys/userns v0.1.0 // indirect
	github.com/moby/term v0.5.2 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.1 // indirect
	github.com/pelletier/go-toml v1.9.5 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pkg/sftp v1.13.9 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/quic-go/quic-go v0.54.0 // indirect
	github.com/rs/xid v1.6.0 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/spf13/pflag v1.0.6 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasttemplate v1.2.2 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.62.0 // indirect
	go.opentelemetry.io/otel v1.37.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.37.0 // indirect
	go.opentelemetry.io/otel/metric v1.37.0 // indirect
	go.opentelemetry.io/otel/sdk v1.37.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.37.0 // indirect
	go.opentelemetry.io/otel/trace v1.37.0 // indirect
	go.uber.org/atomic v1.9.0 // indirect
	go.uber.org/mock v0.5.0 // indirect
	golang.org/x/mod v0.25.0 // indirect
	golang.org/x/net v0.42.0 // indirect
	golang.org/x/sync v0.16.0 // indirect
	golang.org/x/sys v0.36.0 // indirect
	golang.org/x/text v0.27.0 // indirect
	golang.org/x/time v0.12.0 // indirect
	golang.org/x/tools v0.34.0 // indirect
	golang.org/x/xerrors v0.0.0-20240903120638-7835f813f4da // indirect
	google.golang.org/api v0.242.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250715232539-7130f93afb79 // indirect
	google.golang.org/grpc v1.73.0 // indirect
	google.golang.org/protobuf v1.36.6 // indirect
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7 // indirect
)
