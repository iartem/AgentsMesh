package channel

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStripPodMentions(t *testing.T) {
	tests := []struct {
		name    string
		content string
		podKeys []string
		want    string
	}{
		{
			name:    "single mention with trailing space",
			content: "@abcd1234 please fix the bug",
			podKeys: []string{"abcd1234efgh5678"},
			want:    "please fix the bug",
		},
		{
			name:    "single mention at end (no trailing space)",
			content: "hey @abcd1234",
			podKeys: []string{"abcd1234efgh5678"},
			want:    "hey",
		},
		{
			name:    "short pod key (less than 8 chars)",
			content: "@short hello",
			podKeys: []string{"short"},
			want:    "hello",
		},
		{
			name:    "multiple pod mentions",
			content: "@abcd1234 @efgh5678 collaborate on this",
			podKeys: []string{"abcd1234xxxxx", "efgh5678yyyyy"},
			want:    "collaborate on this",
		},
		{
			name:    "no mentions",
			content: "just a regular message",
			podKeys: []string{"abcd1234efgh5678"},
			want:    "just a regular message",
		},
		{
			name:    "empty pod keys",
			content: "@abcd1234 hello",
			podKeys: []string{},
			want:    "@abcd1234 hello",
		},
		{
			name:    "mention embedded in text",
			content: "tell @abcd1234 to review",
			podKeys: []string{"abcd1234efgh5678"},
			want:    "tell to review",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripPodMentions(tt.content, tt.podKeys)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestBuildPodPrompt(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		channelName string
		channelID   int64
		podKeys     []string
		want        string
	}{
		{
			name:        "basic prompt with mention stripped",
			content:     "@abcd1234 fix the login bug",
			channelName: "dev-team",
			channelID:   42,
			podKeys:     []string{"abcd1234efgh5678"},
			want:        "Message from channel(#dev-team, channel_id=42): fix the login bug\n\nIf you finish it, please reply to this channel using send_channel_message(channel_id=42).",
		},
		{
			name:        "no mentions to strip",
			content:     "deploy to staging",
			channelName: "ops",
			channelID:   7,
			podKeys:     []string{"abcd1234efgh5678"},
			want:        "Message from channel(#ops, channel_id=7): deploy to staging\n\nIf you finish it, please reply to this channel using send_channel_message(channel_id=7).",
		},
		{
			name:        "multiple mentions stripped",
			content:     "@aabbccdd @eeffgghh review PR #42",
			channelName: "code-review",
			channelID:   100,
			podKeys:     []string{"aabbccddxxxxxxxx", "eeffgghhyyyyyyyy"},
			want:        "Message from channel(#code-review, channel_id=100): review PR #42\n\nIf you finish it, please reply to this channel using send_channel_message(channel_id=100).",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildPodPrompt(tt.content, tt.channelName, tt.channelID, tt.podKeys)
			assert.Equal(t, tt.want, got)
		})
	}
}
