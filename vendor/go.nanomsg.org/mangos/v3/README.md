# mangos&trade; v3


[![Linux](https://img.shields.io/github/workflow/status/nanomsg/mangos/linux?logoColor=grey&logo=linux&label=)](https://github.com/nanomsg/mangos/actions)
[![Windows](https://img.shields.io/github/workflow/status/nanomsg/mangos/windows?logoColor=grey&logo=windows&label=)](https://github.com/nanomsg/mangos/actions)
[![macOS](https://img.shields.io/github/workflow/status/nanomsg/mangos/darwin?logoColor=grey&logo=apple&label=)](https://github.com/nanomsg/mangos/actions)
[![Coverage](https://img.shields.io/codecov/c/github/nanomsg/mangos?logoColor=grey&logo=codecov&label=)](https://codecov.io/gh/nanomsg/mangos)
[![Discord](https://img.shields.io/discord/639573728212156478?label=&logo=discord)](https://discord.gg/wewTkby)
[![Documentation](https://img.shields.io/badge/godoc-docs-blue.svg?label=&logo=go)](https://pkg.go.dev/go.nanomsg.org/mangos/v3)
[![License](https://img.shields.io/github/license/nanomsg/mangos.svg?logoColor=silver&logo=opensourceinitiative&label=&color=blue)](https://github.com/nanomsg/mangos/blob/master/LICENSE)
[![Version](https://img.shields.io/github/v/tag/nanomsg/mangos?logo=github&sort=semver&label=)](https://github.com/nanomsg/mangos/releases)



_Mangos&trade;_  is an implementation in pure Go of the *SP*
(`Scalability Protocols`) messaging system.
These are colloquially  known as `nanomsg`.

> *NOTE*: The import path has changed! Please change any references
to `go.nanomsg.org/mangos/v3`.
The old v2 imports will still work for old applications, provided that
a sufficiently modern version of Go is used.  However, no further work
will be done on earlier versions.
Earlier versions will still inter-operate with this version, except that
within the same process the `inproc` transport can only be used by
consumers using the same version of mangos.

The modern C implementation of the SP protocols is available as
[NNG&trade;](https://github.com/nanomsg/nng).

The original implementation of the SP protocols is available as
[nanomsg&trade;](http://www.nanomsg.org).

Generally (modulo a few caveats) all of these implementations can inter-operate.

The design is intended to make it easy to add new transports,
as well as new topologies (`protocols` in SP parlance.)

At present, all the Req/Rep, Pub/Sub, Pair, Bus, Push/Pull, and
Surveyor/Respondent patterns are supported.
This project also supports an experimental protocol called Star.

Supported transports include TCP, inproc, IPC, WebSocket, WebSocket/TLS and TLS.

Basic interoperability with nanomsg and NNG has been verified (you can do
so yourself with `nanocat` and `macat`) for all protocols and transports
that _NNG_ and _nanomsg_ support, except for the _ZeroTier_ transport and the PAIRv1
protocol, which are only supported in _NNG_ at this time.

There are a number of projects that use these products together.

## Documentation

For API documentation, see https://pkg.go.dev/go.nanomsg.org/mangos/v3.

## Testing

This package supports internal self tests, which can be run in
the idiomatic Go way.
(Note that most of the tests are in a test subdirectory.)

    $ go test go.nanomsg.org/mangos/v3/...

There are also internal benchmarks available:

    $ go test -bench=. go.nanomsg.org/mangos/v3/test

## Commercial Support

[Staysail Systems, Inc.](mailto:info@staysail.tech) offers
[commercial support](http://staysail.tech/support/mangos) for mangos.

## Examples

Some examples are posted in the directories under `examples/` in this project.

These examples are rewrites (in Go) of Tim Dysinger's
[Getting Started with Nanomsg](http://nanomsg.org/gettingstarted/index.html).

Running `go doc` in the example directories will yield information about how
to run each example program.

Enjoy!

______

Copyright 2021 The Mangos Authors

mangos&trade;, Nanomsg&trade; and NNG&trade; are [trademarks](http://nanomsg.org/trademarks.html) of Garrett D'Amore.
