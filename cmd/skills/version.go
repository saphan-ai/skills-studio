package main

// version is set at build time with -ldflags "-X main.version=<v>"; a plain `go build` reports "dev".
var version = "dev"
