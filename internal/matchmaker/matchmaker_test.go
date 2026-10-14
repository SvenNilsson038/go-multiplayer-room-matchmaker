package matchmaker

import (
	"reflect"
	"testing"
)

func TestDecide(t *testing.T) {
	tests := []struct {
		name string
		req  Request
		want Decision
	}{
		{
			name: "approved assets enter a stable room",
			req: Request{RequestID: "match-42", Mode: "Duo", Region: "eu-west", Players: []Player{
				{ID: "p2", DisplayName: "Lin", Assets: []Asset{{ID: "skin-2", ModerationStatus: "approved"}}},
				{ID: "p1", DisplayName: "Mira"},
			}},
			want: Decision{State: "matched", Room: "duo-eu-west-9decf9e8"},
		},
		{
			name: "pending assets stop room creation",
			req: Request{RequestID: "match-43", Mode: "Duo", Region: "eu-west", Players: []Player{
				{ID: "p1", DisplayName: "Mira", Assets: []Asset{{ID: "banner-9", ModerationStatus: "pending"}, {ID: "skin-2", ModerationStatus: "pending"}}},
			}},
			want: Decision{State: "moderation_queued", QueueAssets: []string{"banner-9", "skin-2"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Decide(tt.req)
			if err != nil {
				t.Fatalf("Decide() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("Decide() = %#v, want %#v", got, tt.want)
			}
		})
	}
}
