// Package modules resolves which module binaries a wizard answer set needs.
// Ported from lib/modules.sh — acquisition/indexer/downloader modules never
// ship from the public installer.
package modules

import "strings"

// PlatformModules always run regardless of library/playback choices.
var PlatformModules = []string{
	"muxcored", "api-rest", "auth-local", "secrets-file", "encryption-aesgcm",
	"call-policy-default", "publish-policy-default", "cache-local",
	"ratelimit-tokenbucket", "health-monitor", "admin-ui", "notification-default",
}

// AcquisitionNever is stripped from the resolved set no matter what — the
// public installer must never deploy these.
var AcquisitionNever = map[string]bool{
	"media-automation": true, "request-media": true,
	"downloader-native-torrent": true, "downloader-native-usenet": true,
	"downloader-qbittorrent": true, "downloader-sabnzbd": true, "downloader-debrid": true,
	"indexer-piratebay": true, "indexer-torznab": true,
}

// Answers is the subset of wizard state that determines enabled modules.
type Answers struct {
	Libraries []string // Movies, TV, Music, Books, Comics, Audiobooks
	Playback  []string // MuxCore player, Jellyfin, Plex, Emby, DLNA
	Profile   string   // sqlite | postgres
}

func has(list []string, needle string) bool {
	for _, v := range list {
		if strings.EqualFold(v, needle) {
			return true
		}
	}
	return false
}

// Resolve returns the ordered, de-duplicated, filtered module list.
func Resolve(a Answers) []string {
	seen := map[string]bool{}
	var out []string
	add := func(names ...string) {
		for _, n := range names {
			if AcquisitionNever[n] || seen[n] {
				continue
			}
			seen[n] = true
			out = append(out, n)
		}
	}

	add(PlatformModules...)
	if a.Profile == "postgres" {
		add("database-postgres")
	} else {
		add("database-sqlite")
	}

	anyLib, video := false, false
	if has(a.Libraries, "Movies") {
		add("media-movies")
		anyLib, video = true, true
	}
	if has(a.Libraries, "TV") {
		add("media-tvshows")
		anyLib, video = true, true
	}
	if has(a.Libraries, "Music") {
		add("media-music", "metadata-musicbrainz")
		anyLib = true
	}
	if has(a.Libraries, "Books") {
		add("media-books")
		anyLib = true
	}
	if has(a.Libraries, "Comics") {
		add("media-comics")
		anyLib = true
	}
	if has(a.Libraries, "Audiobooks") {
		add("media-audiobooks")
		anyLib = true
	}
	if anyLib {
		add("media-scanner", "media-root-folders")
	}
	if video {
		add("metadata-tmdb", "media-rename", "media-ffprobe", "media-subtitles", "media-custom-formats")
	}

	if has(a.Playback, "MuxCore player") {
		add("media-transcoder", "mediauiprox")
	}
	if has(a.Playback, "Jellyfin") {
		add("jellyfin")
	}
	if has(a.Playback, "Plex") {
		add("plex")
	}
	if has(a.Playback, "Emby") {
		add("emby")
	}
	if has(a.Playback, "DLNA") {
		add("media-dlna")
	}

	return out
}

// RepoForBin maps a binary name to its release repo when they differ.
func RepoForBin(name string) string {
	switch name {
	case "muxcored":
		return "core"
	case "mediauiprox":
		return "media-ui"
	default:
		return name
	}
}
