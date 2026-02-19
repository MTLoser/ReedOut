package minecraft

import (
	"regexp"
	"strings"

	"github.com/reedfamily/reedout/internal/game"
)

func init() {
	game.Register(&Adapter{})
}

type Adapter struct{}

var (
	joinRe  = regexp.MustCompile(`\[Server thread/INFO\].*: (\w+) joined the game`)
	leaveRe = regexp.MustCompile(`\[Server thread/INFO\].*: (\w+) left the game`)
	chatRe  = regexp.MustCompile(`\[Server thread/INFO\].*: <(\w+)> (.+)`)
)

func (a *Adapter) Game() string { return "minecraft" }

func (a *Adapter) ParseLogLine(line string) *game.LogEvent {
	if m := joinRe.FindStringSubmatch(line); m != nil {
		return &game.LogEvent{Type: "player_join", Player: m[1]}
	}
	if m := leaveRe.FindStringSubmatch(line); m != nil {
		return &game.LogEvent{Type: "player_leave", Player: m[1]}
	}
	if m := chatRe.FindStringSubmatch(line); m != nil {
		return &game.LogEvent{Type: "chat", Player: m[1], Message: m[2]}
	}
	if strings.Contains(line, "ERROR") || strings.Contains(line, "FATAL") {
		return &game.LogEvent{Type: "error", Message: line}
	}
	return nil
}

func (a *Adapter) PlayerCommand() string { return "list" }
func (a *Adapter) StopCommand() string   { return "stop" }
