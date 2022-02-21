package bot

// Peer to peer bot.
type P2PBot interface {
	// everything a basic bot has
	BasicBot
}

type P2PBotStruct struct {
	base *SimpleBot
}

// Create a new simple bot
func NewP2PBotStruct() *P2PBotStruct {
	return &P2PBotStruct{
		base: NewSimpleBot(),
	}
}
