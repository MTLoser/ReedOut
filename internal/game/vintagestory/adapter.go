package vintagestory

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
	joinRe  = regexp.MustCompile(`Player (\w+) joins`)
	leaveRe = regexp.MustCompile(`Player (\w+) left`)
)

func (a *Adapter) Game() string { return "vintagestory" }

func (a *Adapter) ParseLogLine(line string) *game.LogEvent {
	if m := joinRe.FindStringSubmatch(line); m != nil {
		return &game.LogEvent{Type: "player_join", Player: m[1]}
	}
	if m := leaveRe.FindStringSubmatch(line); m != nil {
		return &game.LogEvent{Type: "player_leave", Player: m[1]}
	}
	if strings.Contains(line, "Error") || strings.Contains(line, "Exception") {
		return &game.LogEvent{Type: "error", Message: line}
	}
	return nil
}

func (a *Adapter) PlayerCommand() string { return "/list" }
func (a *Adapter) StopCommand() string   { return "/stop" }
