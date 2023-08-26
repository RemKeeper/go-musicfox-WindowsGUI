package model

import (
	"time"

	"github.com/anhoder/foxful-cli/util"
	tea "github.com/charmbracelet/bubbletea"
)

type Options struct {
	StartupOptions
	ProgressOptions

	AppName             string
	WhetherDisplayTitle bool
	LoadingText         string
	PrimaryColor        string
	DualColumn          bool // The menu list is displayed as a dual column
	HideMenu            bool

	TeaOptions []tea.ProgramOption // Tea program options

	InitPage      InitPage
	MainMenuTitle *MenuItem
	Ticker        Ticker      // Ticker for render
	MainMenu      Menu        // Entry menu of app
	Components    []Component // Custom Extra components

	KBControllers    []KeyboardController
	MouseControllers []MouseController

	InitHook  func(a *App)
	CloseHook func(a *App)
}

type StartupOptions struct {
	EnableStartup     bool
	LoadingDuration   time.Duration
	TickDuration      time.Duration
	ProgressOutBounce bool
	Welcome           string
}

func DefaultOptions() *Options {
	return &Options{
		StartupOptions: StartupOptions{
			EnableStartup:     true,
			LoadingDuration:   time.Second * 2,
			TickDuration:      time.Millisecond * 16,
			ProgressOutBounce: true,
			Welcome:           util.PkgName,
		},
		ProgressOptions: ProgressOptions{
			FirstEmptyChar: '.',
			EmptyChar:      '.',
			LastEmptyChar:  '.',
			FirstFullChar:  '#',
			FullChar:       '#',
			LastFullChar:   '#',
		},
		WhetherDisplayTitle: true,
		DualColumn:          true,
		AppName:             util.PkgName,
		LoadingText:         util.LoadingText,
		PrimaryColor:        util.RandomColor,
		MainMenu:            &DefaultMenu{},
	}
}
