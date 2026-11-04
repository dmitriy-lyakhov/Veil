package server

import (
	"errors"
	"log/slog"

	"veil/internal/config"
)

type NetSetup struct{}

func SetupNetwork(*config.Server, string, *slog.Logger) (*NetSetup, error) {
	return nil, errors.New("серверная часть поддерживается только на Linux")
}

func (n *NetSetup) Teardown() {}
