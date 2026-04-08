package logging

import (
	"log/slog"
	"os"

	tea "charm.land/bubbletea/v2"
)

// Setup initializes structured file logging. Logs are written to the given
// path using slog's TextHandler. The log level defaults to INFO; set the
// DEBUG environment variable to any non-empty value for DEBUG-level output.
//
// Returns a cleanup function that closes the underlying file.
func Setup(path string) (func(), error) {
	f, err := tea.LogToFile(path, "")
	if err != nil {
		return nil, err
	}

	handler := slog.NewTextHandler(f, &slog.HandlerOptions{
		Level: level(),
	})
	slog.SetDefault(slog.New(handler))

	return func() { f.Close() }, nil
}

func level() slog.Level {
	if os.Getenv("DEBUG") != "" {
		return slog.LevelDebug
	}
	return slog.LevelInfo
}
