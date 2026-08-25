package core

import "testing"

func TestValidWebSocketToken(t *testing.T) {
	if !validWebSocketToken("expected-token", "expected-token") {
		t.Fatal("validWebSocketToken rejected the expected token")
	}
	if validWebSocketToken("wrong-token", "expected-token") {
		t.Fatal("validWebSocketToken accepted a wrong token")
	}
}

func TestOneBotOutboundTextIncludesConfiguredImage(t *testing.T) {
	message := OutboundMessage{Text: "宠物近况", Image: "https://cdn.example.com/pet.png"}
	got := oneBotOutboundText(InboundEvent{Platform: PlatformOneBot, SceneType: SceneGroup, ActorID: "42"}, message)
	want := "[CQ:at,qq=42]\n[CQ:image,file=https://cdn.example.com/pet.png]\n宠物近况"
	if got != want {
		t.Fatalf("unexpected OneBot image message %q", got)
	}
	if got := oneBotImageCQ("../secret.png"); got != "" {
		t.Fatalf("path traversal must not become image CQ: %q", got)
	}
}

func TestOneBotDirectReplyDoesNotMentionThePlayer(t *testing.T) {
	got := oneBotOutboundText(InboundEvent{Platform: PlatformOneBot, SceneType: SceneDirect, ActorID: "42"}, OutboundMessage{Text: "悄悄告诉你"})
	if got != "悄悄告诉你" {
		t.Fatalf("direct reply must stay natural, got %q", got)
	}
}
