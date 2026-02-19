package game

// GameAdapter provides game-specific behavior for a server type.
type GameAdapter interface {
	// Game returns the game identifier (e.g., "minecraft", "vintagestory")
	Game() string

	// ParseLogLine extracts structured events from log lines
	ParseLogLine(line string) *LogEvent

	// PlayerCommand returns the command to list online players
	PlayerCommand() string

	// StopCommand returns the graceful stop command for the server
	StopCommand() string
}

type LogEvent struct {
	Type    string // "player_join", "player_leave", "chat", "info", "error"
	Player  string
	Message string
}
