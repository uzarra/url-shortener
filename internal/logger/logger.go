package logger

import (
	"os"
	"strings"

	"github.com/rs/zerolog"
)

func New(level string) (zerolog.Logger, error) {
	lvl, err := zerolog.ParseLevel(strings.ToLower(level))
	if err != nil {
		return zerolog.Logger{}, err
	}
	return zerolog.New(os.Stdout).Level(lvl).With().Timestamp().Logger(), nil
}
