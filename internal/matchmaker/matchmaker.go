package matchmaker

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
)

type Asset struct {
	ID               string `json:"id"`
	ModerationStatus string `json:"moderation_status"`
}

type Player struct {
	ID          string  `json:"id"`
	DisplayName string  `json:"display_name"`
	Assets      []Asset `json:"assets"`
}

type Request struct {
	RequestID string   `json:"request_id"`
	Mode      string   `json:"mode"`
	Region    string   `json:"region"`
	Players   []Player `json:"players"`
}

type Decision struct {
	State       string   `json:"state"`
	Room        string   `json:"room,omitempty"`
	QueueAssets []string `json:"queue_assets,omitempty"`
}

func Decide(req Request) (Decision, error) {
	if req.RequestID == "" || req.Mode == "" || req.Region == "" || len(req.Players) == 0 {
		return Decision{}, errors.New("request_id, mode, region, and players are required")
	}
	queued := make([]string, 0)
	playerIDs := make([]string, 0, len(req.Players))
	for _, player := range req.Players {
		if player.ID == "" || player.DisplayName == "" {
			return Decision{}, errors.New("each player needs id and display_name")
		}
		playerIDs = append(playerIDs, player.ID)
		for _, asset := range player.Assets {
			if asset.ID == "" || asset.ModerationStatus == "" {
				return Decision{}, errors.New("each asset needs id and moderation_status")
			}
			if asset.ModerationStatus == "pending" {
				queued = append(queued, asset.ID)
			}
		}
	}
	if len(queued) > 0 {
		sort.Strings(queued)
		return Decision{State: "moderation_queued", QueueAssets: queued}, nil
	}
	sort.Strings(playerIDs)
	digest := sha256.Sum256([]byte(strings.Join(playerIDs, ",") + "|" + req.Mode + "|" + req.Region))
	room := fmt.Sprintf("%s-%s-%s", slug(req.Mode), slug(req.Region), hex.EncodeToString(digest[:4]))
	return Decision{State: "matched", Room: room}, nil
}

func slug(value string) string {
	value = strings.ToLower(value)
	var b strings.Builder
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		} else if b.Len() > 0 && !strings.HasSuffix(b.String(), "-") {
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}
