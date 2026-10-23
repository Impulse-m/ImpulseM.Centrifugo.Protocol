# protocol

_Centrifugal client-server protocol definitions and codecs for Go._

[![build](https://github.com/centrifugal/protocol/actions/workflows/ci.yml/badge.svg)](https://github.com/centrifugal/protocol/actions/workflows/ci.yml)
[![GoDoc](https://pkg.go.dev/badge/github.com/centrifugal/protocol)](https://pkg.go.dev/github.com/centrifugal/protocol)
[![Release](https://img.shields.io/github/v/release/centrifugal/protocol?sort=semver)](https://github.com/centrifugal/protocol/releases)
[![License](https://img.shields.io/github/license/centrifugal/protocol)](https://github.com/centrifugal/protocol/blob/master/LICENSE)
[![Telegram](https://img.shields.io/badge/Telegram-join-26A5E4?logo=telegram&logoColor=white)](https://t.me/joinchat/ABFVWBE0AhkyyhREoaboXQ)
[![Discord](https://img.shields.io/badge/Discord-join-5865F2?logo=discord&logoColor=white)](https://discord.gg/tYgADKx)

## Fork

This repository is a fork of [centrifugal/protocol](https://github.com/centrifugal/protocol) (MIT), based on its master branch at commit `4455d8e`. The fork is maintained in [Impulse-m/ImpulseM.Centrifugo.Protocol](https://github.com/Impulse-m/ImpulseM.Centrifugo.Protocol). Everything not listed below works as in upstream.

### Install

The module path is still `github.com/centrifugal/protocol`, so point it at the fork with a `replace` directive:

```bash
go mod edit -replace github.com/centrifugal/protocol=github.com/Impulse-m/ImpulseM.Centrifugo.Protocol@v0.23.2-0.20261013130323-70f37c2667eb
go mod tidy
```

The fork has no release tags of its own. The version above is the pseudo-version of commit `70f37c2`, the one the server and Core currently use. Use the pseudo-version of another fork commit if you need one.

### What the fork adds

- **Fork fields 1000-1999.** `Error`, `Reply`, `ConnectRequest`, `ConnectResult`, `RefreshResult`, `RPCRequest` and `RPCResult` have extra fields numbered from 1000. Upstream field numbers are unchanged. See [client.proto](client.proto) and the tests in [fork_fields_test.go](fork_fields_test.go).
- **Packed repeated `uint64` in cfprotobuf.** The Protobuf generator supports this type, which the fork fields use. See [cfprotobuf](cfprotobuf/README.md).
- **Weekly upstream check.** [upstream-sync.yml](.github/workflows/upstream-sync.yml) keeps an issue open while master is behind upstream.

### Related repositories

- [ImpulseM.Centrifugo](https://github.com/Impulse-m/ImpulseM.Centrifugo): server, a fork of Centrifugo.
- [ImpulseM.Centrifugo.Core](https://github.com/Impulse-m/ImpulseM.Centrifugo.Core): fork of the Centrifuge library, used by the server.
- [ImpulseM.Centrifugo.Client.JS](https://github.com/Impulse-m/ImpulseM.Centrifugo.Client.JS): JavaScript client SDK.
- [ImpulseM.Centrifugo.Client.Java](https://github.com/Impulse-m/ImpulseM.Centrifugo.Client.Java): Java client SDK.
- [ImpulseM.Centrifugo.Client.Swift](https://github.com/Impulse-m/ImpulseM.Centrifugo.Client.Swift): Swift client SDK.

## Upstream documentation

This package contains the client-server protocol used by [Centrifugo](https://github.com/centrifugal/centrifugo) and the [Centrifuge](https://github.com/centrifugal/centrifuge) library, together with the encoders and decoders they use on hot paths.

The protocol is defined once in [client.proto](client.proto) and can be serialized either as **JSON** or as **Protobuf** – the two representations are generated from the same definitions, so they never drift apart. Client SDKs in other languages generate their own code from the very same file.

## Install from upstream

```bash
go get github.com/centrifugal/protocol
```

Most applications never import this package directly – they use Centrifugo or Centrifuge, which depend on it. Import it directly when implementing a client, a transport, or tooling that speaks the Centrifugal protocol.

## Protocol in a nutshell

There are three top-level messages:

| Message   | Direction        | Purpose                                                                            |
|-----------|------------------|------------------------------------------------------------------------------------|
| `Command` | client -> server | Carries exactly one request, e.g. `ConnectRequest`, `SubscribeRequest`, `PublishRequest`. |
| `Reply`   | server -> client | Answers a `Command` with a result or an `Error`, or wraps an asynchronous `Push`.   |
| `Push`    | server -> client | Asynchronous message, e.g. `Publication`, `Join`, `Leave`, `Disconnect`.            |

Several messages may be streamed inside a single transport frame. In JSON they are separated by a `\n` delimiter, in Protobuf each message is prefixed with its length encoded as a varint.

Application payloads (such as `Publication.Data`) use the `Raw` type – a `[]byte` passed through encoding as is, so a subscriber decodes the payload its publisher sent. The one exception is required by the JSON framing above and is documented on `Raw.MarshalJSON`.

## Usage

Codecs are selected by protocol type and are usually taken from pools to keep the allocation count low:

```go
decoder := protocol.GetCommandDecoder(protocol.TypeJSON, data)
defer protocol.PutCommandDecoder(protocol.TypeJSON, decoder)

for {
    cmd, err := decoder.Decode()
    if cmd != nil {
        // Handle the command.
    }
    if err != nil {
        if errors.Is(err, io.EOF) {
            break
        }
        return err
    }
}
```

See the [package documentation](https://pkg.go.dev/github.com/centrifugal/protocol) for the full set of encoders and decoders, and [client.proto](client.proto) for the message definitions with comments.

For a description of the protocol from the client point of view see the [client protocol](https://centrifugal.dev/docs/transports/client_protocol) documentation on centrifugal.dev.

## Generated code

`client.pb.go`, `client.pb_cfprotobuf.go` and `client.pb_cfjson.go` are generated and committed to the repo. After changing `client.proto`, regenerate them with:

```bash
make generate
```

The required tools are listed at the top of [generate.sh](generate.sh). There `protoc-gen-go` is named at `@latest`, so pin it yourself for reproducible output. The fork's generated files were last produced with protoc 34.0 and protoc-gen-go v1.36.11 (commit `cc0f8a7`). The structs come from `protoc-gen-go`; the JSON and the Protobuf encoding and decoding code is produced by generators which live in this repo, see [cfjson](cfjson) and [cfprotobuf](cfprotobuf).

## Development

```bash
make test   # run tests with race detector
make bench  # run benchmarks
make fuzz   # run fuzz targets for a short time
make lint   # run golangci-lint (CI pins its version in .golangci-lint-version)
```

Decoders are fuzzed nightly in CI, see [.github/workflows/fuzz.yml](.github/workflows/fuzz.yml).

## Security

To report a vulnerability, see [SECURITY.md](SECURITY.md).

## License

MIT, see [LICENSE](LICENSE). The fork's changes are under the same license. The bundled JSON test suite in [cfjson/testdata/JSONTestSuite](cfjson/testdata/JSONTestSuite) is also MIT licensed, with its own [LICENSE](cfjson/testdata/JSONTestSuite/LICENSE) file.
