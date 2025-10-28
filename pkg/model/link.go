package model

type Type string

const (
	TypeChannel  = Type("channel")
	TypePlaylist = Type("playlist")
	TypeSeason   = Type("season")
	TypeSeries   = Type("series")
	TypeUser     = Type("user")
	TypeGroup    = Type("group")
	TypeHandle   = Type("handle")
)

type Provider string

const (
	ProviderBilibili   = Provider("bilibili")
	ProviderYoutube    = Provider("youtube")
	ProviderVimeo      = Provider("vimeo")
	ProviderSoundcloud = Provider("soundcloud")
	ProviderTwitch     = Provider("twitch")
)

// Info represents data extracted from URL
type Info struct {
	LinkType Type     // Either group, channel or user
	Provider Provider // Youtube, Vimeo, SoundCloud or Twitch
	ItemID   string
}
