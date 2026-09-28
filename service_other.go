//go:build !windows

package main

const serviceName = "sql-relay"

func runningAsService() bool { return false }

func runService(func(stop <-chan struct{})) {}
