package bot

type RemRepoBot interface {
	// everything a basic bot has
	BasicBot
	// getter and setter
	// remote repository url
	RemoteRepoUrl() string
	SetRemoteRepoUrl(url string)
}
