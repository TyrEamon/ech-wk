//go:build !windows

package main

type trayManager struct{}

func (a *App) setupTray() {}

func (a *App) cleanupTray() {}
