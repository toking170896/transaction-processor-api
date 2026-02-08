package logger

import (
	"os"

	"github.com/rs/zerolog"
)

func New(pretty bool) zerolog.Logger {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	var output zerolog.ConsoleWriter
	if pretty {
		output = zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: "2006-01-02 15:04:05"}
		return zerolog.New(output).With().Timestamp().Caller().Logger()
	}

	return zerolog.New(os.Stdout).With().Timestamp().Logger()
}
