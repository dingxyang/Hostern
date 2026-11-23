//go:build !darwin && !linux

package pty

func (p *PTY) WorkingDirectory() string { return "" }
